// SPDX-FileCopyrightText: 2025 Gthulhu Team
//
// SPDX-License-Identifier: Apache-2.0
// Author: Ian Chen <ychen.desl@gmail.com>

#include "wrapper.h"
#include <bpf/libbpf.h>

struct main_bpf *global_obj;

void *open_skel() {
    struct main_bpf *obj = NULL;
    obj = main_bpf__open();
    main_bpf__create_skeleton(obj);
    global_obj = obj;
    return obj->obj;
}

u32 get_usersched_pid() {
    return global_obj->rodata->usersched_pid;
}

void set_usersched_pid(u32 id) {
    global_obj->rodata->usersched_pid = id;
}

void set_kugepagepid(u32 id) {
    global_obj->rodata->khugepaged_pid = id;
}

void set_early_processing(bool enabled) {
    global_obj->rodata->early_processing = enabled;
}

void set_default_slice(u64 t) {
    global_obj->rodata->default_slice = t;
}

void set_debug(bool enabled) {
    global_obj->rodata->debug = enabled;
}

void set_builtin_idle(bool enabled) {
    global_obj->rodata->builtin_idle = enabled;
}

void enable_kernel_mode() {
    global_obj->rodata->kernel_mode = true;
}

void disable_max_time_watchdog() {
    global_obj->rodata->max_time_watchdog = false;
}

u64 get_nr_scheduled() {
    return global_obj->bss->nr_scheduled;
}

u64 get_nr_queued() {
    return global_obj->bss->nr_queued;
}

void notify_complete(u64 nr_pending) {
    global_obj->bss->nr_scheduled = nr_pending;
}

void sub_nr_queued() {
    if (global_obj->bss->nr_queued){
        __sync_fetch_and_sub(&global_obj->bss->nr_queued, 1);
    }
}

void dec_nr_queued(u64 num) {
    if (global_obj->bss->nr_queued >= num){
        __sync_fetch_and_sub(&global_obj->bss->nr_queued, num);
    }
}

void destroy_skel(void*skel) {
    main_bpf__destroy(skel);
}

int update_priority_task(u32 pid, u64 slice) {
    if (!global_obj || !global_obj->maps.priority_tasks)
        return -1;
    return bpf_map__update_elem(global_obj->maps.priority_tasks, 
                                &pid, sizeof(pid), 
                                &slice, sizeof(slice), 
                                BPF_ANY);
}

/*
 * Note: There is a potential race condition in the two-step update process.
 * When priority_tasks is updated first and then priority_tasks_prio, there's
 * a brief window where the BPF code could observe a task in priority_tasks
 * but not in priority_tasks_prio. During this window, the BPF code will
 * default the task to priority 0 (highest priority with preemption).
 * This is considered acceptable as it's a transient state and the BPF code
 * handles missing priority gracefully. A more atomic approach is limited
 * by BPF map API constraints.
 */
int update_priority_task_with_prio(u32 pid, u64 slice, u32 prio) {
    if (!global_obj || !global_obj->maps.priority_tasks || 
        !global_obj->maps.priority_tasks_prio)
        return -1;
    
    int ret = bpf_map__update_elem(global_obj->maps.priority_tasks, 
                                   &pid, sizeof(pid), 
                                   &slice, sizeof(slice), 
                                   BPF_ANY);
    if (ret != 0)
        return ret;
    
    return bpf_map__update_elem(global_obj->maps.priority_tasks_prio, 
                                &pid, sizeof(pid), 
                                &prio, sizeof(prio), 
                                BPF_ANY);
}

int remove_priority_task(u32 pid) {
    if (!global_obj || !global_obj->maps.priority_tasks)
        return -1;

    int ret;

    ret = bpf_map__delete_elem(global_obj->maps.priority_tasks,
                               &pid, sizeof(pid),
                               0);
    if (ret != 0)
        return ret;

    /* Also remove from priority_tasks_prio if it exists */
    if (global_obj->maps.priority_tasks_prio) {
        ret = bpf_map__delete_elem(global_obj->maps.priority_tasks_prio,
                                   &pid, sizeof(pid),
                                   0);
        if (ret != 0)
            return ret;
    }

    return 0;
}

void set_scx_enums(
    u64 SCX_OPS_NAME_LEN,
    u64 SCX_SLICE_DFL,
    u64 SCX_SLICE_INF,
    u64 SCX_RQ_ONLINE,
    u64 SCX_RQ_CAN_STOP_TICK,
    u64 SCX_RQ_BAL_PENDING,
    u64 SCX_RQ_BAL_KEEP,
    u64 SCX_RQ_BYPASSING,
    u64 SCX_RQ_CLK_VALID,
    u64 SCX_RQ_IN_WAKEUP,
    u64 SCX_RQ_IN_BALANCE,
    u64 SCX_DSQ_FLAG_BUILTIN,
    u64 SCX_DSQ_FLAG_LOCAL_ON,
    u64 SCX_DSQ_INVALID,
    u64 SCX_DSQ_GLOBAL,
    u64 SCX_DSQ_LOCAL,
    u64 SCX_DSQ_LOCAL_ON,
    u64 SCX_DSQ_LOCAL_CPU_MASK,
    u64 SCX_TASK_QUEUED,
    u64 SCX_TASK_RESET_RUNNABLE_AT,
    u64 SCX_TASK_DEQD_FOR_SLEEP,
    u64 SCX_TASK_STATE_SHIFT,
    u64 SCX_TASK_STATE_BITS,
    u64 SCX_TASK_STATE_MASK,
    u64 SCX_TASK_CURSOR,
    u64 SCX_TASK_NONE,
    u64 SCX_TASK_INIT,
    u64 SCX_TASK_READY,
    u64 SCX_TASK_ENABLED,
    u64 SCX_TASK_NR_STATES,
    u64 SCX_TASK_DSQ_ON_PRIQ,
    u64 SCX_KICK_IDLE,
    u64 SCX_KICK_PREEMPT,
    u64 SCX_KICK_WAIT,
    u64 SCX_ENQ_WAKEUP,
    u64 SCX_ENQ_HEAD,
    u64 SCX_ENQ_PREEMPT,
    u64 SCX_ENQ_REENQ,
    u64 SCX_ENQ_LAST,
    u64 SCX_ENQ_CLEAR_OPSS,
    u64 SCX_ENQ_DSQ_PRIQ
) {
    if (!global_obj || !global_obj->rodata) return;
    global_obj->rodata->__SCX_OPS_NAME_LEN = SCX_OPS_NAME_LEN;
    global_obj->rodata->__SCX_SLICE_DFL = SCX_SLICE_DFL;
    global_obj->rodata->__SCX_SLICE_INF = SCX_SLICE_INF;
    global_obj->rodata->__SCX_RQ_ONLINE = SCX_RQ_ONLINE;
    global_obj->rodata->__SCX_RQ_CAN_STOP_TICK = SCX_RQ_CAN_STOP_TICK;
    global_obj->rodata->__SCX_RQ_BAL_PENDING = SCX_RQ_BAL_PENDING;
    global_obj->rodata->__SCX_RQ_BAL_KEEP = SCX_RQ_BAL_KEEP;
    global_obj->rodata->__SCX_RQ_BYPASSING = SCX_RQ_BYPASSING;
    global_obj->rodata->__SCX_RQ_CLK_VALID = SCX_RQ_CLK_VALID;
    global_obj->rodata->__SCX_RQ_IN_WAKEUP = SCX_RQ_IN_WAKEUP;
    global_obj->rodata->__SCX_RQ_IN_BALANCE = SCX_RQ_IN_BALANCE;
    global_obj->rodata->__SCX_DSQ_FLAG_BUILTIN = SCX_DSQ_FLAG_BUILTIN;
    global_obj->rodata->__SCX_DSQ_FLAG_LOCAL_ON = SCX_DSQ_FLAG_LOCAL_ON;
    global_obj->rodata->__SCX_DSQ_INVALID = SCX_DSQ_INVALID;
    global_obj->rodata->__SCX_DSQ_GLOBAL = SCX_DSQ_GLOBAL;
    global_obj->rodata->__SCX_DSQ_LOCAL = SCX_DSQ_LOCAL;
    global_obj->rodata->__SCX_DSQ_LOCAL_ON = SCX_DSQ_LOCAL_ON;
    global_obj->rodata->__SCX_DSQ_LOCAL_CPU_MASK = SCX_DSQ_LOCAL_CPU_MASK;
    global_obj->rodata->__SCX_TASK_QUEUED = SCX_TASK_QUEUED;
    global_obj->rodata->__SCX_TASK_RESET_RUNNABLE_AT = SCX_TASK_RESET_RUNNABLE_AT;
    global_obj->rodata->__SCX_TASK_DEQD_FOR_SLEEP = SCX_TASK_DEQD_FOR_SLEEP;
    global_obj->rodata->__SCX_TASK_STATE_SHIFT = SCX_TASK_STATE_SHIFT;
    global_obj->rodata->__SCX_TASK_STATE_BITS = SCX_TASK_STATE_BITS;
    global_obj->rodata->__SCX_TASK_STATE_MASK = SCX_TASK_STATE_MASK;
    global_obj->rodata->__SCX_TASK_CURSOR = SCX_TASK_CURSOR;
    global_obj->rodata->__SCX_TASK_NONE = SCX_TASK_NONE;
    global_obj->rodata->__SCX_TASK_INIT = SCX_TASK_INIT;
    global_obj->rodata->__SCX_TASK_READY = SCX_TASK_READY;
    global_obj->rodata->__SCX_TASK_ENABLED = SCX_TASK_ENABLED;
    global_obj->rodata->__SCX_TASK_NR_STATES = SCX_TASK_NR_STATES;
    global_obj->rodata->__SCX_TASK_DSQ_ON_PRIQ = SCX_TASK_DSQ_ON_PRIQ;
    global_obj->rodata->__SCX_KICK_IDLE = SCX_KICK_IDLE;
    global_obj->rodata->__SCX_KICK_PREEMPT = SCX_KICK_PREEMPT;
    global_obj->rodata->__SCX_KICK_WAIT = SCX_KICK_WAIT;
    global_obj->rodata->__SCX_ENQ_WAKEUP = SCX_ENQ_WAKEUP;
    global_obj->rodata->__SCX_ENQ_HEAD = SCX_ENQ_HEAD;
    global_obj->rodata->__SCX_ENQ_PREEMPT = SCX_ENQ_PREEMPT;
    global_obj->rodata->__SCX_ENQ_REENQ = SCX_ENQ_REENQ;
    global_obj->rodata->__SCX_ENQ_LAST = SCX_ENQ_LAST;
    global_obj->rodata->__SCX_ENQ_CLEAR_OPSS = SCX_ENQ_CLEAR_OPSS;
    global_obj->rodata->__SCX_ENQ_DSQ_PRIQ = SCX_ENQ_DSQ_PRIQ;
}
