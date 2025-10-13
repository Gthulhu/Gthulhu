<todos title="Todos" rule="Review steps frequently throughout the conversation and DO NOT stop between steps unless they explicitly require it.">
- No current todos
</todos>

# Gthulhu Scheduler Development Guide

Gthulhu is a Linux sched_ext scheduler that uses eBPF for kernel-level scheduling and Go for user-space policy implementation. This is a hybrid scheduler architecture with priority-aware task scheduling.

## Architecture Overview

### Core Components
- **BPF Component** (`main.bpf.c`): Implements sched-ext low-level kernel interface
- **Go Scheduler** (`main.go`): User-space scheduling policy implementation  
- **API Server** (`api/`): REST API for dynamic scheduling strategy configuration
- **Configuration** (`internal/config/`): YAML-based configuration management

### Hybrid BPF-Userspace Design
The scheduler uses a ringbuffer communication pattern:
- BPF enqueues tasks to user-space via `queued` ringbuffer
- User-space returns dispatch decisions via `dispatched` user ringbuffer
- BPF dispatcher is agnostic to scheduling policy - all logic is in Go

### Key Data Flow
# Copilot instructions for Gthulhu (sched_ext + eBPF + Go)

Purpose: make AI agents productive immediately in this repo by capturing the true architecture, data flow, build/test flows, and house rules with concrete file references.

Architecture and data flow
- BPF backend: `qumun/main.bpf.c` implements sched_ext ops; contracts in `qumun/intf.h`.
- User-space scheduler: `main.go` wires plugins and the BPF module from `github.com/Gthulhu/qumun/goland_core` and `github.com/Gthulhu/plugin/*`.
- API server: `api/` serves strategies and metrics with JWT auth.
- Communication: BPF ringbuf `queued` → Go; user ringbuf `dispatched` → BPF. Structs: `queued_task_ctx` and `dispatched_task_ctx` (see `qumun/intf.h`).
- Flow: `goland_enqueue` enqueues → Go computes vtime/slice/CPU → dispatch back → BPF does final DSQ insert/kick.

Key files and concepts
- `qumun/main.bpf.c`: ringbuffers, DSQs (per-CPU, SHARED_DSQ, SCHED_DSQ), priority path and maps (`priority_tasks`, `running_task`), idle CPU picking, heartbeat timer.
- `main.go`: config loading (`internal/config/config.go`, YAML `config/config.yaml`), plugin selection (simple|gthulhu), strategy fetcher + metrics, topology init via `qumun/util`.
- `qumun/intf.h`: exact fields for task structs; keep these in sync with Go models in the plugin layer.
- `api/main.go`: endpoints `/api/v1/scheduling/strategies`, `/api/v1/metrics`, JWT token `/api/v1/auth/token`; Kubernetes label-to-PID mapping.

Build, run, test (Makefile-backed)
- Prereqs: Linux 6.12+ with CONFIG_SCHED_CLASS_EXT, clang/LLVM 17+, Go 1.22+, root (CAP_SYS_ADMIN).
- First-time deps: `make dep` then build scx submodule: `cd scx && meson setup build --prefix ~ && meson compile -C build`.
- Build all: `make build` (compiles BPF `qumun/main.bpf.o`, generates skeleton, links `libbpf.a`, builds Go `./main`).
- Test in VM kernel: `make test` (runs with vng v6.12.2). Local run: `sudo ./main`.
- Optional image: `make image`; runtime needs `--privileged --pid host` when containerized.

Scheduling policy patterns (what “priority” means here)
- Priority is expressed by setting dispatched `vtime==0`. BPF tracks such PIDs in `priority_tasks` and may preempt or head-insert accordingly.
- Default slice and min slice come from config; Go may override per-task via strategy (see `bpfModule.DetermineTimeSlice` usage in `main.go`).
- Idle CPU selection honors cache domains; initialize domains via `cache.InitCacheDomains(bpfModule)`.

Configuration
- Primary YAML: `config/config.yaml` (overridable via `-config`). Fallbacks in `internal/config/config.go`.
- Notable keys: `scheduler.slice_ns_default`, `scheduler.slice_ns_min`, `scheduler.mode` ("simple" or default), `api.{enabled,url,interval,public_key_path}`, `debug`, `early_processing`, `builtin_idle`.

API integration (JWT-protected)
- Obtain token: POST `/api/v1/auth/token` with your PEM public key; include `Authorization: Bearer <token>` for subsequent calls (enforced except for token/health/static).
- Get/Set strategies: `/api/v1/scheduling/strategies` (GET/POST). Example POST payload snippet:
    {"strategies":[{"priority":true,"execution_time":20000000,"selectors":[{"key":"nf","value":"upf"}],"command_regex":".*"}]}
- Metrics: POST `/api/v1/metrics`; current metrics: GET `/api/v1/metrics`. Pod→PID map: GET `/api/v1/pods/pids` (Kubernetes lookup with caching; falls back to mock on failure).

Debugging essentials
- Lint/vet: `make lint`. Trace BPF: `sudo cat /sys/kernel/debug/tracing/trace_pipe`. Inspect: `sudo bpftool prog list` / `sudo bpftool map list`.
- User exit info (UEI) is recorded by BPF and printed by `main.go` on shutdown.

Conventions and limits you should respect
- Keep algorithms in user-space; BPF is minimal (stack limits, no dyn alloc). Use provided ringbuffers and maps; do not change struct layouts lightly.
- Task pool: lockless circular buffer of 4096 (see `qumun/main.go` ref impl); ringbuffers sized for that magnitude.
- Topology aware CPU choice and DSQ usage are centralized in BPF; user-space provides hints (cpu, vtime, slice_ns) only.

Common gotchas (seen in this repo)
- libelf conflict: if you hit `eu_search_tree_init` linker errors, prefer arachsys libelf (see root README troubleshooting).
- Ensure scx headers are built (`scx/build/...`) before BPF compile; skeleton is generated via Makefile target `wrapper`.

