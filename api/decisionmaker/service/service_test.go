package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Gthulhu/api/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupFakeProcDir creates a temporary fake /proc directory structure for testing
func setupFakeProcDir(t *testing.T) string {
	root := t.TempDir()
	// fake pid 1234
	pidDir := filepath.Join(root, "1234")
	if err := os.Mkdir(pidDir, 0755); err != nil {
		t.Fatal(err)
	}

	// cgroup
	cgroupContent := "0::/kubelet.slice/kubelet-kubepods.slice/kubelet-kubepods-pod20da609e_6973_4463_a1f9_2db9bcc5becc.slice/cri-containerd-10ec3c89629f71226b227e6510b2d465168b24005bbdcc5d7940517080830635.scope\n"
	if err := os.WriteFile(filepath.Join(pidDir, "cgroup"), []byte(cgroupContent), 0644); err != nil {
		t.Fatal(err)
	}

	// comm
	if err := os.WriteFile(filepath.Join(pidDir, "comm"), []byte("nginx\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// stat
	statLine := "1234 (nginx) S 1 2 3 4 5 0 0 0 0 0 0 0 0 0 0 0 0 0 0"
	if err := os.WriteFile(filepath.Join(pidDir, "stat"), []byte(statLine), 0644); err != nil {
		t.Fatal(err)
	}

	pid2Dir := filepath.Join(root, "5678")
	if err := os.Mkdir(pid2Dir, 0755); err != nil {
		t.Fatal(err)
	}

	// cgroup for pid 5678 (not in kubepods)
	cgroupContent2 := "0::/kubelet.slice/kubelet-kubepods.slice/kubelet-kubepods-besteffort.slice/kubelet-kubepods-besteffort-pode52d4a2a_6e5f_44d9_a8b8_37ff3daa7413.slice/cri-containerd-bc96d8a88e39e8be4ff9fc02f431c7db802002c1456a56166265f19d1a3cbbc3.scope\n"
	if err := os.WriteFile(filepath.Join(pid2Dir, "cgroup"), []byte(cgroupContent2), 0644); err != nil {
		t.Fatal(err)
	}

	// comm for pid 5678
	if err := os.WriteFile(filepath.Join(pid2Dir, "comm"), []byte("busybox\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// stat for pid 5678
	statLine2 := "5678 (busybox) S 1 2 3 4 5 0 0 0 0 0 0 0 0 0 0 0 0 0 0"
	if err := os.WriteFile(filepath.Join(pid2Dir, "stat"), []byte(statLine2), 0644); err != nil {
		t.Fatal(err)
	}

	return root
}

// TestFindPodInfoFrom tests the FindPodInfoFrom function
func TestFindPodInfoFrom(t *testing.T) {
	logger.InitLogger()
	fakeProc := setupFakeProcDir(t)
	svc := &Service{}

	pods, err := svc.FindPodInfoFrom(context.Background(), fakeProc)
	require.NoError(t, err, "FindPodInfoFrom should not return error")
	require.Len(t, pods, 2, "should find one pod info")
	p := pods["20da609e-6973-4463-a1f9-2db9bcc5becc"]
	require.NotNil(t, p, "pod info should not be nil")
	assert.EqualValues(t, p.PodUID, "20da609e-6973-4463-a1f9-2db9bcc5becc", "unexpected podUID")
	assert.EqualValues(t, p.Processes[0].ContainerID, "10ec3c89629f71226b227e6510b2d465168b24005bbdcc5d7940517080830635", "unexpected containerID")
	require.Len(t, p.Processes, 1, "should have one process")
	assert.EqualValues(t, p.Processes[0].Command, "nginx", "unexpected command")

	p2 := pods["e52d4a2a-6e5f-44d9-a8b8-37ff3daa7413"]
	require.NotNil(t, p2, "pod info should not be nil")
	assert.EqualValues(t, p2.PodUID, "e52d4a2a-6e5f-44d9-a8b8-37ff3daa7413", "unexpected podUID")
	assert.EqualValues(t, p2.Processes[0].ContainerID, "bc96d8a88e39e8be4ff9fc02f431c7db802002c1456a56166265f19d1a3cbbc3", "unexpected containerID")
	require.Len(t, p2.Processes, 1, "should have one process")
	assert.EqualValues(t, p2.Processes[0].Command, "busybox", "unexpected command")
}
