package service

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"github.com/Gthulhu/api/decisionmaker/domain"
)

// TaskSource enumerates the node's kernel scheduling entities. The Linux
// scheduler runs threads, not processes, so a node policy must see every
// thread (TID), not just the thread-group leader that /proc lists at top level.
type TaskSource interface {
	Snapshot(ctx context.Context) ([]domain.TaskInfo, error)
}

// procTaskSource implements TaskSource by walking /proc/<tgid>/task/<tid>.
type procTaskSource struct {
	rootDir string
}

// NewProcTaskSource creates a TaskSource backed by /proc scanning. An empty
// rootDir defaults to "/proc".
func NewProcTaskSource(rootDir string) TaskSource {
	if rootDir == "" {
		rootDir = "/proc"
	}
	return &procTaskSource{rootDir: rootDir}
}

// Snapshot walks every thread of every process. The top level of /proc lists
// only thread-group leaders (TGIDs); the threads live under
// /proc/<tgid>/task/<tid>, so a single-level scan misses every non-leader
// thread. A task that vanishes mid-scan (ENOENT/ESRCH) is skipped, but a real
// error (permission, I/O) is returned rather than silently dropping tasks - an
// incomplete snapshot must not be mistaken for the full desired state. Paths
// are built from parsed integers, not raw directory names. Tasks are sorted by
// TID for deterministic output.
func (p *procTaskSource) Snapshot(ctx context.Context) ([]domain.TaskInfo, error) {
	tgidEntries, err := os.ReadDir(p.rootDir)
	if err != nil {
		return nil, err
	}

	var tasks []domain.TaskInfo
	for _, tgidEntry := range tgidEntries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !tgidEntry.IsDir() {
			continue
		}
		tgid, err := strconv.Atoi(tgidEntry.Name())
		if err != nil {
			continue // non-numeric entries such as "acpi" or "bus"
		}

		taskDir := p.rootDir + "/" + strconv.Itoa(tgid) + "/task"
		tidEntries, err := os.ReadDir(taskDir)
		if err != nil {
			if isTransientProcError(err) {
				continue // the process exited between the two reads
			}
			return nil, fmt.Errorf("read task dir for tgid %d: %w", tgid, err)
		}

		for _, tidEntry := range tidEntries {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if !tidEntry.IsDir() {
				continue
			}
			tid, err := strconv.Atoi(tidEntry.Name())
			if err != nil {
				continue
			}
			commPath := p.rootDir + "/" + strconv.Itoa(tgid) + "/task/" + strconv.Itoa(tid) + "/comm"
			data, err := os.ReadFile(commPath)
			if err != nil {
				if isTransientProcError(err) {
					continue // the thread exited between readdir and read
				}
				return nil, fmt.Errorf("read comm for tid %d: %w", tid, err)
			}
			tasks = append(tasks, domain.TaskInfo{
				TGID: tgid,
				TID:  tid,
				Comm: strings.TrimSpace(string(data)),
			})
		}
	}

	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].TID < tasks[j].TID
	})
	return tasks, nil
}

// isTransientProcError reports whether err is just a task vanishing mid-scan
// (ENOENT/ESRCH). Permission and I/O errors are not transient: they mean the
// snapshot is incomplete, and surfacing them stops an incomplete scan being
// mistaken for the full desired state (which would delete strategies for the
// tasks that could not be read).
func isTransientProcError(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ESRCH)
}
