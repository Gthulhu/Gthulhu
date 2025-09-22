FROM ubuntu:25.04

RUN apt-get update
RUN apt-get install -y clang llvm vim libelf-dev \
    libpcap-dev gcc-multilib build-essential make sudo

WORKDIR /gthulhu

COPY main ./main
ENTRYPOINT ["bash"]