package remote

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolvePreviewReturnsOriginalDocumentAndIdentity(t *testing.T) {
	repo := makeGitRepo(t)
	content := []byte("---\nname: declared-name\ndescription: Original bytes.\n---\n# Heading\n\nBody.\n")
	writePreviewSkill(t, repo, "skills/exact-name", content)
	gitCommit(t, repo, "initial")
	commit := gitOutput(t, repo, "rev-parse", "HEAD")
	cacheRoot := filepath.Join(t.TempDir(), "cache")
	cache := NewCheckoutCache(cacheRoot)
	source := GitSource{
		CloneURL: repo,
		Owner:    "owner",
		Repo:     "repo",
	}

	result, err := ResolvePreview(t.Context(), cache, PreviewRequest{
		Source: source,
		Name:   "exact-name",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Repository != "owner/repo" {
		t.Fatalf("Repository = %q, want owner/repo", result.Repository)
	}
	if result.RequestedName != "exact-name" {
		t.Fatalf("RequestedName = %q, want exact-name", result.RequestedName)
	}
	if result.SkillPath != "skills/exact-name/SKILL.md" {
		t.Fatalf("SkillPath = %q, want skills/exact-name/SKILL.md", result.SkillPath)
	}
	if result.SkillDir == "" || !filepath.IsAbs(result.SkillDir) {
		t.Fatalf("SkillDir = %q, want absolute checkout path", result.SkillDir)
	}
	if string(result.SkillMD) != string(content) {
		t.Fatalf("SkillMD = %q, want original bytes %q", result.SkillMD, content)
	}
	if result.Commit != commit {
		t.Fatalf("Commit = %q, want %q", result.Commit, commit)
	}

	second, err := ResolvePreview(t.Context(), cache, PreviewRequest{
		Source: source,
		Name:   "declared-name",
	})
	if err != nil {
		t.Fatal(err)
	}
	if second.SkillPath != result.SkillPath || second.Commit != result.Commit {
		t.Fatalf("repeated result = %#v, want same path and commit as %#v", second, result)
	}
	entries, err := os.ReadDir(cacheRoot)
	if err != nil {
		t.Fatal(err)
	}
	checkoutCount := 0
	for _, entry := range entries {
		if entry.IsDir() {
			checkoutCount++
		}
	}
	if checkoutCount != 1 {
		t.Fatalf("checkout directories = %d, want 1", checkoutCount)
	}
}

func TestResolvePreviewUsesPreferredPath(t *testing.T) {
	repo := makeGitRepo(t)
	writePreviewSkill(t, repo, "packs/one", []byte("---\nname: duplicate\n---\none\n"))
	selected := []byte("---\nname: duplicate\n---\ntwo\n")
	writePreviewSkill(t, repo, "packs/two", selected)
	gitCommit(t, repo, "initial")

	result, err := ResolvePreview(
		t.Context(),
		NewCheckoutCache(filepath.Join(t.TempDir(), "cache")),
		PreviewRequest{
			Source:        GitSource{CloneURL: repo},
			Name:          "duplicate",
			PreferredPath: "packs/two",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Repository != repo {
		t.Fatalf("Repository = %q, want clone URL %q", result.Repository, repo)
	}
	if result.SkillPath != "packs/two/SKILL.md" || string(result.SkillMD) != string(selected) {
		t.Fatalf("result = %#v, want selected preferred path", result)
	}
}

func TestResolvePreviewReturnsTypedLookupErrors(t *testing.T) {
	repo := makeGitRepo(t)
	writeRemoteSkill(t, repo, "packs/one", "duplicate", "One.")
	writeRemoteSkill(t, repo, "packs/two", "duplicate", "Two.")
	gitCommit(t, repo, "initial")
	cache := NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))

	_, err := ResolvePreview(t.Context(), cache, PreviewRequest{
		Source: GitSource{CloneURL: repo},
		Name:   "missing",
	})
	var missing *MissingSkillError
	if !errors.As(err, &missing) {
		t.Fatalf("missing error = %T %[1]v, want *MissingSkillError", err)
	}

	_, err = ResolvePreview(t.Context(), cache, PreviewRequest{
		Source: GitSource{CloneURL: repo},
		Name:   "duplicate",
	})
	var ambiguous *AmbiguousSkillError
	if !errors.As(err, &ambiguous) {
		t.Fatalf("ambiguous error = %T %[1]v, want *AmbiguousSkillError", err)
	}
	if len(ambiguous.Paths) != 2 {
		t.Fatalf("ambiguous paths = %#v, want two", ambiguous.Paths)
	}
}

func TestResolvePreviewReturnsMalformedDocumentError(t *testing.T) {
	repo := makeGitRepo(t)
	writePreviewSkill(t, repo, "skills/broken", []byte("---\nname: [\n---\nbody\n"))
	gitCommit(t, repo, "initial")

	_, err := ResolvePreview(
		t.Context(),
		NewCheckoutCache(filepath.Join(t.TempDir(), "cache")),
		PreviewRequest{
			Source:        GitSource{CloneURL: repo},
			Name:          "broken",
			PreferredPath: "skills/broken",
		},
	)
	if err == nil {
		t.Fatal("ResolvePreview returned nil error for malformed document")
	}
	var readErr *PreviewReadError
	if errors.As(err, &readErr) {
		t.Fatalf("malformed lookup error unexpectedly classified as raw read error: %v", err)
	}
}

func TestResolvePreviewReturnsTypedReadErrorAfterMatch(t *testing.T) {
	repo := makeGitRepo(t)
	writeRemoteSkill(t, repo, "skills/preview", "preview", "Preview.")
	gitCommit(t, repo, "initial")
	ctx := t.Context()
	readCalled := false

	_, err := resolvePreview(
		ctx,
		NewCheckoutCache(filepath.Join(t.TempDir(), "cache")),
		PreviewRequest{Source: GitSource{CloneURL: repo}, Name: "preview"},
		func(gotCtx context.Context, path string) ([]byte, error) {
			readCalled = true
			if gotCtx != ctx {
				t.Errorf("read context was not propagated")
			}
			if filepath.Base(path) != "SKILL.md" {
				t.Errorf("read path = %q, want SKILL.md", path)
			}
			return nil, os.ErrPermission
		},
	)
	if !readCalled {
		t.Fatal("document reader was not called after matching skill")
	}
	var readErr *PreviewReadError
	if !errors.As(err, &readErr) {
		t.Fatalf("error = %T %[1]v, want *PreviewReadError", err)
	}
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("error = %v, want os.ErrPermission in chain", err)
	}
}

func TestResolvePreviewReturnsTypedCheckoutError(t *testing.T) {
	_, err := ResolvePreview(
		t.Context(),
		NewCheckoutCache(filepath.Join(t.TempDir(), "cache")),
		PreviewRequest{
			Source: GitSource{CloneURL: filepath.Join(t.TempDir(), "missing-repo")},
			Name:   "skill",
		},
	)
	var checkoutErr *PreviewCheckoutError
	if !errors.As(err, &checkoutErr) {
		t.Fatalf("error = %T %[1]v, want *PreviewCheckoutError", err)
	}
}

func TestResolvePreviewPreservesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := ResolvePreview(
		ctx,
		NewCheckoutCache(filepath.Join(t.TempDir(), "cache")),
		PreviewRequest{Source: GitSource{CloneURL: t.TempDir()}, Name: "skill"},
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func writePreviewSkill(t *testing.T, root, rel string, content []byte) {
	t.Helper()
	dir := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), content, 0o644); err != nil {
		t.Fatal(err)
	}
}

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := gitCommandOutput(t.Context(), dir, args...)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(out)
}
