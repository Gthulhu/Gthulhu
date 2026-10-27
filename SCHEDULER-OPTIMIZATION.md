# Gthulhu 排程最佳化：cgo 熱路徑與 adaptive priority

## 1. 結論與適用範圍

這次最佳化分成兩個問題：

1. **降低單次 Go/C 呼叫成本**：保留 qumun 已有的 `asmcgocall` bridge，
   避免熱路徑反覆經過標準 cgo 與不必要的配置、序列化。
2. **降低 task 的完整排程延遲**：修正 BPF forward progress、ringbuffer
   發布順序，以及 Go 與 BPF 的工作交接。單次呼叫變快不代表 task 能及時執行。

最後，Gthulhu 的 Go 排程模式預設啟用 adaptive wakeup、直接消費 queued
ringbuffer、完成後交還 CPU，以及 minimum backend slice。不再需要
`-adaptive-wakeup`；該旗標已移除。`-usersched-preempt` 仍是獨立、預設關閉的實驗選項。
Kernel mode 不套用這組 Go loop 行為；qumun 的低階啟用 API 仍保留給其他使用者選擇。

使用 repository 的 monitor-enabled 設定、不帶上述兩個旗標，三次
`adaptive_priority` 的最後一個實際量測 ratio 為 **0.22、0.19、0.21**，
都低於要求的 **0.50**。這是指定主機上的結果，不是所有負載的延遲保證。

## 2. 原始架構與問題定位

```text
task becomes runnable
  -> BPF queued ringbuffer
  -> 原本：libbpfgo poll goroutine -> Go channel -> pinned scheduler loop
  -> Go plugin 選 task、計算 deadline/slice、選 CPU
  -> dispatched user ringbuffer
  -> BPF dispatch callback
  -> DSQ -> task running
```

重要程式：

- [Go dispatch loop](./internal/scheduler/dispatch_loop.go)
- [Go adaptive policy](./internal/scheduler/adaptive.go)
- [BPF callbacks](./qumun/ebpf/main.bpf.c)
- [C hotpath wrappers](./qumun/hotpath.c)
- [Go hotpath bridge](./qumun/goland_core/hotpath.go)
- [asmcgocall bridge](./qumun/internal/schedbridge/bridge.go)

`SelectCPU` 的 wall time 包含搶佔、等待 CPU、kernel 執行及函式本身，
不能直接當作函式的純運算成本。先前樣本曾出現 wall time 約 12 ms，
thread CPU 卻只有數十 µs；這表示主要時間沒有在該 thread 上執行，
但不能僅憑這個差值判定是哪一個 queue 或 runtime 事件造成。

此外，`watchdog failed to check in` 與某個應用 task 的 runnable stall
不是完全相同的診斷：前者表示 kernel watchdog 工作沒有及時更新時間戳。
不能由這段訊息直接推論某個 Go/C 呼叫執行了五秒。

## 3. cgo 加速：保留快路徑，清楚界定成本與限制

### 3.1 Bridge 的作用

qumun 既有的 direct bridge 用 `//go:linkname` 呼叫 `runtime.asmcgocall`，
切換到目前 M 的 g0 stack，執行單一 `void *` 參數的 C ABI wrapper。
它略過標準 `runtime.cgocall` 的 syscall/runtime bookkeeping。

這不是「消除 kernel syscall」。例如 `SelectCPU` 仍執行一次
`BPF_PROG_TEST_RUN`；最佳化的是呼叫框架、context 序列化與 Go allocations。
CPU 選擇保留既有負值 fallback 與單次 `EINTR` 處理，不在訊號中斷後重複執行程式。

目前熱路徑包括：

- `SelectCPU`：快取 C function pointer、傳遞固定 ABI 的參數。
- `DispatchTask`：nonblocking reserve/submit，不使用 reserve-blocking。
- 直接 dequeue：一次 `ring_buffer__consume_n(..., 1)`，不回呼 Go。
- BSS scalar writes：透過小型 wrapper。
- `GetNrQueued` / `GetNrScheduled`：對對齊 BSS 位址直接做 Go atomic load，
  不需要為了讀值呼叫 C。

### 3.2 必須維持的限制

- direct `asmcgocall` 不會透過標準 syscall bookkeeping 釋放 P。
  **不可把可能長時間等待的 poll/read 放進這條路徑。**
- C 不得回呼 Go、保留 Go 參數指標，或假設可以任意進行 Go runtime 操作。
- 參數只包含 scalar 與 C-owned addresses；保留 `runtime.KeepAlive`。
- Go/C 的大小、對齊、field offsets 在編譯時驗證。
- `runtime.LockOSThread` 固定的是 goroutine 所用的 OS thread/M，
  不是固定 P，也不是禁止搶佔。
- `//go:linkname` 依賴 Go runtime 的非公開介面；升級 toolchain 必須重新驗證。
  此專案連結需要 `-checklinkname=0`，不是對一般程式的通用建議。
- 若確實需要等待 syscall，應採用有 runtime bookkeeping 的途徑；
  qumun 另有 `CallSyscall`，但這次沒有把 `SelectCPU` 改回該路徑。

### 3.3 Microbenchmark 證據

2026-10-27，在 Intel Xeon Platinum 8272CL、linux/amd64 上，
同一組 scalar-load benchmark 跑三次：

| 路徑 | ns/op 範圍 | Go allocations |
|---|---:|---:|
| Standard cgo | 219.6–228.1 | 0 |
| `asmcgocall` | 20.72–21.28 | 0 |
| Go atomic | 1.130–1.369 | 0 |

這組小操作約有十倍呼叫成本差距，但**不能把這個倍數套到整個 scheduler、
真實 BPF syscall，或 adaptive priority ratio**。執行時主機已有 scheduler，
並非隔離、固定頻率的 benchmark 環境。

`BenchmarkSelectCPU` 另用同一個小型 syscall BPF program 比較
`ebpf.Program.Run()` 與 direct wrapper。它用於區分框架成本，
不是完整 CPU-selection policy 或 Rust/Go 語言效能的對照。

## 4. 先修 correctness，再討論 latency

### 4.1 Scheduler-process 排除條件一致

先前 `running` 只排除 scheduler 的 process leader，`stopping` 卻排除
同 TGID 的所有 thread，造成 running 計數不對稱。
現在 user-space mode 的 running/stopping/tick 對 scheduler-process threads
採一致排除條件，並在 scheduler thread running 時更新 heartbeat。

Tick 也不能用未初始化的應用 task timestamp 檢查 scheduler threads，
否則可能產生不必要的 watchdog 搶佔與大量訊息。

### 4.2 沒有成功 dispatch，不能提前 return

有 pending work 不代表目前 CPU 能從 `SCHED_DSQ` 取得 scheduler thread。
只有實際取得 task 才能結束該次 dispatch；否則必須繼續處理其他 DSQ。
同時修正 scheduler dispatch batch 的邊界。

### 4.3 Count-before-publication

queued counter 必須在 `bpf_ringbuf_submit` **之前**增加。
否則 Go 可能已取得 record，卻看到尚未增加的 counter，讓 plugin 丟掉 task；
之後才增加的 counter 無法把 task 找回。

### 4.4 確保 binary 真正包含新 BPF

Root build 先前生成的 skeleton 路徑與 qumun include 的路徑不同，
可能編譯了新 BPF，最後卻連到舊 skeleton。

現在 root `wrapper` target 依賴 BPF object，生成
`qumun/main.skeleton.h`，把 wrapper 與 hotpath 一起 archive。
Root build 使用 `go build -a`，避免 external C archive/header 變更漏掉 Go cache
失效；驗證時也檢查當前 BPF ELF bytes 完整存在於最終 executable。

## 5. adaptive_priority 真正在量什麼

測試來源為本機 schtest checkout `ae6450a` 的
`src/cases/latency.rs`，不是只看 histogram 中央顯示值：

- 兩組 parent/worker 經 semaphore 交接，都綁到同一 logical CPU。
- Slow worker 忙等約 10 ms。
- Fast worker 忙等約 100 µs，再 sleep 約 9.9 ms。
- 比較的是 forward semaphore wake latency 的 **p90**：

```text
ratio = p90(fast semaphore wake) / p90(slow semaphore wake)
通過要求：ratio < 0.50
```

Spinner 使用 wall-clock elapsed，不是純 thread CPU 計時。
測試程式的文字註解與實作數值有出入時，這裡以實際程式為準。

`converge` 的 5/10 秒是每個 measurement batch，不是整個測試的總時間。
該版本在 batch 超時後會回傳 0，所以**不能只用 PASS 或最後回傳值當證據**。
本次另外確認 >=5-second batch 與最後輸出的實際非零 ratio 都低於門檻，
沒有改測試 threshold、workload 或 convergence 邏輯。

## 6. 最後保留的改善

### 6.1 完整 burst，不是最後一小段

長工作可能被切成許多短 slice。如果只看最後一段執行時間，
會把 CPU-intensive worker 誤認成短工作。

BPF 在 runnable transition 保存上一個完整 burst，再重設 current accumulator。
Wakeup record 的既有 `exec_runtime` 欄位回報 previous full burst；
reenqueue 仍回報 current burst。沒有改 queued/dispatched wire layout。

Go 只有在 wakeup、previous burst 與 final segment 都已知、為正且 <=1 ms 時，
才要求 adaptive 搶佔。未知或長 burst 不直接視為互動工作。

### 6.2 Deadline bonus 與一次性搶佔

Go 對 qualifying task 的非零 dispatch deadline 減去一個 default slice，
下限為 1；明確 priority 的 0 保持不變。
Bonus 在 plugin pop task 後套用，**不改變 plugin heap 內的排序**。

BPF 重新檢查 affinity 與 target CPU，僅在 ordinary current task 仍在該 CPU
執行、且 deadline 較晚時 kick。Scheduler-process threads 保持受保護。
單 CPU task 在 idle selection 回傳 ANY 時，Go 保留該 task 的 CPU hint。

原先增加的「current task 至少執行 100 µs」門檻被移除：
trace 顯示快速 worker 已符合條件、deadline 也更早，卻被該門檻拒絕；
這個一次性請求沒有 retry，結果繼續等到後續排程事件。
帶診斷的拒絕組 wake-to-running p90 約 1.8 ms，成功組約 191 µs。
這是受 instrumentation 影響的分類證據，不是 production latency 保證。

### 6.3 移除 goroutine/channel 中轉

現在 pinned Go loop 直接 nonblocking consume queued ringbuffer。
C callback 只驗證固定長度並複製到 C-owned memory，不再把 record 推回 Go channel。
錯誤傳回 Go loop；ready check 暫存一筆，避免重複探測遺失 record。

這減少一次 poll goroutine 到 scheduler goroutine 的喚醒交接，
也讓此路徑不再經過原本的 receive-then-requeue channel wait。
舊 channel 路徑的 requeue deadlock reproduction 仍保留；
**這不代表所有 qumun 低階 channel consumers 的舊問題都已修復。**

### 6.4 先發布 Go 決策，再排 scheduler

BPF dispatch callback 現在**先 drain dispatched ringbuffer**，
再考慮安排 pending scheduler work。

原順序可能先選 scheduler thread 並 return，讓已經算好的 Go 決策
仍留在 ringbuffer。尤其 Go 完成後 yield，若下一次又優先排 scheduler，
就不是有效的應用 task 交接。

### 6.5 交還 CPU 與 backend slice

Go pool 清空、完成通知後，使用 Linux `sched_yield` 交還 CPU；
空 queue 也用此途徑，而非在 fast C call 內 blocking poll。
這不是 Go `runtime.Gosched`：目的是進入 Linux 排程交接。

Go mode 的 BPF fallback/scheduler slice 採 `scheduler.slice_ns_min`
（測試設定 1 ms），而 `slice_ns_default`（20 ms）仍作為 Go fairness/deadline budget。
這避免沒有新工作時以較長 fallback slice 延續執行。

## 7. 與 scx_rustland 的比較及實驗紀錄

本機 rustland 本來就有：

- direct ringbuffer consumption；
- `notify_complete` 更新 pending count 後 Linux thread yield；
- BPF drain 完成決策先於安排 scheduler；
- backend 使用 minimum slice；
- 預設啟用 BPF idle direct dispatch。

所以 rustland 沒有同名旗標，並不表示它缺少基礎交接機制。
反過來，也不能只由「整包開啟可通過」推論額外 deadline bonus 或每一種搶佔都必要。

| 階段 | 實際 ratio / 結果 |
|---|---|
| 早期 adaptive experiment | 多次約 10–13，失敗 |
| 移除 100 µs gate，其他舊機制保留 | 約 0.9，仍失敗 |
| 只有 direct consumer、仍採 blocking wait 的實驗 | 約 0.82，仍失敗 |
| 改用 yield，但尚未搭配最後的交接組合 | 約 24.48，失敗 |
| drain-first + direct consumer + yield + minimum backend slice | 三次 0.24、0.22、0.21 |
| 同組合、monitor-enabled 設定 | 三次 0.21、0.22、0.22 |
| 成為預設後，不帶 adaptive/usersched-preempt 旗標 | 三次 0.22、0.19、0.21 |

最後一組 >=5-second batches 分別約 0.21、0.26、0.24，均 <0.50。
最後預設模式的 fairness metric 為 0.98，通過其 >=0.60 要求。

這些是逐步實驗，不是完整 factorial A/B，不能把改善精確分攤給每個因素。
本機 rustland 曾量到約 0.13–0.27，但第一次 bounded capture 在整個測試完成前
就停止，**不能宣稱那次 rustland 完整測試已通過**。
它預設開啟 idle bypass，與本次 `builtin_idle: false` 也不是等價條件。

Gthulhu 仍保留既有 kthread、priority、congestion 等 BPF 路徑；
「沒有新增短 task bypass」不表示所有歷史 kernel dispatch 路徑都被移除。

## 8. 重現與驗證

環境：Linux `6.14.0-37-generic`、4 logical CPUs、Go `1.25.0`、
clang 20；`gthulhu` mode、default/min slices 20/1 ms、debug/API/early_processing/
builtin_idle 關閉。最後三次使用 repository 的 monitor-enabled configuration。

先完成 repository 既有依賴安裝；以下從 Gthulhu 根目錄執行：

```bash
make wrapper

export GOTOOLCHAIN=go1.25.0 CC=clang
export CGO_CFLAGS="-I$PWD -I$PWD/qumun -I$PWD/output"
export CGO_LDFLAGS="-lelf -lz $PWD/output/libbpf.a -lzstd $PWD/libwrapper.a"

go build -a -ldflags='-w -s -checklinkname=0' -o main main.go
go test -race -ldflags='-checklinkname=0' -count=1 \
  ./internal/scheduler/... \
  github.com/Gthulhu/qumun/goland_core \
  github.com/Gthulhu/qumun/internal/schedbridge \
  github.com/Gthulhu/qumun/util
python3 qumun/scripts/test_bpf_progress.py

go test -ldflags='-checklinkname=0' \
  github.com/Gthulhu/qumun/internal/schedbridge \
  -run '^$' -bench '^BenchmarkLoad' -benchmem -benchtime=100ms -count=3
```

結果：build、race suites、production BPF callback mock regressions 與 scalar
benchmark 通過；`qumun/util` 無 test files。Callback harness 不取代 kernel verifier。
另外已用 root 執行 real-kernel ring tests，驗證 FIFO、ready cache、malformed record、
invalid/empty ring、取消、standard-cgo bypass、CPU selection 與 ring-full error，
均通過。一般非 root 測試可能因 BPF 權限而 skip，不能當成 privileged coverage。

執行 scheduler 與測試時使用不同終端：

```bash
# Terminal 1；確認沒有其他 scheduler attach，測完用 SIGINT 停止。
sudo ./main scheduler -config config/config.yaml

# Terminal 2；使用既有 schtest checkout/build。
sudo schtest/target/debug/schtest --filter adaptive_priority
sudo schtest/target/debug/schtest --filter fairness
```

上述預設指令已實測，不需要 `-adaptive-wakeup` 或 `-usersched-preempt`。
記錄每個 measurement batch 的 ratio、總退出狀態、設定與 binary 版本，
不要只保留 PASS。不要同時 attach 兩個 scheduler。

## 9. 診斷工具與後續限制

- Gthulhu `-trace-scheduler-loop`：每階段 wall time、active stage 與 histogram。
- `-trace-scheduler-cpu`：成對 thread CPU/wall 資訊，會增加計時 syscall 成本。
- qumun standalone `-measure-latency`：SelectCPU 與 BPF enqueue-to-dispatch-receive
  histogram；它不是 wakeup-to-running 指標。
- Histogram percentile 是 bucket upper bound，不是精確 percentile；
  不同 population 的 p99 不能相減當作某一段的 p99。
- Perf sched 與 Go runtime trace 必須先校正 clock domain，再做逐事件關聯。
- 診斷、BPF printk 與 tracing 都會改變負載；最終 ratio 使用關閉診斷的結果。
- 公平性測試通過不等於所有 throughput、power、topology 或 workload 都沒有退化；
  本次沒有完成完整跨主機、跨 kernel、跨架構測試，也未量化 yield 的 idle power 成本。
- 本次保留完整 adaptive policy；尚未證明移除 bonus/應用搶佔後依然可通過。
  這應是獨立、可重現的後續簡化實驗，而不是根據單次成功直接推論。
