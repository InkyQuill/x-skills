package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrateWithYesFlag(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	active := makeSkill(t, filepath.Join(project, ".codex", "skills"), "local-only", "Local.")

	var out bytes.Buffer
	err := Execute([]string{"--home", home, "--project-root", project, "-y", "migrate", "local-only", "--project", "--target", "codex"}, strings.NewReader(""), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	archived := filepath.Join(home, ".x-skills", "skills", "local-only")
	resolved, err := filepath.EvalSymlinks(active)
	if err != nil {
		t.Fatal(err)
	}
	if resolved != archived {
		t.Fatalf("resolved = %q, want %q", resolved, archived)
	}
}

func TestMigrateWithoutYesReturnsConfirmationError(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	makeSkill(t, filepath.Join(project, ".codex", "skills"), "local-only", "Local.")

	var stderr bytes.Buffer
	err := Execute([]string{"--home", home, "--project-root", project, "migrate", "local-only", "--project", "--target", "codex"}, strings.NewReader(""), &bytes.Buffer{}, &stderr)
	if err == nil {
		t.Fatal("expected confirmation error")
	}
	if !strings.Contains(err.Error(), "requires confirmation") {
		t.Fatalf("error = %q, want confirmation", err)
	}
}

func TestUnlinkUnmanagedDeleteWithYes(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	active := makeSkill(t, filepath.Join(project, ".codex", "skills"), "local-only", "Local.")

	var out bytes.Buffer
	err := Execute([]string{"--home", home, "--project-root", project, "-y", "unlink", "local-only", "--project", "--target", "codex", "--delete-unmanaged"}, strings.NewReader(""), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(active); !os.IsNotExist(err) {
		t.Fatalf("active still exists or unexpected err: %v", err)
	}
	if !strings.Contains(out.String(), "removed unmanaged") {
		t.Fatalf("unlink output:\n%s", out.String())
	}
}

func TestUnlinkDeleteUnmanagedWithoutYesReturnsConfirmationError(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	makeSkill(t, filepath.Join(project, ".codex", "skills"), "local-only", "Local.")

	var stderr bytes.Buffer
	err := Execute([]string{"--home", home, "--project-root", project, "unlink", "local-only", "--project", "--target", "codex", "--delete-unmanaged"}, strings.NewReader(""), &bytes.Buffer{}, &stderr)
	if err == nil {
		t.Fatal("expected confirmation error")
	}
	if !strings.Contains(err.Error(), "requires confirmation") {
		t.Fatalf("error = %q, want confirmation", err)
	}
}
