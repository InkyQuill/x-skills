package remote

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
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

func TestCheckoutCacheHitUsesCurrentSourceMetadata(t *testing.T) {
	repo := makeGitRepo(t)
	writeRemoteSkill(t, repo, "skills/svelte-coder", "svelte-coder", "Svelte help.")
	gitCommit(t, repo, "initial")

	cache := NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	first, err := cache.Checkout(t.Context(), GitSource{CloneURL: repo})
	if err != nil {
		t.Fatal(err)
	}
	second, err := cache.Checkout(t.Context(), GitSource{
		CloneURL: repo,
		Owner:    "octo",
		Repo:     "skills",
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.Path != second.Path {
		t.Fatalf("cache did not reuse checkout: %q != %q", first.Path, second.Path)
	}
	found, err := second.FindSkill("svelte-coder", "")
	if err != nil {
		t.Fatal(err)
	}
	if found.Metadata.SourceType != SourceTypeGitHub ||
		found.Metadata.Owner != "octo" ||
		found.Metadata.Repo != "skills" {
		t.Fatalf("metadata = %#v", found.Metadata)
	}
}

func TestCheckoutCacheConcurrentCheckoutReusesClone(t *testing.T) {
	repo := makeGitRepo(t)
	writeRemoteSkill(t, repo, "skills/svelte-coder", "svelte-coder", "Svelte help.")
	gitCommit(t, repo, "initial")

	cacheRoot := filepath.Join(t.TempDir(), "cache")
	cache := NewCheckoutCache(cacheRoot)
	source := GitSource{CloneURL: repo}

	const callers = 16
	var wg sync.WaitGroup
	wg.Add(callers)
	start := make(chan struct{})
	results := make(chan Checkout, callers)
	errs := make(chan error, callers)

	for range callers {
		go func() {
			defer wg.Done()
			<-start
			checkout, err := cache.Checkout(t.Context(), source)
			if err != nil {
				errs <- err
				return
			}
			results <- checkout
		}()
	}

	close(start)
	wg.Wait()
	close(results)
	close(errs)

	for err := range errs {
		t.Errorf("Checkout returned error: %v", err)
	}
	if t.Failed() {
		t.FailNow()
	}

	var path string
	count := 0
	for checkout := range results {
		if checkout.Path == "" {
			t.Fatal("Checkout returned empty path")
		}
		if path == "" {
			path = checkout.Path
		} else if checkout.Path != path {
			t.Fatalf("Checkout path = %q, want %q", checkout.Path, path)
		}
		count++
	}
	if count != callers {
		t.Fatalf("received %d successful checkouts, want %d", count, callers)
	}

	entries, err := os.ReadDir(cacheRoot)
	if err != nil {
		t.Fatal(err)
	}
	clones := 0
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "repo-") {
			clones++
		}
	}
	if clones != 1 {
		t.Fatalf("cache contains %d clone dirs, want 1", clones)
	}
}

func TestCheckoutListSkillsFindsStandardAndNestedSkills(t *testing.T) {
	root := t.TempDir()
	writeRemoteSkill(t, root, "skills/alpha-skill", "alpha-skill", "Alpha.")
	writeRemoteSkill(t, root, "packs/beta-skill", "beta-skill", "Beta.")
	writeRemoteSkill(t, root, "packs/beta-skill/references/nested-skill", "nested-skill", "Nested.")
	checkout := Checkout{
		Path: root,
		Source: GitSource{
			CloneURL: "https://github.com/vercel-labs/skills.git",
			Owner:    "vercel-labs",
			Repo:     "skills",
			Ref:      "main",
		},
		Commit: "abc123",
	}

	found, err := checkout.ListSkillsContext(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 2 {
		t.Fatalf("len(found) = %d, want 2: %#v", len(found), found)
	}
	if found[0].Info.Name != "alpha-skill" || found[0].Metadata.SkillPath != "skills/alpha-skill" {
		t.Fatalf("first found = %#v", found[0])
	}
	if found[1].Info.Name != "beta-skill" || found[1].Metadata.SkillPath != "packs/beta-skill" {
		t.Fatalf("second found = %#v", found[1])
	}
	meta := found[0].Metadata
	if meta.SourceType != SourceTypeGitHub ||
		meta.Owner != "vercel-labs" ||
		meta.Repo != "skills" ||
		meta.Ref != "main" ||
		meta.Commit != "abc123" ||
		meta.UpstreamName != "alpha-skill" {
		t.Fatalf("metadata = %#v", meta)
	}
}

func TestCheckoutListSkillsSortsByName(t *testing.T) {
	root := t.TempDir()
	writeRemoteSkill(t, root, "zeta/path", "same-name", "Zeta.")
	writeRemoteSkill(t, root, "alpha/path", "same-name", "Alpha.")
	writeRemoteSkill(t, root, "middle/path", "aaa-skill", "Middle.")
	checkout := Checkout{
		Path:   root,
		Source: GitSource{CloneURL: "https://gitlab.com/acme/skills.git"},
		Commit: "def456",
	}

	found, err := checkout.ListSkillsContext(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 3 {
		t.Fatalf("len(found) = %d, want 3: %#v", len(found), found)
	}
	got := []string{
		found[0].Info.Name + " " + found[0].Metadata.SkillPath,
		found[1].Info.Name + " " + found[1].Metadata.SkillPath,
		found[2].Info.Name + " " + found[2].Metadata.SkillPath,
	}
	want := []string{
		"aaa-skill middle/path",
		"same-name alpha/path",
		"same-name zeta/path",
	}
	if !sameStrings(got, want) {
		t.Fatalf("order = %#v, want %#v", got, want)
	}
	if found[0].Metadata.SourceType != SourceTypeGit ||
		found[0].Metadata.CloneURL != "https://gitlab.com/acme/skills.git" ||
		found[0].Metadata.Commit != "def456" {
		t.Fatalf("metadata = %#v", found[0].Metadata)
	}
}

func TestCheckoutFindSkillWithoutPreferredPathReturnsAmbiguousWhenDuplicateNames(t *testing.T) {
	root := t.TempDir()
	writeRemoteSkill(t, root, "packs/one", "dup-skill", "One.")
	writeRemoteSkill(t, root, "packs/two", "dup-skill", "Two.")
	checkout := Checkout{Path: root}

	_, err := checkout.FindSkill("dup-skill", "")
	if err == nil || !strings.Contains(err.Error(), "ambiguous skill") {
		t.Fatalf("err = %v, want ambiguous skill", err)
	}
}

func TestCheckoutListSkillsContextStopsWhenCanceled(t *testing.T) {
	root := t.TempDir()
	writeRemoteSkill(t, root, "skills/svelte-coder", "svelte-coder", "Svelte help.")
	checkout := Checkout{Path: root}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := checkout.ListSkillsContext(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
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

func TestFindSkillUsesValidPreferredPath(t *testing.T) {
	repo := makeGitRepo(t)
	writeRemoteSkill(t, repo, "packs/one", "dup-skill", "One.")
	writeRemoteSkill(t, repo, "packs/two", "dup-skill", "Two.")
	gitCommit(t, repo, "initial")
	cache := NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	checkout, err := cache.Checkout(t.Context(), GitSource{CloneURL: repo})
	if err != nil {
		t.Fatal(err)
	}
	found, err := checkout.FindSkill("dup-skill", "packs/two")
	if err != nil {
		t.Fatal(err)
	}
	if found.Metadata.SkillPath != "packs/two" {
		t.Fatalf("skill path = %q, want packs/two", found.Metadata.SkillPath)
	}
}

func TestFindSkillPreferredPathValidatesName(t *testing.T) {
	root := t.TempDir()
	writeRemoteSkill(t, root, "packs/actual-skill", "manifest-name", "Actual.")
	checkout := Checkout{
		Path:   root,
		Source: GitSource{CloneURL: "https://github.com/example/skills.git"},
	}

	found, err := checkout.FindSkill("actual-skill", "packs/actual-skill")
	if err != nil {
		t.Fatalf("path basename match returned error: %v", err)
	}
	if found.Info.Name != "manifest-name" {
		t.Fatalf("Info.Name = %q, want manifest-name", found.Info.Name)
	}

	_, err = checkout.FindSkill("requested-skill", "packs/actual-skill")
	var missing *MissingSkillError
	if !errors.As(err, &missing) {
		t.Fatalf("err = %T %[1]v, want MissingSkillError", err)
	}
	if missing.Name != "requested-skill" || missing.PreferredPath != "packs/actual-skill" {
		t.Fatalf("missing = %#v", missing)
	}
}

func TestFindSkillFallsBackToRepoSearchWhenPreferredPathIsStale(t *testing.T) {
	root := t.TempDir()
	writeRemoteSkill(t, root, "skills/golang-cli", "golang-cli", "Go CLI help.")
	checkout := Checkout{
		Path:   root,
		Source: GitSource{CloneURL: "https://github.com/samber/cc-skills-golang.git"},
		Commit: "abc123",
	}

	found, err := checkout.FindSkill("golang-cli", "golang-cli")
	if err != nil {
		t.Fatal(err)
	}
	if found.Metadata.SkillPath != "skills/golang-cli" {
		t.Fatalf("SkillPath = %q, want skills/golang-cli", found.Metadata.SkillPath)
	}
}

func TestFindSkillFallsBackToRepoSearchWhenPreferredPathNameDiffers(t *testing.T) {
	root := t.TempDir()
	writeRemoteSkill(t, root, "skills/other", "other", "Other.")
	writeRemoteSkill(t, root, "skills/golang-cli", "golang-cli", "Go CLI help.")
	checkout := Checkout{Path: root}

	found, err := checkout.FindSkill("golang-cli", "skills/other")
	if err != nil {
		t.Fatal(err)
	}
	if found.Metadata.SkillPath != "skills/golang-cli" {
		t.Fatalf("SkillPath = %q, want skills/golang-cli", found.Metadata.SkillPath)
	}
}

func TestFindSkillReturnsMissingSkillErrorWhenPreferredPathAndRepoSearchMiss(t *testing.T) {
	root := t.TempDir()
	writeRemoteSkill(t, root, "skills/other", "other", "Other.")
	checkout := Checkout{
		Path:   root,
		Source: GitSource{CloneURL: "https://github.com/example/skills.git"},
	}

	_, err := checkout.FindSkill("next-best-practices", "next-best-practices")
	var missing *MissingSkillError
	if !errors.As(err, &missing) {
		t.Fatalf("err = %T %[1]v, want MissingSkillError", err)
	}
	if missing.Name != "next-best-practices" {
		t.Fatalf("Name = %q, want next-best-practices", missing.Name)
	}
	if missing.RepoURL != "https://github.com/example/skills.git" {
		t.Fatalf("RepoURL = %q", missing.RepoURL)
	}
}

func TestFindSkillContextStopsWhenCanceled(t *testing.T) {
	root := t.TempDir()
	writeRemoteSkill(t, root, "skills/svelte-coder", "svelte-coder", "Svelte help.")
	checkout := Checkout{Path: root}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := checkout.FindSkillContext(ctx, "svelte-coder", "")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestFindSkillRejectsPreferredPathTraversal(t *testing.T) {
	checkout := Checkout{Path: t.TempDir()}
	tests := []struct {
		name string
		path string
	}{
		{name: "parent", path: "../outside"},
		{name: "nested parent", path: "skills/../../outside"},
		{name: "contained parent", path: "skills/../outside"},
		{name: "absolute", path: filepath.Join(checkout.Path, "outside")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := checkout.FindSkill("skill", tt.path)
			if err == nil || !strings.Contains(err.Error(), "invalid skill path") {
				t.Fatalf("err = %v, want invalid skill path", err)
			}
		})
	}
}

func TestFindSkillRejectsPreferredPathSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	writeRemoteSkill(t, outside, "outside-skill", "outside-skill", "Outside.")
	if err := os.Symlink(
		filepath.Join(outside, "outside-skill"),
		filepath.Join(root, "linked-skill"),
	); err != nil {
		t.Fatal(err)
	}
	checkout := Checkout{Path: root}
	_, err := checkout.FindSkill("outside-skill", "linked-skill")
	if err == nil || !strings.Contains(err.Error(), "invalid skill path") {
		t.Fatalf("err = %v, want invalid skill path", err)
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
