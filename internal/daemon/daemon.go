package daemon

import (
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Gthulhu/Gthulhu/internal/config"
	"github.com/Gthulhu/Gthulhu/internal/schedext"
	"gopkg.in/yaml.v3"
)

const modeScheduler = "scheduler"

var schedExtSupportChecker = schedext.CheckSupport

func Run(args []string) error {
	runtimeStore := RuntimeConfigStore(fileRuntimeConfigStore{})
	commandResolver := SchedulerCommandResolver(defaultSchedulerCommandResolver{})

	fs := flag.NewFlagSet("daemon", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	fs.Usage = func() {
		fmt.Fprintf(os.Stdout, "Usage: %s daemon [flags]\n", os.Args[0])
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

	if err := runtimeStore.InitializeRuntimeConfig(*configFile, *runtimeConfigPath); err != nil {
		return err
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	restartReqCh := make(chan struct{}, 1)
	state := &controlState{runtimeConfigPath: *runtimeConfigPath}
	state.set("bootstrap", false)
	if err := startControlServer(*controlAddr, *runtimeConfigPath, binPath, state, runtimeStore, restartReqCh); err != nil {
		return err
	}

	gracefulStopTimeout := 5 * time.Second

	for {
		childBinPath, childArgs, enabled, err := commandResolver.Resolve(*runtimeConfigPath, binPath)
		if err != nil {
			if errors.Is(err, schedext.ErrUnsupported) {
				state.recordError(err.Error())
				slog.Error("sched_ext is unavailable; not starting scheduler child", "error", err)
				select {
				case sig := <-sigCh:
					slog.Info("daemon received signal while scheduler unsupported", "signal", sig)
					return nil
				case <-restartReqCh:
					continue
				}
			}
			return err
		}
		if !enabled {
			slog.Info("scheduler disabled by runtime config, waiting for config update", "configPath", *runtimeConfigPath)
			select {
			case sig := <-sigCh:
				slog.Info("daemon received signal while scheduler disabled", "signal", sig)
				return nil
			case <-restartReqCh:
				continue
			}
		}

		cmd := exec.Command(childBinPath, childArgs...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin

		slog.Info("starting scheduler child process", "binary", childBinPath, "args", childArgs)
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
			state.recordRestart()
			if isUnsupportedSchedExtExit(err) {
				errMsg := fmt.Sprintf("scheduler child exited because sched_ext is unsupported by this kernel: %v", err)
				state.recordError(errMsg)
				slog.Error("scheduler child reported unsupported sched_ext; not restarting", "error", err)
				select {
				case sig := <-sigCh:
					slog.Info("daemon received signal while scheduler unsupported", "signal", sig)
					return nil
				case <-restartReqCh:
					continue
				}
			}
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

func isUnsupportedSchedExtExit(err error) bool {
	var exitErr *exec.ExitError
	return errors.As(err, &exitErr) && exitErr.ExitCode() == schedext.UnsupportedExitCode
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

func applyRuntimeConfigToFile(runtimeConfigPath string, schedulerBinPath string, req runtimeConfigRequest) (bool, error) {
	cfg, err := config.LoadConfig(runtimeConfigPath)
	if err != nil {
		return false, fmt.Errorf("load current runtime config: %w", err)
	}

	prev := struct {
		Mode              string
		SchedulerName     string
		SliceNsDefault    uint64
		SliceNsMin        uint64
		KernelMode        bool
		MaxTimeWatchdog   bool
		EarlyProcessing   bool
		BuiltinIdle       bool
		MonitoringEnabled bool
	}{
		Mode:              cfg.Scheduler.Mode,
		SchedulerName:     cfg.Scheduler.SchedulerName,
		SliceNsDefault:    cfg.Scheduler.SliceNsDefault,
		SliceNsMin:        cfg.Scheduler.SliceNsMin,
		KernelMode:        cfg.Scheduler.KernelMode,
		MaxTimeWatchdog:   cfg.Scheduler.MaxTimeWatchdog,
		EarlyProcessing:   cfg.EarlyProcessing,
		BuiltinIdle:       cfg.BuiltinIdle,
		MonitoringEnabled: cfg.Monitor.Enabled,
	}

	cfg.Scheduler.Mode = req.Mode
	cfg.Scheduler.SchedulerName = req.SchedulerName
	cfg.Scheduler.SliceNsDefault = req.SliceNsDefault
	cfg.Scheduler.SliceNsMin = req.SliceNsMin
	cfg.Scheduler.KernelMode = req.KernelMode
	cfg.Scheduler.MaxTimeWatchdog = req.MaxTimeWatchdog
	cfg.EarlyProcessing = req.EarlyProcessing
	cfg.BuiltinIdle = req.BuiltinIdle
	if !req.SchedulerEnabled && cfg.Scheduler.Mode == "" {
		cfg.Scheduler.Mode = "none"
	}
	if req.SchedulerEnabled && cfg.Scheduler.Mode == "" {
		cfg.Scheduler.Mode = "gthulhu"
	}
	if err := validateScheduler(cfg.Scheduler.Mode, cfg.Scheduler.SchedulerName); err != nil {
		return false, err
	}
	if cfg.Scheduler.Mode == "scx" {
		if err := ensureExecutableInDir(filepath.Dir(schedulerBinPath), cfg.Scheduler.SchedulerName); err != nil {
			return false, err
		}
	}
	cfg.Monitor.Enabled = req.MonitoringEnabled

	changed := prev.Mode != cfg.Scheduler.Mode ||
		prev.SchedulerName != cfg.Scheduler.SchedulerName ||
		prev.SliceNsDefault != cfg.Scheduler.SliceNsDefault ||
		prev.SliceNsMin != cfg.Scheduler.SliceNsMin ||
		prev.KernelMode != cfg.Scheduler.KernelMode ||
		prev.MaxTimeWatchdog != cfg.Scheduler.MaxTimeWatchdog ||
		prev.EarlyProcessing != cfg.EarlyProcessing ||
		prev.BuiltinIdle != cfg.BuiltinIdle ||
		prev.MonitoringEnabled != cfg.Monitor.Enabled

	if !changed {
		return false, nil
	}

	if err := writeConfigFile(runtimeConfigPath, cfg); err != nil {
		return true, fmt.Errorf("write runtime config: %w", err)
	}
	return true, nil
}

func writeConfigFile(path string, cfg *config.Config) error {
	yamlBytes, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, yamlBytes, 0o644)
}

func schedulerCommandFromConfig(configPath string, gthulhuBin string) (string, []string, bool, error) {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return "", nil, false, fmt.Errorf("load runtime config: %w", err)
	}
	mode := strings.TrimSpace(cfg.Scheduler.Mode)
	if mode == "" {
		mode = "gthulhu"
	}
	if mode == "none" {
		if cfg.IsMonitorEnabled() {
			return gthulhuBin, []string{modeScheduler, "-config", configPath}, true, nil
		}
		return "", nil, false, nil
	}
	if err := validateScheduler(mode, cfg.Scheduler.SchedulerName); err != nil {
		return "", nil, false, err
	}
	if err := schedExtSupportChecker(); err != nil {
		if cfg.IsMonitorEnabled() {
			slog.Warn("sched_ext is unavailable; starting Gthulhu in monitor-only mode", "error", err)
			return gthulhuBin, []string{modeScheduler, "-config", configPath}, true, nil
		}
		return "", nil, false, err
	}
	switch mode {
	case "gthulhu", "simple":
		return gthulhuBin, []string{modeScheduler, "-config", configPath}, true, nil
	case "scx":
		if err := ensureExecutableInDir(filepath.Dir(gthulhuBin), cfg.Scheduler.SchedulerName); err != nil {
			return "", nil, false, err
		}
		path := filepath.Clean(filepath.Join(filepath.Dir(gthulhuBin), cfg.Scheduler.SchedulerName))
		return path, nil, true, nil
	default:
		return "", nil, false, fmt.Errorf("unsupported scheduler mode %q", mode)
	}
}

func validateScheduler(mode string, schedulerName string) error {
	mode = strings.TrimSpace(mode)
	if mode == "" {
		mode = "gthulhu"
	}
	switch mode {
	case "none", "gthulhu", "simple":
		return nil
	case "scx":
		name := strings.TrimSpace(schedulerName)
		if name == "" {
			return fmt.Errorf("schedulerName is required when mode is scx")
		}
		if filepath.Base(name) != name || strings.Contains(name, string(filepath.Separator)) {
			return fmt.Errorf("schedulerName must be a binary name, got %q", schedulerName)
		}
		if _, ok := allowedSCXSchedulers[name]; !ok {
			return fmt.Errorf("schedulerName %q is not in the allowed scx scheduler list", name)
		}
		return nil
	default:
		return fmt.Errorf("unsupported scheduler mode %q", mode)
	}
}

// ensureExecutableInDir verifies that name resolves to an executable file
// strictly inside dir.  It constructs the path, cleans it, and checks that
// the result is actually confined to dir before touching the file system.
func ensureExecutableInDir(dir, name string) error {
	cleanDir := filepath.Clean(dir)
	cleanPath := filepath.Clean(filepath.Join(cleanDir, name))
	// Prevent path traversal: the joined path must be a direct child of cleanDir.
	if cleanPath == cleanDir || !strings.HasPrefix(cleanPath, cleanDir+string(filepath.Separator)) {
		return fmt.Errorf("scheduler binary path %q is outside the allowed directory", name)
	}
	return ensureExecutable(cleanPath)
}

func ensureExecutable(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("scheduler binary not found: %s", path)
		}
		return fmt.Errorf("stat scheduler binary %s: %w", path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("scheduler binary path is a directory: %s", path)
	}
	if info.Mode()&0o111 == 0 {
		return fmt.Errorf("scheduler binary is not executable: %s", path)
	}
	return nil
}
