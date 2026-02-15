# Multi-stage Dockerfile for multi-architecture builds
# Builder stage
FROM --platform=$BUILDPLATFORM ubuntu:25.04 AS builder

# Install build dependencies
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    clang \
    llvm \
    libelf-dev \
    libpcap-dev \
    build-essential \
    make \
    git \
    ca-certificates \
    wget \
    curl \
    pkg-config \
    libzstd-dev \
    zlib1g-dev \
    && rm -rf /var/lib/apt/lists/*

# Install Go
ARG GO_VERSION=1.22.10
ARG TARGETOS
ARG TARGETARCH
RUN wget -q https://go.dev/dl/go${GO_VERSION}.${TARGETOS:-linux}-${TARGETARCH:-amd64}.tar.gz && \
    tar -C /usr/local -xzf go${GO_VERSION}.${TARGETOS:-linux}-${TARGETARCH:-amd64}.tar.gz && \
    rm go${GO_VERSION}.${TARGETOS:-linux}-${TARGETARCH:-amd64}.tar.gz

ENV PATH="/usr/local/go/bin:${PATH}"

# Install Rust/Cargo for building scx_rustland
RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y
ENV PATH="/root/.cargo/bin:${PATH}"

WORKDIR /build

# Copy source files
COPY . .

# Build dependencies and binary
# Note: We need to handle architecture-specific builds
ARG TARGETPLATFORM
RUN if [ "$TARGETPLATFORM" = "linux/arm64" ]; then \
        export ARCH=arm64; \
    else \
        export ARCH=x86_64; \
    fi && \
    make dep && \
    git submodule init && \
    git submodule sync && \
    git submodule update && \
    cd scx && \
    cargo build --release -p scx_rustland && \
    cd .. && \
    cd libbpfgo && \
    make && \
    cd .. && \
    make build ARCH=${ARCH}

# Runtime stage
FROM ubuntu:25.04

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    clang \
    llvm \
    vim \
    libelf-dev \
    libpcap-dev \
    build-essential \
    make \
    sudo \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /gthulhu

# Copy the built binary from builder stage
COPY --from=builder /build/main ./main

ENTRYPOINT ["bash"]