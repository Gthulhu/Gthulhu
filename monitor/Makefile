# Gthulhu Monitor Makefile
# Builds the eBPF object for the scheduling event monitor.
# The Go code is compiled as part of the main scheduler binary.

.PHONY: all bpf clean

all: bpf

bpf:
	$(MAKE) -C bpf all

clean:
	$(MAKE) -C bpf clean
