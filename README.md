# Gthulhu Project

<a href="https://landscape.cncf.io/?item=provisioning--automation-configuration--gthulhu" target="_blank"><img src="https://img.shields.io/badge/CNCF%20Landscape-5699C6?style=for-the-badge&logo=cncf&label=cncf" alt="cncf landscape" /></a>

[![Go](https://github.com/Gthulhu/Gthulhu/actions/workflows/go.yaml/badge.svg)](https://github.com/Gthulhu/Gthulhu/actions/workflows/go.yaml)
[![Portability](https://github.com/Gthulhu/Gthulhu/actions/workflows/portability.yaml/badge.svg)](https://github.com/Gthulhu/Gthulhu/actions/workflows/portability.yaml)
[![Go Report Card](https://goreportcard.com/badge/github.com/Gthulhu/Gthulhu)](https://goreportcard.com/report/github.com/Gthulhu/Gthulhu)

<img src="./assets/logo.png" alt="logo" width="300"/>

## Overview

Gthulhu is a next-generation scheduler designed for the cloud-native ecosystem, built with Golang and powered by the [qumun](https://github.com/Gthulhu/qumun) framework.
- The qumun provides a series of API and abstractions to facilitate the development of custom Linux kernel schedulers using eBPF and Go.
- Gthulhu implements a virtual runtime (vruntime) based scheduling policy, inspired by the concepts of proportional share scheduling and the needs of modern cloud-native applications.
- Gthulhu supports preemptive multitasking, latency-sensitive task prioritization, and dynamic time slice adjustment to optimize CPU utilization and responsiveness. User can define what kind of tasks should be prioritized by invoking HTTP based APIs, provided by the [api server](https://github.com/Gthulhu/api).

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

To build and run Gthulhu, ensure you have the following prerequisites installed on your system
- Go 1.22+
- LLVM/Clang 17+
- libbpf
- Linux kernel 6.12+ with sched_ext support

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

Cross-compilation for arm64 is supported by setting the `ARCH` variable:

```bash
make build ARCH=arm64
```

### Testing the Scheduler

To test the scheduler in a virtual environment using kernel v6.12.2:

```bash
make test
```

This uses `vng` (virtual kernel playground) to run the scheduler with the appropriate kernel version.

You can also test with a specific kernel version:

```bash
make test KERNEL_VERSION=6.13
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

This software is distributed under the terms of the GNU General Public License version 2.

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
