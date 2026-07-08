package policy

import "github.com/Gthulhu/Gthulhu/internal/config"

// ShouldRunMonitorOnly decides whether scheduler should fallback to monitor-only mode.
// It returns (monitorOnly, err). If err is non-nil, caller should abort startup.
func ShouldRunMonitorOnly(cfg *config.Config, schedExtErr error) (bool, error) {
	if !cfg.IsSchedulerEnabled() {
		return true, nil
	}
	if schedExtErr == nil {
		return false, nil
	}
	if cfg.IsMonitorEnabled() {
		return true, nil
	}
	return false, schedExtErr
}
