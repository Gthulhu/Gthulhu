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

# Gthulhu API Server - Copilot Instructions

## Architecture Overview

This is a **dual-mode Go API server** that bridges Linux kernel scheduling (sched_ext) with Kubernetes:

- **Manager** (`manager/`): Central management service (port 8081) - handles users, RBAC, strategies, and distributes scheduling intents
- **Decision Maker** (`decisionmaker/`): DaemonSet per-node (port 8080) - receives intents, scans `/proc` for PIDs, and interfaces with eBPF scheduler

Both modes share the same binary, selected via `main.go manager` or `main.go decisionmaker` subcommands.

## Project Structure & Layered Architecture

Each service follows **Clean Architecture** with strict layer separation:

```
{manager,decisionmaker}/
├── app/        # Fx modules for DI wiring
├── cmd/        # Cobra command definitions
├── domain/     # Interfaces, entities, DTOs (Repository, Service, K8SAdapter)
├── rest/       # Echo handlers, routes, middleware
├── service/    # Business logic implementations
└── repository/ # MongoDB persistence (manager only)
```

Key interfaces in `manager/domain/interface.go`:
- `Repository` - data persistence
- `Service` - business logic
- `K8SAdapter` - Kubernetes Pod queries via Informer
- `DecisionMakerAdapter` - sends intents to DM nodes

## Development Commands

```bash
# Local infrastructure
make local-infra-up              # Start MongoDB via docker-compose
make local-run-manager           # Run Manager locally

# Testing
make test-all                    # Run all tests sequentially (required for integration tests)
go test -v ./manager/rest/...    # Run specific package tests

# Mocks & Docs
make gen-mock                    # Generate mocks via mockery (from domain interfaces)
make gen-manager-swagger         # Generate Swagger docs

# Kind cluster
make local-kind-setup            # Setup local Kind cluster
make local-kind-teardown         # Teardown Kind cluster
```

## Testing Patterns

Integration tests use **testcontainers** pattern (`pkg/container/`):
- `HandlerTestSuite` in `manager/rest/handler_test.go` spins up a real MongoDB container
- Use `app.TestRepoModule()` for container setup with Fx
- Mock K8S/DM adapters with mockery-generated mocks from `domain/mock_domain.go`
- Each test cleans DB via `util.MongoCleanup()` in `SetupTest()`

Example test structure:
```go
func (suite *HandlerTestSuite) TestSomething() {
    suite.MockK8SAdapter.EXPECT().QueryPods(...).Return(...)
    // Call handler, assert response
}
```

## Key Conventions

### Dependency Injection
- Use **Uber Fx** for DI - see `manager/app/module.go` for module composition
- Service constructors take `Params struct` with `fx.In` tag

### Configuration
- TOML configs in `config/` with `_config.go` parsers using Viper
- Sensitive values use `SecretValue` type (masked in logs)
- Test config: `manager_config.test.toml`

### REST API
- **Echo** framework with custom handler wrapper: `h.echoHandler(h.MethodName)`
- Auth middleware: `h.GetAuthMiddleware(domain.PermissionKey)`
- Routes in `rest/routes.go`, all versioned under `/api/v1`

### Error Handling
- Domain errors in `manager/domain/errors.go` and `manager/errs/errors.go`
- Use `pkg/errors` for wrapping

### Database
- MongoDB v2 driver (`go.mongodb.org/mongo-driver/v2`)
- Migrations in `manager/migration/` (JSON format, run via golang-migrate)
- Collections: `users`, `roles`, `permissions`, `schedule_strategies`, `schedule_intents`

## Important Entities

- `ScheduleStrategy` - Pod label selectors + scheduling params (priority, execution time)
- `ScheduleIntent` - Concrete intent for a specific Pod, distributed to DM nodes
- `User`, `Role`, `Permission` - RBAC model with JWT (RSA) authentication
