package k8sadapter

import (
	"regexp"
	"testing"

	apiv1 "k8s.io/api/core/v1"
)

func TestBuildContainersMatchesExecutableCommName(t *testing.T) {
	pod := apiv1.Pod{
		Spec: apiv1.PodSpec{
			Containers: []apiv1.Container{{
				Name:    "worker",
				Command: []string{"/usr/bin/python3"},
				Args:    []string{"main.py", "--serve"},
			}},
		},
	}

	if got := buildContainers(pod, regexp.MustCompile(`^python3$`)); len(got) != 1 {
		t.Fatalf("expected executable name to match, got %d containers", len(got))
	}
	if got := buildContainers(pod, regexp.MustCompile(`main\.py`)); len(got) != 0 {
		t.Fatalf("expected arguments not to participate in commandRegex matching, got %d containers", len(got))
	}
}

func TestBuildContainersCannotValidateImageDefaultCommand(t *testing.T) {
	pod := apiv1.Pod{
		Spec: apiv1.PodSpec{
			Containers: []apiv1.Container{{
				Name: "worker",
				Args: []string{"--serve"},
			}},
		},
	}

	if got := buildContainers(pod, regexp.MustCompile(`.*`)); len(got) != 0 {
		t.Fatalf("expected container without an explicit command not to match, got %d containers", len(got))
	}
}
