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

func TestReadParsesYamlFrontmatter(t *testing.T) {
	dir := t.TempDir()
	skill := filepath.Join(dir, "yaml-skill")
	if err := os.MkdirAll(skill, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: yaml-skill\ndescription: >\n  Manage richer YAML\n  metadata.\ntags:\n  - go\n---\n# Body\n"
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	info, err := Read(skill)
	if err != nil {
		t.Fatal(err)
	}
	if info.Description != "Manage richer YAML metadata.\n" {
		t.Fatalf("Description = %q", info.Description)
	}
}

func TestReadParsesCRLFFrontmatterWithoutCarriageReturn(t *testing.T) {
	dir := t.TempDir()
	skill := filepath.Join(dir, "crlf-skill")
	if err := os.MkdirAll(skill, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\r\nname: crlf-skill\r\ndescription: Uses CRLF metadata.\r\n---\r\n# Body\r\n"
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	info, err := Read(skill)
	if err != nil {
		t.Fatal(err)
	}
	if info.Description != "Uses CRLF metadata." {
		t.Fatalf("Description = %q, want CRLF-free value", info.Description)
	}
}

func TestReadReturnsEmptyDescriptionWhenMissing(t *testing.T) {
	dir := t.TempDir()
	skill := filepath.Join(dir, "no-description")
	if err := os.MkdirAll(skill, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("---\nname: no-description\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	info, err := Read(skill)
	if err != nil {
		t.Fatal(err)
	}
	if info.Description != "" {
		t.Fatalf("Description = %q, want empty", info.Description)
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
