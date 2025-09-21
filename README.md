# Gthulhu Project
[![Go](https://github.com/Gthulhu/Gthulhu/actions/workflows/go.yaml/badge.svg)](https://github.com/Gthulhu/Gthulhu/actions/workflows/go.yaml)

<img src="./assets/logo.png" alt="logo" width="300"/>

## Overview

Gthulhu is a next-generation scheduler designed for the cloud-native ecosystem, built with Golang and powered by the qumun framework.

The name Gthulhu is inspired by Cthulhu, a mythical creature known for its many tentacles. Just as tentacles can grasp and steer, Gthulhu symbolizes the ability to take the helm and navigate the complex world of modern distributed systems — much like how Kubernetes uses a ship’s wheel as its emblem.

The prefix “G” comes from Golang, the language at the core of this project, highlighting both its technical foundation and its developer-friendly design.

Underneath, Gthulhu runs on the qumun framework (qumun means “heart” in the Bunun language, an Indigenous people of Taiwan), reflecting the role of a scheduler as the beating heart of the operating system. This not only emphasizes its central importance in orchestrating workloads but also shares a piece of Taiwan’s Indigenous culture with the global open-source community.

## DEMO

Click the image below to see our DEMO on YouTube!

[![IMAGE ALT TEXT HERE](./assets/preview.png)](https://www.youtube.com/watch?v=MfU64idQcHg)

## Key Features

- Virtual runtime (vruntime) based scheduling
- Latency-sensitive task prioritization
- Dynamic time slice adjustment
- CPU topology aware task placement
- Automatic idle CPU selection

## How It Works

The scheduling policy is based on virtual runtime:
- Each task receives a time slice of execution (slice_ns)
- The actual execution time is adjusted based on task's static priority (weight)
- Tasks are dispatched from lowest to highest vruntime
- Latency-sensitive tasks receive priority boost based on voluntary context switches

## Building

Prerequisites:
- Go 1.22+
- LLVM/Clang 17+
- libbpf
- Linux kernel 6.12+ with sched_ext support

## Usage

### Setting Up Dependencies

First, clone the required dependencies:

```bash
make dep
git submodule init
git submodule sync
git submodule update
cd scx
meson setup build --prefix ~
meson compile -C build
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

### Testing the Scheduler

To test the scheduler in a virtual environment using kernel v6.12.2:

```bash
make test
```

This uses `vng` (virtual kernel playground) to run the scheduler with the appropriate kernel version.

### Running in Production

To run the scheduler on your system:

```bash
sudo ./main
```

The scheduler will run until terminated with Ctrl+C (SIGINT) or SIGTERM.

### Running with Docker
To run the scheduler in a Docker container, you can use the provided Dockerfile:

```bash
make image
docker run --privileged=true --pid host --rm  gthulhu:latest /gthulhu/main
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

## License

This software is distributed under the terms of the GNU General Public License version 2.

## Contributing

Contributions are welcome! Please feel free to submit pull requests or open issues for bugs and feature requests.

## Community Resources

- [NotebookLM](https://notebooklm.google.com/notebook/89a6a260-3d54-4760-93a2-dcc06c6d8043): includes all of materials used in the project, including the pptx, design documents, and more.
- [GitHub Discussion](https://github.com/Gthulhu/Gthulhu/discussions): a place for community discussions, questions, and feature requests.

## Special Thanks

- [scx](https://github.com/sched-ext/scx)
- [libbpfgo](https://github.com/aquasecurity/libbpfgo)
