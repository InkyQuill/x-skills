package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
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

func TestVersionCommandSkipsGlobalConfig(t *testing.T) {
	home := t.TempDir()
	configPath := filepath.Join(home, ".x-skills", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("version: 99\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	err := Execute(
		[]string{"--home", home, "version"},
		strings.NewReader(""),
		&stdout,
		&bytes.Buffer{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := stdout.String(), "dev\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}
