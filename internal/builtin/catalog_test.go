package builtin

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/InkyQuill/x-skills/internal/config"
)

func TestListReturnsCanonicalBuiltInSkills(t *testing.T) {
	skills, err := List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	var names []string
	for _, skill := range skills {
		names = append(names, skill.Name)
	}
	want := []string{"x-find-skills", "x-manage-skills", "x-port-skill"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("List() names = %v, want %v", names, want)
	}
}

func TestListRejectsNonPrefixedEmbeddedDirectory(t *testing.T) {
	original := builtInSkills
	builtInSkills = fstest.MapFS{
		"skills/not-built-in/SKILL.md": {Data: []byte("---\nname: not-built-in\n---\n")},
	}
	t.Cleanup(func() { builtInSkills = original })

	if _, err := List(); err == nil {
		t.Fatal("List() error = nil, want invalid built-in name error")
	}
}

func TestArchiveCopiesCompleteBuiltInDirectory(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())

	archived, err := Archive(cfg, []string{"x-manage-skills"})
	if err != nil {
		t.Fatalf("Archive() error = %v", err)
	}
	if want := []string{"x-manage-skills"}; !reflect.DeepEqual(archived, want) {
		t.Fatalf("Archive() = %v, want %v", archived, want)
	}
	for _, rel := range []string{"SKILL.md", filepath.Join("agents", "openai.yaml")} {
		if _, err := os.Stat(filepath.Join(cfg.ArchiveSkillsRoot(), "x-manage-skills", rel)); err != nil {
			t.Fatalf("stat archived %s: %v", rel, err)
		}
	}
}

func TestArchiveLeavesIdenticalArchiveUnchanged(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	if _, err := Archive(cfg, []string{"x-port-skill"}); err != nil {
		t.Fatalf("first Archive() error = %v", err)
	}

	archived, err := Archive(cfg, []string{"x-port-skill"})
	if err != nil {
		t.Fatalf("second Archive() error = %v", err)
	}
	if len(archived) != 0 {
		t.Fatalf("second Archive() = %v, want no changed archives", archived)
	}
}

func TestArchiveRejectsDivergentArchiveWithoutReplacingIt(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	destination := filepath.Join(cfg.ArchiveSkillsRoot(), "x-find-skills")
	if err := os.MkdirAll(destination, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(destination, "SKILL.md")
	if err := os.WriteFile(marker, []byte("local content"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Archive(cfg, []string{"x-find-skills"})
	if !errors.Is(err, ErrArchiveConflict) {
		t.Fatalf("Archive() error = %v, want ErrArchiveConflict", err)
	}
	got, readErr := os.ReadFile(marker)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != "local content" {
		t.Fatalf("divergent archive was replaced: %q", got)
	}
}

func TestArchivePublishDoesNotReplaceConcurrentDestination(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	original := publishArchive
	publishArchive = func(staged, destination string) error {
		if err := os.Mkdir(destination, 0o755); err != nil {
			return err
		}
		return original(staged, destination)
	}
	t.Cleanup(func() { publishArchive = original })

	_, err := Archive(cfg, []string{"x-find-skills"})
	if !errors.Is(err, ErrArchiveConflict) {
		t.Fatalf("Archive() error = %v, want ErrArchiveConflict", err)
	}
	entries, readErr := os.ReadDir(filepath.Join(cfg.ArchiveSkillsRoot(), "x-find-skills"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("concurrent destination was replaced with %d entries", len(entries))
	}
}

func TestArchiveCleansTempAndPreservesPartialSuccessAfterCopyFailure(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	original := copyBuiltIn
	calls := 0
	copyBuiltIn = func(source, destination string) error {
		calls++
		if calls == 2 {
			if err := os.WriteFile(filepath.Join(destination, "partial"), []byte("partial"), 0o644); err != nil {
				return err
			}
			return fmt.Errorf("injected copy failure")
		}
		return original(source, destination)
	}
	t.Cleanup(func() { copyBuiltIn = original })

	archived, err := Archive(cfg, []string{"x-find-skills", "x-manage-skills"})
	if err == nil || !strings.Contains(err.Error(), "injected copy failure") {
		t.Fatalf("Archive() error = %v, want injected copy failure", err)
	}
	if want := []string{"x-find-skills"}; !reflect.DeepEqual(archived, want) {
		t.Fatalf("Archive() = %v, want %v", archived, want)
	}
	if _, err := os.Stat(filepath.Join(cfg.ArchiveSkillsRoot(), "x-find-skills", "SKILL.md")); err != nil {
		t.Fatalf("first archive was not preserved: %v", err)
	}
	entries, err := os.ReadDir(cfg.ArchiveSkillsRoot())
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".x-manage-skills-") {
			t.Fatalf("temporary directory was not cleaned up: %s", entry.Name())
		}
	}
}

func TestArchiveRejectsInvalidAndUnknownNames(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	for _, name := range []string{"../x-find-skills", "find-skills", "x-unknown"} {
		t.Run(name, func(t *testing.T) {
			if _, err := Archive(cfg, []string{name}); err == nil {
				t.Fatalf("Archive(%q) error = nil", name)
			}
		})
	}
}
