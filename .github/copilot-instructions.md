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
1. Tasks are enqueued in BPF (`goland_enqueue`)
2. Task info sent to user-space via `struct queued_task_ctx`
3. Go scheduler applies vruntime/priority logic
4. Tasks dispatched back via `struct dispatched_task_ctx`
5. BPF handles final CPU assignment and dispatch

## Development Workflows

### Building
```bash
make dep                    # Clone libbpf and scx dependencies
git submodule init && git submodule sync && git submodule update
cd scx && meson setup build --prefix ~ && meson compile -C build
make build                  # Compile BPF, generate skeleton, build Go binary
```

### Testing
```bash
make test                   # Run in vng virtual environment with kernel v6.12.2
sudo ./main                 # Run on actual system (requires sched_ext kernel)
```

### Debugging
- Use `make lint` for code quality checks
- BPF debugging: `sudo cat /sys/kernel/debug/tracing/trace_pipe`
- Inspect BPF state: `sudo bpftool prog list` and `sudo bpftool map list`

## Project-Specific Patterns

### Configuration Management
- YAML config in `config/config.yaml` or via `-config` flag
- Fallback to sensible defaults in `internal/config/config.go`
- Runtime reconfiguration via API server

### Scheduling Strategy System
Priority tasks get vtime=0 for immediate scheduling:
```go
// In strategy.go
if strategy.Priority {
    task.Vtime = 0  // Minimum vtime = highest priority
}
```

### CPU Topology Awareness
- `cache.GetTopology()` and `cache.InitCacheDomains()` for NUMA/cache-aware placement
- Priority CPU tracking to avoid interference with regular tasks

### Memory Management
Ring buffers use explicit size limits:
- Task pool: 4096 entries circular buffer
- BPF ringbuffer communication for task passing

## Critical Dependencies

### External Submodules
- `libbpf/`: eBPF library (specific commit 09b9e83)
- `scx/`: sched_ext framework and example schedulers
- `libbpfgo/`: Custom Go BPF bindings (replaced via go.mod)

### Build Dependencies
- Kernel 6.12+ with sched_ext support
- LLVM/Clang 17+ for BPF compilation
- Go 1.22.6+

### Runtime Requirements
- `CAP_SYS_ADMIN` or root for BPF program loading
- sched_ext enabled kernel with CONFIG_SCHED_CLASS_EXT=y

## API Integration Points

### REST API Server (`api/`)
- Dynamic strategy updates via `/api/v1/scheduling/strategies`
- Pod-to-PID mapping for Kubernetes integration
- Metrics collection for observability

### Kubernetes Integration
- Pod label-based scheduling strategies
- Process command regex matching
- Automatic PID discovery from cgroups

## Common Anti-Patterns

### BPF Limitations
- Avoid complex algorithms in BPF - keep scheduling logic in Go user-space
- BPF has limited stack size and no dynamic memory allocation
- Use libbpfgo wrappers instead of raw BPF syscalls

### Concurrency
- Task pool uses lockless circular buffer design
- Strategy map updates use RWMutex for concurrent access
- Context cancellation for graceful shutdown

### Error Handling
- BPF errors bubble up via `uei` (user exit info) mechanism
- Fallback to kernel scheduling on user-space scheduler failure
- Comprehensive logging with structured JSON output
