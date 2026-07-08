package remote

import "testing"

func TestAuditSummaryPill(t *testing.T) {
	tests := []struct {
		name string
		in   AuditSummary
		want string
	}{
		{name: "safe", in: AuditSummary{Available: true, Alerts: 0}, want: "✓ safe"},
		{name: "warn", in: AuditSummary{Available: true, Alerts: 2}, want: "⚠ warn"},
		{name: "risky", in: AuditSummary{Available: true, Critical: 1}, want: "‼ risky"},
		{name: "missing", in: AuditSummary{}, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.in.Pill(); got != tt.want {
				t.Fatalf("Pill() = %q, want %q", got, tt.want)
			}
		})
	}
}
