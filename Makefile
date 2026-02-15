# Gthulhu Scheduler Makefile
# 
# Usage:
#   make build              # Build for default architecture (x86_64)
#   make build ARCH=arm64   # Build for ARM64 architecture
#   make build-arm64        # Convenience target for ARM64 build
#   make test               # Test on x86_64
#   make test-arm64         # Test on ARM64
#   make image              # Build Docker image for x86_64
#   make image-arm64        # Build Docker image for ARM64
#

# Minimum required clang version
MIN_CLANG_VERSION = 17

# Auto-detect clang or use CLANG environment variable
CLANG ?= $(shell which clang 2>/dev/null)
ifeq ($(CLANG),)
    $(error clang not found. Please install clang $(MIN_CLANG_VERSION)+ or set CLANG environment variable)
endif

# Extract clang version
CLANG_VERSION := $(shell $(CLANG) --version | head -n1 | sed -E 's/.*version ([0-9]+).*/\1/')

# Validate clang version
ifeq ($(shell test $(CLANG_VERSION) -ge $(MIN_CLANG_VERSION); echo $$?),1)
    $(error clang version $(CLANG_VERSION) is too old. Minimum required: $(MIN_CLANG_VERSION))
endif

# Auto-detect clang resource directory for include paths
CLANG_RESOURCE_DIR := $(shell $(CLANG) --print-resource-dir)

$(info Using clang version $(CLANG_VERSION) at $(CLANG))
$(info Clang resource directory: $(CLANG_RESOURCE_DIR))

# Architecture configuration (default: x86_64, can override with ARCH=arm64)
ARCH ?= x86_64

# Architecture-specific settings
ifeq ($(ARCH),arm64)
    ARCH_DEFINE = -D__TARGET_ARCH_arm64
    ARCH_CPU_FLAGS = -mcpu=v3
    ARCH_SCHED_INCLUDE = -I scx/scheds/include/arch/aarch64
    ARCH_INCLUDE_DIR = aarch64-linux-gnu
    GOARCH_ENV = CGO_ENABLED=1 GOARCH=arm64
    CGO_CC = aarch64-linux-gnu-gcc
    LIBBPF_CC = aarch64-linux-gnu-gcc
else
    ARCH_DEFINE = -D__TARGET_ARCH_x86
    ARCH_CPU_FLAGS = -mcpu=v3
    ARCH_SCHED_INCLUDE = -I scx/scheds/include/arch/x86
    ARCH_INCLUDE_DIR = x86_64-linux-gnu
    GOARCH_ENV = 
    CGO_CC = clang
    LIBBPF_CC = gcc
endif

# Detect sudo availability (empty in Docker/root environments)
SUDO := $(shell command -v sudo 2>/dev/null)

OUTPUT = output
LIBBPF_SRC = $(abspath libbpf/src)
LIBBPF_OBJ = $(abspath $(OUTPUT)/libbpf.a)
LIBBPF_OBJDIR = $(abspath ./$(OUTPUT)/libbpf)
LIBBPF_DESTDIR = $(abspath ./$(OUTPUT))


TARGET = qumun/ebpf/main
BPF_TARGET = ${TARGET:=.bpf}
BPF_C = ${BPF_TARGET:=.c}
BPF_OBJ = ${BPF_C:.c=.o}

BASEDIR = $(abspath .)
LIBBPF_INCLUDE_UAPI = $(abspath ./libbpf/include/uapi)
CLANG_BPF_SYS_INCLUDES := `shell $(CLANG) -v -E - </dev/null 2>&1 | sed -n '/<...> search starts here:/,/End of search list./{ s| \(/.*\)|-idirafter \1|p }'`
CGOFLAG = $(GOARCH_ENV) CC=$(CGO_CC) CGO_CFLAGS="-I$(BASEDIR) -I$(BASEDIR)/qumun -I$(BASEDIR)/$(OUTPUT)" CGO_LDFLAGS="-lelf -lz $(LIBBPF_OBJ) -lzstd $(BASEDIR)/libwrapper.a"
STATIC=

.PHONY: build
build: clean $(BPF_OBJ) libbpf libbpf-uapi wrapper
	$(CGOFLAG) go build -ldflags "-w -s" main.go

# Build for ARM64 architecture
.PHONY: build-arm64
build-arm64:
	$(MAKE) build ARCH=arm64

# Build for x86_64 architecture (explicit)
.PHONY: build-x86_64
build-x86_64:
	$(MAKE) build ARCH=x86_64

.PHONY: lint
lint: build
	$(CGOFLAG) go vet -ldflags "-w -s $(STATIC)" main.go
	$(CGOFLAG) go vet -ldflags "-w -s $(STATIC)" ./internal/...
	$(CGOFLAG) go vet -ldflags "-w -s $(STATIC)" ./util/...

image: build
	docker build -t 127.0.0.1:32000/gthulhu:latest .

# Build ARM64 Docker image
.PHONY: image-arm64
image-arm64:
	$(MAKE) build-arm64
	docker build -t 127.0.0.1:32000/gthulhu:latest-arm64 .

# Default kernel version for testing
KERNEL_VERSION ?= 6.12.2

# Schtest configuration
SCHTEST_REPO = https://github.com/sched-ext/schtest.git
SCHTEST_DIR = schtest

test: build
	@echo "Running scheduler test for $(KERNEL_VERSION)..."
	@chmod +x scripts/test_scheduler.sh
	@vng -r v$(KERNEL_VERSION) -- bash scripts/test_scheduler.sh

# Test with ARM64 build
.PHONY: test-arm64
test-arm64:
	$(MAKE) build-arm64
	@echo "Running ARM64 scheduler test for $(KERNEL_VERSION)..."
	@chmod +x scripts/test_scheduler.sh
	@vng --arch arm64 -r v$(KERNEL_VERSION) -- bash scripts/test_scheduler.sh

# Schtest targets
.PHONY: schtest-dep
schtest-dep:
	@if [ ! -d "$(SCHTEST_DIR)" ]; then \
		echo "Cloning schtest repository..."; \
		git clone $(SCHTEST_REPO) $(SCHTEST_DIR); \
	fi
	@if [ -d "$(SCHTEST_DIR)" ]; then \
		cd $(SCHTEST_DIR) && git pull || true; \
	fi

.PHONY: schtest-build
schtest-build: schtest-dep
	@if [ -d "$(SCHTEST_DIR)" ]; then \
		echo "Building schtest..."; \
		cd $(SCHTEST_DIR) && \
		if [ -f "Cargo.toml" ]; then \
			echo "Building schtest debug version..."; \
			cargo build || echo "Warning: schtest debug build failed, continuing..."; \
			echo "Building schtest release version..."; \
			cargo build --release || echo "Warning: schtest release build failed, continuing..."; \
		elif [ -f "Makefile" ]; then \
			$(MAKE) || echo "Warning: schtest build failed, continuing..."; \
		elif [ -f "meson.build" ]; then \
			meson setup build --prefix ~ || echo "Warning: schtest meson setup failed, continuing..."; \
			meson compile -C build || echo "Warning: schtest meson compile failed, continuing..."; \
		fi; \
	fi

.PHONY: schtest
schtest: build schtest-build
	@echo "Running schtest for $(KERNEL_VERSION)..."
	@chmod +x scripts/test_schtest.sh
	@vng -r v$(KERNEL_VERSION) -- bash scripts/test_schtest.sh

.PHONY: libbpf-uapi
libbpf-uapi: $(LIBBPF_SRC)
	UAPIDIR=$(LIBBPF_DESTDIR) \
		$(MAKE) -C $(LIBBPF_SRC) install_uapi_headers

.PHONY: libbpf
libbpf: $(LIBBPF_SRC) $(wildcard $(LIBBPF_SRC)/*.[ch])
	$(MAKE) -C $(LIBBPF_SRC) clean
	CC="$(LIBBPF_CC)" CFLAGS="-g -O2 -Wall -fpie" \
	   $(MAKE) -C $(LIBBPF_SRC) \
		BUILD_STATIC_ONLY=1 \
		OBJDIR=$(LIBBPF_OBJDIR) \
		DESTDIR=$(LIBBPF_DESTDIR) \
		INCLUDEDIR= LIBDIR= UAPIDIR= install
	$(eval STATIC=-extldflags -static)

dep:
	git clone https://github.com/libbpf/libbpf.git && \
	cd libbpf && \
	git checkout 09b9e83 && \
	cd src && \
	make && \
	$(SUDO) make install
	git clone -b v7.6.0 --recursive https://github.com/libbpf/bpftool.git && \
	cd bpftool/src && make 


$(BPF_OBJ): %.o: %.c
	$(CLANG) \
		-O2 -g -Wall -target bpf \
		$(ARCH_DEFINE) $(ARCH_CPU_FLAGS) -mlittle-endian \
		-idirafter $(CLANG_RESOURCE_DIR)/include -idirafter /usr/local/include -idirafter /usr/include/$(ARCH_INCLUDE_DIR) -idirafter /usr/include \
		-I scx/scheds/vmlinux -I scx/build/libbpf/src/usr/include -I scx/build/libbpf/include/uapi -I scx/scheds/include $(ARCH_SCHED_INCLUDE) -I scx/scheds/include/bpf-compat -I scx/scheds/include/lib \
		-Wno-compare-distinct-pointer-types \
		-c $< -o $@

wrapper:
	bpftool/src/bpftool gen skeleton qumun/ebpf/main.bpf.o > main.skeleton.h
	$(CGO_CC) -g -O2 -Wall -fPIC -I ./ -c qumun/wrapper.c -o wrapper.o
	ar rcs libwrapper.a wrapper.o

clean:
	rm -rf output || true
	rm libwrapper.a || true
	rm *.skeleton.h || true
	rm *.ll *.o || true
	rm main || true