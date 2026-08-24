package service

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeTask creates <root>/<tgid>/task/<tid>/comm to mimic a running thread.
func writeTask(t *testing.T, root string, tgid, tid int, comm string) {
	t.Helper()
	dir := filepath.Join(root, strconv.Itoa(tgid), "task", strconv.Itoa(tid))
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "comm"), []byte(comm+"\n"), 0o644))
}

func TestProcTaskSourceEnumeratesNonLeaderThreads(t *testing.T) {
	root := t.TempDir()
	// One process whose worker threads have a comm different from the leader -
	// exactly what a top-level-only /proc scan cannot see.
	writeTask(t, root, 1000, 1000, "python")
	writeTask(t, root, 1000, 1001, "cuda-EvtHandlr")
	writeTask(t, root, 1000, 1002, "tokenizers")
	writeTask(t, root, 2000, 2000, "sshd")
	// Non-numeric top-level entries must be ignored.
	require.NoError(t, os.MkdirAll(filepath.Join(root, "acpi"), 0o755))

	tasks, err := NewProcTaskSource(root).Snapshot(context.Background())
	require.NoError(t, err)
	require.Len(t, tasks, 4)

	comm := map[int]string{}
	tgid := map[int]int{}
	for _, task := range tasks {
		comm[task.TID] = task.Comm
		tgid[task.TID] = task.TGID
	}
	assert.Equal(t, "cuda-EvtHandlr", comm[1001])
	assert.Equal(t, "tokenizers", comm[1002])
	assert.Equal(t, "sshd", comm[2000])
	// Every worker thread maps back to its group leader.
	assert.Equal(t, 1000, tgid[1001])
	assert.Equal(t, 1000, tgid[1002])
}

func TestProcTaskSourceDeterministicOrder(t *testing.T) {
	root := t.TempDir()
	writeTask(t, root, 5, 30, "c")
	writeTask(t, root, 5, 10, "a")
	writeTask(t, root, 5, 20, "b")

	tasks, err := NewProcTaskSource(root).Snapshot(context.Background())
	require.NoError(t, err)
	require.Len(t, tasks, 3)
	assert.Equal(t, 10, tasks[0].TID)
	assert.Equal(t, 20, tasks[1].TID)
	assert.Equal(t, 30, tasks[2].TID)
}

func TestProcTaskSourceSkipsProcessWithoutTaskDir(t *testing.T) {
	root := t.TempDir()
	writeTask(t, root, 1000, 1000, "ok")
	// A numeric dir without task/ mimics a process that exited between the
	// top-level readdir and the task readdir - skipped, not an error.
	require.NoError(t, os.MkdirAll(filepath.Join(root, "1234"), 0o755))

	tasks, err := NewProcTaskSource(root).Snapshot(context.Background())
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	assert.Equal(t, 1000, tasks[0].TID)
}

func TestProcTaskSourceSkipsTaskWithoutComm(t *testing.T) {
	root := t.TempDir()
	writeTask(t, root, 1000, 1000, "ok")
	// A task dir with no comm file mimics a thread that vanished mid-scan; that
	// thread is skipped while its readable sibling survives.
	require.NoError(t, os.MkdirAll(filepath.Join(root, "1000", "task", "1001"), 0o755))

	tasks, err := NewProcTaskSource(root).Snapshot(context.Background())
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	assert.Equal(t, 1000, tasks[0].TID)
}

func TestProcTaskSourceSkipsNonDirEntry(t *testing.T) {
	root := t.TempDir()
	writeTask(t, root, 1000, 1000, "ok")
	// A numeric regular file at the top level must be skipped, not fail the scan.
	require.NoError(t, os.WriteFile(filepath.Join(root, "42"), []byte("x"), 0o644))

	tasks, err := NewProcTaskSource(root).Snapshot(context.Background())
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	assert.Equal(t, 1000, tasks[0].TID)
}

func TestProcTaskSourceReadDirErrorSurfaces(t *testing.T) {
	// A missing root is the one fatal case: /proc itself is unusable.
	_, err := NewProcTaskSource(filepath.Join(t.TempDir(), "nope")).Snapshot(context.Background())
	require.Error(t, err)
}

func TestProcTaskSourceContextCancelled(t *testing.T) {
	root := t.TempDir()
	writeTask(t, root, 1000, 1000, "ok")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := NewProcTaskSource(root).Snapshot(ctx)
	require.ErrorIs(t, err, context.Canceled)
}

func TestIsTransientProcError(t *testing.T) {
	// A task vanishing mid-scan is transient and skipped.
	assert.True(t, isTransientProcError(fs.ErrNotExist))
	assert.True(t, isTransientProcError(&os.PathError{Err: syscall.ENOENT}))
	assert.True(t, isTransientProcError(&os.PathError{Err: syscall.ESRCH}))
	// Permission and I/O errors are NOT transient: an incomplete scan must
	// surface so it is not mistaken for the full desired state.
	assert.False(t, isTransientProcError(&os.PathError{Err: syscall.EACCES}))
	assert.False(t, isTransientProcError(&os.PathError{Err: syscall.EIO}))
	assert.False(t, isTransientProcError(errors.New("boom")))
}
