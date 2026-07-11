package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/InkyQuill/x-skills/internal/builtin"
	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/roots"
	"github.com/InkyQuill/x-skills/internal/skills"
)

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

	issues, err := Diagnose(cfg, Filter{})
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

func TestFixBuiltInsArchivesOnlyOrLinksToExplicitGlobalDestinations(t *testing.T) {
	t.Run("archive only", func(t *testing.T) {
		cfg := config.Default(t.TempDir(), t.TempDir())
		results, err := Fix(cfg, FixOptions{Yes: true, ArchiveOnlyBuiltIns: true})
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
		results, err := Fix(cfg, FixOptions{Yes: true, BuiltInDestinations: []roots.ActiveRoot{destination}})
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
		_, err := Fix(cfg, FixOptions{Yes: true, BuiltInDestinations: []roots.ActiveRoot{destination}})
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
	issues, err := Diagnose(cfg, Filter{})
	if err != nil {
		t.Fatal(err)
	}
	results, err := FixBuiltIns(cfg, issues, FixOptions{BuiltInDestinations: []roots.ActiveRoot{destination}})
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

	issues, err := Diagnose(cfg, Filter{})
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

	issues, err := Diagnose(cfg, Filter{})
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

	issues, err := Diagnose(cfg, Filter{})
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

	results, err := Fix(cfg, FixOptions{Yes: true})
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

	results, err := Fix(cfg, FixOptions{Yes: true})
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

	results, err := Fix(cfg, FixOptions{
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

	_, err := Fix(cfg, FixOptions{})
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

func TestDoctorIgnoresUnmanagedDirectories(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	unmanaged := makeSkill(t, cfg.MustActiveRoot("project", "claude"), "local-skill")

	issues, err := Diagnose(cfg, Filter{})
	if err != nil {
		t.Fatal(err)
	}
	for _, issue := range issues {
		if issue.Name == "local-skill" {
			t.Fatalf("unmanaged skill diagnosed: %#v", issue)
		}
	}

	results, err := Fix(cfg, FixOptions{Yes: true})
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

	results, err := FixIssues([]Issue{
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
