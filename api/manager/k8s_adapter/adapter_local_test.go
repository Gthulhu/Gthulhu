//go:build k3d
// +build k3d

/*
For local testing, use `k3d`.

Install `k3d` with one of the following:
  - curl -s https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | bash
  - brew install k3d
*/
package k8sadapter

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/Gthulhu/api/manager/domain"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

/*
Using `k3d` for local testing. To install k3d, please checkout the package doc top.
*/
func tryGetKubeConfigPathFromK3d(t *testing.T) (string, func()) {
	t.Helper()

	k3dPath, err := exec.LookPath("k3d")
	if err != nil {
		t.Skip("k3d not installed; skip integration test")
	}

	t.Logf("k3d founded: %s", k3dPath)

	clusterName := fmt.Sprintf("api-adapter-%d", time.Now().UnixNano())
	_ = exec.Command(k3dPath, "cluster", "delete", clusterName).Run()

	createCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	createCmd := exec.CommandContext(createCtx, k3dPath, "cluster", "create", clusterName, "--wait")
	if out, err := createCmd.CombinedOutput(); err != nil {
		t.Skipf("failed to create k3d cluster: %v, output: %s", err, string(out))
	}

	cleanupFunc := func() { exec.Command(k3dPath, "cluster", "delete", clusterName).Run() }

	kubeconfigPath := filepath.Join(t.TempDir(), "kubeconfig")
	kubeconfigCmd := exec.Command(k3dPath, "kubeconfig", "get", clusterName)
	kubeconfigOut, err := kubeconfigCmd.Output()
	if err != nil {
		t.Skipf("failed to get kubeconfig: %v", err)
	}

	if err := os.WriteFile(kubeconfigPath, kubeconfigOut, 0o600); err != nil {
		t.Skipf("failed to write kubeconfig: %v", err)
	}

	return kubeconfigPath, cleanupFunc
}

func TestQueryPodsWithLocalKubeconfig(t *testing.T) {
	t.Parallel()

	kubeconfigPath, cleanupK3d := tryGetKubeConfigPathFromK3d(t)
	defer cleanupK3d()

	adapter, err := NewAdapter(Options{
		KubeConfigPath: kubeconfigPath,
	})
	if err != nil {
		t.Skipf("cannot initialize adapter with kubeconfig %s: %v", kubeconfigPath, err)
	}
	defer adapter.StopPodWatcher()

	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		t.Skipf("failed to build config: %v", err)
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		t.Skipf("failed to create clientset: %v", err)
	}

	ns := "default"
	pod := &apiv1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "adapter-local-test",
			Namespace: ns,
			Labels: map[string]string{
				"app": "demo",
			},
		},
		Spec: apiv1.PodSpec{
			Containers: []apiv1.Container{
				{
					Name:    "pause",
					Image:   "registry.k8s.io/pause:3.9",
					Command: []string{"/pause"},
				},
			},
		},
	}

	{ /*** Test Creating Pod ***/
		created, err := client.CoreV1().Pods(ns).Create(context.Background(), pod, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("failed to create pod: %v", err)
		}
		t.Cleanup(func() {
			_ = client.CoreV1().Pods(ns).Delete(context.Background(), created.Name, metav1.DeleteOptions{})
		})

		opt := &domain.QueryPodsOptions{
			K8SNamespace: []string{metav1.NamespaceAll},
			LabelSelectors: []domain.LabelSelector{
				{Key: "app", Value: "demo"},
			},
		}

		waitForLocal(t, func(ctx context.Context) (bool, error) {

			results, err := adapter.QueryPods(ctx, opt)
			if err != nil {
				return false, err
			}
			return len(results) == 1, nil
		})

		{ /*** Test Modifying Pod ***/
			if err := retryUpdatePodLabel(context.Background(), client, ns, created.Name, "app", "demo2"); err != nil {
				t.Fatalf("failed to update pod: %v", err)
			}

			opt.LabelSelectors = []domain.LabelSelector{{Key: "app", Value: "demo2"}}
			waitForLocal(t, func(ctx context.Context) (bool, error) {

				results, err := adapter.QueryPods(ctx, opt)
				if err != nil {
					return false, err
				}
				return len(results) == 1 && results[0].Labels["app"] == "demo2", nil
			})
		}

		{ /*** Test Deleting Pod ***/
			if err := client.CoreV1().Pods(ns).Delete(context.Background(), created.Name, metav1.DeleteOptions{}); err != nil {
				t.Fatalf("failed to delete pod: %v", err)
			}

			waitForLocal(t, func(ctx context.Context) (bool, error) {

				results, err := adapter.QueryPods(ctx, opt)
				if err != nil {
					return false, err
				}
				return len(results) == 0, nil
			})
		}
	}
}

func waitForLocal(t *testing.T, cond func(ctx context.Context) (bool, error)) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := wait.PollUntilContextTimeout(ctx, 200*time.Millisecond, 30*time.Second, false, func(ctx context.Context) (bool, error) {
		return cond(ctx)
	}); err != nil {
		t.Fatalf("condition not met: %v", err)
	}
}

func retryUpdatePodLabel(ctx context.Context, client *kubernetes.Clientset, ns, name, key, val string) error {
	return wait.PollUntilContextTimeout(ctx, 200*time.Millisecond, 10*time.Second, false, func(ctx context.Context) (bool, error) {
		p, err := client.CoreV1().Pods(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if p.Labels == nil {
			p.Labels = make(map[string]string)
		}
		p.Labels[key] = val
		_, err = client.CoreV1().Pods(ns).Update(ctx, p, metav1.UpdateOptions{})
		if err != nil {
			return false, nil
		}
		return true, nil
	})
}

func TestQueryDecisionMakerPodsWithLocalKubeconfig(t *testing.T) {
	t.Parallel()

	kubeconfigPath, cleanupK3d := tryGetKubeConfigPathFromK3d(t)
	defer cleanupK3d()

	adapter, err := NewAdapter(Options{
		KubeConfigPath: kubeconfigPath,
	})
	if err != nil {
		t.Skipf("cannot initialize adapter with kubeconfig %s: %v", kubeconfigPath, err)
	}
	defer adapter.StopPodWatcher()

	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		t.Skipf("failed to build config: %v", err)
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		t.Skipf("failed to create clientset: %v", err)
	}

	ns := "default"
	pod := &apiv1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "adapter-local-dm-test",
			Namespace: ns,
			Labels: map[string]string{
				"dm": "true",
			},
		},
		Spec: apiv1.PodSpec{
			Containers: []apiv1.Container{
				{
					Name:  "pause",
					Image: "registry.k8s.io/pause:3.9",
					Ports: []apiv1.ContainerPort{{ContainerPort: 8080}},
				},
			},
		},
	}

	{ /*** Test Creating DecisionMaker Pod ***/
		created, err := client.CoreV1().Pods(ns).Create(context.Background(), pod, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("failed to create pod: %v", err)
		}
		t.Cleanup(func() {
			_ = client.CoreV1().Pods(ns).Delete(context.Background(), created.Name, metav1.DeleteOptions{})
		})

		opt := &domain.QueryDecisionMakerPodsOptions{
			K8SNamespace: []string{metav1.NamespaceAll},
			DecisionMakerLabel: domain.LabelSelector{
				Key:   "dm",
				Value: "true",
			},
		}

		waitForLocal(t, func(ctx context.Context) (bool, error) {
			results, err := adapter.QueryDecisionMakerPods(ctx, opt)
			if err != nil {
				return false, err
			}
			if len(results) != 1 {
				return false, nil
			}
			return results[0].NodeID != "" && results[0].Host != "", nil
		})

		{ /*** Test Modifying DecisionMaker Pod ***/
			if err := retryUpdatePodLabel(context.Background(), client, ns, created.Name, "dm", "dm2"); err != nil {
				t.Fatalf("failed to update pod: %v", err)
			}

			opt.DecisionMakerLabel = domain.LabelSelector{Key: "dm", Value: "dm2"}
			waitForLocal(t, func(ctx context.Context) (bool, error) {

				results, err := adapter.QueryDecisionMakerPods(ctx, opt)
				if err != nil {
					return false, err
				}
				if len(results) != 1 {
					return false, nil
				}
				return results[0].Host != "" && results[0].NodeID != "" && results[0].Port == 8080, nil
			})
		}

		{ /*** Test Deleting DecisionMaker Pod ***/
			if err := client.CoreV1().Pods(ns).Delete(context.Background(), created.Name, metav1.DeleteOptions{}); err != nil {
				t.Fatalf("failed to delete pod: %v", err)
			}

			waitForLocal(t, func(ctx context.Context) (bool, error) {
				results, err := adapter.QueryDecisionMakerPods(ctx, opt)
				if err != nil {
					return false, err
				}
				return len(results) == 0, nil
			})
		}
	}
}
