package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/InkyQuill/x-skills/internal/builtin"
	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/doctor"
	"github.com/InkyQuill/x-skills/internal/roots"
)

func TestFilterDoctorGitHygieneByMultipleLocations(t *testing.T) {
	project := t.TempDir()
	cfg := config.Default(project, t.TempDir())
	agents := roots.ActiveRoots(cfg, roots.Filter{Scope: config.ScopeProject, Target: config.TargetAgents})[0]
	claude := roots.ActiveRoots(cfg, roots.Filter{Scope: config.ScopeProject, Target: config.TargetClaude})[0]
	globalAgents := roots.ActiveRoots(cfg, roots.Filter{Scope: config.ScopeGlobal, Target: config.TargetAgents})[0]
	issues := []doctor.Issue{
		{Kind: doctor.KindRecommendedManifestUntracked, Path: filepath.Join(project, ".x-skills.yaml")},
		{Kind: doctor.KindLocalManifestTracked, Path: filepath.Join(project, ".x-skills.local.yaml")},
		{Kind: doctor.KindSkillsFolderTracked, Path: agents.Path},
		{Kind: doctor.KindSkillsFolderTracked, Path: claude.Path},
	}

	tests := []struct {
		name      string
		locations []roots.ActiveRoot
		wantKinds []string
		wantPaths []string
	}{
		{name: "global only excludes project hygiene", locations: []roots.ActiveRoot{globalAgents}, wantKinds: nil, wantPaths: nil},
		{name: "selected project includes manifests and selected folder", locations: []roots.ActiveRoot{agents, globalAgents}, wantKinds: []string{doctor.KindRecommendedManifestUntracked, doctor.KindLocalManifestTracked, doctor.KindSkillsFolderTracked}, wantPaths: []string{filepath.Join(project, ".x-skills.yaml"), filepath.Join(project, ".x-skills.local.yaml"), agents.Path}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterDoctorIssuesByLocations(issues, tt.locations)
			if len(got) != len(tt.wantKinds) {
				t.Fatalf("filtered issues = %#v, want %d", got, len(tt.wantKinds))
			}
			for i := range got {
				if got[i].Kind != tt.wantKinds[i] || got[i].Path != tt.wantPaths[i] {
					t.Fatalf("issue[%d] = %#v, want kind %q path %q", i, got[i], tt.wantKinds[i], tt.wantPaths[i])
				}
			}
		})
	}
}

func TestDoctorMultipleLocationsScopeGitHygiene(t *testing.T) {
	project := t.TempDir()
	home := t.TempDir()
	runDoctorGit(t, project, "init", "--quiet")
	if err := os.WriteFile(filepath.Join(project, ".x-skills.yaml"), []byte("version: 1\nskills: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{".agents/skills/agent/SKILL.md", ".claude/skills/claude/SKILL.md"} {
		path := filepath.Join(project, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("---\nname: test\ndescription: test\n---\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		runDoctorGit(t, project, "add", "--", rel)
	}

	var out bytes.Buffer
	if err := Execute([]string{"--home", home, "--project-root", project, "doctor", "--at", "global:agents", "--at", "global:claude"}, strings.NewReader(""), &out, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "manifest-untracked") || strings.Contains(out.String(), "skills-folder-tracked") {
		t.Fatalf("global-only Doctor leaked project Git hygiene:\n%s", out.String())
	}

	out.Reset()
	if err := Execute([]string{"--home", home, "--project-root", project, "doctor", "--at", "project:agents", "--at", "global:agents"}, strings.NewReader(""), &out, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"recommended-manifest-untracked", filepath.Join(project, ".agents", "skills")} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("mixed-scope Doctor missing %q:\n%s", want, out.String())
		}
	}
	if strings.Contains(out.String(), filepath.Join(project, ".claude", "skills")) {
		t.Fatalf("mixed-scope Doctor leaked unselected project folder:\n%s", out.String())
	}
}

func TestDoctorFixUsesLocationFilteredGitHygieneIssues(t *testing.T) {
	project := t.TempDir()
	cfg := config.Default(project, t.TempDir())
	agents := roots.ActiveRoots(cfg, roots.Filter{Scope: config.ScopeProject, Target: config.TargetAgents})[0]
	claude := roots.ActiveRoots(cfg, roots.Filter{Scope: config.ScopeProject, Target: config.TargetClaude})[0]
	globalAgents := roots.ActiveRoots(cfg, roots.Filter{Scope: config.ScopeGlobal, Target: config.TargetAgents})[0]
	issues := []doctor.Issue{
		{Kind: doctor.KindLocalManifestTracked, Name: ".x-skills.local.yaml", Path: filepath.Join(project, ".x-skills.local.yaml"), ProjectRoot: project, SafeFix: "git rm --cached -- '.x-skills.local.yaml'"},
		{Kind: doctor.KindSkillsFolderTracked, Name: agents.Label, Path: agents.Path, ProjectRoot: project, SafeFix: "git rm -r --cached -- '.agents/skills'"},
		{Kind: doctor.KindSkillsFolderTracked, Name: claude.Label, Path: claude.Path, ProjectRoot: project, SafeFix: "git rm -r --cached -- '.claude/skills'"},
	}
	filtered := filterDoctorIssuesByLocations(issues, []roots.ActiveRoot{agents, globalAgents})
	if _, err := doctor.FixIssues(filtered); err != nil {
		t.Fatal(err)
	}
	ignore, err := os.ReadFile(filepath.Join(project, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(ignore), ".x-skills.local.yaml") || !strings.Contains(string(ignore), "/.agents/skills/") {
		t.Fatalf("selected Git hygiene fixes missing:\n%s", ignore)
	}
	if strings.Contains(string(ignore), "/.claude/skills/") {
		t.Fatalf("unselected project folder was fixed:\n%s", ignore)
	}
}

func TestDoctorReportsGitHygieneAndOnlyFixesGitignore(t *testing.T) {
	project := t.TempDir()
	home := t.TempDir()
	runDoctorGit(t, project, "init", "--quiet")
	runDoctorGit(t, project, "config", "user.name", "Test User")
	runDoctorGit(t, project, "config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(project, ".x-skills.yaml"), []byte("version: 1\nskills: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, ".x-skills.local.yaml"), []byte("version: 1\nskills: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	skill := filepath.Join(project, ".agents", "skills", "tracked", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(skill), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(skill, []byte("---\nname: tracked\ndescription: test\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runDoctorGit(t, project, "add", "--", ".x-skills.local.yaml", ".agents/skills/tracked/SKILL.md")

	var out bytes.Buffer
	if err := Execute([]string{"--home", home, "--project-root", project, "doctor"}, strings.NewReader(""), &out, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"git add -- '.x-skills.yaml'",
		"git rm --cached -- '.x-skills.local.yaml'",
		"git rm -r --cached -- '.agents/skills'",
	} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("doctor output missing %q:\n%s", want, out.String())
		}
	}

	out.Reset()
	if err := Execute([]string{"--home", home, "--project-root", project, "-y", "doctor", "--fix"}, strings.NewReader(""), &out, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"run: git add -- '.x-skills.yaml'",
		"git rm --cached -- '.x-skills.local.yaml'",
		"git rm -r --cached -- '.agents/skills'",
	} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("doctor fix output missing %q:\n%s", want, out.String())
		}
	}
	tracked := runDoctorGit(t, project, "ls-files")
	if !strings.Contains(tracked, ".x-skills.local.yaml") || !strings.Contains(tracked, ".agents/skills/tracked/SKILL.md") {
		t.Fatalf("doctor changed Git index:\n%s", tracked)
	}
}

func TestDoctorQuotesCustomSkillsFolderSuggestionAndEscapesIgnorePattern(t *testing.T) {
	project := t.TempDir()
	home := t.TempDir()
	runDoctorGit(t, project, "init", "--quiet")
	rootRel := "team skills;$(touch nope)*[x]?"
	skill := filepath.Join(project, rootRel, "tracked", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(skill), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(skill, []byte("---\nname: tracked\ndescription: test\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	configDir := filepath.Join(home, ".x-skills")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	configData := "active_roots:\n  - scope: project\n    target: team\n    path: '" + rootRel + "'\n    label: .Tm\n"
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(configData), 0o644); err != nil {
		t.Fatal(err)
	}
	runDoctorGit(t, project, "add", "--", filepath.ToSlash(filepath.Join(rootRel, "tracked", "SKILL.md")))

	var out bytes.Buffer
	if err := Execute([]string{"--home", home, "--project-root", project, "doctor"}, strings.NewReader(""), &out, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	wantCommand := "git rm -r --cached -- '" + rootRel + "'"
	if !strings.Contains(out.String(), wantCommand) {
		t.Fatalf("doctor output missing quoted command %q:\n%s", wantCommand, out.String())
	}

	out.Reset()
	if err := Execute([]string{"--home", home, "--project-root", project, "-y", "doctor", "--fix"}, strings.NewReader(""), &out, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	ignore, err := os.ReadFile(filepath.Join(project, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	wantIgnore := `/team\ skills;$(touch\ nope)\*\[x\]\?/`
	if !strings.Contains(string(ignore), wantIgnore) {
		t.Fatalf(".gitignore missing literal pattern %q:\n%s", wantIgnore, ignore)
	}
}

func runDoctorGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestDoctorFixBuiltInsNonInteractiveDoesNotGuessDestination(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	var out bytes.Buffer
	if err := Execute([]string{"--home", home, "--project-root", project, "-y", "doctor", "--fix"}, strings.NewReader(""), &out, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "archived but inactive") {
		t.Fatalf("output missing archive-only status:\n%s", out.String())
	}
	catalog, _ := builtin.List()
	cfg := config.Default(project, home)
	for _, skill := range catalog {
		if _, err := os.Stat(filepath.Join(cfg.ArchiveSkillsRoot(), skill.Name)); err != nil {
			t.Fatalf("archive %s: %v", skill.Name, err)
		}
		for _, target := range []string{config.TargetAgents, config.TargetClaude, config.TargetCodex} {
			if _, err := os.Lstat(filepath.Join(cfg.MustActiveRoot(config.ScopeGlobal, target), skill.Name)); !os.IsNotExist(err) {
				t.Fatalf("doctor guessed %s destination for %s: %v", target, skill.Name, err)
			}
		}
	}
}

func TestDoctorFixBuiltInsLinksOnlyExplicitGlobalDestination(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	var out bytes.Buffer
	err := Execute([]string{"--home", home, "--project-root", project, "-y", "doctor", "--fix", "--at", "global:agents"}, strings.NewReader(""), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "archived and linked") {
		t.Fatalf("output missing linked status:\n%s", out.String())
	}
}

func TestDoctorFixBuiltInsRejectsProjectDestination(t *testing.T) {
	err := Execute([]string{"--home", t.TempDir(), "--project-root", t.TempDir(), "-y", "doctor", "--fix", "--at", "project:agents"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "global") {
		t.Fatalf("error = %v, want global destination rejection", err)
	}
}

func TestDoctorFixBuiltInsRejectsProjectDestinationWithBrokenSymlink(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	root := filepath.Join(project, ".agents", "skills")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	broken := filepath.Join(root, "broken")
	if err := os.Symlink(filepath.Join(home, "missing"), broken); err != nil {
		t.Fatal(err)
	}
	err := Execute([]string{"--home", home, "--project-root", project, "-y", "doctor", "--fix", "--at", "project:agents"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "global") {
		t.Fatalf("error = %v, want global destination rejection", err)
	}
	if _, statErr := os.Lstat(broken); statErr != nil {
		t.Fatalf("broken symlink mutated before destination validation: %v", statErr)
	}
}

func TestDoctorFixBuiltInsInteractiveShowsGlobalChecklistWithAgentsPreselected(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	var out bytes.Buffer
	err := Execute([]string{"--home", home, "--project-root", project, "doctor", "--fix"}, strings.NewReader("\n"), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"[x] ~Ag", "[ ] ~Cl", "[ ] ~Cd", "Archive only"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("interactive checklist missing %q:\n%s", want, out.String())
		}
	}
}

func TestDoctorFixBuiltInsInteractiveDefaultsToArchiveOnlyWithoutGlobalRoots(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	configDir := filepath.Join(home, ".x-skills")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	data := []byte("active_roots:\n  - scope: global\n    target: agents\n    enabled: false\n  - scope: global\n    target: claude\n    enabled: false\n  - scope: global\n    target: codex\n    enabled: false\n")
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	err := Execute([]string{"--home", home, "--project-root", project, "doctor", "--fix"}, strings.NewReader("\n"), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "[x] Archive only") || !strings.Contains(out.String(), "archived but inactive") {
		t.Fatalf("archive-only default missing:\n%s", out.String())
	}
}

func TestDoctorReportsAndFixesBrokenSymlink(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	root := filepath.Join(project, ".claude", "skills")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "chapter-drafter")
	if err := os.Symlink(filepath.Join(home, "missing"), link); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	err := Execute([]string{"--home", home, "--project-root", project, "doctor"}, strings.NewReader(""), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "broken") || !strings.Contains(out.String(), "chapter-drafter") {
		t.Fatalf("doctor output:\n%s", out.String())
	}
	if !strings.Contains(out.String(), link) || !strings.Contains(out.String(), "resolve symlink") {
		t.Fatalf("doctor output missing path or reason:\n%s", out.String())
	}

	out.Reset()
	err = Execute([]string{"--home", home, "--project-root", project, "-y", "doctor", "--fix"}, strings.NewReader(""), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(link); !os.IsNotExist(err) {
		t.Fatalf("link still exists or unexpected err: %v", err)
	}
	if !strings.Contains(out.String(), "removed") {
		t.Fatalf("fix output:\n%s", out.String())
	}
}

func TestDoctorFixRejectsProjectScopeBeforeMutatingBrokenLinks(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	projectRoot := filepath.Join(project, ".claude", "skills")
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	projectLink := filepath.Join(projectRoot, "project-broken")
	if err := os.Symlink(filepath.Join(home, "missing-project"), projectLink); err != nil {
		t.Fatal(err)
	}

	globalRoot := filepath.Join(home, ".claude", "skills")
	if err := os.MkdirAll(globalRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	globalLink := filepath.Join(globalRoot, "global-broken")
	if err := os.Symlink(filepath.Join(home, "missing-global"), globalLink); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	err := Execute([]string{"--home", home, "--project-root", project, "-y", "doctor", "--fix", "--at", ".Cl"}, strings.NewReader(""), &out, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "global") {
		t.Fatalf("error = %v, want global destination rejection", err)
	}
	if _, err := os.Lstat(projectLink); err != nil {
		t.Fatalf("project link changed: %v", err)
	}
	if _, err := os.Lstat(globalLink); err != nil {
		t.Fatalf("global link was changed: %v", err)
	}
}

func TestDoctorFixWithoutYesReturnsErrorAndDoesNotMutate(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	root := filepath.Join(project, ".claude", "skills")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "chapter-drafter")
	if err := os.Symlink(filepath.Join(home, "missing"), link); err != nil {
		t.Fatal(err)
	}

	err := Execute([]string{"--home", home, "--project-root", project, "doctor", "--fix"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected confirmation error")
	}
	if !strings.Contains(err.Error(), "requires confirmation") {
		t.Fatalf("error = %q, want confirmation", err)
	}
	info, statErr := os.Lstat(link)
	if statErr != nil {
		t.Fatalf("link was removed: %v", statErr)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("mode = %v, want symlink", info.Mode())
	}
}
