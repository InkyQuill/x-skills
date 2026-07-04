package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepoListsArchivedSkills(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	makeSkill(t, filepath.Join(home, ".x-skills", "skills"), "unused-skill", "Not linked.")

	var out bytes.Buffer
	err := Execute([]string{"--home", home, "--project-root", project, "repo"}, strings.NewReader(""), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "unused-skill") || !strings.Contains(out.String(), "Not linked.") {
		t.Fatalf("repo output:\n%s", out.String())
	}
}

func TestLinkAcceptsMultipleNames(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	archive := filepath.Join(home, ".x-skills", "skills")
	first := makeSkill(t, archive, "first-skill", "First.")
	second := makeSkill(t, archive, "second-skill", "Second.")

	var out bytes.Buffer
	err := Execute([]string{"--home", home, "--project-root", project, "link", "first-skill", "second-skill", "--project", "--target", "codex"}, strings.NewReader(""), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Summary:") || !strings.Contains(out.String(), "linked: first-skill, second-skill") {
		t.Fatalf("link output:\n%s", out.String())
	}
	for name, source := range map[string]string{"first-skill": first, "second-skill": second} {
		target := filepath.Join(project, ".codex", "skills", name)
		resolved, err := filepath.EvalSymlinks(target)
		if err != nil {
			t.Fatal(err)
		}
		if resolved != source {
			t.Fatalf("%s resolved to %q, want %q", name, resolved, source)
		}
	}
}
