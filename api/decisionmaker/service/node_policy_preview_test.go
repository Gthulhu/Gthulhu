package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Gthulhu/api/decisionmaker/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPreviewNodePolicyWorkerAndConflicts(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeTask(t, root, 1000, 1000, "python")
	writeTask(t, root, 1000, 1001, "Engine)Core")
	writeTask(t, root, 1000, 1002, "pause")
	cgroup := "/kubepods.slice/kubepods-pod20da609e_6973_4463_a1f9_2db9bcc5becc.slice/container.scope"
	require.NoError(t, os.WriteFile(filepath.Join(root, "1000", "task", "1001", "cgroup"),
		[]byte("0::/\n2:cpu:"+cgroup+"\n"), 0o644))

	svc := &Service{taskSource: NewProcTaskSource(root)}
	old := &domain.NodePolicy{PolicyID: "a", Generation: 9, CommandRegex: "Engine"}
	policy := &domain.NodePolicy{PolicyID: "a", Generation: 10, Node: "gx10", CommandRegex: `^Engine\)Core$`, Priority: 5}
	other := &domain.NodePolicy{PolicyID: "b", Generation: 1, Node: "gx10", CommandRegex: "Core$"}
	require.NoError(t, svc.ProcessNodePolicies(ctx, []*domain.NodePolicy{other, old, policy}))
	before := svc.GetNodePolicyMerkleRootHash()
	result, err := svc.PreviewNodePolicy(ctx, domain.NodePolicyPreviewRequest{PolicyID: "a"})
	require.NoError(t, err)
	wantTask := domain.TaskInfo{
		Node: "gx10", TGID: 1000, TID: 1001, Comm: "Engine)Core", StartTime: 182736123,
		CgroupPath: cgroup, PodUID: "20da609e-6973-4463-a1f9-2db9bcc5becc",
	}
	assert.Equal(t, *policy, result.Policy)
	assert.Equal(t, []domain.TaskInfo{wantTask}, result.MatchedTasks)
	assert.Empty(t, result.Warnings)
	require.Len(t, result.Conflicts, 1)
	assert.Equal(t, []domain.NodePolicy{*old, *policy, *other}, result.Conflicts[0].Policies)
	assert.Equal(t, *other, result.Conflicts[0].Winner)
	assert.Equal(t, before, svc.GetNodePolicyMerkleRootHash())

	intents, err := svc.resolveNodeSchedulingIntents(ctx)
	require.NoError(t, err)
	require.Len(t, intents, 1)
	assert.Equal(t, &domain.NodePolicyMatch{Policy: *other, Task: wantTask, IntendedKey: 1001}, intents[0].NodePolicyMatch)

	inline := &domain.NodePolicy{Node: "gx10", CommandRegex: "python"}
	result, err = svc.PreviewNodePolicy(ctx, domain.NodePolicyPreviewRequest{Policy: inline})
	require.NoError(t, err)
	require.Len(t, result.MatchedTasks, 1)
	assert.Equal(t, 1000, result.MatchedTasks[0].TID)
	assert.Equal(t, before, svc.GetNodePolicyMerkleRootHash())
}

func TestPreviewNodePolicyWarnings(t *testing.T) {
	svc := &Service{taskSource: &fakeTaskSource{tasks: []domain.TaskInfo{
		{TGID: 2, TID: 2, Comm: "kworker/0:12345", KernelThread: true},
	}}}
	for _, tc := range []struct {
		regex string
		codes []string
	}{
		{"(", []string{"invalidRegex"}},
		{"", []string{"invalidRegex"}},
		{"^kworker/0:123456$", []string{"noMatches", "commLimit"}},
		{"^kworker/", []string{"commLimit", "kernelThread"}},
	} {
		t.Run(tc.regex, func(t *testing.T) {
			result, err := svc.PreviewNodePolicy(context.Background(), domain.NodePolicyPreviewRequest{
				Policy: &domain.NodePolicy{CommandRegex: tc.regex},
			})
			require.NoError(t, err)
			var codes []string
			for _, warning := range result.Warnings {
				codes = append(codes, warning.Code)
			}
			assert.Equal(t, tc.codes, codes)
		})
	}
}
