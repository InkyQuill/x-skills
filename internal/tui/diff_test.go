package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildDirectoryDiffShowsFullFileUnifiedDiff(t *testing.T) {
	active := t.TempDir()
	archive := t.TempDir()
	makeSkill(t, active, "zen-of-go", "Active.")
	makeSkill(t, archive, "zen-of-go", "Archived.")

	diff, err := buildDirectoryDiff(filepath.Join(active, "zen-of-go"), filepath.Join(archive, "zen-of-go"))
	if err != nil {
		t.Fatal(err)
	}
	if len(diff.Files) == 0 {
		t.Fatal("diff files is empty")
	}
	text := diff.Files[0].Text
	for _, want := range []string{"---", " name: zen-of-go", "-description: Archived.", "+description: Active."} {
		if !strings.Contains(text, want) {
			t.Fatalf("diff text missing %q:\n%s", want, text)
		}
	}
}

func TestBuildDirectoryDiffMarksAddedAndRemovedFiles(t *testing.T) {
	active := t.TempDir()
	archive := t.TempDir()
	makeSkill(t, active, "skill", "Active.")
	makeSkill(t, archive, "skill", "Active.")
	if err := os.WriteFile(filepath.Join(active, "skill", "new.md"), []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(archive, "skill", "old.md"), []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	diff, err := buildDirectoryDiff(filepath.Join(active, "skill"), filepath.Join(archive, "skill"))
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]string{}
	for _, file := range diff.Files {
		kinds[file.Path] = file.Kind
	}
	if kinds["new.md"] != "added" {
		t.Fatalf("new.md kind = %q, want added", kinds["new.md"])
	}
	if kinds["old.md"] != "removed" {
		t.Fatalf("old.md kind = %q, want removed", kinds["old.md"])
	}
}

func TestBuildDirectoryDiffShowsBinaryMetadata(t *testing.T) {
	active := t.TempDir()
	archive := t.TempDir()
	makeSkill(t, active, "skill", "Active.")
	makeSkill(t, archive, "skill", "Active.")
	if err := os.WriteFile(filepath.Join(active, "skill", "logo.png"), []byte{0, 1, 2, 3}, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(archive, "skill", "logo.png"), []byte{0, 1, 9, 9}, 0o644); err != nil {
		t.Fatal(err)
	}

	diff, err := buildDirectoryDiff(filepath.Join(active, "skill"), filepath.Join(archive, "skill"))
	if err != nil {
		t.Fatal(err)
	}
	var binary string
	for _, file := range diff.Files {
		if file.Path == "logo.png" {
			binary = file.Text
		}
	}
	for _, want := range []string{"Binary file", "archive:", "active:", "sha256:"} {
		if !strings.Contains(binary, want) {
			t.Fatalf("binary metadata missing %q:\n%s", want, binary)
		}
	}
}
