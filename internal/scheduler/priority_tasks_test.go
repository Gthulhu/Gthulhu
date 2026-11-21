package scheduler

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/Gthulhu/api/decisionmaker/domain"
)

type testPriorityMap struct {
	entries      map[uint32]priorityTaskReadback
	updated      []uint32
	removed      []uint32
	lookups      []uint32
	updateErr    error
	lookupErr    error
	ignoreUpdate bool
	ignoreRemove bool
}

func (m *testPriorityMap) UpdatePriorityTaskWithPrio(key uint32, slice uint64, level uint32) error {
	m.updated = append(m.updated, key)
	if m.updateErr != nil {
		return m.updateErr
	}
	if !m.ignoreUpdate {
		m.entries[key] = priorityTaskReadback{true, true, slice, level}
	}
	return nil
}

func (m *testPriorityMap) RemovePriorityTask(key uint32) error {
	m.removed = append(m.removed, key)
	if !m.ignoreRemove {
		delete(m.entries, key)
	}
	return nil
}

func (m *testPriorityMap) lookup(key uint32) (priorityTaskReadback, error) {
	m.lookups = append(m.lookups, key)
	return m.entries[key], m.lookupErr
}

func threadStrategy() kernelStrategy {
	return kernelStrategy{
		PID: 77, Priority: 9, ExecutionTime: 2000000,
		NodePolicy: &domain.NodePolicyMatch{
			Policy: domain.NodePolicy{PolicyID: "policy-a", Generation: 7, CommandRegex: "^worker$"},
			Task: domain.TaskInfo{
				Node: "node-a", TID: 77, TGID: 55, Comm: "worker", StartTime: 240000,
				CgroupPath: "/runtime/workload", PodUID: "pod-a", KernelThread: false,
			},
			IntendedKey: 77,
		},
	}
}

func readPriorityLogs(t *testing.T, logs *bytes.Buffer) []map[string]any {
	t.Helper()
	var records []map[string]any
	decoder := json.NewDecoder(logs)
	for decoder.More() {
		var record map[string]any
		if err := decoder.Decode(&record); err != nil {
			t.Fatal(err)
		}
		records = append(records, record)
	}
	return records
}

func TestKernelPriorityProvenanceLifecycle(t *testing.T) {
	m := &testPriorityMap{entries: map[uint32]priorityTaskReadback{}}
	var logs bytes.Buffer
	p := kernelPriorityTasks{writer: m, lookup: m.lookup, logger: slog.New(slog.NewJSONHandler(&logs, nil))}
	first := threadStrategy()
	p.reconcile([]kernelStrategy{first})
	p.reconcile([]kernelStrategy{threadStrategy()}) // freshly decoded but unchanged
	replacement := threadStrategy()
	replacement.NodePolicy.Task.StartTime++
	replacement.NodePolicy.Policy.Generation++
	p.reconcile([]kernelStrategy{replacement}) // reused TID, identical priority and slice
	p.reconcile(nil)
	p.reconcile(nil)

	if !reflect.DeepEqual(m.updated, []uint32{77, 77}) || !reflect.DeepEqual(m.removed, []uint32{77}) ||
		!reflect.DeepEqual(m.lookups, []uint32{77, 77, 77}) {
		t.Fatalf("wrong map operations: update=%v remove=%v lookup=%v", m.updated, m.removed, m.lookups)
	}
	records := readPriorityLogs(t, &logs)
	if len(records) != 3 {
		t.Fatalf("want exactly three transition records, got %v", records)
	}
	for i, record := range records {
		want := map[string]any{
			"policy_id": "policy-a", "generation": float64(7), "node": "node-a", "tgid": float64(55),
			"tid": float64(77), "start_time": float64(240000), "comm": "worker", "intended_key": float64(77),
			"command_regex": "^worker$",
			"cgroup_path":   "/runtime/workload", "pod_uid": "pod-a", "kernel_thread": false,
			"lookup_key": float64(77), "msg": "node_policy_apply", "result": "verified",
			"slice_present": true, "priority_present": true, "actual_map_key": float64(77),
			"actual_execution_time": float64(2000000), "actual_priority": float64(9),
		}
		if i > 0 {
			want["generation"] = float64(8)
			want["start_time"] = float64(240001)
		}
		if i == 2 {
			want["msg"] = "node_policy_remove"
			want["reason"] = "strategy_removed"
			want["slice_present"] = false
			want["priority_present"] = false
			want["map_removed"] = true
			want["actual_map_key"] = nil
			want["actual_execution_time"] = nil
			want["actual_priority"] = nil
		}
		for key, value := range want {
			if record[key] != value {
				t.Errorf("record %d field %s = %v, want %v", i, key, record[key], value)
			}
		}
	}
}

func TestKernelPriorityDoesNotClaimUnverifiedMapChanges(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(*testPriorityMap)
		want  string
	}{
		{"missing key", func(m *testPriorityMap) { m.ignoreUpdate = true }, "readback_mismatch"},
		{"wrong value", func(m *testPriorityMap) {
			m.ignoreUpdate = true
			m.entries[77] = priorityTaskReadback{true, true, 2000000, 10}
		}, "readback_mismatch"},
		{"lookup failure", func(m *testPriorityMap) { m.lookupErr = errors.New("lookup failed") }, "readback_failed"},
		{"operation failure", func(m *testPriorityMap) { m.updateErr = errors.New("update failed") }, "operation_failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := &testPriorityMap{entries: map[uint32]priorityTaskReadback{}}
			tc.setup(m)
			var logs bytes.Buffer
			p := kernelPriorityTasks{writer: m, lookup: m.lookup, logger: slog.New(slog.NewJSONHandler(&logs, nil))}
			p.reconcile([]kernelStrategy{threadStrategy()})
			records := readPriorityLogs(t, &logs)
			if len(records) != 1 || records[0]["result"] != tc.want || records[0]["level"] != "WARN" {
				t.Fatalf("unexpected records: %v", records)
			}
		})
	}
}

func TestKernelPriorityRemovalVerifiesBothMaps(t *testing.T) {
	m := &testPriorityMap{entries: map[uint32]priorityTaskReadback{}}
	var logs bytes.Buffer
	p := kernelPriorityTasks{writer: m, lookup: m.lookup, logger: slog.New(slog.NewJSONHandler(&logs, nil))}
	p.reconcile([]kernelStrategy{threadStrategy()})
	logs.Reset()
	m.ignoreRemove = true
	m.entries[77] = priorityTaskReadback{levelPresent: true, level: 9}
	replacement := threadStrategy()
	replacement.Priority = 0
	replacement.NodePolicy.Policy.Generation++
	p.reconcile([]kernelStrategy{replacement})
	records := readPriorityLogs(t, &logs)
	if len(records) != 1 {
		t.Fatalf("want one transition record, got %v", records)
	}
	got := records[0]
	if got["msg"] != "node_policy_remove" || got["reason"] != "non_positive_priority" || got["result"] != "readback_mismatch" ||
		got["generation"] != float64(7) || got["actual_map_key"] != float64(77) || got["slice_present"] != false ||
		got["priority_present"] != true || got["map_removed"] != false {
		t.Errorf("unexpected removal record: %v", got)
	}
}

func TestKernelPriorityReplacementKeepsKey(t *testing.T) {
	m := &testPriorityMap{entries: map[uint32]priorityTaskReadback{}}
	var logs bytes.Buffer
	p := kernelPriorityTasks{writer: m, lookup: m.lookup, logger: slog.New(slog.NewJSONHandler(&logs, nil))}
	p.reconcile([]kernelStrategy{threadStrategy()})
	logs.Reset()
	replacement := threadStrategy()
	replacement.NodePolicy.Policy.PolicyID = "policy-b"
	replacement.Priority++
	p.reconcile([]kernelStrategy{replacement})

	if len(m.removed) != 0 || !reflect.DeepEqual(m.lookups, []uint32{77, 77}) {
		t.Fatalf("expected one readback for both replacement events and no map delete: remove=%v lookup=%v", m.removed, m.lookups)
	}
	records := readPriorityLogs(t, &logs)
	if len(records) != 2 {
		t.Fatalf("expected old and new provenance, got %v", records)
	}
	if got := records[0]; got["msg"] != "node_policy_remove" || got["reason"] != "strategy_replaced" ||
		got["policy_id"] != "policy-a" || got["replacement_policy_id"] != "policy-b" ||
		got["result"] != "verified" || got["map_removed"] != false || got["actual_map_key"] != float64(77) ||
		got["actual_priority"] != float64(10) {
		t.Errorf("unexpected old policy record: %v", got)
	}
	if got := records[1]; got["msg"] != "node_policy_apply" || got["policy_id"] != "policy-b" ||
		got["result"] != "verified" || got["actual_map_key"] != float64(77) {
		t.Errorf("unexpected new policy record: %v", got)
	}
}

type strategyClientFunc func(string, string, io.Reader) (*http.Response, error)

func (f strategyClientFunc) MakeAuthenticatedRequest(method, url string, body io.Reader) (*http.Response, error) {
	return f(method, url, body)
}

func TestKernelStrategyFetchPreservesNodePolicyMatch(t *testing.T) {
	const fixture = `{"success":true,"scheduling":[
		{"pid":55,"priority":10,"execution_time":1000000},
		{"pid":77,"priority":9,"execution_time":2000000,"nodePolicyMatch":{
			"policy":{"policyID":"policy-a","generation":7,"commandRegex":"^worker$"},
			"task":{"node":"node-a","tgid":55,"tid":77,"comm":"worker","startTime":240000,"cgroupPath":"/runtime/workload","podUID":"pod-a"},
			"intendedKey":77}}]}`
	client := strategyClientFunc(func(method, url string, body io.Reader) (*http.Response, error) {
		if method != "GET" || url != "http://decisionmaker/api/v1/scheduling/strategies" || body != nil {
			t.Errorf("unexpected request: %s %s %v", method, url, body)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(fixture))}, nil
	})
	got, err := fetchKernelStrategies(client, "http://decisionmaker/")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || !got[0].equal(kernelStrategy{PID: 55, Priority: 10, ExecutionTime: 1000000}) ||
		!got[1].equal(threadStrategy()) {
		t.Fatalf("strategies = %+v", got)
	}
}

func TestKernelStrategyFetchRequiresAuthoritativeSnapshot(t *testing.T) {
	for _, tc := range []struct {
		code int
		body string
		ok   bool
	}{
		{http.StatusOK, `{"success":true,"scheduling":[]}`, true},
		{http.StatusOK, `{"success":true}`, false},
		{http.StatusOK, `{"success":true,"scheduling":null}`, false},
		{http.StatusOK, `{"success":false,"scheduling":[]}`, false},
		{http.StatusOK, `{`, false},
		{http.StatusServiceUnavailable, `{"success":true,"scheduling":[]}`, false},
	} {
		client := strategyClientFunc(func(string, string, io.Reader) (*http.Response, error) {
			return &http.Response{StatusCode: tc.code, Body: io.NopCloser(strings.NewReader(tc.body))}, nil
		})
		got, err := fetchKernelStrategies(client, "http://decisionmaker")
		if (err == nil) != tc.ok || (got != nil) != tc.ok {
			t.Errorf("HTTP %d %s: got (%v, %v), successful empty snapshot = %t", tc.code, tc.body, got, err, tc.ok)
		}
	}
}
