#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_core_read.h>
#include <linux/ip.h>
#include <linux/ptrace.h>

// For x86_64
#define PT_REGS_PARM1(x) ((x)->di)

struct event_t {
    __u64 ts_ns;
    __u32 cpu;
    __u32 pid;
    __u32 tgid;
    char comm[16];
    __u32 src_ip;
    __u32 dst_ip;
    __u16 src_port;
    __u16 dst_port;
    __u32 pkt_len;
    __u32 teid;
    __u32 func_id;
};

struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 1 << 24);
} events SEC(".maps");

// kprobe for gtp5g_handle_skb_ipv4
SEC("kprobe/gtp5g_handle_skb_ipv4") 
int kprobe__gtp5g_handle_skb_ipv4(struct pt_regs *ctx)
{
    struct event_t *e;
    e = bpf_ringbuf_reserve(&events, sizeof(*e), 0);
    if (!e)
        return 0;

    e->ts_ns = bpf_ktime_get_ns();
    e->cpu = bpf_get_smp_processor_id();
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    e->pid = (__u32)pid_tgid; // lower 32
    e->tgid = (__u32)(pid_tgid >> 32);
    bpf_get_current_comm(&e->comm, sizeof(e->comm));
    e->pkt_len = 0; // for toy example we don't parse skb
    e->teid = 0;
    e->func_id = 1; // id for gtp5g_handle_skb_ipv4

    bpf_ringbuf_submit(e, 0);
    return 0;
}

// kprobe for gtp5g_dev_xmit
SEC("kprobe/gtp5g_dev_xmit")
int kprobe__gtp5g_dev_xmit(struct pt_regs *ctx)
{
    struct event_t *e;
    e = bpf_ringbuf_reserve(&events, sizeof(*e), 0);
    if (!e)
        return 0;

    e->ts_ns = bpf_ktime_get_ns();
    e->cpu = bpf_get_smp_processor_id();
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    e->pid = (__u32)pid_tgid; // lower 32
    e->tgid = (__u32)(pid_tgid >> 32);
    bpf_get_current_comm(&e->comm, sizeof(e->comm));
    e->pkt_len = 0;
    e->teid = 0;
    e->func_id = 2; // id for gtp5g_dev_xmit

    bpf_ringbuf_submit(e, 0);
    return 0;
}

char LICENSE[] SEC("license") = "GPL";
