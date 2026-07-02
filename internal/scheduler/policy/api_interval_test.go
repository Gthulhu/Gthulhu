package policy

import "testing"

func TestNormalizeAPIInterval(t *testing.T) {
	tests := []struct {
		name     string
		interval int
		enabled  bool
		want     int
	}{
		{name: "enabled positive interval", interval: 9, enabled: true, want: 9},
		{name: "enabled non-positive interval", interval: 0, enabled: true, want: 5},
		{name: "api disabled", interval: 100, enabled: false, want: 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeAPIInterval(tt.interval, tt.enabled); got != tt.want {
				t.Fatalf("NormalizeAPIInterval(%d,%v)=%d, want %d", tt.interval, tt.enabled, got, tt.want)
			}
		})
	}
}
