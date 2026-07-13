package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/manifest"
	"github.com/InkyQuill/x-skills/internal/pathidentity"
	"github.com/InkyQuill/x-skills/internal/remote"
)

type repoRecord struct {
	Identity     string                 `json:"identity"`
	DeclaredName string                 `json:"declared_name,omitempty"`
	Description  string                 `json:"description,omitempty"`
	Path         string                 `json:"path"`
	Source       *remote.SourceMetadata `json:"source,omitempty"`
}

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

func TestRepoShowsDeclaredNameOnlyWhenDifferent(t *testing.T) {
	home, project := t.TempDir(), t.TempDir()
	cfg := setupActiveIdentityMismatch(t, home, project)
	makeSkill(t, cfg.ArchiveSkillsRoot(), "matching", "Matching.")

	var out bytes.Buffer
	err := Execute(
		[]string{"--home", home, "--project-root", project, "repo"},
		strings.NewReader(""),
		&out,
		&bytes.Buffer{},
	)
	if err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, "composition-patterns (declared: vercel-composition-patterns)") {
		t.Fatalf("repo output missing divergent declared name:\n%s", text)
	}
	if strings.Contains(text, "matching (declared:") {
		t.Fatalf("repo output repeats matching declared name:\n%s", text)
	}
}

func TestRepoJSON(t *testing.T) {
	home, project := t.TempDir(), t.TempDir()
	cfg := setupActiveIdentityMismatch(t, home, project)
	archivePath := filepath.Join(cfg.ArchiveSkillsRoot(), "composition-patterns")
	source := remote.SourceMetadata{
		SourceType: remote.SourceTypeGitHub,
		Owner:      "InkyQuill",
		Repo:       "skills",
		CloneURL:   "https://github.com/InkyQuill/skills.git",
		Commit:     "abc123",
		SkillPath:  "skills/composition-patterns",
	}
	if err := remote.WriteSourceMetadata(archivePath, source); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	err := Execute(
		[]string{"--home", home, "--project-root", project, "--json", "repo"},
		strings.NewReader(""),
		&out,
		&bytes.Buffer{},
	)
	if err != nil {
		t.Fatal(err)
	}
	var records []repoRecord
	if err := json.Unmarshal(out.Bytes(), &records); err != nil {
		t.Fatalf("unmarshal repo JSON: %v\n%s", err, out.String())
	}
	if len(records) != 1 {
		t.Fatalf("records = %#v, want one", records)
	}
	record := records[0]
	if record.Identity != "composition-patterns" || record.DeclaredName != "vercel-composition-patterns" ||
		record.Description != "Compose." || record.Path != archivePath {
		t.Fatalf("record = %#v", record)
	}
	if record.Source == nil || record.Source.SourceType != remote.SourceTypeGitHub ||
		record.Source.Owner != "InkyQuill" || record.Source.Repo != "skills" ||
		record.Source.SkillPath != "skills/composition-patterns" {
		t.Fatalf("source = %#v", record.Source)
	}
	if strings.Contains(out.String(), "\x1b[") {
		t.Fatalf("repo JSON contains ANSI styling: %q", out.String())
	}
}

func TestRepoJSONOmitsMatchingDeclaredName(t *testing.T) {
	home, project := t.TempDir(), t.TempDir()
	makeSkill(t, filepath.Join(home, ".x-skills", "skills"), "matching", "Matching.")

	var out bytes.Buffer
	err := Execute(
		[]string{"--home", home, "--project-root", project, "--json", "repo"},
		strings.NewReader(""),
		&out,
		&bytes.Buffer{},
	)
	if err != nil {
		t.Fatal(err)
	}
	var raw []map[string]any
	if err := json.Unmarshal(out.Bytes(), &raw); err != nil {
		t.Fatalf("unmarshal repo JSON: %v\n%s", err, out.String())
	}
	if len(raw) != 1 {
		t.Fatalf("records = %#v, want one", raw)
	}
	if _, ok := raw[0]["declared_name"]; ok {
		t.Fatalf("matching declared_name present: %#v", raw[0])
	}
}

func TestRepoJSONEmptyArray(t *testing.T) {
	var out bytes.Buffer
	err := Execute(
		[]string{"--home", t.TempDir(), "--project-root", t.TempDir(), "--json", "repo"},
		strings.NewReader(""),
		&out,
		&bytes.Buffer{},
	)
	if err != nil {
		t.Fatal(err)
	}
	var records []repoRecord
	if err := json.Unmarshal(out.Bytes(), &records); err != nil {
		t.Fatalf("unmarshal repo JSON: %v\n%s", err, out.String())
	}
	if records == nil || len(records) != 0 {
		t.Fatalf("records = %#v, want non-nil empty slice", records)
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

func TestLinkReconcilesExistingDeclaredNameMismatchByIdentity(t *testing.T) {
	home, project := t.TempDir(), t.TempDir()
	cfg := setupActiveIdentityMismatch(t, home, project)
	archive := makeSkill(t, cfg.ArchiveSkillsRoot(), "other", "Other.")
	active := filepath.Join(cfg.MustActiveRoot(config.ScopeProject, config.TargetCodex), "other")

	err := Execute(
		[]string{
			"--home", home,
			"--project-root", project,
			"link", "other",
			"--at", "project:codex",
		},
		strings.NewReader(""),
		&bytes.Buffer{},
		&bytes.Buffer{},
	)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(active)
	if err != nil {
		t.Fatalf("linked skill %q: %v", active, err)
	}
	assertSamePath(t, resolved, archive)
	assertLocalManifestHasIdentity(t, cfg, "composition-patterns")
}

func setupActiveIdentityMismatch(t *testing.T, home, project string) config.Config {
	t.Helper()
	cfg := config.Default(project, home)
	archive := makeSkill(t, cfg.ArchiveSkillsRoot(), "composition-patterns", "Compose.")
	content := []byte("---\nname: vercel-composition-patterns\ndescription: Compose.\n---\n")
	if err := os.WriteFile(filepath.Join(archive, "SKILL.md"), content, 0o644); err != nil {
		t.Fatal(err)
	}
	activeRoot := cfg.MustActiveRoot(config.ScopeProject, config.TargetAgents)
	if err := os.MkdirAll(activeRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(archive, filepath.Join(activeRoot, "composition-patterns")); err != nil {
		t.Fatal(err)
	}
	return cfg
}

func assertLocalManifestHasIdentity(t *testing.T, cfg config.Config, identity string) {
	t.Helper()
	local, err := manifest.LoadLocal(cfg.ProjectRoot)
	if err != nil {
		t.Fatal(err)
	}
	for _, skill := range local.Skills {
		if skill.Name == identity {
			return
		}
	}
	t.Fatalf("local manifest skills = %#v, want identity %q", local.Skills, identity)
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
