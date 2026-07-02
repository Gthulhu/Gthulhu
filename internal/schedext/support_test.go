package schedext

import "testing"

func TestParseKernelMajorMinor(t *testing.T) {
	tests := []struct {
		name      string
		release   string
		wantMajor int
		wantMinor int
		wantErr   bool
	}{
		{name: "standard kernel", release: "6.12.3", wantMajor: 6, wantMinor: 12, wantErr: false},
		{name: "release suffix", release: "6.14.0-29-generic", wantMajor: 6, wantMinor: 14, wantErr: false},
		{name: "invalid", release: "foo", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			major, minor, err := parseKernelMajorMinor(tt.release)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseKernelMajorMinor(%q) err=%v, wantErr=%v", tt.release, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if major != tt.wantMajor || minor != tt.wantMinor {
				t.Fatalf("parseKernelMajorMinor(%q)=(%d,%d), want (%d,%d)", tt.release, major, minor, tt.wantMajor, tt.wantMinor)
			}
		})
	}
}

func TestCStringToGoString(t *testing.T) {
	got := cStringToGoString([]int8{'6', '.', '1', '2', 0, 'x'})
	if got != "6.12" {
		t.Fatalf("cStringToGoString got %q, want %q", got, "6.12")
	}
}
