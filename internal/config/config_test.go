// SPDX-FileCopyrightText: 2025 Gthulhu Team
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"strings"
	"testing"
)

func TestExplainConfigContainsAllKeys(t *testing.T) {
	output := ExplainConfig()

	expectedKeys := []string{
		"scheduler.slice_ns_default",
		"scheduler.slice_ns_min",
		"scheduler.mode",
		"scheduler.kernel_mode",
		"scheduler.max_time_watchdog",
		"simple_scheduler.enable_fifo",
		"debug",
		"early_processing",
		"builtin_idle",
		"api.url",
		"api.interval",
		"api.public_key_path",
		"api.enabled",
		"api.auth_enabled",
		"api.mtls.enable",
		"api.mtls.cert_pem",
		"api.mtls.key_pem",
		"api.mtls.ca_pem",
	}

	for _, key := range expectedKeys {
		if !strings.Contains(output, key) {
			t.Errorf("ExplainConfig output missing key %q", key)
		}
	}
}

func TestExplainConfigContainsDescriptions(t *testing.T) {
	output := ExplainConfig()

	expectedDescriptions := []string{
		"Default time slice in nanoseconds",
		"Minimum time slice in nanoseconds",
		"Enable debug mode",
		"Base URL of the Gthulhu API server",
		"Enable mutual TLS",
	}

	for _, desc := range expectedDescriptions {
		if !strings.Contains(output, desc) {
			t.Errorf("ExplainConfig output missing description %q", desc)
		}
	}
}

func TestExplainConfigHeader(t *testing.T) {
	output := ExplainConfig()
	if !strings.HasPrefix(output, "Gthulhu Configuration Keys:") {
		t.Error("ExplainConfig output should start with header")
	}
}
