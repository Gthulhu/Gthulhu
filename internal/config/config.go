package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// SchedulerConfig represents scheduler-specific configuration
type SchedulerConfig struct {
	SliceNsDefault uint64 `yaml:"slice_ns_default"`
	SliceNsMin     uint64 `yaml:"slice_ns_min"`
}

// Config represents the application configuration
type Config struct {
	Scheduler SchedulerConfig `yaml:"scheduler"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Scheduler: SchedulerConfig{
			SliceNsDefault: 5000 * 1000, // 5ms
			SliceNsMin:     500 * 1000,  // 0.5ms
		},
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
