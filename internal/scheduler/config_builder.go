package scheduler

import (
	"os"

	"github.com/Gthulhu/Gthulhu/internal/config"
	"github.com/Gthulhu/Gthulhu/monitor"
	"github.com/Gthulhu/plugin/plugin"
)

func buildMonitorConfig(cfg *config.Config) monitor.Config {
	return monitor.Config{
		BPFObjectPath:         cfg.Monitor.BPFObjectPath,
		CollectionIntervalSec: cfg.Monitor.CollectionIntervalSec,
		MonitorAll:            cfg.Monitor.MonitorAll,
		StreamEvents:          cfg.Monitor.StreamEvents,
		PrometheusPort:        cfg.Monitor.PrometheusPort,
		NodeName:              os.Getenv("NODE_NAME"),
		EnableCRDWatcher:      cfg.Monitor.EnableCRDWatcher,
		KubeConfigPath:        cfg.Monitor.KubeConfigPath,
	}
}

func buildPluginConfig(cfg *config.Config) *plugin.SchedConfig {
	schedConfig := cfg.GetSchedulerConfig()
	pluginConfig := &plugin.SchedConfig{
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
	if pluginConfig.Mode == "" {
		pluginConfig.Mode = "gthulhu"
	}
	return pluginConfig
}
