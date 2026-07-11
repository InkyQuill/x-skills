package remote

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSourceMetadataRoundTrip(t *testing.T) {
	dir := t.TempDir()
	meta := SourceMetadata{
		SchemaVersion: 2,
		SourceType:    SourceTypeGitHub,
		Owner:         "vercel-labs",
		Repo:          "skills",
		CloneURL:      "https://github.com/vercel-labs/skills.git",
		Ref:           "main",
		Commit:        "abc123",
		SkillPath:     "skills/svelte-coder",
		UpstreamName:  "svelte-coder",
		Compatibility: &CompatibilityProfile{
			Agents: []string{"claude"},
		},
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
	if !reflect.DeepEqual(got, meta) {
		t.Fatalf("metadata = %#v, want %#v", got, meta)
	}
}

func TestSourceMetadataRoundTripSortsCompatibilityAgents(t *testing.T) {
	dir := t.TempDir()
	meta := SourceMetadata{
		SchemaVersion: 2,
		SourceType:    SourceTypeGitHub,
		Owner:         "acme",
		Repo:          "skills",
		CloneURL:      "https://github.com/acme/skills.git",
		Commit:        "abc",
		SkillPath:     "skills/review",
		Compatibility: &CompatibilityProfile{Agents: []string{"codex", "claude"}},
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
	want := []string{"claude", "codex"}
	if !reflect.DeepEqual(got.Compatibility.Agents, want) {
		t.Fatalf("compatibility agents = %#v, want %#v", got.Compatibility.Agents, want)
	}
}

func TestSourceMetadataRoundTripAgnosticCompatibility(t *testing.T) {
	dir := t.TempDir()
	meta := SourceMetadata{
		SourceType:    SourceTypeGitHub,
		Owner:         "acme",
		Repo:          "skills",
		CloneURL:      "https://github.com/acme/skills.git",
		Commit:        "abc",
		SkillPath:     "skills/review",
		Compatibility: &CompatibilityProfile{Agnostic: true},
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
	if got.SchemaVersion != 2 {
		t.Fatalf("schema version = %d, want 2", got.SchemaVersion)
	}
	if got.Compatibility == nil || !got.Compatibility.Agnostic || len(got.Compatibility.Agents) != 0 {
		t.Fatalf("compatibility = %#v, want agnostic profile", got.Compatibility)
	}
}

func TestWriteSourceMetadataRejectsAgnosticCompatibilityWithAgents(t *testing.T) {
	meta := SourceMetadata{
		Compatibility: &CompatibilityProfile{Agnostic: true, Agents: []string{"claude"}},
	}
	err := WriteSourceMetadata(t.TempDir(), meta)
	if err == nil || !strings.Contains(err.Error(), "compatibility") {
		t.Fatalf("error = %v, want compatibility validation error", err)
	}
}

func TestReadSourceMetadataTreatsMissingSchemaVersionAsV1(t *testing.T) {
	dir := t.TempDir()
	data := []byte(`{"source_type":"github","owner":"acme","repo":"skills","clone_url":"https://github.com/acme/skills.git","commit":"abc","skill_path":"skills/review"}`)
	if err := os.WriteFile(filepath.Join(dir, MetadataFile), data, 0o644); err != nil {
		t.Fatal(err)
	}

	got, ok, err := ReadSourceMetadata(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("metadata not found")
	}
	if got.SchemaVersion != 1 {
		t.Fatalf("schema version = %d, want 1", got.SchemaVersion)
	}
}

func TestReadSourceMetadataRejectsUnsupportedSchemaVersion(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, MetadataFile), []byte(`{"schema_version":3}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, ok, err := ReadSourceMetadata(dir)
	if err == nil || !strings.Contains(err.Error(), "unsupported source metadata schema version 3") {
		t.Fatalf("error = %v, want unsupported schema version", err)
	}
	if ok {
		t.Fatal("ok = true for unsupported schema")
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

func TestSourceIdentityMatchesSameGitSkill(t *testing.T) {
	left := SourceMetadata{SourceType: SourceTypeGit, CloneURL: "https://example.com/skills.git", SkillPath: "skills/svelte-coder"}
	right := SourceMetadata{SourceType: SourceTypeGit, CloneURL: "https://example.com/skills.git", SkillPath: filepath.ToSlash("skills/svelte-coder")}
	if !left.SameIdentity(right) {
		t.Fatalf("expected same identity: %#v %#v", left, right)
	}
}

func TestSourceIdentityDoesNotMatchUnknownSourceType(t *testing.T) {
	left := SourceMetadata{SourceType: "archive", CloneURL: "https://example.com/skills.tar.gz", SkillPath: "skills/svelte-coder"}
	right := SourceMetadata{SourceType: "archive", CloneURL: "https://example.com/skills.tar.gz", SkillPath: "skills/svelte-coder"}
	if left.SameIdentity(right) {
		t.Fatalf("expected unknown source type not to match: %#v %#v", left, right)
	}
}

func TestSourceIdentityDoesNotMatchDifferentSourceTypes(t *testing.T) {
	left := SourceMetadata{SourceType: SourceTypeGitHub, Owner: "vercel-labs", Repo: "skills", CloneURL: "https://github.com/vercel-labs/skills.git", SkillPath: "skills/svelte-coder"}
	right := SourceMetadata{SourceType: SourceTypeGit, CloneURL: "https://github.com/vercel-labs/skills.git", SkillPath: "skills/svelte-coder"}
	if left.SameIdentity(right) {
		t.Fatalf("expected different source types not to match: %#v %#v", left, right)
	}
}

func TestSourceIdentityIgnoresCompatibility(t *testing.T) {
	left := SourceMetadata{
		SourceType:    SourceTypeGitHub,
		Owner:         "vercel-labs",
		Repo:          "skills",
		SkillPath:     "skills/svelte-coder",
		Compatibility: &CompatibilityProfile{Agnostic: true},
	}
	right := SourceMetadata{
		SourceType:    SourceTypeGitHub,
		Owner:         "vercel-labs",
		Repo:          "skills",
		SkillPath:     "skills/svelte-coder",
		Compatibility: &CompatibilityProfile{Agents: []string{"claude"}},
	}
	if !left.SameIdentity(right) {
		t.Fatalf("expected compatibility not to affect identity: %#v %#v", left, right)
	}
}
