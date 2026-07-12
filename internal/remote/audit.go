package remote

type AuditSummary struct {
	Available bool `json:"available"`
	Alerts    int  `json:"alerts"`
	Critical  int  `json:"critical"`
}

func (a AuditSummary) Pill() string {
	if !a.Available {
		return ""
	}
	if a.Critical > 0 {
		return "‼ risky"
	}
	if a.Alerts > 0 {
		return "⚠ warn"
	}
	return "✓ safe"
}
