package scheduler

/*
#include "wrapper.h"
#include <errno.h>
#include <bpf/libbpf.h>

// Use the same loaded skeleton and maps as qumun's update/delete helpers.
// In particular, never discover maps by their (possibly truncated) names.
static int read_priority_slice(u32 key, u64 *slice) {
	if (!global_obj || !global_obj->maps.priority_tasks)
		return -EBADF;
	return bpf_map__lookup_elem(global_obj->maps.priority_tasks,
		&key, sizeof(key), slice, sizeof(*slice), 0);
}

static int read_priority_level(u32 key, u32 *level) {
	if (!global_obj || !global_obj->maps.priority_tasks_prio)
		return -EBADF;
	return bpf_map__lookup_elem(global_obj->maps.priority_tasks_prio,
		&key, sizeof(key), level, sizeof(*level), 0);
}
*/
import "C"

import (
	"errors"
	"fmt"
	"syscall"
)

func readPriorityTask(key uint32) (priorityTaskReadback, error) {
	var slice C.u64
	var level C.u32
	slicePresent, sliceErr := priorityLookupResult("priority_tasks", C.read_priority_slice(C.u32(key), &slice))
	levelPresent, levelErr := priorityLookupResult("priority_tasks_prio", C.read_priority_level(C.u32(key), &level))
	return priorityTaskReadback{
		slicePresent: slicePresent,
		levelPresent: levelPresent,
		slice:        uint64(slice),
		level:        uint32(level),
	}, errors.Join(sliceErr, levelErr)
}

func priorityLookupResult(name string, ret C.int) (bool, error) {
	switch ret {
	case 0:
		return true, nil
	case -C.ENOENT:
		return false, nil
	default:
		return false, fmt.Errorf("read %s: %w", name, syscall.Errno(-ret))
	}
}
