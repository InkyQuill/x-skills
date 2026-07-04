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
