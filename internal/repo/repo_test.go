package repo

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/remote"
	"github.com/InkyQuill/x-skills/internal/skills"
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
	if len(skills) != 1 || skills[0].Identity != "golang-testing" {
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
	if skills[0].Identity != "linkable-name" {
		t.Fatalf("Identity = %q, want archive directory name", skills[0].Identity)
	}
	if skills[0].DeclaredName != "display-only" {
		t.Fatalf("DeclaredName = %q, want display-only", skills[0].DeclaredName)
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
	if skills[0].Identity != "alpha" || skills[1].Identity != "zeta" {
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
	if err := os.WriteFile(filepath.Join(unreadable, "SKILL.md"), []byte("---\nname: unreadable\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	originalReadSkill := readSkill
	t.Cleanup(func() {
		readSkill = originalReadSkill
	})
	readSkill = func(path string) (skills.Document, error) {
		if filepath.Base(path) == "unreadable" {
			return skills.Document{}, fmt.Errorf("simulated unreadable metadata")
		}
		return originalReadSkill(path)
	}

	skills, err := List(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 || skills[0].Identity != "readable" {
		t.Fatalf("skills = %#v, want only readable", skills)
	}
}

func TestListRepoSkillsIncludesSourceMetadata(t *testing.T) {
	home := t.TempDir()
	cfg := config.Default(t.TempDir(), home)
	skill := filepath.Join(cfg.ArchiveSkillsRoot(), "svelte-coder")
	if err := os.MkdirAll(skill, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("---\nname: svelte-coder\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	want := remote.SourceMetadata{
		SchemaVersion: 2,
		SourceType:    remote.SourceTypeGitHub,
		Owner:         "vercel-labs",
		Repo:          "skills",
		CloneURL:      "https://github.com/vercel-labs/skills.git",
		Ref:           "main",
		Commit:        "abc123",
		SkillPath:     "skills/svelte-coder",
		UpstreamName:  "svelte-coder",
	}
	if err := remote.WriteSourceMetadata(skill, want); err != nil {
		t.Fatal(err)
	}

	skills, err := List(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatalf("len(skills) = %d, want 1: %#v", len(skills), skills)
	}
	if skills[0].Source == nil {
		t.Fatal("Source = nil, want metadata")
	}
	if *skills[0].Source != want {
		t.Fatalf("Source = %#v, want %#v", *skills[0].Source, want)
	}
}

func TestListRepoSkillsIgnoresInvalidSourceMetadata(t *testing.T) {
	home := t.TempDir()
	cfg := config.Default(t.TempDir(), home)
	skill := filepath.Join(cfg.ArchiveSkillsRoot(), "svelte-coder")
	if err := os.MkdirAll(skill, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("---\nname: svelte-coder\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skill, remote.MetadataFile), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}

	skills, err := List(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatalf("len(skills) = %d, want 1: %#v", len(skills), skills)
	}
	if skills[0].Source != nil {
		t.Fatalf("Source = %#v, want nil", skills[0].Source)
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
