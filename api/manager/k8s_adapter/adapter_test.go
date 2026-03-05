//go:build k3d
// +build k3d

package k8sadapter

import (
	"context"
	"testing"
	"time"

	"github.com/Gthulhu/api/manager/domain"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/fake"
)

func TestPodWatcherCacheLifecycle(t *testing.T) {
	t.Parallel()

	client := fake.NewSimpleClientset()
	adapter := &Adapter{
		client:   client,
		podCache: make(map[string]apiv1.Pod),
		stopCh:   make(chan struct{}),
	}
	adapter.startPodWatcher()
	t.Cleanup(adapter.StopPodWatcher)

	pod := &apiv1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-1",
			Namespace: "ns",
			UID:       "uid-1",
			Labels: map[string]string{
				"app": "demo",
			},
		},
		Spec: apiv1.PodSpec{
			NodeName: "node-1",
			Containers: []apiv1.Container{
				{
					Name:    "c1",
					Command: []string{"run"},
				},
			},
		},
		Status: apiv1.PodStatus{
			Phase: apiv1.PodRunning,
			ContainerStatuses: []apiv1.ContainerStatus{
				{
					Name:        "c1",
					ContainerID: "docker://abc",
				},
			},
		},
	}

	{ /*** Test Creating Pod ***/
		if _, err := client.CoreV1().Pods("ns").Create(context.Background(), pod, metav1.CreateOptions{}); err != nil {
			t.Fatalf("failed to create pod: %v", err)
		}

		waitFor(t, func() bool {
			adapter.podCacheMu.RLock()
			defer adapter.podCacheMu.RUnlock()
			return len(adapter.podCache) == 1
		})

		opt := &domain.QueryPodsOptions{
			K8SNamespace: []string{"ns"},
			LabelSelectors: []domain.LabelSelector{
				{Key: "app", Value: "demo"},
			},
		}

		results, err := adapter.QueryPods(context.Background(), opt)
		if err != nil {
			t.Fatalf("query pods after create: %v", err)
		} else if len(results) != 1 {
			t.Fatalf("expected 1 pod after create, got %d", len(results))
		}
		if results[0].Containers[0].ContainerID != "docker://abc" {
			t.Fatalf("unexpected container ID: %s", results[0].Containers[0].ContainerID)
		}
	}

	{ /*** Test Modifying Pod ***/
		pod.Labels["app"] = "demo2"
		if _, err := client.CoreV1().Pods("ns").Update(context.Background(), pod, metav1.UpdateOptions{}); err != nil {
			t.Fatalf("failed to update pod: %v", err)
		}

		waitFor(t, func() bool {
			adapter.podCacheMu.RLock()
			defer adapter.podCacheMu.RUnlock()
			p, ok := adapter.podCache["uid-1"]
			return ok && p.Labels["app"] == "demo2"
		})

		opt := &domain.QueryPodsOptions{
			K8SNamespace: []string{"ns"},
			LabelSelectors: []domain.LabelSelector{
				{Key: "app", Value: "demo2"},
			},
		}
		results, err := adapter.QueryPods(context.Background(), opt)
		if err != nil {
			t.Fatalf("query pods after update: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 pod after update, got %d", len(results))
		}
	}

	{ /*** Test Deleting Pod ***/
		if err := client.CoreV1().Pods("ns").Delete(context.Background(), pod.Name, metav1.DeleteOptions{}); err != nil {
			t.Fatalf("failed to delete pod: %v", err)
		}

		waitFor(t, func() bool {
			adapter.podCacheMu.RLock()
			defer adapter.podCacheMu.RUnlock()
			_, ok := adapter.podCache["uid-1"]
			return !ok
		})

		opt := &domain.QueryPodsOptions{
			K8SNamespace: []string{"ns"},
			LabelSelectors: []domain.LabelSelector{
				{Key: "app", Value: "demo2"},
			},
		}
		results, err := adapter.QueryPods(context.Background(), opt)
		if err != nil {
			t.Fatalf("query pods after delete: %v", err)
		}
		if len(results) != 0 {
			t.Fatalf("expected no pod after delete, got %d", len(results))
		}
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := wait.PollUntilContextTimeout(ctx, 10*time.Millisecond, 2*time.Second, false, func(context.Context) (done bool, err error) {
		return cond(), nil
	}); err != nil {
		t.Fatalf("condition not met: %v", err)
	}
}

func TestQueryPodsUsesCache(t *testing.T) {
	t.Parallel()

	adapter := &Adapter{
		client:   fake.NewSimpleClientset(),
		podCache: make(map[string]apiv1.Pod),
	}
	adapter.cacheHasSynced.Store(true)

	pod := apiv1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:       "uid-1",
			Namespace: "ns1",
			Labels: map[string]string{
				"app": "test",
			},
		},
		Spec: apiv1.PodSpec{
			NodeName: "node-1",
			Containers: []apiv1.Container{
				{
					Name:    "c1",
					Command: []string{"cmd"},
					Args:    []string{"--flag"},
				},
			},
		},
		Status: apiv1.PodStatus{
			ContainerStatuses: []apiv1.ContainerStatus{
				{
					Name:        "c1",
					ContainerID: "docker://123",
				},
			},
		},
	}
	adapter.setPodCache(pod)

	opt := &domain.QueryPodsOptions{
		K8SNamespace: []string{"ns1"},
		LabelSelectors: []domain.LabelSelector{
			{Key: "app", Value: "test"},
		},
	}

	results, err := adapter.QueryPods(context.Background(), opt)
	if err != nil {
		t.Fatalf("QueryPods returned error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 pod, got %d", len(results))
	}

	got := results[0]
	if got.PodID != "uid-1" {
		t.Fatalf("unexpected PodID %q", got.PodID)
	}
	if got.NodeID != "node-1" {
		t.Fatalf("unexpected NodeID %q", got.NodeID)
	}
	if len(got.Containers) != 1 || got.Containers[0].ContainerID != "docker://123" {
		t.Fatalf("unexpected container data %+v", got.Containers)
	}
}

func TestQueryDecisionMakerPodsUsesCache(t *testing.T) {
	t.Parallel()

	adapter := &Adapter{
		client:   fake.NewSimpleClientset(),
		podCache: make(map[string]apiv1.Pod),
	}
	adapter.cacheHasSynced.Store(true)

	pod := apiv1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:       "uid-dm-1",
			Namespace: "ns1",
			Labels: map[string]string{
				"dm": "true",
			},
		},
		Spec: apiv1.PodSpec{
			NodeName: "node-1",
			Containers: []apiv1.Container{
				{
					Name:  "c1",
					Ports: []apiv1.ContainerPort{{ContainerPort: 8080}},
				},
			},
		},
		Status: apiv1.PodStatus{
			Phase:  apiv1.PodRunning,
			PodIP:  "10.0.0.1",
			HostIP: "10.0.0.2",
		},
	}
	adapter.setPodCache(pod)

	opt := &domain.QueryDecisionMakerPodsOptions{
		K8SNamespace: []string{"ns1"},
		NodeIDs:      []string{"node-1"},
		DecisionMakerLabel: domain.LabelSelector{
			Key:   "dm",
			Value: "true",
		},
	}

	results, err := adapter.QueryDecisionMakerPods(context.Background(), opt)
	if err != nil {
		t.Fatalf("QueryDecisionMakerPods returned error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 decision maker pod, got %d", len(results))
	}

	got := results[0]
	if got.NodeID != "node-1" {
		t.Fatalf("unexpected NodeID %q", got.NodeID)
	}
	if got.Port != 8080 {
		t.Fatalf("unexpected port %d", got.Port)
	}
	if got.Host != "10.0.0.1" {
		t.Fatalf("unexpected host %q", got.Host)
	}
	if got.State != domain.NodeStateOnline {
		t.Fatalf("unexpected state %v", got.State)
	}
}
