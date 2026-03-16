# Gthulhu Monitor — BPF object build only
# The Go monitor code is compiled into the main scheduler binary.
# This Dockerfile only builds the sched_monitor.bpf.o object file.
#
# To build the full Gthulhu binary (scheduler + monitor), use the root Dockerfile.

FROM golang:1.24-bookworm AS builder

RUN apt-get update && apt-get install -y --no-install-recommends \
    clang llvm libelf-dev bpftool linux-headers-generic make gcc \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /src/monitor

COPY monitor/bpf/ bpf/

# Generate vmlinux.h and compile BPF
RUN make bpf

# The BPF object is at /src/monitor/bpf/sched_monitor.bpf.o
# Copy it into the main build context as needed.
