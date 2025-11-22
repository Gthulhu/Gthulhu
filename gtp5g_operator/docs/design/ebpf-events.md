# GTP5G Operator — eBPF 事件設計說明

目的：定義要在 Kernel 層追蹤的 kfuncs、事件格式 (payload)，以及 user-space 的接收/上報流程，作為後續實作（eBPF C、Go loader、collector）之藍圖。

---

## 1. 背景與總覽

我們的目標是利用 eBPF 追蹤 free5gc/gtp5g kernel module 中處理 GTP-U 封包的關鍵函數（kfuncs/kprobes），收集執行上下文與封包 metadata，並把事件經由 ring buffer （或 perf buffer）傳到 User Space 的 Operator，最終批次上報到 API Server。

資料流：
Kernel (gtp5g kfunc) -> eBPF hook (entry/exit 或 trace) -> bpf_ringbuf_output -> User Space (Operator Collector) -> API Client -> API Server

---

## 2. 候選要追蹤的 kfuncs（優先順序）
（來源：free5gc/gtp5g codebase）

優先追蹤：
- gtp5g_handle_skb_ipv4
  - 為 inbound/downlink IPv4 GTP 處理入口（pktinfo/encap）。適合捕捉解封/匹配 PDR 的時機。
- gtp5g_dev_xmit
  - netdevice 的 transmit entry，適合監控出站/上行封包及 device transmit 路徑。

次要/補充：
- gtp5g_xmit_skb_ipv4
- gtp1u_udp_encap_recv
- gtp5g_fwd_skb_ipv4

為何選這些：
- 這些函數是 GTP 封包在 kernel 層處理的關鍵節點，在這裡能拿到 skb、IP 以及 PDR/FAR 等資訊，能直接反映封包流向、被 drop/forward/buffer 的決策。

---

## 3. 事件 (Event) schema（建議）

事件以二進位 struct 傳送到 ring buffer，User space 依序解析。使用固定對齊 (packed) layout，注意大小端與對齊。

建議欄位（Order matters, keep compact）：

C struct 範例（bpf-side）

```c
struct event_t {
    u64 ts_ns;        // Timestamp (ns)
    u32 cpu;          // cpu id
    u32 pid;          // task pid
    u32 tgid;         // thread group id
    char comm[16];    // comm/name (null-terminated or truncated)

    // Net / packet metadata
    u32 src_ip;       // IPv4 src
    u32 dst_ip;       // IPv4 dst
    u16 src_port;     // UDP/TCP port
    u16 dst_port;
    u32 pkt_len;      // skb->len

    // gtp5g-specific metadata (if available)
    u32 teid;         // TEID (0 if not present)
    u32 pdr_id;       // PDR id (if retrievable)

    u32 func_id;      // small enum for which kfunc triggered
    u64 duration_ns;  // (optional) time from entry->exit (0 if entry-only)
};
```

Notes:
- 如果需支援 IPv6，可擴充為 16-byte IPv6 地址或另外的版本欄位。
- `func_id` 用 enum 對應不同 kfunc，幫助 user space 判斷來源。
- `duration_ns` 需在 entry/exit pairs 或 TOCTOU 時取差值；若僅監控 entry，則設為 0。

---

## 4. 選用傳輸機制：Ring Buffer vs Perf Buffer

- ring buffer (libbpf 的 bpf_ringbuf_output)
  - 優點：API 更現代、效率高、零拷貝 (preferred)、適合高頻事件
  - 缺點：需要較新的 libbpf / kernel 支援 (已在現代 kernel 廣泛可用)

- perf events (BPF_PERF_OUTPUT)
  - 優點：傳統可用性好、跨平台成熟
  - 缺點：頻寬及效率通常不如 ringbuf，在頻繁事件下可能需要更大緩衝/CPU

建議：採用 ring buffer 作為主流實作；若 target 環境舊或需要兼容，可同時提供 perf path。

---

## 5. eBPF 程式的 attach 類型：kprobe vs kfunc vs tracepoint

- kprobe：通用、低版本可用，但受內核符號變動影響 (less stable)。
- kfunc（BPF_PROG_TYPE_TRACING / BPF_TRAMPOLINE / BPF_KFUNC_*）：能直接 attach 到 kernel 函式符號，語義更清楚、效率高，但需要內核與 libbpf 支援。
- tracepoints：若 target 函式有 tracepoint，使用 tracepoint 穩定，但 GTP5G 沒太多 tracepoint。

建議：若內核支援 `kfunc`（你環境為 6.12，支援程度高），優先使用 `kfunc` 或 `kprobe` 作 fallback。

---

## 6. 事件捕獲策略（Entry / Exit / Duration）

- Entry-only：在函數入點收集 timestamp & metadata -> 較簡單、對性能影響小。
- Entry + Exit：收集 entry ts、exit ts -> 可計算 duration，對性能測量有幫助，但更複雜 (需儲存內核 map state，或使用 bpf_get_current_pid_tgid 做 correlation)。

建議先實作 Entry-only，後續若需要 latency/duration 再加上 exit probe 用 stack map 或 per-cpu map 暫存 entry timestamp。

---

## 7. 事件速率、RingBuffer 設計

- ringbuffer_size：參考 `config/config.yaml` 預設 256KB，實測若事件頻繁（每秒數千/萬），應提升到 1MB 或更高。
- user-space 讀取：使用 libbpfgo 或 cilium/ebpf 的 ringbuf reader，採用批次消費與後端批次上報（例如每 100~1000 次或 1s）來減少 API 請求量。

---

## 8. User-space Payload 解碼與上報策略

- User-space 使用 Go，建議：libbpfgo (如果選 C skeleton) 或 cilium/ebpf（go-native）
- 解碼：定義 Go 對應的 struct（與 C struct 使用相同記憶體 layout），使用 binary.Read 或 reflect/unsafe 讀 raw bytes → struct。
- 上報：採用 batch queue，依 `report_interval`（config）與 max batch size 傳送；失敗重試與 backoff。
- 權限：要載入 eBPF 並 attach kernel probes 需要 root；替代方案是用 CAP_BPF/CAP_PERFMON capabilities 或使用容器中的 privileged mode。

---

## 9. 測試與驗證步驟（開發環境）

1. Unit / integration 測試：寫小型 Go 測試讀取 ring buffer (可以模擬 raw event bytes)。
2. 本地驗證：在受控環境（VM 或測試機）編譯並載入 BPF 程式，使用簡單的網路流量模擬（例如 iperf, ping, curl）來觸發 GTP5G 函式。檢查 ring buffer 的事件到達。
3. End-to-end：Operator 解析事件，將批次上報到 API Server，檢查 API Server 是否收到並正確紀錄。
4. 性能測試：測試高頻事件輸入並調整 ringbuffer_size、batch size 和上報頻率。

---

## 10. 權限、相依性與工具

- kernel: 建議 5.10+（更好：6.x）——你的環境 6.12 非常合適
- libbpf / clang: libbpf >= 0.5 / clang/LLVM 11+ 通常可用；使用更高版本（如 13/17）更佳。
- go: 建議 1.24.x
- 建議工具：libbpf (C-side), libbpfgo 或 cilium/ebpf (Go-side), bpftool, clang/llvm, bpftool/probes 用於 debug

---

## 11. 日後擴充（Roadmap）

- 支援 IPv6 和更多欄位（如 container/pod context, namespace id）
- 支援 entry+exit 跟踪來測量 duration & latency分佈
- 支援動態的 tracking function 列表（從 config 或 API 下發）
- 支援 eBPF map cleanup 與 graceful unload

---

## 12. 下一步（我將依你的要求執行）

我會：
1. 在 `bpf/` 建立初步的 eBPF skeleton (kprobe/kfunc attach) —— 先針對 `gtp5g_handle_skb_ipv4` 與 `gtp5g_dev_xmit`。
2. 在 `pkg/ebpf` 與 `cmd/` 實作 Go loader 與 ring buffer reader 的最小骨架。

在開始前，請確認：
- 是否同意先以 *Entry-only* 事件（不暫存 entry->exit timestamp）為 1.0 版本？
- 你偏好使用哪個 Go side eBPF library？（`libbpfgo` 或 `cilium/ebpf`）

我會等你確認後，再開始實作範例程式碼與測試。