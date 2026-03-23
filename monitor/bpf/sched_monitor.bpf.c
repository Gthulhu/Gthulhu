// SPDX-FileCopyrightText: 2025 Gthulhu Team
//
// SPDX-License-Identifier: Apache-2.0
// Gthulhu Scheduling Event Monitor - eBPF program
//
// Collects per-process scheduling metrics using kernel tracepoints.
// Works on Linux 5.2+ (BTF-enabled kernels) — does NOT require sched_ext.
//
// Hooks:
//   tp_btf/sched_switch   — context switch events
//   tp_btf/sched_process_exit — cleanup on process exit
//
// Data flow:
//   BPF hash map (task_metrics)  →  Go collector reads periodically
//   BPF ring buffer (events_rb)  →  Go real-time consumer (optional)

#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include <bpf/bpf_core_read.h>

#include "sched_monitor.h"

char LICENSE[] SEC("license") = "GPL";

// ========================== Maps ==========================

// Per-PID cumulative scheduling metrics.
// Key: pid (u32), Value: struct task_sched_metrics
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 65536);
    __type(key, __u32);
    __type(value, struct task_sched_metrics);
} task_metrics SEC(".maps");

// PIDs to monitor. If this map is non-empty AND monitor_all == false,
// only PIDs present here (or whose tgid is in monitored_tgids) are tracked.
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 65536);
    __type(key, __u32);
    __type(value, __u8);
} monitored_pids SEC(".maps");

// TGIDs (process groups) to monitor.
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 8192);
    __type(key, __u32);
    __type(value, __u8);
} monitored_tgids SEC(".maps");

// Optional ring buffer for streaming events to user-space in real time.
struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 256 * 1024); // 256 KB
} events_rb SEC(".maps");

// ========================== Globals ==========================

// When true every task is monitored; when false only entries in
// monitored_pids / monitored_tgids are tracked.
volatile const bool monitor_all = false;

// When true switch events are also pushed to events_rb.
volatile const bool stream_events = false;

// ========================== Helpers ==========================

static __always_inline bool should_monitor(__u32 pid, __u32 tgid)
{
    if (monitor_all)
        return true;
    if (bpf_map_lookup_elem(&monitored_pids, &pid))
        return true;
    if (bpf_map_lookup_elem(&monitored_tgids, &tgid))
        return true;
    return false;
}

static __always_inline struct task_sched_metrics *get_or_init_metrics(__u32 pid, __u32 tgid)
{
    struct task_sched_metrics *m = bpf_map_lookup_elem(&task_metrics, &pid);
    if (m)
        return m;

    // First time seeing this PID — initialise an empty entry.
    struct task_sched_metrics init = {
        .pid  = pid,
        .tgid = tgid,
    };
    bpf_map_update_elem(&task_metrics, &pid, &init, BPF_NOEXIST);
    return bpf_map_lookup_elem(&task_metrics, &pid);
}

// ========================== Tracepoints ==========================

// tp_btf/sched_switch is invoked on every context switch.
// prev = task being switched OUT, next = task being switched IN.
// prev_state encodes whether the switch was voluntary.
//
// Kernel prototype (simplified):
//   void sched_switch(bool preempt,
//                     struct task_struct *prev,
//                     struct task_struct *next,
//                     unsigned int prev_state);
SEC("tp_btf/sched_switch")
int BPF_PROG(handle_sched_switch,
             bool preempt,
             struct task_struct *prev,
             struct task_struct *next,
             unsigned int prev_state)
{
    __u64 now = bpf_ktime_get_ns();
    __u32 prev_pid  = BPF_CORE_READ(prev, pid);
    __u32 prev_tgid = BPF_CORE_READ(prev, tgid);
    __u32 next_pid  = BPF_CORE_READ(next, pid);
    __u32 next_tgid = BPF_CORE_READ(next, tgid);
    __u32 cpu       = bpf_get_smp_processor_id();

    // ---- Handle PREV (switch-out) ----
    if (prev_pid && should_monitor(prev_pid, prev_tgid)) {
        struct task_sched_metrics *pm = get_or_init_metrics(prev_pid, prev_tgid);
        if (pm) {
            // Accumulate CPU time since last switch-in.
            __u64 delta = 0;
            if (pm->last_run_ts && now > pm->last_run_ts)
                delta = now - pm->last_run_ts;
            pm->cpu_time_ns += delta;
            pm->last_run_ts = 0; // no longer running

            // Classify context switch type.
            // prev_state == 0 (TASK_RUNNING) → involuntary (preempted)
            // prev_state != 0 → voluntary (blocked on IO / sleep / etc.)
            if (prev_state == 0)
                pm->involuntary_ctx_switches++;
            else
                pm->voluntary_ctx_switches++;

            // Record enqueue timestamp for wait-time accounting on next switch-in.
            pm->last_enqueue_ts = now;

            // Optional: stream event
            if (stream_events) {
                struct sched_event *evt = bpf_ringbuf_reserve(&events_rb, sizeof(*evt), 0);
                if (evt) {
                    evt->pid         = prev_pid;
                    evt->tgid        = prev_tgid;
                    evt->cpu         = cpu;
                    evt->event_type  = prev_state == 0
                                        ? SCHED_EVENT_SWITCH_OUT_INVOLUNTARY
                                        : SCHED_EVENT_SWITCH_OUT_VOLUNTARY;
                    evt->timestamp   = now;
                    evt->duration_ns = delta;
                    bpf_ringbuf_submit(evt, 0);
                }
            }
        }
    }

    // ---- Handle NEXT (switch-in) ----
    if (next_pid && should_monitor(next_pid, next_tgid)) {
        struct task_sched_metrics *nm = get_or_init_metrics(next_pid, next_tgid);
        if (nm) {
            nm->last_run_ts = now;
            nm->run_count++;

            // Compute wait time if we recorded an enqueue timestamp.
            if (nm->last_enqueue_ts && now > nm->last_enqueue_ts)
                nm->wait_time_ns += now - nm->last_enqueue_ts;
            nm->last_enqueue_ts = 0;

            // Detect CPU migration.
            if (nm->last_cpu != 0 && nm->last_cpu != cpu)
                nm->cpu_migrations++;
            nm->last_cpu = cpu;

            if (stream_events) {
                struct sched_event *evt = bpf_ringbuf_reserve(&events_rb, sizeof(*evt), 0);
                if (evt) {
                    evt->pid         = next_pid;
                    evt->tgid        = next_tgid;
                    evt->cpu         = cpu;
                    evt->event_type  = SCHED_EVENT_SWITCH_IN;
                    evt->timestamp   = now;
                    evt->duration_ns = 0;
                    bpf_ringbuf_submit(evt, 0);
                }
            }
        }
    }

    return 0;
}

// tp_btf/sched_process_exit — clean up map entries when a process exits.
SEC("tp_btf/sched_process_exit")
int BPF_PROG(handle_sched_process_exit, struct task_struct *task)
{
    __u32 pid  = BPF_CORE_READ(task, pid);
    __u32 tgid = BPF_CORE_READ(task, tgid);

    if (!should_monitor(pid, tgid))
        return 0;

    // Emit a final event before deleting.
    if (stream_events) {
        struct sched_event *evt = bpf_ringbuf_reserve(&events_rb, sizeof(*evt), 0);
        if (evt) {
            evt->pid         = pid;
            evt->tgid        = tgid;
            evt->cpu         = bpf_get_smp_processor_id();
            evt->event_type  = SCHED_EVENT_EXIT;
            evt->timestamp   = bpf_ktime_get_ns();
            evt->duration_ns = 0;
            bpf_ringbuf_submit(evt, 0);
        }
    }

    // Remove from the metrics map; the Go side should have already read it.
    bpf_map_delete_elem(&task_metrics, &pid);
    return 0;
}
