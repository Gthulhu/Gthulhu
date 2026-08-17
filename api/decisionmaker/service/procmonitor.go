package service

import (
	"context"
	"os"
	"sort"
	"strconv"
	"strings"

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
// thread. A task can vanish or be unreadable mid-scan, so per-entry read
// errors just skip that entry - only an unreadable /proc root is fatal. Paths
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
			continue // the process exited, or its task dir is unreadable
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
				continue // the thread exited, or its comm is unreadable
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
