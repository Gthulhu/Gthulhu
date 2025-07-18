#include "wrapper.h"

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
        global_obj->bss->nr_queued--;
    }
}

void destroy_skel(void*skel) {
    main_bpf__destroy(skel);
}