package remote

import (
	"path/filepath"
	"testing"
)

func TestSourceMetadataRoundTrip(t *testing.T) {
	dir := t.TempDir()
	meta := SourceMetadata{
		SourceType:   SourceTypeGitHub,
		Owner:        "vercel-labs",
		Repo:         "skills",
		CloneURL:     "https://github.com/vercel-labs/skills.git",
		Ref:          "main",
		Commit:       "abc123",
		SkillPath:    "skills/svelte-coder",
		UpstreamName: "svelte-coder",
	}
	if err := WriteSourceMetadata(dir, meta); err != nil {
		t.Fatal(err)
	}
	got, ok, err := ReadSourceMetadata(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("metadata not found")
	}
	if got != meta {
		t.Fatalf("metadata = %#v, want %#v", got, meta)
	}
}

func TestReadSourceMetadataMissing(t *testing.T) {
	got, ok, err := ReadSourceMetadata(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatalf("ok = true for missing metadata: %#v", got)
	}
}

func TestSourceIdentityMatchesSameGitHubSkill(t *testing.T) {
	left := SourceMetadata{SourceType: SourceTypeGitHub, Owner: "vercel-labs", Repo: "skills", SkillPath: "skills/svelte-coder"}
	right := SourceMetadata{SourceType: SourceTypeGitHub, Owner: "vercel-labs", Repo: "skills", SkillPath: filepath.ToSlash("skills/svelte-coder")}
	if !left.SameIdentity(right) {
		t.Fatalf("expected same identity: %#v %#v", left, right)
	}
}
