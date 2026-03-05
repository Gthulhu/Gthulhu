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
