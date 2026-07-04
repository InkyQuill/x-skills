package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadSkillDescription(t *testing.T) {
	dir := t.TempDir()
	skill := filepath.Join(dir, "react-state")
	if err := os.MkdirAll(skill, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("---\nname: react-state\ndescription: Manage React state.\n---\n# Body\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	info, err := Read(skill)
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "react-state" {
		t.Fatalf("Name = %q", info.Name)
	}
	if info.Description != "Manage React state." {
		t.Fatalf("Description = %q", info.Description)
	}
}

func TestReadUsesDirectoryNameWhenNameMissing(t *testing.T) {
	dir := t.TempDir()
	skill := filepath.Join(dir, "fallback-name")
	if err := os.MkdirAll(skill, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("---\ndescription: Uses fallback.\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	info, err := Read(skill)
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "fallback-name" {
		t.Fatalf("Name = %q", info.Name)
	}
	if info.Description != "Uses fallback." {
		t.Fatalf("Description = %q", info.Description)
	}
}

func TestValidateRejectsMissingSkillMD(t *testing.T) {
	dir := t.TempDir()
	_, err := Read(dir)
	if err == nil {
		t.Fatal("expected error for missing SKILL.md")
	}
}

func TestIsDirRequiresSkillMD(t *testing.T) {
	dir := t.TempDir()
	skill := filepath.Join(dir, "skill")
	if err := os.MkdirAll(skill, 0o755); err != nil {
		t.Fatal(err)
	}
	if IsDir(skill) {
		t.Fatal("IsDir returned true without SKILL.md")
	}

	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if !IsDir(skill) {
		t.Fatal("IsDir returned false with SKILL.md")
	}
}
