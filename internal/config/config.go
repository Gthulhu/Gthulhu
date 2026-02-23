// SPDX-FileCopyrightText: 2025 Gthulhu Team
//
// SPDX-License-Identifier: Apache-2.0
// Author: Ian Chen <ychen.desl@gmail.com>

package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// SchedulerConfig represents scheduler-specific configuration
type SchedulerConfig struct {
	SliceNsDefault  uint64 `yaml:"slice_ns_default"`
	SliceNsMin      uint64 `yaml:"slice_ns_min"`
	Mode            string `yaml:"mode,omitempty"` // Optional mode field
	KernelMode      bool   `yaml:"kernel_mode,omitempty"`
	MaxTimeWatchdog bool   `yaml:"max_time_watchdog,omitempty"`
}

type SimpleSchedulerConfig struct {
	EnableFifo bool `yaml:"enable_fifo,omitempty"` // Optional FIFO scheduling flag
}

// MTLSConfig holds the mutual TLS configuration used for scheduler → API server communication.
// CertPem and KeyPem are the scheduler's own certificate/key pair signed by the private CA.
// CAPem is the private CA certificate used to verify the API server's certificate.
type MTLSConfig struct {
	Enable  bool   `yaml:"enable"`
	CertPem string `yaml:"cert_pem"`
	KeyPem  string `yaml:"key_pem"`
	CAPem   string `yaml:"ca_pem"`
}

// ApiConfig represents API-specific configuration
type ApiConfig struct {
	Url           string     `yaml:"url"`
	Interval      int        `yaml:"interval"`        // Interval in seconds
	PublicKeyPath string     `yaml:"public_key_path"` // Path to JWT public key for authentication
	Enabled       bool       `yaml:"enabled,omitempty"`
	AuthEnabled   bool       `yaml:"auth_enabled,omitempty"`
	MTLS          MTLSConfig `yaml:"mtls,omitempty"`
}

// Config represents the application configuration
type Config struct {
	Scheduler       SchedulerConfig       `yaml:"scheduler"`
	SimpleScheduler SimpleSchedulerConfig `yaml:"simple_scheduler,omitempty"`
	Debug           bool                  `yaml:"debug,omitempty"`            // Optional debug flag
	EarlyProcessing bool                  `yaml:"early_processing,omitempty"` // Optional early processing flag
	BuiltinIdle     bool                  `yaml:"builtin_idle,omitempty"`     // Optional flag for built-in idle CPU selection
	Api             ApiConfig             `yaml:"api"`
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
