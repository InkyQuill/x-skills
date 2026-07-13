package buildinfo

import "testing"

func TestInfoDisplayAndRelease(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		want      string
		isRelease bool
	}{
		{name: "empty", want: "dev"},
		{name: "development", raw: "dev", want: "dev"},
		{name: "prefixed stable", raw: "v1.2.3", want: "v1.2.3", isRelease: true},
		{name: "unprefixed stable", raw: "1.2.3", want: "v1.2.3", isRelease: true},
		{name: "malformed", raw: "main", want: "dev"},
		{name: "prerelease", raw: "v1.2.3-rc.1", want: "dev"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := New(tt.raw)
			if got := info.Display(); got != tt.want {
				t.Fatalf("Display() = %q, want %q", got, tt.want)
			}
			if got := info.IsRelease(); got != tt.isRelease {
				t.Fatalf("IsRelease() = %t, want %t", got, tt.isRelease)
			}
		})
	}
}

func TestInfoNewerStable(t *testing.T) {
	tests := []struct {
		name, current, latest, want string
		available                   bool
	}{
		{name: "newer", current: "v1.2.3", latest: "v1.3.0", want: "v1.3.0", available: true},
		{name: "missing prefix", current: "1.2.3", latest: "1.2.4", want: "v1.2.4", available: true},
		{name: "equal", current: "v1.2.3", latest: "v1.2.3"},
		{name: "older", current: "v1.2.3", latest: "v1.1.9"},
		{name: "dev", current: "dev", latest: "v9.0.0"},
		{name: "prerelease", current: "v1.2.3", latest: "v1.3.0-rc.1"},
		{name: "invalid", current: "v1.2.3", latest: "main"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, available := New(tt.current).NewerStable(tt.latest)
			if got != tt.want || available != tt.available {
				t.Fatalf("NewerStable(%q) = (%q, %t), want (%q, %t)",
					tt.latest, got, available, tt.want, tt.available)
			}
		})
	}
}
