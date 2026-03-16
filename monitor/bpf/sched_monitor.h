// SPDX-FileCopyrightText: 2025 Gthulhu Team
//
// SPDX-License-Identifier: Apache-2.0
// Gthulhu Scheduling Event Monitor - eBPF interface header
//
// Shared data structures between BPF kernel-space and Go user-space.
// This file MUST stay in sync with the Go structs in
//   api/decisionmaker/domain/pod_scheduling_metrics.go
//   monitor/collector/collector.go

#ifndef __SCHED_MONITOR_H
#define __SCHED_MONITOR_H

#ifndef __VMLINUX_H__
typedef unsigned char      __u8;
typedef unsigned short     __u16;
typedef unsigned int       __u32;
typedef unsigned long long __u64;
typedef signed int         __s32;
typedef signed long long   __s64;
typedef int pid_t;
#endif

// ---- Per-PID scheduling metrics stored in BPF hash map ----
struct task_sched_metrics {
    __u32 pid;
    __u32 tgid;
    __u64 voluntary_ctx_switches;      // task yielded CPU voluntarily (sleep/IO/etc.)
    __u64 involuntary_ctx_switches;    // task was preempted
    __u64 cpu_time_ns;                 // cumulative CPU time
    __u64 wait_time_ns;                // cumulative run-queue wait time
    __u64 last_run_ts;                 // ktime when task last started running
    __u64 last_enqueue_ts;             // ktime when task was last enqueued
    __u64 run_count;                   // how many times the task was dispatched
    __u32 last_cpu;                    // last CPU id
    __u32 cpu_migrations;              // number of cross-CPU migrations
};

// ---- Optional ring-buffer event for real-time streaming ----
enum sched_event_type {
    SCHED_EVENT_SWITCH_OUT_VOLUNTARY   = 0,
    SCHED_EVENT_SWITCH_OUT_INVOLUNTARY = 1,
    SCHED_EVENT_SWITCH_IN              = 2,
    SCHED_EVENT_EXIT                   = 3,
};

struct sched_event {
    __u32 pid;
    __u32 tgid;
    __u32 cpu;
    __u8  event_type;
    __u8  _pad[3];
    __u64 timestamp;
    __u64 duration_ns;   // filled for switch-out events
};

#endif /* __SCHED_MONITOR_H */
