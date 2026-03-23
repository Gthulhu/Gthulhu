// SPDX-FileCopyrightText: 2025 Gthulhu Team
//
// SPDX-License-Identifier: Apache-2.0
// Author: Ian Chen <ychen.desl@gmail.com>

package config

import (
	"fmt"
	"os"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"
)

// SchedulerConfig represents scheduler-specific configuration
type SchedulerConfig struct {
	SliceNsDefault  uint64 `yaml:"slice_ns_default" description:"Default time slice in nanoseconds for task scheduling"`
	SliceNsMin      uint64 `yaml:"slice_ns_min" description:"Minimum time slice in nanoseconds for task scheduling"`
	Mode            string `yaml:"mode,omitempty" description:"Scheduler mode (e.g. 'simple' or 'gthulhu')"`
	KernelMode      bool   `yaml:"kernel_mode,omitempty" description:"Enable kernel-mode scheduling (BPF-only dispatching without user-space loop)"`
	MaxTimeWatchdog bool   `yaml:"max_time_watchdog,omitempty" description:"Enable watchdog to detect scheduling stalls"`
}

type SimpleSchedulerConfig struct {
	EnableFifo bool `yaml:"enable_fifo,omitempty" description:"Enable FIFO scheduling in simple scheduler mode"`
}

// MonitorConfig represents the pod-level scheduling metrics monitor configuration.
// The monitor is the base (default) functionality; the scheduler is advanced.
type MonitorConfig struct {
	Enabled               bool   `yaml:"enabled" description:"Enable eBPF scheduling event monitor (base feature)"`
	BPFObjectPath         string `yaml:"bpf_object_path,omitempty" description:"Path to compiled sched_monitor.bpf.o"`
	CollectionIntervalSec int    `yaml:"collection_interval_sec,omitempty" description:"Interval in seconds for reading BPF maps and aggregating metrics"`
	MonitorAll            bool   `yaml:"monitor_all,omitempty" description:"Monitor all processes (if false, only CRD-selected pods are tracked)"`
	StreamEvents          bool   `yaml:"stream_events,omitempty" description:"Enable real-time event streaming via BPF ring buffer"`
	PrometheusPort        int    `yaml:"prometheus_port,omitempty" description:"Port to expose Prometheus /metrics endpoint for pod scheduling metrics"`
	EnableCRDWatcher      bool   `yaml:"enable_crd_watcher,omitempty" description:"Enable Kubernetes CRD watcher for PodSchedulingMetrics resources"`
	KubeConfigPath        string `yaml:"kubeconfig_path,omitempty" description:"Path to kubeconfig file (uses in-cluster config if empty)"`
}

// MTLSConfig holds the mutual TLS configuration used for scheduler → API server communication.
// CertPem and KeyPem are the scheduler's own certificate/key pair signed by the private CA.
// CAPem is the private CA certificate used to verify the API server's certificate.
type MTLSConfig struct {
	Enable  bool   `yaml:"enable" description:"Enable mutual TLS for scheduler-API communication"`
	CertPem string `yaml:"cert_pem" description:"Path to scheduler client certificate PEM file"`
	KeyPem  string `yaml:"key_pem" description:"Path to scheduler client private key PEM file"`
	CAPem   string `yaml:"ca_pem" description:"Path to CA certificate PEM file for server verification"`
}

// ApiConfig represents API-specific configuration
type ApiConfig struct {
	Url           string     `yaml:"url" description:"Base URL of the Gthulhu API server"`
	Interval      int        `yaml:"interval" description:"Interval in seconds for fetching strategies and sending metrics"`
	PublicKeyPath string     `yaml:"public_key_path" description:"Path to JWT public key for API authentication"`
	Enabled       bool       `yaml:"enabled,omitempty" description:"Enable communication with the API server"`
	AuthEnabled   bool       `yaml:"auth_enabled,omitempty" description:"Enable JWT authentication for API requests"`
	MTLS          MTLSConfig `yaml:"mtls,omitempty" description:"Mutual TLS configuration for API communication"`
}

// Config represents the application configuration
type Config struct {
	Scheduler       SchedulerConfig       `yaml:"scheduler" description:"Scheduler-specific configuration (advanced feature)"`
	SimpleScheduler SimpleSchedulerConfig `yaml:"simple_scheduler,omitempty" description:"Simple scheduler mode configuration"`
	Monitor         MonitorConfig         `yaml:"monitor" description:"Pod-level scheduling metrics monitor (base feature)"`
	Debug           bool                  `yaml:"debug,omitempty" description:"Enable debug mode (pprof server on :6060)"`
	EarlyProcessing bool                  `yaml:"early_processing,omitempty" description:"Enable early processing of tasks in BPF before user-space dispatch"`
	BuiltinIdle     bool                  `yaml:"builtin_idle,omitempty" description:"Enable built-in idle CPU selection in BPF"`
	Api             ApiConfig             `yaml:"api" description:"API server connection configuration"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Debug:           false,
		EarlyProcessing: false,
		Scheduler: SchedulerConfig{
			SliceNsDefault:  20000 * 1000, // 20ms
			SliceNsMin:      1000 * 1000,  // 1ms
			MaxTimeWatchdog: true,
		},
		Monitor: MonitorConfig{
			Enabled:               true, // monitor is the base feature, enabled by default
			BPFObjectPath:         "sched_monitor.bpf.o",
			CollectionIntervalSec: 10,
			MonitorAll:            false,
			StreamEvents:          false,
			PrometheusPort:        9090,
		},
		Api: ApiConfig{},
	}
}

// LoadConfig loads configuration from YAML file or returns default config
func LoadConfig(filename string) (*Config, error) {
	config := DefaultConfig()

	// If no filename provided, return default config
	if filename == "" {
		return config, nil
	}

	// Try to load from file
	file, err := os.Open(filename)
	if err != nil {
		// If file doesn't exist, return default config
		if os.IsNotExist(err) {
			return config, nil
		}
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(config); err != nil {
		return nil, fmt.Errorf("failed to decode YAML config: %w", err)
	}

	return config, nil
}

// GetSchedulerConfig returns the scheduler configuration
func (c *Config) GetSchedulerConfig() SchedulerConfig {
	return c.Scheduler
}

func (c *Config) IsDebugEnabled() bool {
	return c.Debug
}

func (c *Config) IsBuiltinIdleEnabled() bool {
	return c.BuiltinIdle
}

// GetApiConfig returns the API configuration
func (c *Config) GetApiConfig() ApiConfig {
	return c.Api
}

// GetMonitorConfig returns the monitor configuration
func (c *Config) GetMonitorConfig() MonitorConfig {
	return c.Monitor
}

// IsMonitorEnabled returns whether the scheduling event monitor is enabled
func (c *Config) IsMonitorEnabled() bool {
	return c.Monitor.Enabled
}

// IsSchedulerEnabled returns whether the advanced scheduler is enabled.
// The scheduler is considered enabled when a scheduler mode is explicitly set.
func (c *Config) IsSchedulerEnabled() bool {
	return c.Scheduler.Mode != ""
}

// ExplainConfig prints all configuration keys with their descriptions.
func ExplainConfig() string {
	var sb strings.Builder
	sb.WriteString("Gthulhu Configuration Keys:\n\n")
	explainStruct(&sb, reflect.TypeOf(Config{}), "")
	return sb.String()
}

func explainStruct(sb *strings.Builder, t reflect.Type, prefix string) {
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		yamlTag := field.Tag.Get("yaml")
		desc := field.Tag.Get("description")

		// Extract yaml key name (before any comma options)
		yamlKey := yamlTag
		if idx := strings.Index(yamlTag, ","); idx != -1 {
			yamlKey = yamlTag[:idx]
		}

		fullKey := yamlKey
		if prefix != "" {
			fullKey = prefix + "." + yamlKey
		}

		if desc != "" {
			sb.WriteString(fmt.Sprintf("  %-40s %s\n", fullKey, desc))
		}

		// Recurse into nested structs
		ft := field.Type
		if ft.Kind() == reflect.Struct {
			explainStruct(sb, ft, fullKey)
		}
	}
}
