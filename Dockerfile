# Multi-stage Dockerfile for multi-architecture builds
# Builder stage: always runs on the build host's native platform to avoid
# slow QEMU emulation (especially for the Rust/cargo scx_rustland build).
# Cross-compilation is used when TARGETARCH differs from BUILDARCH.
FROM --platform=$BUILDPLATFORM ubuntu:25.04 AS builder

# Install build dependencies
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    clang \
    llvm \
    libelf-dev \
    libpcap-dev \
    libseccomp-dev \
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

# Install cross-compilation toolchain for arm64.
# This is in a separate layer keyed on TARGETARCH so Docker does not
# share the cached layer between the amd64 and arm64 target builds.
# Ubuntu arm64 packages live on ports.ubuntu.com (not archive.ubuntu.com),
# so we restrict the existing sources to amd64 and add a ports source for arm64.
ARG TARGETARCH
RUN if [ "$TARGETARCH" = "arm64" ]; then \
        dpkg --add-architecture arm64 && \
        sed -i "s/^Types: deb$/Types: deb\nArchitectures: amd64/" /etc/apt/sources.list.d/ubuntu.sources && \
        . /etc/os-release && \
        printf 'Types: deb\nURIs: http://ports.ubuntu.com/ubuntu-ports/\nSuites: %s %s-updates\nComponents: main universe restricted multiverse\nArchitectures: arm64\nSigned-By: /usr/share/keyrings/ubuntu-archive-keyring.gpg\n' \
            "$VERSION_CODENAME" "$VERSION_CODENAME" \
            > /etc/apt/sources.list.d/arm64-cross.sources && \
        apt-get update && \
        apt-get install -y --no-install-recommends \
        gcc-aarch64-linux-gnu \
        libelf-dev:arm64 \
        libzstd-dev:arm64 \
        zlib1g-dev:arm64 \
        && rm -rf /var/lib/apt/lists/*; \
    fi

# Install Go for the BUILD platform (runs natively, cross-compiles via GOARCH)
ARG BUILDARCH
ARG GO_VERSION=1.22.10
RUN wget -q https://go.dev/dl/go${GO_VERSION}.linux-${BUILDARCH}.tar.gz && \
    tar -C /usr/local -xzf go${GO_VERSION}.linux-${BUILDARCH}.tar.gz && \
    rm go${GO_VERSION}.linux-${BUILDARCH}.tar.gz

ENV PATH="/usr/local/go/bin:${PATH}"

# Install Rust/Cargo for building scx_rustland (runs natively, no QEMU needed)
RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y
ENV PATH="/root/.cargo/bin:${PATH}"

WORKDIR /build

# Copy source files
COPY . .

# Build: scx_rustland runs natively on the build host (fast), then
# cross-compile the Go binary for the target architecture via make build.
RUN set -e; \
    case "$TARGETARCH" in \
        "arm64") \
            export BUILD_ARCH=arm64 ;; \
        "amd64") \
            export BUILD_ARCH=x86_64 ;; \
        *) \
            echo "Unsupported target arch: $TARGETARCH" >&2; \
            exit 1 ;; \
    esac && \
    make dep && \
    cd scx && \
    cargo build --release -p scx_rustland && \
    cd .. && \
    cd libbpfgo && \
    unset ARCH && make && \
    cd .. && \
    make build ARCH=${BUILD_ARCH}

# Runtime stage: uses the actual target platform
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