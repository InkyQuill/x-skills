package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListShowsStatuses(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	archive := filepath.Join(home, ".x-skills", "skills")
	managed := makeSkill(t, archive, "managed-codex", "Managed codex skill.")
	root := filepath.Join(project, ".codex", "skills")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(managed, filepath.Join(root, "managed-codex")); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	err := Execute([]string{
		"--project-root", project,
		"--home", home,
		"list", "--at", "project:codex",
	}, strings.NewReader(""), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{"PROJECT codex", ".Cd", "managed-codex", "managed", "Managed codex skill."} {
		if !strings.Contains(text, want) {
			t.Fatalf("list output missing %q:\n%s", want, text)
		}
	}
}

func TestListRejectsUnexpectedArgs(t *testing.T) {
	var out bytes.Buffer
	var stderr bytes.Buffer
	err := Execute([]string{"list", "unexpected"}, strings.NewReader(""), &out, &stderr)
	if err == nil {
		t.Fatal("expected error")
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", out.String())
	}
}

func TestListRejectsInvalidGlobalConfig(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	configPath := filepath.Join(home, ".x-skills", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("version: 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	var stderr bytes.Buffer
	err := Execute([]string{"--home", home, "--project-root", project, "list"}, strings.NewReader(""), &out, &stderr)
	if err == nil || !strings.Contains(err.Error(), "unsupported version 0") {
		t.Fatalf("err = %v, want unsupported version 0", err)
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", out.String())
	}
}

func TestListRejectsUnknownLocation(t *testing.T) {
	var out bytes.Buffer
	var stderr bytes.Buffer
	err := Execute([]string{"list", "--at", "project:bogus"}, strings.NewReader(""), &out, &stderr)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "unknown --at location") {
		t.Fatalf("error = %q, want unknown --at location", err)
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", out.String())
	}
}
