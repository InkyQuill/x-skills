package doctor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/InkyQuill/x-skills/internal/builtin"
	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/roots"
	"github.com/InkyQuill/x-skills/internal/skills"
)

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out.String())
	}
	return strings.TrimSpace(out.String())
}

func initGitRepo(t *testing.T) string {
	t.Helper()
	project := t.TempDir()
	runGit(t, project, "init", "--quiet")
	runGit(t, project, "config", "user.name", "Test User")
	runGit(t, project, "config", "user.email", "test@example.com")
	return project
}

func issueByKind(t *testing.T, issues []Issue, kind string) Issue {
	t.Helper()
	for _, issue := range issues {
		if issue.Kind == kind {
			return issue
		}
	}
	t.Fatalf("issue kind %q not found in %#v", kind, issues)
	return Issue{}
}

func TestDiagnoseGitHygieneUsesGitTrackingState(t *testing.T) {
	t.Run("untracked recommended manifest", func(t *testing.T) {
		project := initGitRepo(t)
		if err := os.WriteFile(filepath.Join(project, ".x-skills.yaml"), []byte("version: 1\nskills: []\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		issues, err := Diagnose(context.Background(), config.Default(project, t.TempDir()), Filter{})
		if err != nil {
			t.Fatal(err)
		}
		issue := issueByKind(t, issues, KindRecommendedManifestUntracked)
		if issue.SafeFix != "git add -- '.x-skills.yaml'" {
			t.Fatalf("SafeFix = %q", issue.SafeFix)
		}
	})

	t.Run("tracked local manifest", func(t *testing.T) {
		project := initGitRepo(t)
		path := filepath.Join(project, ".x-skills.local.yaml")
		if err := os.WriteFile(path, []byte("version: 1\nskills: []\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		runGit(t, project, "add", "--", ".x-skills.local.yaml")

		issues, err := Diagnose(context.Background(), config.Default(project, t.TempDir()), Filter{})
		if err != nil {
			t.Fatal(err)
		}
		issue := issueByKind(t, issues, KindLocalManifestTracked)
		if issue.SafeFix != "git rm --cached -- '.x-skills.local.yaml'" {
			t.Fatalf("SafeFix = %q", issue.SafeFix)
		}
	})

	t.Run("tracked file in project Skills Folder", func(t *testing.T) {
		project := initGitRepo(t)
		makeSkill(t, filepath.Join(project, ".agents", "skills"), "tracked-skill")
		runGit(t, project, "add", "--", ".agents/skills/tracked-skill/SKILL.md")

		issues, err := Diagnose(context.Background(), config.Default(project, t.TempDir()), Filter{})
		if err != nil {
			t.Fatal(err)
		}
		issue := issueByKind(t, issues, KindSkillsFolderTracked)
		if issue.SafeFix != "git rm -r --cached -- '.agents/skills'" {
			t.Fatalf("SafeFix = %q", issue.SafeFix)
		}
	})

	t.Run("ignored recommended manifest", func(t *testing.T) {
		project := initGitRepo(t)
		if err := os.WriteFile(filepath.Join(project, ".gitignore"), []byte(".x-skills.yaml\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(project, ".x-skills.yaml"), []byte("version: 1\nskills: []\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		issues, err := Diagnose(context.Background(), config.Default(project, t.TempDir()), Filter{})
		if err != nil {
			t.Fatal(err)
		}
		issue := issueByKind(t, issues, KindRecommendedManifestUntracked)
		if issue.SafeFix != "git add -f -- '.x-skills.yaml'" {
			t.Fatalf("SafeFix = %q", issue.SafeFix)
		}
	})

	t.Run("outside Git", func(t *testing.T) {
		project := t.TempDir()
		if err := os.WriteFile(filepath.Join(project, ".x-skills.yaml"), []byte("version: 1\nskills: []\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		issues, err := Diagnose(context.Background(), config.Default(project, t.TempDir()), Filter{})
		if err != nil {
			t.Fatal(err)
		}
		for _, issue := range issues {
			if issue.Kind == KindRecommendedManifestUntracked {
				t.Fatalf("Git hygiene issue outside repository: %#v", issue)
			}
		}
	})

	t.Run("ignored untracked local files", func(t *testing.T) {
		project := initGitRepo(t)
		if err := os.WriteFile(filepath.Join(project, ".gitignore"), []byte(".x-skills.local.yaml\n.agents/\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(project, ".x-skills.local.yaml"), []byte("version: 1\nskills: []\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		makeSkill(t, filepath.Join(project, ".agents", "skills"), "local-skill")

		issues, err := Diagnose(context.Background(), config.Default(project, t.TempDir()), Filter{})
		if err != nil {
			t.Fatal(err)
		}
		for _, issue := range issues {
			if issue.Kind == KindLocalManifestTracked || issue.Kind == KindSkillsFolderTracked {
				t.Fatalf("untracked local file diagnosed: %#v", issue)
			}
		}
	})
}

func TestDiagnoseGitHygieneSurfacesBrokenRepository(t *testing.T) {
	project := t.TempDir()
	if err := os.Mkdir(filepath.Join(project, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := Diagnose(context.Background(), config.Default(project, t.TempDir()), Filter{})
	if err == nil || !strings.Contains(err.Error(), "inspect Git work tree") {
		t.Fatalf("error = %v, want Git probe failure", err)
	}
}

func TestShellQuote(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "spaces", path: "team skills", want: "'team skills'"},
		{name: "metacharacters", path: "skills;$(touch nope)", want: "'skills;$(touch nope)'"},
		{name: "single quote", path: "team's skills", want: "'team'\"'\"'s skills'"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shellQuote(tt.path); got != tt.want {
				t.Fatalf("shellQuote(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestLiteralGitignorePattern(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "spaces", path: "team skills", want: "/team\\ skills/"},
		{name: "comment", path: "#skills", want: "/\\#skills/"},
		{name: "negation", path: "!skills", want: "/\\!skills/"},
		{name: "globs", path: "skill*[a]?", want: "/skill\\*\\[a\\]\\?/"},
		{name: "backslash", path: `team\skills`, want: `/team\\skills/`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := literalGitignorePattern(tt.path)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("literalGitignorePattern(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestFixGitHygieneOnlyEditsGitignore(t *testing.T) {
	project := initGitRepo(t)
	local := filepath.Join(project, ".x-skills.local.yaml")
	if err := os.WriteFile(local, []byte("version: 1\nskills: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	makeSkill(t, filepath.Join(project, ".agents", "skills"), "tracked-skill")
	runGit(t, project, "add", "--", ".x-skills.local.yaml", ".agents/skills/tracked-skill/SKILL.md")

	cfg := config.Default(project, t.TempDir())
	issues, err := Diagnose(context.Background(), cfg, Filter{})
	if err != nil {
		t.Fatal(err)
	}
	results, err := FixIssues(context.Background(), issues)
	if err != nil {
		t.Fatal(err)
	}
	ignore, err := os.ReadFile(filepath.Join(project, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{".x-skills.local.yaml", ".agents/skills/"} {
		if !strings.Contains(string(ignore), want) {
			t.Fatalf(".gitignore missing %q:\n%s", want, ignore)
		}
	}
	tracked := runGit(t, project, "ls-files")
	for _, want := range []string{".x-skills.local.yaml", ".agents/skills/tracked-skill/SKILL.md"} {
		if !strings.Contains(tracked, want) {
			t.Fatalf("Doctor changed Git index; missing %q from:\n%s", want, tracked)
		}
	}
	joined := fmt.Sprint(results)
	for _, want := range []string{"git rm --cached -- '.x-skills.local.yaml'", "git rm -r --cached -- '.agents/skills'"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("results missing %q: %#v", want, results)
		}
	}
}

func TestFixGitHygieneAppendsNormalizedIgnoreEntryOnce(t *testing.T) {
	project := initGitRepo(t)
	if err := os.WriteFile(filepath.Join(project, ".gitignore"), []byte(".env\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	issue := Issue{Kind: KindLocalManifestTracked, Name: ".x-skills.local.yaml", Path: filepath.Join(project, ".x-skills.local.yaml"), ProjectRoot: project, SafeFix: "git rm --cached -- .x-skills.local.yaml"}
	if _, err := FixIssues(context.Background(), []Issue{issue, issue}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(project, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	want := ".env\n.x-skills.local.yaml\n"
	if string(got) != want {
		t.Fatalf(".gitignore = %q, want %q", got, want)
	}
}

func makeSkill(t *testing.T, root, name string) string {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: "+name+"\ndescription: test\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestDiagnoseBuiltInsReportsMissingInactiveAndActive(t *testing.T) {
	home := t.TempDir()
	cfg := config.Default(t.TempDir(), home)
	catalog, err := builtin.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog) < 3 {
		t.Fatalf("catalog has %d skills, want at least 3", len(catalog))
	}

	if _, err := builtin.Archive(cfg, []string{catalog[1].Name, catalog[2].Name}); err != nil {
		t.Fatal(err)
	}
	global := roots.ActiveRoots(cfg, roots.Filter{Scope: config.ScopeGlobal})[0]
	if err := os.MkdirAll(global.Path, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(cfg.ArchiveSkillsRoot(), catalog[2].Name), filepath.Join(global.Path, catalog[2].Name)); err != nil {
		t.Fatal(err)
	}

	issues, err := Diagnose(context.Background(), cfg, Filter{})
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]string{}
	for _, issue := range issues {
		kinds[issue.Name] = issue.Kind
	}
	if kinds[catalog[0].Name] != KindMissingBuiltIn {
		t.Fatalf("missing kind = %q, want %q", kinds[catalog[0].Name], KindMissingBuiltIn)
	}
	if kinds[catalog[1].Name] != KindInactiveBuiltIn {
		t.Fatalf("inactive kind = %q, want %q", kinds[catalog[1].Name], KindInactiveBuiltIn)
	}
	if _, found := kinds[catalog[2].Name]; found {
		t.Fatalf("active built-in unexpectedly diagnosed: %#v", issues)
	}
}

func TestDiagnoseProjectFilterSkipsGlobalBuiltIns(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	issues, err := Diagnose(context.Background(), cfg, Filter{Scope: config.ScopeProject})
	if err != nil {
		t.Fatal(err)
	}
	for _, issue := range issues {
		if issue.Kind == KindMissingBuiltIn || issue.Kind == KindInactiveBuiltIn {
			t.Fatalf("project-filtered diagnosis contains global built-in issue: %#v", issue)
		}
	}
}

func TestFixBuiltInsArchivesOnlyOrLinksToExplicitGlobalDestinations(t *testing.T) {
	t.Run("archive only", func(t *testing.T) {
		cfg := config.Default(t.TempDir(), t.TempDir())
		results, err := Fix(context.Background(), cfg, FixOptions{Yes: true, ArchiveOnlyBuiltIns: true})
		if err != nil {
			t.Fatal(err)
		}
		if len(results) == 0 || results[0].Action != "archived but inactive" {
			t.Fatalf("results = %#v, want archived but inactive", results)
		}
	})

	t.Run("global destination", func(t *testing.T) {
		cfg := config.Default(t.TempDir(), t.TempDir())
		destination := roots.ActiveRoots(cfg, roots.Filter{Scope: config.ScopeGlobal, Target: config.TargetAgents})[0]
		results, err := Fix(context.Background(), cfg, FixOptions{Yes: true, BuiltInDestinations: []roots.ActiveRoot{destination}})
		if err != nil {
			t.Fatal(err)
		}
		catalog, _ := builtin.List()
		for _, skill := range catalog {
			if _, err := filepath.EvalSymlinks(filepath.Join(destination.Path, skill.Name)); err != nil {
				t.Fatalf("%s not linked: %v; results=%#v", skill.Name, err, results)
			}
		}
	})

	t.Run("project rejected", func(t *testing.T) {
		cfg := config.Default(t.TempDir(), t.TempDir())
		destination := roots.ActiveRoots(cfg, roots.Filter{Scope: config.ScopeProject})[0]
		_, err := Fix(context.Background(), cfg, FixOptions{Yes: true, BuiltInDestinations: []roots.ActiveRoot{destination}})
		if err == nil || !strings.Contains(err.Error(), "global") {
			t.Fatalf("error = %v, want global destination rejection", err)
		}
	})
}

func TestFixBuiltInsPreservesArchiveResultWhenLinkConflicts(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	destination := roots.ActiveRoots(cfg, roots.Filter{Scope: config.ScopeGlobal, Target: config.TargetAgents})[0]
	catalog, _ := builtin.List()
	name := catalog[0].Name
	makeSkill(t, destination.Path, name)
	issues, err := Diagnose(context.Background(), cfg, Filter{})
	if err != nil {
		t.Fatal(err)
	}
	results, err := FixBuiltIns(context.Background(), cfg, issues, FixOptions{BuiltInDestinations: []roots.ActiveRoot{destination}})
	if err == nil || !strings.Contains(err.Error(), "destination exists") {
		t.Fatalf("error = %v, want destination conflict", err)
	}
	if !skills.IsDir(filepath.Join(cfg.ArchiveSkillsRoot(), name)) {
		t.Fatal("archive was not preserved")
	}
	if len(results) == 0 || results[0].Name != name || results[0].Action != "archived" {
		t.Fatalf("results = %#v, want preserved archive result", results)
	}
}

func TestDiagnoseReportsBrokenSymlink(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	root := cfg.MustActiveRoot("project", "claude")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(home, "missing"), filepath.Join(root, "chapter-drafter")); err != nil {
		t.Fatal(err)
	}

	issues, err := Diagnose(context.Background(), cfg, Filter{})
	if err != nil {
		t.Fatal(err)
	}
	issue := issueByName(t, issues, "chapter-drafter")
	if issue.Kind != KindBrokenSymlink {
		t.Fatalf("Kind = %q", issue.Kind)
	}
}

func TestDiagnoseReportsSymlinkToFile(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	root := cfg.MustActiveRoot("project", "claude")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(home, "not-a-dir")
	if err := os.WriteFile(target, []byte("not a skill"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, "chapter-drafter")); err != nil {
		t.Fatal(err)
	}

	issues, err := Diagnose(context.Background(), cfg, Filter{})
	if err != nil {
		t.Fatal(err)
	}
	issue := issueByName(t, issues, "chapter-drafter")
	if !strings.Contains(issue.Reason, "not a directory") {
		t.Fatalf("Reason = %q, want not a directory", issue.Reason)
	}
}

func TestDiagnoseReportsSymlinkToNonSkillDir(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	root := cfg.MustActiveRoot("project", "claude")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(home, "not-a-skill")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, "chapter-drafter")); err != nil {
		t.Fatal(err)
	}

	issues, err := Diagnose(context.Background(), cfg, Filter{})
	if err != nil {
		t.Fatal(err)
	}
	issue := issueByName(t, issues, "chapter-drafter")
	if !strings.Contains(issue.Reason, "not a skill directory") {
		t.Fatalf("Reason = %q, want not a skill directory", issue.Reason)
	}
}

func issueByName(t *testing.T, issues []Issue, name string) Issue {
	t.Helper()
	for _, issue := range issues {
		if issue.Name == name {
			return issue
		}
	}
	t.Fatalf("issue %q not found in %#v", name, issues)
	return Issue{}
}

func TestFixBrokenSymlinkRelinksWhenRepoSkillExists(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	repoSkill := makeSkill(t, cfg.ArchiveSkillsRoot(), "chapter-drafter")
	root := cfg.MustActiveRoot("project", "claude")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "chapter-drafter")
	if err := os.Symlink(filepath.Join(home, "missing"), link); err != nil {
		t.Fatal(err)
	}

	results, err := Fix(context.Background(), cfg, FixOptions{Yes: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Action != "relinked" {
		t.Fatalf("results = %#v", results)
	}
	resolved, err := filepath.EvalSymlinks(link)
	if err != nil {
		t.Fatal(err)
	}
	if resolved != repoSkill {
		t.Fatalf("resolved = %q, want %q", resolved, repoSkill)
	}
	if matches, err := filepath.Glob(filepath.Join(root, ".chapter-drafter.tmp.*")); err != nil || len(matches) != 0 {
		t.Fatalf("temporary links = %v, err = %v", matches, err)
	}
}

func TestFixBrokenSymlinkRemovesWhenRepoSkillMissing(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	root := cfg.MustActiveRoot("project", "claude")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "chapter-drafter")
	if err := os.Symlink(filepath.Join(home, "missing"), link); err != nil {
		t.Fatal(err)
	}

	results, err := Fix(context.Background(), cfg, FixOptions{Yes: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Action != "removed" {
		t.Fatalf("results = %#v", results)
	}
	if _, err := os.Lstat(link); !os.IsNotExist(err) {
		t.Fatalf("link still exists or unexpected err: %v", err)
	}
}

func TestFixRespectsFilter(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)

	projectRoot := cfg.MustActiveRoot("project", "claude")
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	projectLink := filepath.Join(projectRoot, "project-broken")
	if err := os.Symlink(filepath.Join(home, "missing-project"), projectLink); err != nil {
		t.Fatal(err)
	}

	globalRoot := cfg.MustActiveRoot("global", "claude")
	if err := os.MkdirAll(globalRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	globalLink := filepath.Join(globalRoot, "global-broken")
	if err := os.Symlink(filepath.Join(home, "missing-global"), globalLink); err != nil {
		t.Fatal(err)
	}

	results, err := Fix(context.Background(), cfg, FixOptions{
		Yes:    true,
		Filter: Filter{Scope: "project"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Name != "project-broken" {
		t.Fatalf("results = %#v", results)
	}
	if _, err := os.Lstat(projectLink); !os.IsNotExist(err) {
		t.Fatalf("project link still exists or unexpected err: %v", err)
	}
	if _, err := os.Lstat(globalLink); err != nil {
		t.Fatalf("global link was changed: %v", err)
	}
}

func TestFixWithoutYesDoesNotMutate(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	root := cfg.MustActiveRoot("project", "claude")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "chapter-drafter")
	if err := os.Symlink(filepath.Join(home, "missing"), link); err != nil {
		t.Fatal(err)
	}

	_, err := Fix(context.Background(), cfg, FixOptions{})
	if err == nil {
		t.Fatal("expected confirmation error")
	}
	info, statErr := os.Lstat(link)
	if statErr != nil {
		t.Fatalf("link was removed: %v", statErr)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("mode = %v, want symlink", info.Mode())
	}
}

func TestFixRejectsInvalidScopeWithoutArchiving(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)

	if _, err := Diagnose(context.Background(), cfg, Filter{Scope: "bogus"}); err == nil || !strings.Contains(err.Error(), `invalid scope "bogus"`) {
		t.Fatalf("Diagnose() error = %v, want invalid scope rejection", err)
	}

	_, err := Fix(context.Background(), cfg, FixOptions{Yes: true, Filter: Filter{Scope: "bogus"}, ArchiveOnlyBuiltIns: true})
	if err == nil || !strings.Contains(err.Error(), `invalid scope "bogus"`) {
		t.Fatalf("Fix() error = %v, want invalid scope rejection", err)
	}
	if _, statErr := os.Stat(cfg.ArchiveSkillsRoot()); !os.IsNotExist(statErr) {
		t.Fatalf("archive was mutated for invalid scope: %v", statErr)
	}
}

func TestFixIssuesStopsOnCancelledContext(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	root := cfg.MustActiveRoot("project", "claude")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "chapter-drafter")
	if err := os.Symlink(filepath.Join(home, "missing"), link); err != nil {
		t.Fatal(err)
	}
	issues, err := Diagnose(context.Background(), cfg, Filter{})
	if err != nil || len(issues) == 0 {
		t.Fatalf("issues = %#v, err = %v, want broken symlink issue", issues, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	results, err := FixIssues(ctx, issues)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("FixIssues() error = %v, want context.Canceled", err)
	}
	if len(results) != 0 {
		t.Fatalf("results = %#v, want none", results)
	}
	if _, statErr := os.Lstat(link); statErr != nil {
		t.Fatalf("broken symlink was mutated after cancellation: %v", statErr)
	}
	if _, err := FixBuiltIns(ctx, cfg, []Issue{{Kind: KindMissingBuiltIn, Name: "chapter-drafter"}}, FixOptions{ArchiveOnlyBuiltIns: true}); !errors.Is(err, context.Canceled) {
		t.Fatalf("FixBuiltIns() error = %v, want context.Canceled", err)
	}
	if _, statErr := os.Stat(cfg.ArchiveSkillsRoot()); !os.IsNotExist(statErr) {
		t.Fatalf("archive was mutated after cancellation: %v", statErr)
	}
}

func TestDoctorIgnoresUnmanagedDirectories(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	unmanaged := makeSkill(t, cfg.MustActiveRoot("project", "claude"), "local-skill")

	issues, err := Diagnose(context.Background(), cfg, Filter{})
	if err != nil {
		t.Fatal(err)
	}
	for _, issue := range issues {
		if issue.Name == "local-skill" {
			t.Fatalf("unmanaged skill diagnosed: %#v", issue)
		}
	}

	results, err := Fix(context.Background(), cfg, FixOptions{Yes: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Fatalf("results = %#v, want none", results)
	}
	if _, err := os.Stat(unmanaged); err != nil {
		t.Fatalf("unmanaged directory was changed: %v", err)
	}
}

func TestFixReturnsAppliedResultsWhenLaterIssueFails(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	root := cfg.MustActiveRoot("project", "claude")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}

	first := filepath.Join(root, "a-broken")
	if err := os.Symlink(filepath.Join(home, "missing-a"), first); err != nil {
		t.Fatal(err)
	}
	second := filepath.Join(root, "b-stale")
	validTarget := makeSkill(t, home, "valid-target")
	if err := os.Symlink(validTarget, second); err != nil {
		t.Fatal(err)
	}

	results, err := FixIssues(context.Background(), []Issue{
		{
			Kind:    KindBrokenSymlink,
			Name:    "a-broken",
			Path:    first,
			SafeFix: "remove",
		},
		{
			Kind:    KindBrokenSymlink,
			Name:    "b-stale",
			Path:    second,
			SafeFix: "remove",
		},
	})
	if err == nil {
		t.Fatal("expected stale issue error")
	}
	if len(results) != 1 || results[0].Name != "a-broken" {
		t.Fatalf("results = %#v, want first applied result", results)
	}
	if _, err := os.Lstat(first); !os.IsNotExist(err) {
		t.Fatalf("first link still exists or unexpected err: %v", err)
	}
}

func TestFixBrokenSymlinkRevalidatesBeforeMutation(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	validTarget := makeSkill(t, home, "valid-target")
	link := filepath.Join(root, "stale")
	if err := os.Symlink(validTarget, link); err != nil {
		t.Fatal(err)
	}

	_, err := fixBrokenSymlink(Issue{
		Kind:    KindBrokenSymlink,
		Name:    "stale",
		Path:    link,
		SafeFix: "remove",
	})
	if err == nil {
		t.Fatal("expected stale issue error")
	}
	if !strings.Contains(err.Error(), "no longer broken") {
		t.Fatalf("error = %v, want no longer broken", err)
	}
	if _, err := os.Lstat(link); err != nil {
		t.Fatalf("link was mutated: %v", err)
	}
}

func TestFixBrokenSymlinkVerifiesRepoTargetBeforeRelink(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	link := filepath.Join(root, "stale-repo")
	if err := os.Symlink(filepath.Join(home, "missing"), link); err != nil {
		t.Fatal(err)
	}

	_, err := fixBrokenSymlink(Issue{
		Kind:       KindBrokenSymlink,
		Name:       "stale-repo",
		Path:       link,
		SafeFix:    "relink",
		RepoTarget: filepath.Join(home, "repo-missing"),
	})
	if err == nil {
		t.Fatal("expected stale repo target error")
	}
	if !strings.Contains(err.Error(), "repo target") {
		t.Fatalf("error = %v, want repo target", err)
	}
	resolved, err := os.Readlink(link)
	if err != nil {
		t.Fatal(err)
	}
	if resolved != filepath.Join(home, "missing") {
		t.Fatalf("link target = %q, want original missing target", resolved)
	}
}
