# Monitor BPF build targets
# Requires: clang/LLVM 15+, bpftool, kernel headers with BTF

CLANG      ?= clang
BPFTOOL    ?= bpftool
ARCH       := $(shell uname -m | sed 's/x86_64/x86/' | sed 's/aarch64/arm64/')

VMLINUX_H  ?= vmlinux.h
BPF_SRC     = sched_monitor.bpf.c
BPF_OBJ     = sched_monitor.bpf.o

.PHONY: all clean vmlinux

all: $(BPF_OBJ)

# Generate vmlinux.h from the running kernel's BTF
vmlinux: $(VMLINUX_H)
$(VMLINUX_H):
	$(BPFTOOL) btf dump file /sys/kernel/btf/vmlinux format c > $@

# Compile BPF object
$(BPF_OBJ): $(BPF_SRC) sched_monitor.h $(VMLINUX_H)
	$(CLANG) -g -O2 -target bpf -D__TARGET_ARCH_$(ARCH) \
		-I. \
		-c $(BPF_SRC) -o $@

clean:
	rm -f $(BPF_OBJ) $(VMLINUX_H)
