# Multi-stage Dockerfile for multi-architecture builds
# Builder stage
FROM ubuntu:25.04 AS builder

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
ARG TARGETARCH
ARG GO_VERSION=1.22.10
RUN wget -q https://go.dev/dl/go${GO_VERSION}.linux-${TARGETARCH}.tar.gz && \
    tar -C /usr/local -xzf go${GO_VERSION}.linux-${TARGETARCH}.tar.gz && \
    rm go${GO_VERSION}.linux-${TARGETARCH}.tar.gz

ENV PATH="/usr/local/go/bin:${PATH}"

WORKDIR /build

# Copy source files
COPY . .

# Build dependencies and binary
ARG TARGETPLATFORM
RUN set -e; \
    case "$TARGETPLATFORM" in \
        "linux/arm64") \
            export ARCH=arm64 ;; \
        "linux/amd64") \
            export ARCH=x86_64 ;; \
        *) \
            echo "Unsupported platform: $TARGETPLATFORM" >&2; \
            exit 1 ;; \
    esac && \
    export BUILD_ARCH=${ARCH} && \
    make dep && \
    cd libbpfgo && \
    unset ARCH && make && \
    cd .. && \
    make build ARCH=${BUILD_ARCH}

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