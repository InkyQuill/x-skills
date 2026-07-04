package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/InkyQuill/x-skills/internal/config"
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

func TestDiagnoseReportsBrokenSymlink(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	root := cfg.ActiveRoot("project", "claude")
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
	if len(issues) != 1 {
		t.Fatalf("len(issues) = %d, want 1", len(issues))
	}
	if issues[0].Kind != KindBrokenSymlink {
		t.Fatalf("Kind = %q", issues[0].Kind)
	}
}

func TestDiagnoseReportsSymlinkToFile(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	root := cfg.ActiveRoot("project", "claude")
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
	if len(issues) != 1 {
		t.Fatalf("len(issues) = %d, want 1", len(issues))
	}
	if !strings.Contains(issues[0].Reason, "not a directory") {
		t.Fatalf("Reason = %q, want not a directory", issues[0].Reason)
	}
}

func TestDiagnoseReportsSymlinkToNonSkillDir(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	root := cfg.ActiveRoot("project", "claude")
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
	if len(issues) != 1 {
		t.Fatalf("len(issues) = %d, want 1", len(issues))
	}
	if !strings.Contains(issues[0].Reason, "not a skill directory") {
		t.Fatalf("Reason = %q, want not a skill directory", issues[0].Reason)
	}
}

func TestFixBrokenSymlinkRelinksWhenRepoSkillExists(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	repoSkill := makeSkill(t, cfg.ArchiveSkillsRoot(), "chapter-drafter")
	root := cfg.ActiveRoot("project", "claude")
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
	root := cfg.ActiveRoot("project", "claude")
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

	projectRoot := cfg.ActiveRoot("project", "claude")
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	projectLink := filepath.Join(projectRoot, "project-broken")
	if err := os.Symlink(filepath.Join(home, "missing-project"), projectLink); err != nil {
		t.Fatal(err)
	}

	globalRoot := cfg.ActiveRoot("global", "claude")
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
	root := cfg.ActiveRoot("project", "claude")
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
	unmanaged := makeSkill(t, cfg.ActiveRoot("project", "claude"), "local-skill")

	issues, err := Diagnose(cfg, Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 0 {
		t.Fatalf("issues = %#v, want none", issues)
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
