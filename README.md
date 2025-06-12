# Gthulhu Project

![](./assets/logo.png)

## Overview

Gthulhu optimizes cloud-native workloads using the Linux Scheduler Extension for different application scenarios.

The scheduler consists of two main components:
1. A BPF component that implements low-level sched-ext functionalities
2. A user-space scheduler written in Go with [scx_goland_core](https://github.com/Gthulhu/scx_goland_core) that implements the actual scheduling policy

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

## Special Thanks

- [scx](https://github.com/sched-ext/scx)
- [libbpfgo](https://github.com/aquasecurity/libbpfgo)