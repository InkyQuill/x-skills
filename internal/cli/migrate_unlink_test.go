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

func TestUnlinkWithYesDefaultsToAllMatchingActiveRoots(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	agents := makeSkill(t, filepath.Join(home, ".agents", "skills"), "code-review", "Review.")
	claudeRoot := filepath.Join(home, ".claude", "skills")
	if err := os.MkdirAll(claudeRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	claude := filepath.Join(claudeRoot, "code-review")
	if err := os.Symlink(agents, claude); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	err := Execute([]string{"--home", home, "--project-root", project, "-y", "unlink", "code-review", "--delete-unmanaged"}, strings.NewReader(""), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(agents); !os.IsNotExist(err) {
		t.Fatalf("agents skill still exists or unexpected err: %v", err)
	}
	if _, err := os.Lstat(claude); !os.IsNotExist(err) {
		t.Fatalf("claude link still exists or unexpected err: %v", err)
	}
	if strings.Contains(out.String(), "failed") {
		t.Fatalf("unlink output contains failure:\n%s", out.String())
	}
}

func TestUnlinkGlobalWithYesDefaultsToGlobalMatchingRoots(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	global := makeSkill(t, filepath.Join(home, ".agents", "skills"), "commit-context", "Context.")
	projectSkill := makeSkill(t, filepath.Join(project, ".agents", "skills"), "commit-context", "Project.")

	var out bytes.Buffer
	err := Execute([]string{"--home", home, "--project-root", project, "-y", "unlink", "--global", "commit-context", "--delete-unmanaged"}, strings.NewReader(""), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(global); !os.IsNotExist(err) {
		t.Fatalf("global skill still exists or unexpected err: %v", err)
	}
	if _, err := os.Stat(projectSkill); err != nil {
		t.Fatalf("project skill should remain: %v", err)
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
