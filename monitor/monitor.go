// SPDX-FileCopyrightText: 2025 Gthulhu Team
//
// SPDX-License-Identifier: Apache-2.0

// Package monitor provides the pod-level scheduling metrics collector.
//
// It loads an eBPF program (sched_monitor.bpf.o) that hooks into
// tp_btf/sched_switch and tp_btf/sched_process_exit tracepoints,
// reads per-PID scheduling metrics from BPF maps, aggregates them
// by pod, and exposes the results as Prometheus metrics.
//
// This is the BASE feature of Gthulhu — works on Linux 5.2+ (BTF-enabled
// kernels) and does NOT require sched_ext.
//
// Architecture:
//
// PodSchedulingMetrics CRD  →  CRD Watcher  →  eBPF Collector (tp_btf)
//
//	         ↓
//	  Prometheus /metrics
//	         ↓
//	Prometheus Adapter → KEDA → HPA
package monitor

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/Gthulhu/Gthulhu/monitor/collector"
	"github.com/Gthulhu/Gthulhu/monitor/crdwatcher"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Config holds the monitor configuration passed from the main scheduler.
type Config struct {
	BPFObjectPath         string
	CollectionIntervalSec int
	MonitorAll            bool
	StreamEvents          bool
	PrometheusPort        int
	NodeName              string
	EnableCRDWatcher      bool
	KubeConfigPath        string
}

// StartMonitor loads the eBPF monitor, starts the collector poll loop and
// Prometheus HTTP server. It blocks until ctx is cancelled.
func StartMonitor(ctx context.Context, cfg Config, logger *slog.Logger) error {
	if logger == nil {
		logger = slog.Default()
	}
	logger = logger.With("component", "monitor")

	// Pod mapper: resolves PIDs → Kubernetes pods via /proc/<pid>/cgroup
	podMapper := collector.NewPodMapper(cfg.NodeName, logger)
	done := make(chan struct{})
	defer close(done)
	podMapper.StartPeriodicScan(30*time.Second, done)

	// eBPF Collector
	interval := cfg.CollectionIntervalSec
	if interval <= 0 {
		interval = 10
	}
	col := collector.New(collector.Config{
		BPFObjectPath: cfg.BPFObjectPath,
		PollInterval:  time.Duration(interval) * time.Second,
		MonitorAll:    cfg.MonitorAll,
		StreamEvents:  cfg.StreamEvents,
	}, podMapper, logger)

	// CRD Watcher (optional — needs Kubernetes API access)
	if cfg.EnableCRDWatcher {
		kubeConfig, err := buildKubeConfig(cfg.KubeConfigPath)
		if err != nil {
			logger.Warn("kubeconfig unavailable, CRD watcher disabled", "error", err)
		} else {
			w, err := crdwatcher.New(kubeConfig, col, podMapper, cfg.NodeName, logger)
			if err != nil {
				logger.Warn("CRD watcher creation failed", "error", err)
			} else {
				go func() {
					if err := w.Run(ctx); err != nil {
						logger.Error("CRD watcher error", "error", err)
					}
				}()
				logger.Info("CRD watcher started for PodSchedulingMetrics")
			}
		}
	}

	// Register Prometheus collector with a dedicated registry to avoid
	// polluting the default global registry used by other components.
	reg := prometheus.NewRegistry()
	reg.MustRegister(collector.NewPodSchedMetricsCollector(col))
	reg.MustRegister(prometheus.NewGoCollector())
	reg.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))

	// Prometheus HTTP server
	port := cfg.PrometheusPort
	if port == 0 {
		port = 9090
	}
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "ok")
	})
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	// Start HTTP server in background
	go func() {
		logger.Info("Prometheus metrics server starting", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("metrics server error", "error", err)
		}
	}()

	// Start collector — blocks until ctx is cancelled
	err := col.Start(ctx)

	// Graceful shutdown of HTTP server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = srv.Shutdown(shutdownCtx)

	logger.Info("monitor stopped")
	return err
}

// buildKubeConfig returns a Kubernetes rest.Config from a kubeconfig path
// or falls back to in-cluster config when running inside a pod.
func buildKubeConfig(kubeconfigPath string) (*rest.Config, error) {
	if kubeconfigPath != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	}
	return rest.InClusterConfig()
}
