// SPDX-FileCopyrightText: 2025 Gthulhu Team
//
// SPDX-License-Identifier: Apache-2.0
// Author: Ian Chen <ychen.desl@gmail.com>

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"

	_ "net/http/pprof"

	"github.com/Gthulhu/Gthulhu/internal/config"
	"github.com/Gthulhu/Gthulhu/monitor"
	"github.com/Gthulhu/plugin/models"
	"github.com/Gthulhu/plugin/plugin"
	"github.com/Gthulhu/plugin/plugin/gthulhu"
	core "github.com/Gthulhu/qumun/goland_core"
	cache "github.com/Gthulhu/qumun/util"
	"gopkg.in/yaml.v3"
)

const (
	modeScheduler = "scheduler"
	modeDaemon    = "daemon"
)

type daemonRuntimeConfigRequest struct {
	ConfigVersion     string `json:"configVersion,omitempty"`
	Mode              string `json:"mode,omitempty"`
	SliceNsDefault    uint64 `json:"sliceNsDefault,omitempty"`
	SliceNsMin        uint64 `json:"sliceNsMin,omitempty"`
	KernelMode        bool   `json:"kernelMode,omitempty"`
	MaxTimeWatchdog   bool   `json:"maxTimeWatchdog,omitempty"`
	EarlyProcessing   bool   `json:"earlyProcessing,omitempty"`
	BuiltinIdle       bool   `json:"builtinIdle,omitempty"`
	SchedulerEnabled  bool   `json:"schedulerEnabled"`
	MonitoringEnabled bool   `json:"monitoringEnabled"`
}

type daemonRuntimeConfigStatus struct {
	ConfigVersion string `json:"configVersion,omitempty"`
	Applied       bool   `json:"applied"`
	AppliedAt     string `json:"appliedAt,omitempty"` // ISO 8601 timestamp
	RestartCount  int64  `json:"restartCount,omitempty"`
	LastError     string `json:"lastError,omitempty"`
}

type daemonDetailedStatus struct {
	ConfigVersion string `json:"configVersion,omitempty"`
	Applied       bool   `json:"applied"`
	AppliedAt     string `json:"appliedAt,omitempty" `
	RestartCount  int64  `json:"restartCount"`
	LastError     string `json:"lastError,omitempty"`
}

type daemonControlState struct {
	mu            sync.RWMutex
	configVersion string
	applied       bool
	appliedAt     time.Time
	restartCount  int64
	lastError     string
}

func (s *daemonControlState) set(version string, applied bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.configVersion = version
	s.applied = applied
	if applied {
		s.appliedAt = time.Now()
		s.lastError = ""
	}
}

func (s *daemonControlState) recordError(errMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastError = errMsg
}

func (s *daemonControlState) recordRestart() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.restartCount++
}

func (s *daemonControlState) snapshot() daemonRuntimeConfigStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var appliedAtStr string
	if !s.appliedAt.IsZero() {
		appliedAtStr = s.appliedAt.UTC().Format(time.RFC3339)
	}
	return daemonRuntimeConfigStatus{
		ConfigVersion: s.configVersion,
		Applied:       s.applied,
		AppliedAt:     appliedAtStr,
		RestartCount:  s.restartCount,
		LastError:     s.lastError,
	}
}

func (s *daemonControlState) detailedSnapshot() daemonDetailedStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var appliedAtStr string
	if !s.appliedAt.IsZero() {
		appliedAtStr = s.appliedAt.UTC().Format(time.RFC3339)
	}
	return daemonDetailedStatus{
		ConfigVersion: s.configVersion,
		Applied:       s.applied,
		AppliedAt:     appliedAtStr,
		RestartCount:  s.restartCount,
		LastError:     s.lastError,
	}
}

func main() {
	runtime.GOMAXPROCS(1)

	// Initialize structured logger
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	if isGlobalHelpRequest(os.Args[1:]) {
		printRootUsage(os.Args[0])
		return
	}

	mode, modeArgs := resolveModeAndArgs(os.Args[1:])
	switch mode {
	case modeDaemon:
		if err := runDaemonMode(modeArgs); err != nil {
			slog.Error("daemon exited with error", "error", err)
			os.Exit(1)
		}
	default:
		if err := runSchedulerMode(modeArgs); err != nil {
			slog.Error("scheduler exited with error", "error", err)
			os.Exit(1)
		}
	}
}

func isGlobalHelpRequest(args []string) bool {
	if len(args) == 0 {
		return false
	}
	first := args[0]
	return first == "help" || first == "-h" || first == "--help"
}

func printRootUsage(binary string) {
	fmt.Fprintf(os.Stdout, "Usage:\n")
	fmt.Fprintf(os.Stdout, "  %s [scheduler flags]            # default mode (backward compatible)\n", binary)
	fmt.Fprintf(os.Stdout, "  %s scheduler [scheduler flags]  # explicit scheduler mode\n", binary)
	fmt.Fprintf(os.Stdout, "  %s daemon [daemon flags]        # supervisor mode\n\n", binary)
	fmt.Fprintf(os.Stdout, "Scheduler flags:\n")
	fmt.Fprintf(os.Stdout, "  -config string\tPath to YAML configuration file\n")
	fmt.Fprintf(os.Stdout, "  -help\t\tShow scheduler help message\n")
	fmt.Fprintf(os.Stdout, "  -explain\tExplain configuration options\n\n")
	fmt.Fprintf(os.Stdout, "Daemon flags:\n")
	fmt.Fprintf(os.Stdout, "  -config string\tPath to YAML configuration file passed to child scheduler\n")
	fmt.Fprintf(os.Stdout, "  -restart-delay duration\tDelay before restarting child scheduler (default 2s)\n")
	fmt.Fprintf(os.Stdout, "  -scheduler-bin string\tPath to scheduler binary (default: current executable)\n")
}

func resolveModeAndArgs(args []string) (string, []string) {
	if len(args) == 0 {
		return modeScheduler, args
	}
	if args[0] == modeScheduler || args[0] == modeDaemon {
		return args[0], args[1:]
	}
	return modeScheduler, args
}

func runDaemonMode(args []string) error {
	fs := flag.NewFlagSet(modeDaemon, flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	fs.Usage = func() {
		fmt.Fprintf(os.Stdout, "Usage: %s %s [flags]\n", os.Args[0], modeDaemon)
		fmt.Fprintf(os.Stdout, "Runs gthulhud supervisor mode; starts and restarts scheduler child process.\n\n")
		fs.PrintDefaults()
	}
	configFile := fs.String("config", "", "Path to YAML configuration file")
	restartDelay := fs.Duration("restart-delay", 2*time.Second, "Delay before restarting scheduler process")
	schedulerBin := fs.String("scheduler-bin", "", "Path to scheduler binary (default: current executable)")
	runtimeConfigPath := fs.String("runtime-config-path", "/tmp/gthulhu/runtime-config.yaml", "Path to daemon-managed runtime YAML config file")
	controlAddr := fs.String("control-addr", ":18080", "Daemon control API bind address")
	if err := fs.Parse(args); err != nil {
		return err
	}

	binPath := *schedulerBin
	if binPath == "" {
		exePath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("resolve executable path: %w", err)
		}
		binPath = exePath
	}

	if err := initializeRuntimeConfig(*configFile, *runtimeConfigPath); err != nil {
		return err
	}

	childArgs := []string{modeScheduler, "-config", *runtimeConfigPath}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	restartReqCh := make(chan struct{}, 1)
	state := &daemonControlState{}
	if err := startDaemonControlServer(*controlAddr, *runtimeConfigPath, state, restartReqCh); err != nil {
		return err
	}

	gracefulStopTimeout := 5 * time.Second

	for {
		if !isSchedulerEnabledInConfig(*runtimeConfigPath) {
			slog.Info("scheduler disabled by runtime config, waiting for config update", "configPath", *runtimeConfigPath)
			select {
			case sig := <-sigCh:
				slog.Info("daemon received signal while scheduler disabled", "signal", sig)
				return nil
			case <-restartReqCh:
				continue
			}
		}

		cmd := exec.Command(binPath, childArgs...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin

		slog.Info("starting scheduler child process", "binary", binPath, "args", childArgs)
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("start scheduler child process: %w", err)
		}

		done := make(chan error, 1)
		go func() {
			done <- cmd.Wait()
		}()

		select {
		case sig := <-sigCh:
			slog.Info("daemon received signal, forwarding to scheduler child", "signal", sig)
			err := stopChildProcess(cmd, done, sig, gracefulStopTimeout)
			if err != nil {
				slog.Warn("scheduler child exited after signal", "error", err)
			}
			return nil
		case <-restartReqCh:
			slog.Info("runtime config updated, restarting scheduler child")
			err := stopChildProcess(cmd, done, syscall.SIGTERM, gracefulStopTimeout)
			if err != nil {
				slog.Warn("scheduler child stop during restart", "error", err)
			}
			time.Sleep(200 * time.Millisecond)
			continue
		case err := <-done:
			recordSchedulerRestart(state)
			if err != nil {
				slog.Warn("scheduler child exited unexpectedly, restarting", "error", err, "delay", restartDelay.String())
			} else {
				slog.Warn("scheduler child exited unexpectedly with status 0, restarting", "delay", restartDelay.String())
			}
			time.Sleep(*restartDelay)
		}
	}
}

func stopChildProcess(cmd *exec.Cmd, done <-chan error, sig os.Signal, timeout time.Duration) error {
	if cmd.Process != nil {
		_ = cmd.Process.Signal(sig)
	}
	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		return <-done
	}
}

func recordSchedulerRestart(state *daemonControlState) {
	state.recordRestart()
}

func initializeRuntimeConfig(bootstrapConfigPath string, runtimeConfigPath string) error {
	cfg, err := config.LoadConfig(bootstrapConfigPath)
	if err != nil {
		return fmt.Errorf("load bootstrap config for daemon: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(runtimeConfigPath), 0o755); err != nil {
		return fmt.Errorf("create runtime config directory: %w", err)
	}
	if err := writeConfigFile(runtimeConfigPath, cfg); err != nil {
		return fmt.Errorf("initialize runtime config file: %w", err)
	}
	slog.Info("initialized daemon runtime config", "path", runtimeConfigPath)
	return nil
}

func startDaemonControlServer(addr, runtimeConfigPath string, state *daemonControlState, restartReqCh chan<- struct{}) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/api/v1/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeDaemonJSON(w, http.StatusMethodNotAllowed, map[string]any{"success": false, "error": "method not allowed"})
			return
		}
		writeDaemonJSON(w, http.StatusOK, map[string]any{"success": true, "data": state.detailedSnapshot()})
	})
	mux.HandleFunc("/api/v1/runtime-config", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		switch r.Method {
		case http.MethodGet:
			writeDaemonJSON(w, http.StatusOK, map[string]any{"success": true, "data": state.snapshot()})
			return
		case http.MethodPost:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				writeDaemonJSON(w, http.StatusBadRequest, map[string]any{"success": false, "error": "invalid request body"})
				return
			}
			var req daemonRuntimeConfigRequest
			if err := json.NewDecoder(bytes.NewReader(body)).Decode(&req); err != nil {
				writeDaemonJSON(w, http.StatusBadRequest, map[string]any{"success": false, "error": "invalid request payload"})
				return
			}
			if req.ConfigVersion == "" {
				writeDaemonJSON(w, http.StatusBadRequest, map[string]any{"success": false, "error": "configVersion is required"})
				return
			}

			if err := applyRuntimeConfigToFile(runtimeConfigPath, req); err != nil {
				slog.ErrorContext(ctx, "failed to apply runtime config", "error", err)
				errMsg := err.Error()
				state.recordError(errMsg)
				writeDaemonJSON(w, http.StatusInternalServerError, map[string]any{"success": false, "error": errMsg})
				return
			}
			state.set(req.ConfigVersion, true)
			select {
			case restartReqCh <- struct{}{}:
			default:
			}
			writeDaemonJSON(w, http.StatusOK, map[string]any{"success": true})
			return
		default:
			writeDaemonJSON(w, http.StatusMethodNotAllowed, map[string]any{"success": false, "error": "method not allowed"})
			return
		}
	})

	server := &http.Server{Addr: addr, Handler: mux}
	go func() {
		slog.Info("daemon control server started", "addr", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("daemon control server exited", "error", err)
		}
	}()
	return nil
}

func writeDaemonJSON(w http.ResponseWriter, status int, payload map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func applyRuntimeConfigToFile(runtimeConfigPath string, req daemonRuntimeConfigRequest) error {
	cfg, err := config.LoadConfig(runtimeConfigPath)
	if err != nil {
		return fmt.Errorf("load current runtime config: %w", err)
	}
	cfg.Scheduler.Mode = req.Mode
	cfg.Scheduler.SliceNsDefault = req.SliceNsDefault
	cfg.Scheduler.SliceNsMin = req.SliceNsMin
	cfg.Scheduler.KernelMode = req.KernelMode
	cfg.Scheduler.MaxTimeWatchdog = req.MaxTimeWatchdog
	cfg.EarlyProcessing = req.EarlyProcessing
	cfg.BuiltinIdle = req.BuiltinIdle
	if req.SchedulerEnabled {
		if cfg.Scheduler.Mode == "" {
			cfg.Scheduler.Mode = "gthulhu"
		}
	} else {
		cfg.Scheduler.Mode = ""
	}
	cfg.Monitor.Enabled = req.MonitoringEnabled
	if err := writeConfigFile(runtimeConfigPath, cfg); err != nil {
		return fmt.Errorf("write runtime config: %w", err)
	}
	return nil
}

func writeConfigFile(path string, cfg *config.Config) error {
	yamlBytes, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, yamlBytes, 0o644)
}

func isSchedulerEnabledInConfig(configPath string) bool {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		slog.Warn("failed to read runtime config when checking scheduler enabled", "error", err)
		return true
	}
	return cfg.IsSchedulerEnabled()
}

func runSchedulerMode(args []string) error {

	// Parse command line flags
	fs := flag.NewFlagSet(modeScheduler, flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	fs.Usage = func() {
		fmt.Fprintf(os.Stdout, "Usage: %s [%s] [flags]\n", os.Args[0], modeScheduler)
		fmt.Fprintf(os.Stdout, "Default behavior is scheduler mode when no mode is specified.\n\n")
		fs.PrintDefaults()
	}
	configFile := fs.String("config", "", "Path to YAML configuration file")
	showHelper := fs.Bool("help", false, "Show help message")
	showExplain := fs.Bool("explain", false, "Explain configuration options")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *showHelper {
		fs.Usage()
		return nil
	}

	if *showExplain {
		fmt.Println(config.ExplainConfig())
		return nil
	}

	// Load configuration
	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ── Monitor (base feature, works on Linux 5.2+ BTF kernels) ──
	if cfg.IsMonitorEnabled() {
		monCfg := monitor.Config{
			BPFObjectPath:         cfg.Monitor.BPFObjectPath,
			CollectionIntervalSec: cfg.Monitor.CollectionIntervalSec,
			MonitorAll:            cfg.Monitor.MonitorAll,
			StreamEvents:          cfg.Monitor.StreamEvents,
			PrometheusPort:        cfg.Monitor.PrometheusPort,
			NodeName:              os.Getenv("NODE_NAME"),
			EnableCRDWatcher:      cfg.Monitor.EnableCRDWatcher,
			KubeConfigPath:        cfg.Monitor.KubeConfigPath,
		}
		go func() {
			slog.Info("starting scheduling monitor",
				"bpfObject", monCfg.BPFObjectPath,
				"prometheusPort", monCfg.PrometheusPort,
				"monitorAll", monCfg.MonitorAll,
			)
			if err := monitor.StartMonitor(ctx, monCfg, slog.Default()); err != nil {
				slog.Error("monitor goroutine error", "error", err)
			}
		}()
	}

	// ── Scheduler (advanced feature, requires sched_ext / Linux 6.12+) ──
	if !cfg.IsSchedulerEnabled() {
		slog.Info("running in monitor-only mode (no scheduler mode configured)")
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		select {
		case sig := <-sigCh:
			slog.Info("received signal, shutting down", "signal", sig)
		case <-ctx.Done():
		}
		slog.Info("Gthulhu exit")
		return nil
	}

	// Apply scheduler configuration before loading eBPF program
	schedConfig := cfg.GetSchedulerConfig()

	var p plugin.CustomScheduler
	var SLICE_NS_DEFAULT, SLICE_NS_MIN uint64
	SLICE_NS_DEFAULT = cfg.Scheduler.SliceNsDefault
	SLICE_NS_MIN = cfg.Scheduler.SliceNsMin
	slog.Info("Scheduler configuration", "SliceNsDefault", SLICE_NS_DEFAULT, "SliceNsMin", SLICE_NS_MIN)
	config := &plugin.SchedConfig{
		Mode: schedConfig.Mode,
		Scheduler: plugin.Scheduler{
			SliceNsDefault: cfg.Scheduler.SliceNsDefault,
			SliceNsMin:     cfg.Scheduler.SliceNsMin,
		},
		APIConfig: plugin.APIConfig{
			BaseURL:       cfg.Api.Url,
			Interval:      cfg.Api.Interval,
			PublicKeyPath: cfg.Api.PublicKeyPath,
			Enabled:       cfg.Api.Enabled,
			AuthEnabled:   cfg.Api.AuthEnabled,
			MTLS: plugin.MTLSConfig{
				Enable:  cfg.Api.MTLS.Enable,
				CertPem: cfg.Api.MTLS.CertPem,
				KeyPem:  cfg.Api.MTLS.KeyPem,
				CAPem:   cfg.Api.MTLS.CAPem,
			},
		},
	}
	if config.Mode == "" {
		config.Mode = "gthulhu"
	}
	p, err = plugin.NewSchedulerPlugin(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to create plugin: %w", err)
	}

	bpfModule := core.LoadSched("main.bpf.o")
	defer bpfModule.Close()

	bpfModule.SetPlugin(p)

	if cfg.IsDebugEnabled() {
		slog.Info("Debug mode enabled")
		bpfModule.SetDebug(true)
	}

	if cfg.IsBuiltinIdleEnabled() {
		slog.Info("Built-in idle CPU selection enabled")
		bpfModule.SetBuiltinIdle(true)
	}

	if cfg.Scheduler.KernelMode {
		bpfModule.EnableKernelMode()
	}

	if !cfg.Scheduler.MaxTimeWatchdog {
		slog.Info("Max time watchdog disabled")
		bpfModule.DisableMaxTimeWatchdog()
	}

	if cfg.EarlyProcessing {
		slog.Info("Early processing enabled")
		bpfModule.SetEarlyProcessing(true)
	} else {
		slog.Info("Early processing disabled")
	}

	pid := os.Getpid()
	err = bpfModule.AssignUserSchedPid(pid)
	if err != nil {
		slog.Warn("AssignUserSchedPid failed", "error", err)
	}

	err = cache.ImportScxEnums()
	if err != nil {
		slog.Warn("GetScxEnums failed", "error", err)
	}

	bpfModule.Start()

	topo, err := cache.GetTopology()
	if err != nil {
		return fmt.Errorf("GetTopology failed: %w", err)
	}
	slog.Info("Topology", "topology", topo)

	err = cache.InitCacheDomains(bpfModule)
	if err != nil {
		return fmt.Errorf("InitCacheDomains failed: %w", err)
	}

	if err := bpfModule.Attach(); err != nil {
		return fmt.Errorf("bpfModule attach failed: %w", err)
	}

	slog.Info("UserSched's Pid", "pid", core.GetUserSchedPid())

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	if (cfg.Api.Interval <= 0) || (!cfg.Api.Enabled) {
		cfg.Api.Interval = 5
	}
	oldBss, err := bpfModule.GetBssData()
	if err != nil {
		slog.Warn("GetBssData failed", "error", err)
	}
	timer := time.NewTicker(time.Duration(cfg.Api.Interval) * time.Second)
	cont := true
	go func() {
		defer timer.Stop()
		for cont {
			select {
			case <-ctx.Done():
				slog.Info("context done, exiting signal handler")
				return
			case <-signalChan:
				slog.Info("receive os signal")
				cont = false
			case <-timer.C:
				bss, err := bpfModule.GetBssData()
				if oldBss.Nr_kernel_dispatches == bss.Nr_kernel_dispatches {
					if bpfModule.Stopped() {
						slog.Info("No progress detected and scheduler stopped, exiting")
						cont = false
					}
				}
				oldBss = bss
				bss.Nr_scheduled = bpfModule.GetPoolCount()
				if err != nil {
					slog.Warn("GetBssData failed", "error", err)
				} else {
					b, err := json.Marshal(bss)
					if err != nil {
						slog.Warn("json.Marshal failed", "error", err)
					} else {
						slog.Info("bss data", "data", string(b))
						if cfg.Api.Enabled {
							// Send metrics to API server if metrics client is available
							// Convert BSS data to metrics format
							metricsData := gthulhu.BssData{
								UserschedLastRunAt: bss.Usersched_last_run_at,
								NrQueued:           bss.Nr_queued,
								NrScheduled:        bss.Nr_scheduled,
								NrRunning:          bss.Nr_running,
								NrOnlineCpus:       bss.Nr_online_cpus,
								NrUserDispatches:   bss.Nr_user_dispatches,
								NrKernelDispatches: bss.Nr_kernel_dispatches,
								NrCancelDispatches: bss.Nr_cancel_dispatches,
								NrBounceDispatches: bss.Nr_bounce_dispatches,
								NrFailedDispatches: bss.Nr_failed_dispatches,
								NrSchedCongested:   bss.Nr_sched_congested,
							}
							p.SendMetrics(metricsData)
						}
					}
				}
			}
		}
		cancel()
		uei, err := bpfModule.GetUeiData()
		if err == nil {
			slog.Info("uei", "kind", uei.Kind, "exitCode", uei.ExitCode, "reason", uei.GetReason(), "message", uei.GetMessage())
		} else {
			slog.Warn("GetUeiData failed", "error", err)
		}
	}()

	slog.Info("scheduler started")

	if cfg.IsDebugEnabled() {
		// Start pprof server for debugging
		go func() {
			http.ListenAndServe(":6060", nil)
		}()
	}

	if cfg.Scheduler.KernelMode {
		for {
			changed, removed := p.GetChangedStrategies()
			if len(changed) > 0 || len(removed) > 0 {
				for _, strategy := range changed {
					err = bpfModule.UpdatePriorityTaskWithPrio(uint32(strategy.PID), strategy.ExecutionTime, uint32(strategy.Priority))
					if err != nil {
						slog.Warn("UpdatePriorityTaskWithPrio failed", "error", err, "pid", strategy.PID)
					} else {
						slog.Info("Updated priority task", "pid", strategy.PID, "executionTime", strategy.ExecutionTime, "priority", strategy.Priority)
					}
				}
				for _, strategy := range removed {
					err = bpfModule.RemovePriorityTask(uint32(strategy.PID))
					if err != nil {
						slog.Warn("RemovePriorityTask failed", "error", err, "pid", strategy.PID)
					} else {
						slog.Info("Removed priority task", "pid", strategy.PID)
					}
				}
			}
			if bpfModule.Stopped() {
				uei, err := bpfModule.GetUeiData()
				if err == nil {
					slog.Info("uei", "kind", uei.Kind, "exitCode", uei.ExitCode, "reason", uei.GetReason(), "message", uei.GetMessage())
				} else {
					slog.Warn("GetUeiData failed", "error", err)
				}
				return nil
			}
			select {
			case <-ctx.Done():
				slog.Info("context done, exiting kernel mode scheduler loop")
				return nil
			default:
			}
			time.Sleep(1 * time.Second)
		}
	} else {
		if err = runSchedulerLoop(ctx, bpfModule, p, SLICE_NS_DEFAULT, SLICE_NS_MIN); err != nil {
			slog.Info("Scheduler loop exited with error", "error", err)
			uei, err := bpfModule.GetUeiData()
			if err == nil {
				slog.Info("uei", "kind", uei.Kind, "exitCode", uei.ExitCode, "reason", uei.GetReason(), "message", uei.GetMessage())
			} else {
				slog.Warn("GetUeiData failed", "error", err)
			}
			cancel()
		}
	}
	slog.Info("scheduler exit")
	return nil
}

func runSchedulerLoop(
	ctx context.Context,
	bpfModule *core.Sched,
	p plugin.CustomScheduler,
	SLICE_NS_DEFAULT,
	SLICE_NS_MIN uint64,
) error {
	var t *models.QueuedTask
	var task *core.DispatchedTask
	var cpu int32
	var err error

	slog.Info("scheduler loop started")

	for {
		select {
		case <-ctx.Done():
			slog.Info("context done, exiting scheduler loop")
			return nil
		default:
		}

		// Drain all pending tasks from ringbuf (like scx_rustland)
		cnt := bpfModule.DrainQueuedTask()
		if cnt > 0 {
			err = bpfModule.DecNrQueued(cnt)
			if err != nil {
				slog.Warn("DecNrQueued failed", "error", err)
				return err
			}
		}

		// Dispatch ONE task per iteration (like scx_rustland)
		// This ensures low-latency response for newly enqueued tasks
		t = bpfModule.SelectQueuedTask()
		if t == nil {
			bpfModule.BlockTilReadyForDequeue(ctx)
		} else {
			task = core.NewDispatchedTask(t)
			// Deadline calculation:
			// deadline = vtime + min(exec_runtime, 100 * slice_ns)
			task.Vtime = t.Vtime
			if t.Vtime != 0 {
				task.Vtime += min(t.SumExecRuntime, SLICE_NS_DEFAULT*100)
			}

			// Check if a custom execution time was set by a scheduling strategy
			customTime := bpfModule.DetermineTimeSlice(t)
			if customTime > 0 {
				// Use the custom execution time from the scheduling strategy
				task.SliceNs = min(customTime, (t.StopTs-t.StartTs)*11/10)
			} else {
				// Assign minimum time slice scaled by task weight
				task.SliceNs = SLICE_NS_MIN * t.Weight / 100
			}
			err, cpu = bpfModule.SelectCPU(t)
			if err != nil {
				slog.Warn("SelectCPU failed", "error", err)
				return err
			}
			task.Cpu = cpu

			err = bpfModule.DispatchTask(task)
			if err != nil {
				slog.Warn("DispatchTask failed", "error", err)
				return err
			}

			// Notify completion with pending task count
			if bpfModule.GetPoolCount() == 0 {
				err = core.NotifyComplete(0)
				if err != nil {
					slog.Warn("NotifyComplete failed", "error", err)
					return err
				}
			}
		}
	}
}
