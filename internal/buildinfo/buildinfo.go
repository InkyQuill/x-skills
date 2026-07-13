package buildinfo

import (
	"strings"

	"golang.org/x/mod/semver"
)

var version = "dev"

type Info struct{ version string }

func Current() Info { return New(version) }

func New(raw string) Info {
	normalized := normalizeStable(raw)
	if normalized == "" {
		normalized = "dev"
	}
	return Info{version: normalized}
}

func (i Info) Display() string {
	if i.version == "" {
		return "dev"
	}
	return i.version
}

func (i Info) IsRelease() bool { return i.Display() != "dev" }

func (i Info) NewerStable(raw string) (string, bool) {
	latest := normalizeStable(raw)
	if !i.IsRelease() || latest == "" || semver.Compare(latest, i.version) <= 0 {
		return "", false
	}
	return latest, true
}

func normalizeStable(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" || value == "dev" {
		return ""
	}
	if !strings.HasPrefix(value, "v") {
		value = "v" + value
	}
	if !semver.IsValid(value) || semver.Prerelease(value) != "" {
		return ""
	}
	return value
}
