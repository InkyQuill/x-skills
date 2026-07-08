package remote

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckoutCacheReusesCloneAndFindsSkill(t *testing.T) {
	repo := makeGitRepo(t)
	writeRemoteSkill(t, repo, "skills/svelte-coder", "svelte-coder", "Svelte help.")
	gitCommit(t, repo, "initial")

	cache := NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	checkout, err := cache.Checkout(t.Context(), GitSource{CloneURL: repo})
	if err != nil {
		t.Fatal(err)
	}
	second, err := cache.Checkout(t.Context(), GitSource{CloneURL: repo})
	if err != nil {
		t.Fatal(err)
	}
	if checkout.Path != second.Path {
		t.Fatalf("cache did not reuse checkout: %q != %q", checkout.Path, second.Path)
	}
	found, err := checkout.FindSkill("svelte-coder", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(found.SkillDir, filepath.Join("skills", "svelte-coder")) {
		t.Fatalf("skill dir = %q", found.SkillDir)
	}
	if found.Metadata.SkillPath != "skills/svelte-coder" || found.Metadata.Commit == "" {
		t.Fatalf("metadata = %#v", found.Metadata)
	}
}

func TestFindSkillReportsAmbiguousName(t *testing.T) {
	repo := makeGitRepo(t)
	writeRemoteSkill(t, repo, "packs/one", "dup-skill", "One.")
	writeRemoteSkill(t, repo, "packs/two", "dup-skill", "Two.")
	gitCommit(t, repo, "initial")
	cache := NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	checkout, err := cache.Checkout(t.Context(), GitSource{CloneURL: repo})
	if err != nil {
		t.Fatal(err)
	}
	_, err = checkout.FindSkill("dup-skill", "")
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("err = %v, want ambiguous", err)
	}
}

func makeGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test")
	return dir
}

func writeRemoteSkill(t *testing.T, root, rel, name, desc string) {
	t.Helper()
	dir := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	data := "---\nname: " + name + "\ndescription: " + desc + "\n---\n# " + name + "\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func gitCommit(t *testing.T, repo, msg string) {
	t.Helper()
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", msg)
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}
