package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/InkyQuill/x-skills/internal/pathidentity"
)

// assertSamePath verifies two paths identify the same filesystem location.
func assertSamePath(t *testing.T, got, want string) {
	t.Helper()
	ok, err := pathidentity.EquivalentE(got, want)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatalf("path = %q, want same location as %q", got, want)
	}
}

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
	err := Execute([]string{"--home", home, "--project-root", project, "link", "first-skill", "second-skill", "--at", "project:codex"}, strings.NewReader(""), &out, &bytes.Buffer{})
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
		assertSamePath(t, resolved, source)
	}
}

func TestLinkAcceptsMultipleLocations(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	source := makeSkill(t, filepath.Join(home, ".x-skills", "skills"), "typescript-expert", "TypeScript.")

	var out bytes.Buffer
	err := Execute([]string{"--home", home, "--project-root", project, "link", "typescript-expert", "--at", ".Ag", "--at", "~Cd"}, strings.NewReader(""), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{
		filepath.Join(project, ".agents", "skills", "typescript-expert"),
		filepath.Join(home, ".codex", "skills", "typescript-expert"),
	} {
		resolved, err := filepath.EvalSymlinks(target)
		if err != nil {
			t.Fatal(err)
		}
		assertSamePath(t, resolved, source)
	}
	if !strings.Contains(out.String(), "Summary:") || !strings.Contains(out.String(), "linked: typescript-expert, typescript-expert") {
		t.Fatalf("link output:\n%s", out.String())
	}
}

func TestLinkSupportsConfiguredCustomTarget(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	configPath := filepath.Join(home, ".x-skills", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("active_roots:\n  - scope: project\n    target: opencode\n    path: .opencode/skills\n    label: .Oc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	source := makeSkill(t, filepath.Join(home, ".x-skills", "skills"), "typescript-expert", "TypeScript.")

	var out bytes.Buffer
	err := Execute([]string{"--home", home, "--project-root", project, "link", "typescript-expert", "--at", ".Oc"}, strings.NewReader(""), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(project, ".opencode", "skills", "typescript-expert")
	resolved, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}
	assertSamePath(t, resolved, source)
	if !strings.Contains(out.String(), "linked: typescript-expert") {
		t.Fatalf("link output:\n%s", out.String())
	}
}

func TestLinkBatchReportsPartialFailureAndContinues(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	archive := filepath.Join(home, ".x-skills", "skills")
	second := makeSkill(t, archive, "second-skill", "Second.")

	var out bytes.Buffer
	err := Execute([]string{"--home", home, "--project-root", project, "link", "missing-skill", "second-skill", "--at", "project:codex"}, strings.NewReader(""), &out, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected partial failure error")
	}

	target := filepath.Join(project, ".codex", "skills", "second-skill")
	resolved, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}
	assertSamePath(t, resolved, second)
	if _, err := os.Lstat(filepath.Join(project, ".codex", "skills", "missing-skill")); !os.IsNotExist(err) {
		t.Fatalf("missing-skill stat error = %v, want not exist", err)
	}

	text := out.String()
	for _, want := range []string{"Summary:", "linked: second-skill", "failed: missing-skill ("} {
		if !strings.Contains(text, want) {
			t.Fatalf("link output missing %q:\n%s", want, text)
		}
	}
}

func TestLinkFailsNoInputWhenDestinationIsAmbiguous(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	makeSkill(t, filepath.Join(home, ".x-skills", "skills"), "typescript-expert", "TypeScript.")

	var stderr bytes.Buffer
	err := Execute([]string{"--home", home, "--project-root", project, "--no-input", "link", "typescript-expert"}, strings.NewReader(""), &bytes.Buffer{}, &stderr)
	if err == nil {
		t.Fatal("expected ambiguous destination error")
	}
	if !strings.Contains(err.Error(), "choose a destination") {
		t.Fatalf("error = %q, want choose a destination", err)
	}
	if !strings.Contains(err.Error(), "x-skills link typescript-expert --at project:codex") {
		t.Fatalf("error missing one-shot hint: %v", err)
	}
}

func TestLinkPromptsForAmbiguousDestination(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	source := makeSkill(t, filepath.Join(home, ".x-skills", "skills"), "typescript-expert", "TypeScript.")

	var out bytes.Buffer
	err := Execute([]string{"--home", home, "--project-root", project, "link", "typescript-expert"}, strings.NewReader("3\n"), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Select destination for link [1-6]:") {
		t.Fatalf("link output missing prompt:\n%s", out.String())
	}
	target := filepath.Join(project, ".codex", "skills", "typescript-expert")
	resolved, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}
	assertSamePath(t, resolved, source)
}
