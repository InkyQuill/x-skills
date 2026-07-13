package cli

import (
	"bytes"
	"testing"

	"github.com/InkyQuill/x-skills/internal/buildinfo"
)

func TestVersionCommandPrintsBuildVersion(t *testing.T) {
	var stdout bytes.Buffer
	cmd := newVersionCommand(buildinfo.New("1.2.3"))
	cmd.SetOut(&stdout)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got, want := stdout.String(), "v1.2.3\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}
