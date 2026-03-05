# Gthulhu API Server

Gthulhu API Server is a Golang-based API server designed to integrate the Linux kernel scheduler (sched_ext) with the Kubernetes orchestration system. The core objective of this project is to collect scheduling metrics from eBPF programs and provide intelligent scheduling decisions to the kernel scheduler based on Kubernetes Pod labels and user-defined strategies.

## DEMO

Click the image below to watch our DEMO on YouTube!

[![IMAGE ALT TEXT HERE](https://github.com/Gthulhu/Gthulhu/blob/main/assets/preview.png?raw=true)](https://www.youtube.com/watch?v=R4EmZ18P954)

## System Architecture

This API Server adopts a dual-mode architecture, consisting of two independent services:

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                              Gthulhu Architecture                               │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                 │
│   ┌─────────────┐         ┌─────────────────────┐         ┌─────────────────┐   │
│   │    User     │ ──────▶ │      Manager        │ ──────▶ │    MongoDB      │   │
│   │  (Web UI)   │         │ (Central Management)│         │  (Persistence)  │   │
│   └─────────────┘         └──────────┬──────────┘         └─────────────────┘   │
│                                      │                                          │
│                                      │ Query Pods via K8s API                   │
│                                      ▼                                          │
│                           ┌─────────────────────┐                               │
│                           │   Kubernetes API    │                               │
│                           │   (Pod Informer)    │                               │
│                           └─────────────────────┘                               │
│                                      │                                          │
│              ┌───────────────────────┼───────────────────────┐                  │
│              │                       │                       │                  │
│              ▼                       ▼                       ▼                  │
│   ┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐           │
│   │ Decision Maker  │     │ Decision Maker  │     │ Decision Maker  │           │
│   │   (Node 1)      │     │   (Node 2)      │     │   (Node N)      │           │
│   └────────┬────────┘     └────────┬────────┘     └────────┬────────┘           │
│            │                       │                       │                    │
│            ▼                       ▼                       ▼                    │
│   ┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐           │
│   │  sched_ext      │     │  sched_ext      │     │  sched_ext      │           │
│   │ (eBPF Scheduler)│     │ (eBPF Scheduler)│     │ (eBPF Scheduler)│           │
│   └─────────────────┘     └─────────────────┘     └─────────────────┘           │
│                                                                                 │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### Manager Mode

**Purpose**: Serves as the central management service, responsible for handling user requests, managing scheduling strategies, user authentication, and access control.

**Key Features**:
- User authentication and authorization (JWT Token)
- Role and permission management (RBAC)
- CRUD operations for scheduling strategies
- Monitor Pod status via Kubernetes Informer
- Distribute scheduling intents to Decision Makers on each node
- Data persistence to MongoDB

**Default Port**: `:8081`

### Decision Maker Mode

**Purpose**: Deployed on each Kubernetes node (as DaemonSet), responsible for receiving scheduling intents and interacting with the local sched_ext scheduler.

**Key Features**:
- Receive scheduling intents from Manager
- Scan `/proc` filesystem to discover Pod processes
- Convert scheduling strategies into concrete PID-based scheduling decisions
- Collect eBPF scheduler metrics
- Expose metrics via Prometheus endpoint

**Default Port**: `:8080`

## Core Features

### Manager Service Features
- **User Management**: Create, query users, password reset
- **Role & Permission Management**: RBAC role management, permission assignment
- **Scheduling Strategy Management**: Create Pod label-based scheduling strategies
- **Scheduling Intent Tracking**: Track strategy execution status
- **Kubernetes Integration**: Real-time Pod monitoring via Pod Informer
- **JWT Authentication**: RSA asymmetric encryption Token authentication

### Decision Maker Service Features
- **Intent Processing**: Receive and process scheduling intents from Manager
- **Process Discovery**: Parse cgroup information to map PIDs to Pods
- **Scheduling Strategy Provider**: Provide concrete PID scheduling strategies to sched_ext
- **Metrics Collection**: Collect and expose eBPF scheduler metrics to Prometheus
- **Token Authentication**: Validate requests from Manager

## API Endpoints

### Manager Endpoints

#### System Endpoints
| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Health check |
| `/version` | GET | Version information |
| `/swagger/*` | GET | Swagger documentation |

#### Authentication Endpoints
| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/auth/login` | POST | User login |

#### User Management Endpoints
| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/users` | POST | Create user |
| `/api/v1/users` | GET | List users |
| `/api/v1/users/password` | PUT | Reset password |
| `/api/v1/users/permissions` | PUT | Update permissions |
| `/api/v1/users/self/password` | PUT | Change own password |
| `/api/v1/users/self` | GET | Get own information |

#### Role Management Endpoints
| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/roles` | POST | Create role |
| `/api/v1/roles` | GET | List roles |
| `/api/v1/roles` | PUT | Update role |
| `/api/v1/roles` | DELETE | Delete role |
| `/api/v1/permissions` | GET | List permissions |

#### Scheduling Strategy Endpoints
| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/strategies` | POST | Create scheduling strategy |
| `/api/v1/strategies/self` | GET | List own strategies |
| `/api/v1/intents/self` | GET | List own scheduling intents |

### Decision Maker Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Health check |
| `/version` | GET | Version information |
| `/metrics` | GET | Prometheus metrics |
| `/api/v1/auth/token` | POST | Get authentication token |
| `/api/v1/intents` | POST | Receive scheduling intents |
| `/api/v1/scheduling/strategies` | GET | Get scheduling strategies |
| `/api/v1/metrics` | POST | Update metrics data |

## Data Structures

### ScheduleStrategy
| Field | Type | Description |
|-------|------|-------------|
| `strategyNamespace` | string | Strategy namespace |
| `labelSelectors` | []LabelSelector | Pod label selectors |
| `k8sNamespace` | []string | Kubernetes namespaces |
| `commandRegex` | string | Process command regex |
| `priority` | int | Priority level |
| `executionTime` | int64 | Execution time (nanoseconds) |

### ScheduleIntent
| Field | Type | Description |
|-------|------|-------------|
| `podID` | string | Pod UID |
| `podName` | string | Pod name |
| `nodeID` | string | Node name |
| `k8sNamespace` | string | Kubernetes namespace |
| `commandRegex` | string | Process command regex |
| `priority` | int | Priority level |
| `executionTime` | int64 | Execution time (nanoseconds) |
| `podLabels` | map[string]string | Pod labels |
| `state` | string | Intent state |

### MetricSet
| Field | Type | Description |
|-------|------|-------------|
| `usersched_last_run_at` | uint64 | User scheduler last run timestamp |
| `nr_queued` | uint64 | Number of tasks in scheduling queue |
| `nr_scheduled` | uint64 | Number of scheduled tasks |
| `nr_running` | uint64 | Number of running tasks |
| `nr_online_cpus` | uint64 | Number of online CPUs |
| `nr_user_dispatches` | uint64 | Number of user-space dispatches |
| `nr_kernel_dispatches` | uint64 | Number of kernel-space dispatches |
| `nr_cancel_dispatches` | uint64 | Number of cancelled dispatches |
| `nr_bounce_dispatches` | uint64 | Number of bounce dispatches |
| `nr_failed_dispatches` | uint64 | Number of failed dispatches |
| `nr_sched_congested` | uint64 | Number of scheduler congestion events |

## Quick Start

### 0. Test Environment Setup

For local development, you need to set up a MongoDB instance. You can use Docker to start the infrastructure:

```bash
# Start MongoDB using Docker
$ docker run --rm --network host mongo:6.0 mongosh "mongodb://test:test@localhost:27017/?authSource=admin&authMechanism=SCRAM-SHA-256" --eval 'db.runCommand({ ping: 1 })'
```

After starting MongoDB, create the required user and verify the connection:

```bash
# Create a test user with root privileges in MongoDB
$ docker exec mongodb mongosh --eval 'db.getSiblingDB("admin").createUser({user:"test",pwd:"test",roles:[{role:"root",db:"admin"}]})'

# Verify the connection with the created user credentials
$ docker exec mongodb mongosh "mongodb://test:test@localhost:27017/?authSource=admin&authMechanism=SCRAM-SHA-256" --eval 'db.runCommand({ ping: 1 })'
```

### 1. Install Dependencies
```bash
go mod tidy
```

### 2. Configuration

#### Manager Configuration (`config/manager_config.toml`)
```toml
[server]
host = ":8081"

[logging]
level = "info"

[mongodb]
host = "localhost"
port = "27017"
user = "test"
password = "test"
database = "manager"

[k8s]
kube_config_path = "/path/to/.kube/config"
in_cluster = false

[key]
rsa_private_key_pem = "..."
dm_public_key_pem = "..."
client_id = "your-client-id"

[account]
admin_email = "admin@example.com"
admin_password = "your-password"

# mTLS for Manager → Decision Maker communication (optional, default: disabled)
[mtls]
enable = false
cert_pem = "..."   # Manager's client certificate (signed by private CA)
key_pem  = "..."   # Manager's client private key
ca_pem   = "..."   # Private CA certificate (to verify Decision Maker's server cert)
```

#### Decision Maker Configuration (`config/dm_config.toml`)
```toml
[server]
host = ":8080"

[logging]
level = "info"

[token]
rsa_private_key_pem = "..."
token_duration_hr = 24

# mTLS server for Manager → Decision Maker communication (optional, default: disabled)
[mtls]
enable = false
cert_pem = "..."   # Decision Maker's server certificate (signed by private CA)
key_pem  = "..."   # Decision Maker's server private key
ca_pem   = "..."   # Private CA certificate (to verify Manager's client cert)
```

### 3. Start Services

#### Start Manager
```bash
go run main.go manager

# With custom configuration
go run main.go manager -c manager_config -d /path/to/config
```

#### Start Decision Maker
```bash
go run main.go decisionmaker

# With custom configuration
go run main.go decisionmaker -c dm_config -d /path/to/config
```

### 4. Test API

Please refer to https://github.com/Gthulhu/chart?tab=readme-ov-file#testing for testing the API endpoints using curl.

## mTLS Setup: Manager ↔ Decision Maker

The Manager communicates with every Decision Maker node using **mutual TLS (mTLS)**.  Both sides authenticate each other with certificates signed by a shared **private CA**, so neither plain-text traffic nor untrusted connections are accepted.

> **Note**: The Manager's external HTTP API (web GUI, `/api/v1/…`) intentionally remains plain HTTP.  In a production cluster this endpoint is typically exposed through a Kubernetes Ingress with TLS termination.

### Why mTLS?

Scheduling decisions affect the Linux kernel scheduler on every node.  A compromised connection between the Manager and a Decision Maker could allow an attacker to manipulate per-process CPU priorities.  mTLS provides:

- **Server authentication** – the Manager verifies it is talking to a genuine Decision Maker.
- **Client authentication** – the Decision Maker verifies only the authorised Manager can push intents.
- **Encrypted channel** – all scheduling intents are protected in transit.

### Step-by-step: Generate certificates with a private CA

The commands below use only the OpenSSL CLI. Replace `<DM_IP>` with the actual IP or hostname of each Decision Maker node.

#### 1. Create the private CA

```bash
# Generate CA private key (EC P-256 recommended; RSA-4096 also works)
openssl ecparam -name prime256v1 -genkey -noout -out ca.key

# Self-signed CA certificate (10-year validity)
openssl req -new -x509 -days 3650 \
  -key ca.key \
  -out ca.crt \
  -subj "/CN=Gthulhu-Private-CA"
```

#### 2. Generate the Manager client certificate

```bash
# Manager private key
openssl ecparam -name prime256v1 -genkey -noout -out manager.key

# Certificate signing request
openssl req -new \
  -key manager.key \
  -out manager.csr \
  -subj "/CN=gthulhu-manager"

# Sign with the private CA (2-year validity, client-auth EKU)
openssl x509 -req -days 730 \
  -in manager.csr \
  -CA ca.crt -CAkey ca.key -CAcreateserial \
  -extfile <(printf "extendedKeyUsage=clientAuth") \
  -out manager.crt
```

#### 3. Generate a Decision Maker server certificate

Repeat for each DM node, setting the correct IP/DNS in `subjectAltName`.

```bash
# Decision Maker private key
openssl ecparam -name prime256v1 -genkey -noout -out dm.key

# CSR
openssl req -new \
  -key dm.key \
  -out dm.csr \
  -subj "/CN=gthulhu-decisionmaker"

# Sign with the private CA (2-year validity, server-auth + client-auth EKUs + SAN)
openssl x509 -req -days 730 \
  -in dm.csr \
  -CA ca.crt -CAkey ca.key -CAcreateserial \
  -extfile <(printf "subjectAltName=IP:<DM_IP>\nextendedKeyUsage=serverAuth,clientAuth") \
  -out dm.crt
```

#### 4. Embed certificates in configuration

Paste the PEM file contents into the respective config files.

**`config/manager_config.toml`**

```toml
[mtls]
enable = true
cert_pem = """
-----BEGIN CERTIFICATE-----
<contents of manager.crt>
-----END CERTIFICATE-----
"""
key_pem = """
-----BEGIN EC PRIVATE KEY-----
<contents of manager.key>
-----END EC PRIVATE KEY-----
"""
ca_pem = """
-----BEGIN CERTIFICATE-----
<contents of ca.crt>
-----END CERTIFICATE-----
"""
```

**`config/dm_config.toml`**

```toml
[mtls]
enable = true
cert_pem = """
-----BEGIN CERTIFICATE-----
<contents of dm.crt>
-----END CERTIFICATE-----
"""
key_pem = """
-----BEGIN EC PRIVATE KEY-----
<contents of dm.key>
-----END EC PRIVATE KEY-----
"""
ca_pem = """
-----BEGIN CERTIFICATE-----
<contents of ca.crt>
-----END CERTIFICATE-----
"""
```

#### 5. Verify

Start both services and confirm the Decision Maker log contains:

```
starting dm server with mTLS on port :8080
```

And the Manager log shows successful intent reconciliation without TLS errors.

### Kubernetes: mounting certificates as Secrets

In a production deployment, store PEM content in Kubernetes Secrets and mount them as environment variables or files, then reference them in the TOML config.

```bash
kubectl create secret generic gthulhu-mtls-certs \
  --from-file=ca.crt \
  --from-file=manager.crt \
  --from-file=manager.key
```

## Kubernetes Deployment

### Deployment Architecture
- **Manager**: Deployed as a Deployment, typically single replica
- **Decision Maker**: Deployed as a DaemonSet on every node

### Deployment Manifest Locations
```
deployment/
├── kind/                    # Kind local test environment
│   ├── local_setup.sh
│   ├── decisonmaker/
│   │   ├── daemonset.yaml
│   │   └── service.yaml
│   ├── manager/
│   │   ├── deployment.yaml
│   │   └── service.yaml
│   └── mongo/
│       ├── secret.yaml
│       ├── service.yaml
│       └── statefulset.yaml
└── local/
    └── docker-compose.infra.yaml  # Docker Compose for local development
```

### Quick Deployment
```bash
# Local testing with Kind
cd deployment/kind
./local_setup.sh

# Or manual deployment
kubectl apply -f deployment/kind/mongo/
kubectl apply -f deployment/kind/manager/
kubectl apply -f deployment/kind/decisonmaker/
```

### RBAC Permission Requirements
Manager requires the following Kubernetes RBAC permissions:
- `pods`: list, watch, get
- `namespaces`: list, get

## Development Guide

### Project Structure
```
.
├── main.go                 # Entry point, uses Cobra for subcommands
├── config/                 # Configuration definition and parsing
├── manager/               # Manager service
│   ├── app/              # Application initialization
│   ├── cmd/              # Cobra commands
│   ├── domain/           # Domain models and interfaces
│   ├── k8s_adapter/      # Kubernetes client
│   ├── client/           # Decision Maker client
│   ├── repository/       # MongoDB data access
│   ├── rest/             # HTTP handlers
│   ├── service/          # Business logic
│   └── migration/        # Database migrations
├── decisionmaker/        # Decision Maker service
│   ├── app/             # Application initialization
│   ├── cmd/             # Cobra commands
│   ├── domain/          # Domain models
│   ├── rest/            # HTTP handlers
│   └── service/         # Business logic and process discovery
├── pkg/                  # Shared packages
│   ├── logger/          # Logging utilities
│   ├── middleware/      # HTTP middleware
│   └── util/            # Common utility functions
└── deployment/          # Deployment configurations
```

### Tech Stack
- **Web Framework**: Echo v4
- **Dependency Injection**: Uber fx
- **CLI**: Cobra
- **Logging**: zerolog
- **Database**: MongoDB (mongo-driver v2)
- **Kubernetes Client**: client-go
- **Metrics Collection**: Prometheus client_golang
- **Configuration Management**: Viper

### Build and Test
```bash
# Build
make build

# Run tests
make test

# Build Docker image
make image
```

## Container Images

This project uses GitHub Actions to automatically build and publish to GitHub Container Registry.

```bash
# Use latest version
docker pull ghcr.io/gthulhu/api:main

# Use specific version
docker pull ghcr.io/gthulhu/api:v1.0.0
```

## License

This project is open source.
