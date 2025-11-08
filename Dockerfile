FROM ubuntu:25.04

RUN apt-get update
RUN apt-get install -y clang llvm vim libelf-dev \
    libpcap-dev gcc-multilib build-essential make sudo

WORKDIR /gthulhu

COPY main ./main
ENTRYPOINT ["bash"]

FROM ubuntu:25.04

# Combine all package installations into a single RUN layer for efficiency
# and to reduce the final image size.
RUN apt-get update && apt-get install -y \
    clang \
    llvm \
    vim \
    libelf-dev \
    libpcap-dev \
    gcc-multilib \
    build-essential \
    make \
    sudo \
    git \
    linux-headers-generic \
    && rm -rf /var/lib/apt/lists/*

# Clone the schtest repository for use in CI tests.
RUN git clone https://github.com/sched-ext/schtest.git /opt/schtest

WORKDIR /gthulhu

COPY main ./main
ENTRYPOINT ["bash"]