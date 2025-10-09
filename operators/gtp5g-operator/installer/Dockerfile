FROM ubuntu:22.04

# Install build dependencies
RUN apt-get update && apt-get install -y \
    build-essential \
    git \
    linux-headers-generic \
    kmod \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Copy installer script
COPY install.sh /usr/local/bin/install.sh
RUN chmod +x /usr/local/bin/install.sh

WORKDIR /workspace

ENTRYPOINT ["/usr/local/bin/install.sh"]
