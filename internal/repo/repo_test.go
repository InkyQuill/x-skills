package repo

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/InkyQuill/x-skills/internal/config"
)

func TestListRepoSkills(t *testing.T) {
	home := t.TempDir()
	cfg := config.Default(t.TempDir(), home)
	skill := filepath.Join(cfg.ArchiveSkillsRoot(), "golang-testing")
	if err := os.MkdirAll(skill, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("---\nname: golang-testing\ndescription: Test Go.\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	skills, err := List(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 || skills[0].Name != "golang-testing" {
		t.Fatalf("skills = %#v", skills)
	}
}

func TestListRepoSkillsIgnoresNonSkillsAndSortsByName(t *testing.T) {
	home := t.TempDir()
	cfg := config.Default(t.TempDir(), home)
	archive := cfg.ArchiveSkillsRoot()
	for _, name := range []string{"zeta", "alpha"} {
		skill := filepath.Join(archive, name)
		if err := os.MkdirAll(skill, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("---\nname: "+name+"\n---\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(archive, "not-a-skill"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(archive, "file.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	skills, err := List(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 2 {
		t.Fatalf("len(skills) = %d, want 2: %#v", len(skills), skills)
	}
	if skills[0].Name != "alpha" || skills[1].Name != "zeta" {
		t.Fatalf("skills = %#v", skills)
	}
}
