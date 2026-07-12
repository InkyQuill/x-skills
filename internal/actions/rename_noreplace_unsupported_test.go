//go:build !linux && !darwin && !windows

package actions

import (
	"runtime"
	"strings"
	"testing"
)

func TestRenameNoReplaceReportsUnsupportedPlatform(t *testing.T) {
	err := renameNoReplace("old", "new")
	if err == nil {
		t.Fatal("expected unsupported platform error")
	}
	if !strings.Contains(err.Error(), "unsupported on "+runtime.GOOS) {
		t.Fatalf("error = %q, want unsupported platform", err)
	}
}
