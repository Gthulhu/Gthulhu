package k8sadapter

import (
	"testing"

	apiv1 "k8s.io/api/core/v1"
)

func TestBuildContainersPreservesCommandAndArgs(t *testing.T) {
	pod := apiv1.Pod{
		Spec: apiv1.PodSpec{
			Containers: []apiv1.Container{{
				Name:    "worker",
				Command: []string{"/usr/bin/python3"},
				Args:    []string{"main.py", "--serve"},
			}},
		},
	}

	got := buildContainers(pod)
	if len(got) != 1 {
		t.Fatalf("expected one container, got %d", len(got))
	}
	if len(got[0].Command) != 3 {
		t.Fatalf("expected command and arguments to be preserved, got %v", got[0].Command)
	}
}

func TestBuildContainersPreservesImageEntrypointOnlyContainer(t *testing.T) {
	pod := apiv1.Pod{
		Spec: apiv1.PodSpec{
			Containers: []apiv1.Container{{
				Name: "worker",
				Args: []string{"--serve"},
			}},
		},
	}

	if got := buildContainers(pod); len(got) != 1 {
		t.Fatalf("expected ENTRYPOINT-only container to be preserved, got %d containers", len(got))
	}
}
