# Makefile for GTP5G Operator

GO := go
CLANG := clang
ARCH := $(shell uname -m | sed 's/x86_64/x86/')
BPF_DIR := bpf
CMD_DIR := cmd
OUTPUT_DIR := bin

# BPF compilation flags
CFLAGS := -g -O2 -target bpf -D__TARGET_ARCH_$(ARCH)
INCLUDES := -I$(BPF_DIR) -I../libbpfgo/output

.PHONY: help build bpf go test clean fmt deps

help: ## 顯示幫助訊息
	@echo "使用方法: make [target]"
	@echo ""
	@echo "目標:"
	@echo "  build    🔨 編譯 eBPF 和 Go 程式"
	@echo "  bpf      🔨 只編譯 eBPF 程式"
	@echo "  go       🔨 只編譯 Go 程式"
	@echo "  test     🧪 執行測試"
	@echo "  clean    🧹 清理編譯檔案"
	@echo "  fmt      🎨 格式化程式碼"
	@echo "  deps     📦 安裝依賴"
	@echo "  help     ❓ 顯示此幫助訊息"

build: bpf go ## 編譯 eBPF + Go 程式
	@echo "✅ 建置完成"

bpf: ## 只編譯 eBPF 程式
	@echo "🔨 編譯 eBPF 程式..."
	@mkdir -p $(BPF_DIR)/output
	@if [ -f $(BPF_DIR)/gtp5g_operator.bpf.c ]; then \
		$(CLANG) $(CFLAGS) $(INCLUDES) -c $(BPF_DIR)/gtp5g_operator.bpf.c -o $(BPF_DIR)/output/gtp5g_operator.bpf.o; \
		echo "✅ eBPF 編譯完成: $(BPF_DIR)/output/gtp5g_operator.bpf.o"; \
	else \
		echo "⚠️  找不到 $(BPF_DIR)/gtp5g_operator.bpf.c，跳過編譯"; \
	fi

go: ## 只編譯 Go 程式
	@echo "🔨 編譯 Go 程式..."
	@mkdir -p $(OUTPUT_DIR)
	@if [ -f $(CMD_DIR)/main.go ]; then \
		$(GO) build -o $(OUTPUT_DIR)/gtp5g_operator $(CMD_DIR)/main.go; \
		echo "✅ Go 編譯完成: $(OUTPUT_DIR)/gtp5g_operator"; \
	else \
		echo "⚠️  找不到 $(CMD_DIR)/main.go，跳過編譯"; \
	fi

test: ## 執行測試
	@echo "🧪 執行測試..."
	$(GO) test -v ./...

clean: ## 清理編譯檔案
	@echo "🧹 清理中..."
	@rm -rf $(OUTPUT_DIR)
	@rm -rf $(BPF_DIR)/output
	@echo "✅ 清理完成"

fmt: ## 格式化程式碼
	@echo "🎨 格式化 Go 程式碼..."
	$(GO) fmt ./...

deps: ## 安裝依賴
	@echo "📦 安裝依賴..."
	$(GO) mod tidy
