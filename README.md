# Gthulhu Project

<a href="https://landscape.cncf.io/?item=provisioning--automation-configuration--gthulhu" target="_blank"><img src="https://img.shields.io/badge/CNCF%20Landscape-5699C6?style=for-the-badge&logo=cncf&label=cncf" alt="cncf landscape" /></a>

<a href="https://ebpf.io/applications/" target="_blank"><img src="https://img.shields.io/badge/eBPF%20Application%20Landscape-5699C6?style=for-the-badge&logo=ebpf&label=ebpf" alt="ebpf landscape" /></a>

[![LFX Contributors](https://insights.linuxfoundation.org/api/badge/contributors?project=gthulhu)](https://insights.linuxfoundation.org/project/gthulhu)
[![Go](https://github.com/Gthulhu/Gthulhu/actions/workflows/go.yaml/badge.svg)](https://github.com/Gthulhu/Gthulhu/actions/workflows/go.yaml)
[![Portability](https://github.com/Gthulhu/Gthulhu/actions/workflows/portability.yaml/badge.svg)](https://github.com/Gthulhu/Gthulhu/actions/workflows/portability.yaml)
[![Go Report Card](https://goreportcard.com/badge/github.com/Gthulhu/Gthulhu)](https://goreportcard.com/report/github.com/Gthulhu/Gthulhu)

<img src="./assets/logo.png" alt="logo" width="300"/>

## Overview

Gthulhu is a cloud-native workload orchestration platform that provides granular, pod-level scheduling observability and automated scaling for Kubernetes workloads. Through an intuitive web GUI, users can select workloads running on Kubernetes, monitor fine-grained scheduling metrics collected via eBPF, and configure automatic scaling policies powered by KEDA — all without modifying the kernel or application code. For clusters running Linux 6.12+ with `sched_ext`, Gthulhu further supports defining scheduling strategies and distributing scheduling intents to each node, enabling kernel-level custom CPU scheduling across the cluster.

### Key Capabilities

- **Pod-Level Scheduling Metrics** — Gthulhu uses eBPF to hook into kernel scheduling events (`fentry`/`fexit`), collecting per-process metrics such as voluntary/involuntary context switches, CPU time, wait time, run count, and CPU migrations. These metrics are aggregated at the pod level and exposed via REST APIs.
- **Declarative Configuration** — Users define which workloads to monitor using Kubernetes label selectors and namespaces, either through the web GUI or the `PodSchedulingMetrics` CRD.
- **KEDA Auto-Scaling Integration** — Gthulhu provides out-of-the-box integration with [KEDA](https://keda.sh/), enabling auto-scaling decisions driven by real scheduling behavior rather than generic resource utilization.
- **Advanced: Scheduling Strategies & Intents** *(requires Linux 6.12+ with `sched_ext`)* — Users can define scheduling strategies (priority, time-slice, CPU affinity) for specific workloads via the web GUI or REST API. The Manager converts strategies into scheduling intents and distributes them to Decision Makers on each node, enabling cross-node coordinated scheduling policy enforcement.
- **Advanced: Custom CPU Scheduling** *(requires Linux 6.12+ with `sched_ext`)* — On nodes running a supported kernel, Gthulhu attaches a custom eBPF-based CPU scheduler through the `sched_ext` mechanism, applying the scheduling intents at the kernel level — including priority-based dispatching, dynamic time-slice tuning, and preemption control — without modifying the kernel itself.

### Why Gthulhu?

The default Linux kernel scheduler emphasizes fairness and cannot be optimized for the specific needs of individual applications. Cloud-native workloads — trading systems, big data analytics, ML training — all have different scheduling requirements. Gthulhu bridges this gap by:

1. **Making scheduling visible** — exposing kernel-level scheduling behavior as actionable metrics
2. **Making scaling smarter** — driving auto-scaling from actual scheduling pressure, not just CPU/memory averages
3. **Making scheduling tunable** (advanced) — allowing per-workload CPU scheduling policies on supported kernels

### Architecture

```
┌──────────────────────────────────────────────────────────────────────────────────┐
│                             Gthulhu Architecture                                 │
├──────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│   ┌──────────────┐        ┌──────────────────────┐        ┌─────────────────┐    │
│   │    User      │──────▶ │      Manager         │──────▶ │    MongoDB      │    │
│   │  (Web GUI)   │        │ (Central Management) │        │  (Persistence)  │    │
│   └──────────────┘        └──────────┬───────────┘        └─────────────────┘    │
│                                      │                                           │
│                      ┌───────────────┼───────────────┐                           │
│                      │               │               │                           │
│                      ▼               ▼               ▼                           │
│           ┌──────────────┐ ┌──────────────┐ ┌──────────────┐                     │
│           │Decision Maker│ │Decision Maker│ │Decision Maker│  (DaemonSet)        │
│           │   (Node 1)   │ │   (Node 2)   │ │   (Node N)   │                     │
│           └──────┬───────┘ └──────┬───────┘ └──────┬───────┘                     │
│                  │                │                │                              │
│          ┌───────┴───────┐ ┌──────┴───────┐ ┌─────┴────────┐                     │
│          ▼               ▼ ▼              ▼ ▼              ▼                      │
│   ┌────────────┐ ┌────────────┐ ┌────────────┐ ┌────────────┐                    │
│   │eBPF Metrics│ │ sched_ext  │ │eBPF Metrics│ │ sched_ext  │                    │
│   │ Collector  │ │ Scheduler* │ │ Collector  │ │ Scheduler* │                    │
│   └──────┬─────┘ └────────────┘ └──────┬─────┘ └────────────┘                    │
│          │                              │                                        │
│          ▼                              ▼               ┌─────────────────┐       │
│   ┌────────────────────────────────────────────┐        │      KEDA       │       │
│   │       Prometheus / Grafana Dashboards      │───────▶│  (Auto-Scaler)  │       │
│   └────────────────────────────────────────────┘        └─────────────────┘       │
│                                                                                  │
│   * sched_ext scheduler requires Linux 6.12+ (advanced feature)                  │
└──────────────────────────────────────────────────────────────────────────────────┘
```

**How it works:**

1. Users select Kubernetes workloads through the **Web GUI** (or `PodSchedulingMetrics` CRD) and define monitoring/scheduling policies.
2. The **Manager** persists configurations, queries pods via the Kubernetes API (Informer), and distributes intents to Decision Makers.
3. Each **Decision Maker** (deployed as a DaemonSet) runs on every node and attaches eBPF programs to kernel scheduling hooks to collect per-process metrics in real time.
4. Metrics are aggregated at the pod level and exported to **Prometheus**, enabling Grafana dashboards and **KEDA**-driven auto-scaling.
5. On nodes with Linux 6.12+ and `sched_ext` support, the advanced **custom scheduler** can be activated for priority-based dispatching and time-slice tuning.

**Key links:**
- Core scheduling framework: [qumun](https://github.com/Gthulhu/qumun) — build custom Linux schedulers in Go using eBPF/sched_ext, without modifying the kernel
- Deployment: [Helm chart](https://github.com/Gthulhu/chart/tree/main/gthulhu) for Kubernetes clusters
- Real-world use case: [Improving Network Performance with Custom eBPF-based Schedulers](https://free5gc.org/blog/20250726/index.en/)


### Meaning of the Name

The name Gthulhu is inspired by Cthulhu, a mythical creature known for its many tentacles. Just as tentacles can grasp and steer, Gthulhu symbolizes the ability to take the helm and navigate the complex world of modern distributed systems — much like how Kubernetes uses a ship’s wheel as its emblem.

The prefix “G” comes from Golang, the language at the core of this project, highlighting both its technical foundation and its developer-friendly design.

Underneath, Gthulhu runs on the qumun framework (qumun means “heart” in the Bunun language, an Indigenous people of Taiwan), reflecting the role of a scheduler as the beating heart of the operating system. This not only emphasizes its central importance in orchestrating workloads but also shares a piece of Taiwan’s Indigenous culture with the global open-source community.

## DEMO

Click the image below to see our DEMO on YouTube!
<a href="https://www.youtube.com/watch?v=MfU64idQcHg" target="_blank">
<img src="./assets/preview.png" alt="preview" width="300"/>
</a>

## Prerequisites

To build and run Gthulhu, ensure you have the following prerequisites installed on your system:

**Core (Metrics Collection & Monitoring):**
- Go 1.22+
- LLVM/Clang 17+
- libbpf
- Linux kernel with eBPF support (5.x+)

**Advanced (Custom CPU Scheduling):**
- Linux kernel 6.12+ with `sched_ext` support (`CONFIG_SCHED_CLASS_EXT`)

> **Note:** The eBPF metrics collection feature works on a wide range of kernels and does not require `sched_ext`. The custom CPU scheduling feature requires Linux 6.12+ which may not be available on all managed Kubernetes platforms (e.g., GKE currently does not support it).

See [Installation guide](https://gthulhu.org/installation/) for detailed installation instructions.

## Usage

### Setting Up Dependencies

First, clone the required dependencies:

```bash
make dep
git submodule init
git submodule sync
git submodule update
cd scx
cargo build --release -p scx_rustland
```

This will clone libbpf and the custom libbpfgo fork needed for the project.

### Linting the Code
To ensure code quality, run the linter:

```bash
make lint
```

### Building the Scheduler

Build the scheduler with:

```bash
make build
```

This compiles the BPF program, builds libbpf, generates the skeleton, and builds the Go application.

Cross-compilation for arm64 is supported by setting the `ARCH` variable:

```bash
make build ARCH=arm64
```

### Testing the Scheduler

To test the scheduler in a virtual environment using kernel v6.12:

```bash
make test
```

This uses `vng` (virtual kernel playground) to run the scheduler with the appropriate kernel version.

You can also test with a specific kernel version:

```bash
make test KERNEL_VERSION=6.12
```

#### Portability Testing

Gthulhu is automatically tested for portability across multiple Linux kernel versions (6.12+) through a daily scheduled GitHub Actions workflow. This ensures that the released packages remain compatible with newer kernel versions. The portability tests run against kernel versions 6.12, 6.13, 6.14, 6.15, 6.16, and 6.17.

### Launching Gthulhu by using schedkit

First, install `schedctl` from [schedkit](https://github.com/schedkit/schedctl) (created by @dottorblaster):
```sh
$ git clone https://github.com/schedkit/schedctl.git
$ cd schedctl
$ make install
```

Then, you can launch Gthulhu with:

```sh
$ sudo ./schedctl run gthulhu
Trying to pull ghcr.io/schedkit/gthulhu:latest...
Getting image source signatures
Copying blob sha256:a517a5a43837a7785dad62f579950a8abe4d1bd2ae5a096bda78150a1cc70c64
Copying blob sha256:2d35ebdb57d9971fea0cac1582aa78935adf8058b2cc32db163c98822e5dfa1b
Copying config sha256:85fb26cbfed25f79e321ef4e0a3e4e6fc7f01a957dbcee6aed815bdb93458136
Writing manifest to image destination
Storing signatures
Container ghcr.io/schedkit/gthulhu:latest started successfully
$ sudo podman ps -a
CONTAINER ID  IMAGE                            COMMAND     CREATED        STATUS            PORTS       NAMES
015d1b5fbe96  ghcr.io/schedkit/gthulhu:latest  /main       6 seconds ago  Up 6 seconds ago              gthulhu
```

### Running with Docker
To run the scheduler in a Docker container, you can either build locally or use the pre-built image from GitHub Container Registry:

**Using the pre-built image from GitHub Packages:**
```bash
docker run --privileged=true --pid host --rm ghcr.io/gthulhu/gthulhu:latest /gthulhu/main
```

**Building locally:**
```bash
make image
docker run --privileged=true --pid host --rm  127.0.0.1:25000/gthulhu:latest /gthulhu/main
```

### Debugging

If you need to inspect the BPF components, you can use:

```bash
sudo bpftool prog list            # List loaded BPF programs
sudo bpftool map list             # List BPF maps
sudo cat /sys/kernel/debug/tracing/trace_pipe # View BPF trace output
```

### Stress Testing by using `stress-ng`

```
stress-ng -c 20 --timeout 20s --metrics-brief
```

## Troubleshooting

See [Installation guide](https://gthulhu.org/installation/#troubleshooting).

## License

This software is distributed under the terms of the Apache License 2.0.

## Contributing

See [Contributing guide](https://gthulhu.org/contributing/).

## Community Resources

- [NotebookLM](https://notebooklm.google.com/notebook/89a6a260-3d54-4760-93a2-dcc06c6d8043): includes all of materials used in the project, including the pptx, design documents, and more.
- [GitHub Discussion](https://github.com/Gthulhu/Gthulhu/discussions): a place for community discussions, questions, and feature requests.

## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=Gthulhu/Gthulhu&type=Date)](https://www.star-history.com/#Gthulhu/Gthulhu&Date)

## Special Thanks

- [scx](https://github.com/sched-ext/scx)
- [libbpfgo](https://github.com/aquasecurity/libbpfgo)
