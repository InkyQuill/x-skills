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

func TestListRepoSkillsUsesArchiveDirectoryName(t *testing.T) {
	home := t.TempDir()
	cfg := config.Default(t.TempDir(), home)
	skill := filepath.Join(cfg.ArchiveSkillsRoot(), "linkable-name")
	if err := os.MkdirAll(skill, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("---\nname: display-only\ndescription: Test Go.\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	skills, err := List(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatalf("len(skills) = %d, want 1", len(skills))
	}
	if skills[0].Name != "linkable-name" {
		t.Fatalf("Name = %q, want archive directory name", skills[0].Name)
	}
	if skills[0].Description != "Test Go." {
		t.Fatalf("Description = %q", skills[0].Description)
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

func TestListRepoSkillsSkipsUnreadableSkillMetadata(t *testing.T) {
	home := t.TempDir()
	cfg := config.Default(t.TempDir(), home)
	readable := filepath.Join(cfg.ArchiveSkillsRoot(), "readable")
	if err := os.MkdirAll(readable, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(readable, "SKILL.md"), []byte("---\nname: readable\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	unreadable := filepath.Join(cfg.ArchiveSkillsRoot(), "unreadable")
	if err := os.MkdirAll(unreadable, 0o755); err != nil {
		t.Fatal(err)
	}
	unreadableSkill := filepath.Join(unreadable, "SKILL.md")
	if err := os.WriteFile(unreadableSkill, []byte("---\nname: unreadable\n---\n"), 0o000); err != nil {
		t.Fatal(err)
	}

	skills, err := List(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 || skills[0].Name != "readable" {
		t.Fatalf("skills = %#v, want only readable", skills)
	}
}

func TestSkillPath(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())

	got, err := SkillPath(cfg, "golang-testing")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(cfg.ArchiveSkillsRoot(), "golang-testing")
	if got != want {
		t.Fatalf("SkillPath() = %q, want %q", got, want)
	}
}

func TestSkillPathRejectsInvalidName(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())

	if _, err := SkillPath(cfg, "../outside"); err == nil {
		t.Fatal("expected invalid skill name error")
	}
}

func TestDeleteSkillRemovesArchivedSkill(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	path := filepath.Join(cfg.ArchiveSkillsRoot(), "golang-testing")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "SKILL.md"), []byte("---\nname: golang-testing\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := DeleteSkill(cfg, "golang-testing")
	if err != nil {
		t.Fatal(err)
	}
	if got != path {
		t.Fatalf("DeleteSkill() path = %q, want %q", got, path)
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("archived skill still exists or unexpected error: %v", err)
	}
}

func TestDeleteSkillRejectsInvalidName(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())

	if _, err := DeleteSkill(cfg, "../outside"); err == nil {
		t.Fatal("expected invalid skill name error")
	}
}
