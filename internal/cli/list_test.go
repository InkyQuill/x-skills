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
		"list", "--project", "--target", "codex",
	}, strings.NewReader(""), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{"PROJECT codex", "./.codex", "managed-codex", "managed", "Managed codex skill."} {
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

func TestListRejectsUnknownTarget(t *testing.T) {
	var out bytes.Buffer
	var stderr bytes.Buffer
	err := Execute([]string{"list", "--target", "bogus"}, strings.NewReader(""), &out, &stderr)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "unknown target") {
		t.Fatalf("error = %q, want unknown target", err)
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", out.String())
	}
}
