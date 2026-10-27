# Gthulhu 排程器最佳化實錄：從 `asmcgocall` 熱路徑到通過 `adaptive_priority` 測試

這份文件記錄了我們近期對 Gthulhu（基於 Linux `sched_ext` 的 eBPF + Go 混合排程器）進行的一系列底層最佳化。故事從一個看似矛盾的現象開始：底層 `qumun` 換上 `asmcgocall` 加速 C 呼叫之後，反而頻繁撞上 `watchdog failed to check in for 5.001s` 的 runnable task stall；而在解決卡死問題後，`schtest` 的 `adaptive_priority` 延遲測試又拿到了 `9.66` 這種遠高於及格線（`< 0.50`）的分數。

一路排查下來，我們發現真正的瓶頸往往不在單次跨語言呼叫本身，而在於 **BPF 與 Go user-space 之間的資料發布順序、佇列消費模型，以及排程器交還 CPU 的時機**。在補齊這些機制後，Gthulhu 在 Go user-space 模式下不需要額外實驗旗標，就能穩定以 **0.19～0.22** 的成績通過 `adaptive_priority` 測試，表現與 `scx_rustland`（**0.16**）處於同一水準。

---

## 一、問題起點：為什麼要改用 `asmcgocall`？又為什麼之後會撞上卡死？

> **設計初衷補充：為什麼我們最初決定在熱路徑換上 `asmcgocall`、`runtime.LockOSThread()`，並拿掉 `runtime.GOMAXPROCS(1)`？**
>
> 在 Go 的 GMP 排程模型下，標準的 `cgo` 呼叫（`runtime.cgocall`）每次進入 C 之前都會呼叫 `entersyscall`。如果系統判定這個呼叫稍微花了一點時間，或者在 `GOMAXPROCS(1)` 的限制下只有唯一一個 `P`，Go runtime 很容易把這個 `P` 從正在執行 cgo 的 `M`（OS thread）手上剝奪，轉交給另一個新的或閒置的 `M` 去跑其他 goroutine（例如 ringbuffer polling、metrics 或 GC 背景工作）。
>
> 等到原本的 C 呼叫結束、執行 `exitsyscall` 準備回到 Go 時，如果拿不到 `P`，排程器 goroutine 就會被迫停下來等、甚至在不同 `M` 之間被換手。這會造成一個對 kernel scheduler 非常致命的現象：**明明只是單次排程迴圈（Drain -> SelectCPU -> Dispatch），卻在每次跨 C 呼叫時不斷交出 `P`、喚醒其他 `M` 與 goroutine 穿插執行，整趟排程被切得支離破碎**。
>
> 因此，我們的最佳化核心脈絡是：
> 1. **在排程主迴圈加上 `runtime.LockOSThread()`**（[`internal/scheduler/dispatch_loop.go`](./internal/scheduler/dispatch_loop.go)）：將負責排程決策的 goroutine 死死綁在同一個固定的 OS thread（`M`）上，避免它在阻塞與恢復之間漂移到別的執行緒。
> 2. **移除 `main.go` 裡的 `runtime.GOMAXPROCS(1)`**（[`main.go`](./main.go)）：不再把整個 Go 行程硬鎖在單一 `P` 上，讓監控、API、GC 等其他 goroutine 有自己的 `P` 可以跑，不必跟核心排程迴圈搶同一個 `P`。
> 3. **熱路徑改用 `runtime.asmcgocall`**（[`qumun/internal/schedbridge/bridge.go`](./qumun/internal/schedbridge/bridge.go)）：直接切換到當前 `M` 的 `g0` stack 執行短平快的 C 函式，**完全不經過 `entersyscall` / `exitsyscall`，也不會把當前的 `P` 讓給新的 `M`**，確保從取任務、選 CPU 到派發的整趟排程 loop 都在同一個 goroutine、同一個 `M` 與同一個 `P` 上一氣呵成。

在混合架構的排程器裡，每一筆非直接派發的任務都要走過這趟旅程：

```text
Task 變成 runnable
  -> BPF 寫入 queued ringbuffer
  -> Go scheduler 讀取任務、由 plugin 計算 deadline 與 time slice、呼叫 BPF 挑選 CPU
  -> Go 寫入 dispatched user ringbuffer
  -> BPF 在 ops.dispatch() 取出決策、塞進 DSQ 並讓任務上 CPU 執行
```

這條路徑每秒會跑成千上萬次，因此 `qumun` 透過 `//go:linkname` 直接呼叫 `runtime.asmcgocall`（見 [`qumun/internal/schedbridge/bridge.go`](./qumun/internal/schedbridge/bridge.go)），跳過標準 `runtime.cgocall` 的 `entersyscall` / `exitsyscall` 與 runtime 記帳開銷。

當系統出現 `runnable task stall (watchdog failed to check in for 5.001s)` 時，直覺很容易懷疑是不是 `asmcgocall` 繞過 runtime 導致某個呼叫卡死。但如果冒然退回標準 cgo，熱路徑會白白損失十倍的呼叫速度。我們先在排程迴圈加上階段計時與活動狀態追蹤（[`internal/scheduler/loop_trace.go`](./internal/scheduler/loop_trace.go)），才看清兩個關鍵事實：

1. **Watchdog 超時不等於某個 Go 函式跑了 5 秒**：`sched_ext` 的 watchdog 是在檢查系統中是否有 runnable task 太久沒被排上 CPU，或者排程器太久沒推進。
2. **長尾延遲大多是 Off-CPU 時間**：我們利用 `CLOCK_THREAD_CPUTIME_ID` 同步測量 `SelectCPU` 的 Wall time 與 Thread CPU time，發現當 `SelectCPU` 偶爾出現 4～12 ms 的耗時，該執行緒實際花在 CPU 上的時間只有 **4～48 µs**。換句話說，不是 `asmcgocall` 慢，而是排程器執行緒自己被搶佔、或在等待佇列交接時沒有被及時喚醒。

### `asmcgocall` 的實測效益與使用邊界

在同一台測試機（Intel Xeon Platinum 8272CL, Linux 6.14, Go 1.25.0）上，針對單純的 64-bit 數值讀取跑 microbenchmark，三種路徑的差距非常明顯：

| 呼叫方式 | 耗時 (`ns/op`) | 記憶體配置 (`allocs/op`) | 說明 |
| :--- | :---: | :---: | :--- |
| 標準 `cgo` | 219.6 – 228.1 ns | 0 | 每次進出都要走完整的 Go syscall 狀態切換與記帳 |
| `asmcgocall` (`schedbridge.Call`) | 20.72 – 21.28 ns | 0 | 直接切到目前 M 的 `g0` stack 執行 C wrapper，快約 10 倍 |
| Go `atomic.LoadUint64` | 1.13 – 1.37 ns | 0 | 直接讀取對齊後的 BPF skeleton BSS 映射記憶體 |

因此，**保留 `asmcgocall` 是完全合理的決定**。像是 `SelectCPU`（執行一次 `BPF_PROG_TEST_RUN`）、`DispatchTask`（非阻塞寫入 `user_ring_buffer`）以及更新 BSS 計數器，全部維持走 `schedbridge.Call`；而純讀取的 `GetNrQueued` / `GetNrScheduled` 則直接對 BSS 指標做 Go atomic load，連 C 都不必進。

不過使用 `asmcgocall` 必須嚴格遵守幾條鐵律：
- **絕對不能阻塞**：因為沒有經過 `entersyscall`，Go runtime 不會把當前的 `P` 讓給其他 goroutine。任何可能休眠的操作（例如 `ring_buffer__poll` 或阻塞式 reserve）都不能放進 `asmcgocall`。
- **零配置與零回呼**：C wrapper 只接收一個指向堆疊結構的 `void *`，裡面只能放純數值或 C 自己管理的指標，不能在 C 裡面回呼 Go 函式，也要在 Go 端用 `runtime.KeepAlive` 確保物件生命週期。
- **編譯期 ABI 防呆**：在 [`qumun/goland_core/hotpath.go`](./qumun/goland_core/hotpath.go) 中，我們對所有跨界傳遞的 struct 大小、對齊與欄位位移都加上了編譯期斷言，避免 C 與 Go 結構對不上而默默寫壞記憶體。

---

## 二、先把地基補牢：解決 Task Stall 的四個關鍵 Bug

確認 `asmcgocall` 無罪後，我們順著 BPF 與 Go 的狀態機往下挖，找出了真正讓任務卡住或頻繁觸發警告的四個底層問題（見 [`qumun/ebpf/main.bpf.c`](./qumun/ebpf/main.bpf.c) 與 [`Makefile`](./Makefile)）：

### 1. BPF `running` / `stopping` / `tick` 的排程器執行緒過濾不一致
當我們移除了 `GOMAXPROCS(1)` 並把主迴圈 `LockOSThread()` 之後，Go runtime 更常讓同一個 TGID 下的多個 `M`（OS thread）同時上 CPU 分擔工作，這也剛好暴露出原本 BPF 過濾條件潛藏的不對稱：原本 BPF 在 `goland_stopping` 會排除整個排程器程序（`is_belong_usersched_task`，比對 TGID），但在 `goland_running` 與 `goland_tick` 卻只排除了主執行緒（比對 `usersched_pid`）。
這導致兩個後果：
- 非主執行緒的 Go runtime worker 每次上下 CPU，只有 `running` 把 `nr_running` 加一，`stopping` 卻不減，使 `nr_running` 無限膨脹。
- 這些 worker 執行緒沒有寫入 `tctx->start_ts`，卻在 `goland_tick` 被拿去跟 `scx_bpf_now()` 相減，誤觸 500ms 的 `max_time_watchdog` 狂發搶佔與 `printk`。

將 `running`、`stopping` 與 `tick` 統一改為以 TGID 排除所有排程器執行緒，並在排程器執行緒上 CPU 時更新 `usersched_last_run_at` 心跳，這類假警報與計數失衡就徹底消失了。

### 2. `goland_dispatch` 沒拿到排程器任務卻提早 Return
原本在 `goland_dispatch` 中，只要看到 `usersched_has_pending_tasks()` 為真，呼叫完 `scx_bpf_dsq_move_to_local(SCHED_DSQ, 0)` 就會直接 `return`。問題是，如果當下排程器執行緒正在別顆 CPU 上跑、或是因為 CPU affinity 無法搬過來，`SCHED_DSQ` 其實什麼都沒搬出來。這時提早 return 會導致這顆 CPU 連底下的 per-CPU DSQ 和 `SHARED_DSQ` 都不去消費，眼睜睜看著可執行的任務餓死。修正後，只有在真的從 `SCHED_DSQ` 搬出至少一筆任務時才 return，否則繼續往下消費一般任務佇列。

### 3. Ringbuffer 發布與 `nr_queued` 計數的競爭（Lost Wakeup）
在 [`queue_task_to_userspace`](./qumun/ebpf/main.bpf.c) 中，原本的寫法是先呼叫 `bpf_ringbuf_submit(task, 0)`，下一行才執行 `__sync_fetch_and_add(&nr_queued, 1)`。
當 Go 迴圈剛好在跑時，它可能在 `submit` 完成的瞬間就從 ringbuffer 讀出這筆任務，接著檢查 `GetNrQueued()` 卻讀到還沒加一的 `0`。Plugin 的 `drainQueuedTask` 看到 `count == GetNrQueued()`（`0 == 0`）便立刻結束迴圈，把剛剛讀出來的任務直接丟棄；等幾奈秒後 BPF 把 `nr_queued` 加一，佇列裡已經沒有東西了。
把 `__sync_fetch_and_add(&nr_queued, 1)` 移到 `bpf_ringbuf_submit` **之前**（並由 [`internal/scheduler/queue_publication_test.go`](./internal/scheduler/queue_publication_test.go) 測試保護），這顆會餓死 `containerd-shim` 等行程的地雷才被拆掉。

### 4. Makefile Skeleton 路徑錯置與 Go Cache
排查過程中我們一度發現改了 BPF 程式碼卻完全沒生效。追查 [`Makefile`](./Makefile) 才發現，根目錄的 `make wrapper` 把生成的 skeleton 輸出到根目錄的 `main.skeleton.h`，但 `qumun/wrapper.h` 引入的卻是 `qumun/main.skeleton.h`（一份陳舊的檔案），而且 Go build cache 不會主動偵測外部 `libwrapper.a` 的變動。修正輸出路徑並在打包時加上 `go build -a` 後，每次編譯出來的執行檔才真正嵌入了最新的 BPF ELF。

---

## 三、為什麼 `adaptive_priority` 一開始過不了？

解決穩定度問題後，我們面對的是 `schtest` 的 `adaptive_priority` 測試。當時跑出來的結果是：

```text
converge: iters=370, elapsed=5.00, metric=9.66
FAILED (Failed to achieve target: got 9.66, expected 0.50)
```

要壓低這個數字，得先看懂 [`schtest/src/cases/latency.rs`](./schtest/src/cases/latency.rs) 到底在測什麼：
- 測試會建立兩個行程（Slow 與 Fast），各自有兩個執行緒透過 futex semaphore 互相喚醒，而且**全部綁定在同一顆邏輯 CPU（Core 0 的第一個 Hyperthread）上競爭**。
- **Slow worker** 每次拿到 semaphore 就死命 busy spin **10 ms**。
- **Fast worker** 每次拿到 semaphore 只 busy spin **100 µs**，接著就 `sleep` **9.9 ms**。
- 測試收集單向 semaphore 的喚醒延遲分佈，計算：
  $$\text{metric} = \frac{\text{p90}(\text{Fast wake latency})}{\text{p90}(\text{Slow wake latency})}$$
  必須讓 Fast 的 p90 喚醒延遲不到 Slow 的一半（$\text{metric} < 0.50$）才算通過。

換句話說，當 Fast worker 睡完 9.9 ms 醒來時，同一顆 CPU 幾乎一定正被 Slow worker 霸佔著。如果排程器不能讓剛醒來的短任務及時插隊並搶下這顆 CPU，Fast worker 就得排在 10 ms 的 Slow worker 後面乾等，p90 隨便都會衝上 1～2 ms，算出來的 ratio 自然高達 9～12。

### 第一階段修正：為什麼加了搶佔，Ratio 還是停在 0.9？

我們在維持「每個任務都經過 Go 決策」的前提下，先補上了針對短任務喚醒的優先處理（[`internal/scheduler/adaptive.go`](./internal/scheduler/adaptive.go) 與 [`qumun/ebpf/main.bpf.c`](./qumun/ebpf/main.bpf.c)）：

1. **保存完整的上一輪執行時間（Full Previous Burst）**：長任務可能被切成好幾段短 slice。如果只看最後一次 `stop_ts - start_ts`，Slow worker 很容易在剛被切斷時偽裝成短任務。因此我們在 BPF 的 `goland_runnable`（任務剛從睡眠醒來時）把累積的 `exec_runtime` 存進 `previous_burst_runtime` 並歸零；當這次入隊帶有 `SCX_ENQ_WAKEUP` 時，透過既有的 `exec_runtime` 欄位把「上一次醒著總共跑了多久」傳給 Go，不需修改通訊協定結構。
2. **短任務識別與 Deadline Bonus**：在 Go 端，只有當任務是 wakeup、且上一輪總執行時間與最後一段執行時間都 $\le 1\text{ ms}$ 時，才將其視為互動式短任務，將它的派發 deadline 往前扣掉一個 `sliceNsDefault`，並帶上 `SCX_ENQ_PREEMPT` 請求；同時若單核綁定任務在 `SelectCPU` 拿到 `RL_CPU_ANY`，保留它原本所屬的 CPU 編號。
3. **移除無重試的 100 µs 保護門檻**：我們透過 BPF trace 逐筆分析 Fast worker（PID `2063598`）的喚醒過程，抓出了一個關鍵盲點——原本我們在 BPF 的 `preempt_later_deadline` 設了一個「正在跑的任務若還沒跑滿 100 µs 就不搶佔」的保護，結果 **Fast worker 有高達 267 次喚醒剛好撞上這個保護而被拒絕搶佔**。因為這次搶佔請求是一次性的，被拒絕後沒有任何重試機制，Fast worker 的 p90 立刻從 **191 µs** 暴增到 **1,818 µs**！把這個沒有重試的 100 µs 門檻拿掉（只保護排程器自身執行緒）之後，`adaptive_priority` 的 metric 瞬間從 **9.66 降到了 0.88～0.95**。

但 `0.9` 還是大於 `0.50`。這就引出了一個核心問題：**為什麼 `scx_rustland` 沒有寫這麼複雜的短任務特判，卻能輕鬆跑出 `0.16`？**

---

## 四、對照 `scx_rustland`：真正卡住最後一哩路的四個基礎機制差距

我們直接把 `scx_rustland` 與 Gthulhu 的執行軌跡及原始碼攤開來比對，發現兩者的差距根本不在「給短任務多少 bonus」，而是 Gthulhu 的 user-space 迴圈比 `scx_rustland` 多了好幾層無謂的轉手與延遲：

| 排程環節 | `scx_rustland` 的做法 | Gthulhu 原本的做法 | 造成的影響 |
| :--- | :--- | :--- | :--- |
| **1. 讀取 `queued` ringbuffer** | 主迴圈直接呼叫 `ring_buffer__consume_n(..., 1)` 同步取單筆任務 | `libbpfgo` 開一個背景 goroutine 跑 `ring_buffer__poll(10ms)`，在 C callback 裡轉成 `[]byte` 塞進 Go channel，主迴圈再從 channel 讀 | 每筆任務多一次 C-to-Go callback、記憶體複製，以及 **跨 goroutine 的 channel 喚醒延遲** |
| **2. BPF `dispatch` 的處理順序** | **先 drain `dispatched` user ringbuffer** 把算好的任務發出去，再去 `SCHED_DSQ` 搬排程器執行緒 | **先去 `SCHED_DSQ` 搬排程器執行緒並直接 return**，不先 drain `dispatched` | Go 明明已經把任務算好放進 ringbuffer 了，BPF 卻一直優先排 Go 上來跑，讓算好的任務卡在 ringbuffer 裡下不去 |
| **3. 派發完畢後的 CPU 交接** | `notify_complete` 更新 `nr_scheduled` 後，立刻呼叫 `std::thread::yield_now()`（`sched_yield`） | 只把 `nr_scheduled` 設為 0，沒有向 Linux kernel 交還 CPU，或者卡在 channel 等待 | 在單核競爭測試下，Go 排程器若不主動呼叫 `sched_yield` 觸發 kernel 重新進 `ops.dispatch()`，剛派發出去的 Fast worker 根本拿不到同一顆 CPU |
| **4. BPF Backend 預設 Time Slice** | 初始化時將 `slice_ns_min`（1 ms）傳給 BPF backend 作為預設 slice，20 ms 只留在 user-space 算 deadline | BPF backend 的 `default_slice` 維持 20 ms | 當佇列暫時為空、BPF 為當前任務續命（replenish slice）或排程器自己執行時，20 ms 的時間片太粗，拉長了切換週期 |

找到這四個結構性差異後，我們對 Gthulhu 與 `qumun` 做了對應的手術：

1. **Pinned Loop 直接非阻塞讀取 Ringbuffer**（[`qumun/hotpath.c`](./qumun/hotpath.c)、[`qumun/goland_core/hotpath.go`](./qumun/goland_core/hotpath.go)）：
   新增 `qumun_queue_open` 與 `c_dequeue_task_wrapper`，讓綁定 OS thread 的 Go 排程迴圈直接透過 `asmcgocall` 呼叫 `ring_buffer__consume_n(queue->ring, 1)`。因為 `consume_n` 是完全非阻塞的，C callback 只是把 72 bytes 的 `queued_task_ctx` 複製到 C 緩衝區就返回，既符合 `asmcgocall` 的安全規範，又徹底砍掉了背景 poll goroutine 與 Go channel 的中轉開銷。
2. **`goland_dispatch` 改為先 Drain 已完成的決策**（[`qumun/ebpf/main.bpf.c`](./qumun/ebpf/main.bpf.c)）：
   在 user-space 模式下進入 `goland_dispatch` 時，第一步先執行 `bpf_user_ringbuf_drain(&dispatched, ...)`，確保 Go 已經算好的派發決策（包含搶佔 kick）第一時間生效，接著才檢查是否需要排入 `SCHED_DSQ`。
3. **派發完成與空佇列時呼叫 `SYS_SCHED_YIELD`**（[`qumun/goland_core/task.go`](./qumun/goland_core/task.go)、[`internal/scheduler/dispatch_loop.go`](./internal/scheduler/dispatch_loop.go)）：
   當 Go 迴圈把 task pool 清空並呼叫 `NotifyComplete(0)` 後，或是佇列暫時沒有新任務時，直接執行 `unix.Syscall(unix.SYS_SCHED_YIELD, 0, 0, 0)`（注意不能用只讓出 Go goroutine 的 `runtime.Gosched()`，必須是讓出 Linux OS thread 的 `sched_yield`），把 CPU 立刻交還給 kernel 去跑剛放進 DSQ 的應用任務。
4. **分離 User-space 預算與 BPF Backend Slice**（[`internal/scheduler/scheduler.go`](./internal/scheduler/scheduler.go)）：
   原本 BPF 端的 `rodata.default_slice` 固定維持 20 ms，導致 BPF 直接派發、空佇列續命（replenish slice）與排程器執行緒自身都拿到 20 ms 的粗時間片。修改後，我們對齊 `scx_rustland` 的做法：讓 Go 端繼續使用 `slice_ns_default`（20 ms）計算 vruntime credit 與 deadline，並在啟動時呼叫 `bpfModule.SetDefaultSlice(sliceNsMin)`，將 BPF 端的 `default_slice` 改設為 `slice_ns_min`（1 ms）。

---

## 五、實驗數據與對照結果

以下是在同一台測試環境（Linux `6.14.0-37-generic`、4 Logical CPUs）下，各個演進階段與 `scx_rustland` 的實際測試紀錄。

### 1. 各階段演進對照總表

| 測試配置 / 演進階段 | `adaptive_priority` 最終 Metric（需 `< 0.50`） | 測試結果 | 關鍵原因分析 |
| :--- | :---: | :---: | :--- |
| **修改前：Gthulhu 原始版本** | **`5.70` – `9.66`** | **FAILED** | 無喚醒搶佔、channel 中轉延遲、Fast 喚醒中心值高達 ~275 µs（Slow ~195 µs） |
| 加入短任務分類 + 100 µs 保護門檻 | `10.18` – `12.82` | FAILED | 100 µs 門檻拒絕了 267 次 Fast worker 搶佔且無重試，p90 高達 1.8 ms |
| 移除 100 µs 門檻（仍走舊 channel 與舊 dispatch 順序） | `0.88`、`0.92`、`1.00` | FAILED | 毫秒級卡頓消失（Fast 與 Slow 均約 99 µs），但轉手延遲使兩者拉不開差距 |
| 單獨換上 Direct Ringbuffer + `sched_yield`（BPF 未先 drain） | `24.48` | FAILED | Go `sched_yield` 讓出 CPU 後，BPF 還沒 drain 決策又把 Go 排回來空轉 |
| **修改後：Gthulhu 完整修正（預設開啟，`builtin_idle=false`）** | **`0.22`、`0.19`、`0.21`** | **PASS** | 直接讀取 + BPF 先 drain + `sched_yield` + 1ms backend slice 完整配合 |
| **修改後：Gthulhu 完整修正（預設開啟，`builtin_idle=true`）** | **`0.20`**（5s batch 為 `0.22`） | **PASS** | 與 `scx_rustland` 條件對齊，Fast 喚醒中心值壓到 **~13.4 µs** |
| **對照組：`scx_rustland`（預設 `builtin_idle=true`）** | **`0.16`**（5s batch 為 `0.16`） | **PASS** | Rust user-space 排程基準線，Fast 喚醒中心值約 **~11.2 µs** |

> 補充：在開啟預設最佳化後，我們也執行了 `schtest --filter fairness`，Gthulhu 以 **0.98** 的成績穩定通過（門檻為 `>= 0.60`），確認提升短任務喚醒優先順序並未犧牲多執行緒間的 CPU 公平性。

### 2. 修改前的 Gthulhu 實測輸出（Before Optimization）

在尚未導入本次最佳化前，執行 `sudo schtest/target/debug/schtest --filter adaptive_priority` 的輸出如下（Fast 喚醒延遲中心值高達 `274.843 µs`，最差甚至到 `8.12 ms`，反而比 Slow 的 `195.443 µs` 還慢，最終以 `metric=5.70` 失敗）：

```text
running 1 test
test adaptive_priority ... 
converge: min_time=5.00, max_time=10.00, threshold=0.50
Fast semaphore wake estimates:
0ns | 0ns ▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁| 0ns
Slow semaphore wake estimates:
259.118µs | 259.118µs ▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁| 259.118µs
converge: iters=1, elapsed=0.01, metric=0.00
Fast semaphore wake estimates:
76.423µs |▁▅▆▇ 208.272µs █████████████▇▇▇▇▇▇▇▇▇▇▇▇▇▇▇▇▇▇▇▇▇▇▇▆▆▆▆▆▆▆▆▆▆▆▆▆▆▆▆▆▆▆▆▅▅▅▄▄▄▃▂▁| 2.093504ms
Slow semaphore wake estimates:
70.741µs |▁▅▆▆▆▆▆▆▆▇▇▇▇▇▇▇▇████ 250.269µs ███▇▇▇▇▇▆▆▆▆▆▆▅▅▅▅▅▅▅▅▅▅▅▄▄▄▄▄▄▄▄▄▄▄▃▃▃▃▃▂▂▂▂▁▁▁| 659.967µs
converge: iters=100, elapsed=1.37, metric=5.20
Fast semaphore wake estimates:
64.307µs |▁▇ 274.843µs ███▇▇▇▇▇▇▆▆▆▆▆▅▄▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▁▁▁▁▁▁▁▁▁▁▁| 8.125281ms
Slow semaphore wake estimates:
66.592µs |▁▅▆▇█ 195.443µs █▇▇▆▆▆▆▅▅▅▅▅▅▅▅▅▅▅▅▄▄▄▄▄▄▄▄▄▄▄▄▃▃▃▃▃▃▃▃▃▃▃▃▃▂▂▂▂▂▂▂▂▂▂▂▂▂▁▁▁▁▁▁▁| 1.952516ms
converge: iters=366, elapsed=5.07, metric=5.70
FAILED

failures:

---- adaptive_priority ----
Failed to achieve target: got 5.70, expected 0.50


failures:
    adaptive_priority

test result: FAILED. 0 passed; 1 failed; 0 ignored; 0 measured; 4 filtered out; finished in 6.45s
```

### 3. `scx_rustland` 實測輸出（Baseline）

執行 `sudo schtest/target/debug/schtest --filter adaptive_priority`：

```text
running 1 test
test adaptive_priority ... 
converge: min_time=5.00, max_time=10.00, threshold=0.50
Fast semaphore wake estimates:
1.995787ms | 1.995787ms ▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁| 1.995787ms
Slow semaphore wake estimates:
44.848µs | 44.848µs ▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁| 44.848µs
converge: iters=1, elapsed=0.01, metric=44.50
Fast semaphore wake estimates:
10.397µs | 10.933µs ▁▆▆▅▅▅▅▅▅▅▅▅▅▅▅▅▅▅▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▃▃▃▃▃▃▃▃▃▃▃▃▃▃▂▂▂▂▂▂▂▂▂▂▂▂▂▂▁▁▁▁▁▁▁▁| 1.818591ms
Slow semaphore wake estimates:
10.711µs |▁▆▆▆▆▆▆▆▆▆▆▆▆▆▆▆▇▇▇▇▇▇▇▇▇▇▇▇▇▇▇▇▇▇████████ 131.19µs █▇▆▆▅▅▅▅▅▅▅▅▄▄▄▄▄▄▄▄▃▃▃▂▂▂▁▁| 207.451µs
converge: iters=100, elapsed=1.03, metric=0.13
Fast semaphore wake estimates:
10.151µs | 11.067µs ▁▆▆▆▆▆▅▅▅▅▅▅▅▅▅▅▅▅▅▅▅▅▅▅▅▅▅▅▅▅▅▅▅▅▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▃▃▃▃▃▂▁| 1.992989ms
Slow semaphore wake estimates:
10.5µs |▁▆▇▇ 130.8µs ▇▅▅▄▄▄▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▁▁▁▁▁▁▁▁▁▁▁▁▁▁| 2.262692ms
converge: iters=487, elapsed=5.09, metric=0.16
Fast semaphore wake estimates:
10.035µs | 11.202µs ▁▆▆▆▆▅▅▅▅▅▅▅▅▅▅▅▅▅▅▅▅▅▅▅▅▅▅▅▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▃▃▃▃▃▃▃▃▃▂▂▂▂▂▂▁▁▁| 1.980441ms
Slow semaphore wake estimates:
10.503µs |▁ 131.554µs ▇▅▅▄▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▁▁▁▁▁▁▁▁▁▁▁▁▁▁| 5.970948ms
converge: iters=974, elapsed=10.14, metric=0.16
ok

test result: ok. 1 passed; 0 failed; 0 ignored; 0 measured; 4 filtered out; finished in 16.27s
```

### 4. 修改後的 Gthulhu（User Mode + `builtin_idle=true`）實測輸出

在同樣開啟 `builtin_idle: true` 的設定下，Gthulhu 的實測表現如下：

```text
running 1 test
test adaptive_priority ... 
converge: min_time=5.00, max_time=10.00, threshold=0.50
Fast semaphore wake estimates:
36.161µs | 36.161µs ▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁| 36.161µs
Slow semaphore wake estimates:
27.084µs | 27.084µs ▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁| 27.084µs
converge: iters=1, elapsed=0.01, metric=1.34
Fast semaphore wake estimates:
12.243µs | 13.2µs ▁▇▇▆▆▆▆▆▅▅▅▅▅▅▅▅▅▅▅▅▅▅▅▅▅▅▅▅▅▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▃▃▃▃▃▃▃▃▃▃▂▂▂▂▂▂▂▁▁▁▁| 561.357µs
Slow semaphore wake estimates:
12.862µs |▁▇ 146.189µs █▆▆▆▆▆▅▅▅▅▅▅▅▅▅▅▅▅▅▅▅▅▅▅▅▅▅▅▅▅▅▅▅▅▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▃▃▃▃▃▂▁| 4.00977ms
converge: iters=100, elapsed=1.05, metric=0.22
Fast semaphore wake estimates:
11.306µs | 13.111µs ▁▆▅▅▄▄▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▁▁▁▁▁▁▁▁▁▁▁▁▁▁| 1.511868ms
Slow semaphore wake estimates:
11.242µs |▁▆▇▇█ 145.439µs ▇▅▅▅▄▄▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▁▁▁▁▁▁▁▁▁▁▁▁▁| 1.984615ms
converge: iters=477, elapsed=4.98, metric=0.18
Fast semaphore wake estimates:
10.188µs | 13.328µs ▁▆▆▆▅▅▅▅▅▅▅▅▅▅▅▅▅▅▅▄▄▄▄▄▄▄▄▄▄▄▄▄▄▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▁▁▁▁▁▁▁▁| 1.93372ms
Slow semaphore wake estimates:
11.395µs |▁ 143.592µs ▇▆▅▅▅▅▅▅▄▄▄▄▄▄▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▁▁▁▁▁▁▁▁▁▁▁▁| 6.880901ms
converge: iters=479, elapsed=5.10, metric=0.22
Fast semaphore wake estimates:
7.109µs | 13.454µs ▁▄▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁| 6.264819ms
Slow semaphore wake estimates:
12.026µs |▁▇▇ 146.517µs ▇▆▅▅▅▅▅▅▄▄▄▄▄▄▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▂▁▁▁▁▁▁▁▁▁▁▁| 3.417526ms
converge: iters=958, elapsed=10.04, metric=0.20
ok

test result: ok. 1 passed; 0 failed; 0 ignored; 0 measured; 4 filtered out; finished in 21.19s
```

可以看到兩者的分佈型態幾乎一模一樣：
- **Fast semaphore** 的喚醒延遲中位數都被壓在 **11～13.4 µs** 左右。
- **Slow semaphore** 的喚醒延遲中位數則落在 **131～146 µs** 左右。
- 在每個超過 5 秒與 10 秒的收斂批次中，Gthulhu 的 metric 穩定維持在 **0.18～0.22**，與 `scx_rustland` 的 **0.13～0.16** 僅有微幅差距，遠低於 `0.50` 的測試門檻。

---

## 六、如何重現編譯與測試

### 1. 編譯與執行單元 / 回歸測試

在 Gthulhu 根目錄下執行：

```bash
# 重新編譯 BPF、生成 qumun/main.skeleton.h 並打包 libwrapper.a
make wrapper

export GOTOOLCHAIN=go1.25.0 CC=clang
export CGO_CFLAGS="-I$PWD -I$PWD/qumun -I$PWD/output"
export CGO_LDFLAGS="-lelf -lz $PWD/output/libbpf.a -lzstd $PWD/libwrapper.a"

# 強制重新連結執行檔，確保嵌入最新 BPF skeleton
go build -a -ldflags='-w -s -checklinkname=0' -o main main.go

# 執行 Go 單元測試與 Race 檢查
go test -race -ldflags='-checklinkname=0' -count=1 \
  ./internal/scheduler/... \
  github.com/Gthulhu/qumun/goland_core \
  github.com/Gthulhu/qumun/internal/schedbridge

# 執行 BPF Callback 邏輯與計數回歸測試（Clang mock 測試）
python3 qumun/scripts/test_bpf_progress.py
```

### 2. 執行 `schtest` 驗證

開兩個終端機，第一個終端機啟動排程器（不需加額外實驗旗標）：

```bash
sudo ./main scheduler -config config/config.yaml
```

第二個終端機執行 `schtest`：

```bash
sudo schtest/target/debug/schtest --filter adaptive_priority
sudo schtest/target/debug/schtest --filter fairness
```

### 3. 日後除錯小工具

如果未來要再度剖析排程器迴圈或觀察特定負載下的表現，可以使用這次留下的診斷選項（注意：跑正式效能基準測試時請保持關閉，以免計時 syscall 與 log 影響延遲）：

- `-trace-scheduler-loop`：每秒輸出排程迴圈各階段的呼叫次數、平均、最大值、`>= 1ms` 次數與分位數上界（`p50_le`、`p99_le`、`p999_le`），並顯示當下停在哪個階段。
- `-trace-scheduler-cpu`：在 `-trace-scheduler-loop` 之上，額外針對 `SelectCPU` 量測 Thread CPU time 與 Wall time 的最差樣本對照，用來快速分辨是「C/BPF 呼叫本身變慢」還是「排程器執行緒被搶佔 / Off-CPU」。
- `-usersched-preempt`：預設關閉的實驗性選項。開啟後當 pinned Go 排程器執行緒被喚醒、有待處理任務且找不到閒置 CPU 時，允許以最多每毫秒一次的頻率對其目標 CPU 發出搶佔。
