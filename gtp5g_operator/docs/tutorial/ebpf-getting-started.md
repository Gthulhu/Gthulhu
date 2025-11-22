# eBPF 基礎教學與最小範例（libbpfgo / ring buffer）

目的：一步一步帶你從零開始，理解核心概念、環境需求，並示範如何編寫一個最小的 eBPF 程式（kprobe + ring buffer）與 Go loader（libbpfgo）來讀事件。

> 先備要求：你需要有 root 權限或以能載入 eBPF 程式的帳號 (CAP_BPF / CAP_PERFMON)

---

## 1) 工具與相依性安裝（Ubuntu 範例）

必裝項目：clang/llvm、libbpf、libbpf-dev、make、git、Go、libbpfgo（Go module），bpftool

範例命令：

```bash
# Ubuntu (範例)
sudo apt update
sudo apt install -y clang llvm libelf-dev libbpf-dev libbpfcc-dev build-essential git bpftool lld
# install go if needed (1.24.x)
# 使用 libbpfgo 時，請先安裝 libbpf (系統包) 再下載 libbpfgo module
```

更多 libbpf / libbpfgo 安裝指南請參考官方文件。

---

## 2) 檔案清單（本練習）

- bpf/gtp5g_toy.bpf.c  — eBPF 程式，使用 kprobe attach 到 `gtp5g_handle_skb_ipv4` 與 `gtp5g_dev_xmit`，把簡短事件寫入 ring buffer。
- cmd/toy_loader/main.go — Go 程式 (libbpfgo)，載入 .o、初始化 ring buffer 並列印事件。
- docs/tutorial/ebpf-getting-started.md — 本教學檔案。

---

## 3) 編譯與執行步驟

1. 編譯 eBPF object（需要 clang 與正確的 include 路徑）：

```bash
# 先到 repo 根目錄
cd /home/ubuntu/Gthulhu/gtp5g_operator
clang -O2 -target bpf -c bpf/gtp5g_toy.bpf.c -o bpf/gtp5g_toy.bpf.o
```

2. 下載或安裝 libbpfgo（僅要在 Go module 中引用）：

```bash
# 在 cmd/toy_loader 目錄
cd cmd/toy_loader
go mod init github.com/Gthulhu/gtp5g_operator/cmd/toy_loader
go get github.com/aquasecurity/libbpfgo
```

3. 編譯 Go loader：

```bash
go build -o ../../bin/toy_loader main.go
```

4. 執行（**需要 root**）：

```bash
sudo ./bin/toy_loader ../bpf/gtp5g_toy.bpf.o
# 然後觸發相關 kernel 函數（例如在系統內部執行 gtp5g 動作或測試程式）
```

---

## 4) 測試與排錯

- 如果載入 .o 時出現符號不符（unsatisfied kprobe），代表該函數在當前 kernel 或模組不可見（可能是不同符號名稱或模組未載入）。
- 使用 `bpftool prog`、`bpftool map` 與 `dmesg` 檢查錯誤訊息。
- 開發時先測試通用函數（例如 kprobe 到 `__x64_sys_getpid` 或 `do_sys_open`）來驗證完整 pipeline。

---

## 5) 安全與清理

- 停止 loader (Ctrl-C) 後，libbpfgo 會卸載 BPF 程式。若發現遺留，你可以使用 `bpftool prog` 與 `bpftool map` 列出並清理。

---

## 6) 接下來要做什麼

- 我可以幫你：
  1. 編譯上面提供的 bpf 程式並示範如何在你的環境運行。 (需要 root 及 clang 等工具)
  2. 進一步將事件欄位擴充為你在 `ebpf-events.md` 設計的完整 schema，並實作到 `bpf/` 與 `cmd/`。

請告訴我你想先做哪一件（例如：a) 我幫你在本機編譯並嘗試載入；b) 你先閱讀並讓我解釋每一行程式碼；c) 我們先實作 entry-only 針對實際 gtp5g 函數）