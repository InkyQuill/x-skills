package remote

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDecodeSourceMetadata(t *testing.T) {
	tests := []struct {
		name          string
		data          string
		wantSchema    int
		wantAgnostic  bool
		wantCode      string
		wantField     string
		wantErrString string
	}{
		{
			name:          "unknown field",
			data:          `{"schema_version":2,"unexpected":true}`,
			wantCode:      "metadata.unknown_field",
			wantField:     "unexpected",
			wantErrString: "unknown field",
		},
		{
			name:          "trailing json value",
			data:          `{"schema_version":2} {}`,
			wantCode:      "metadata.trailing_json",
			wantErrString: "multiple JSON values",
		},
		{
			name:          "mistaken top-level agnostic",
			data:          `{"schema_version":2,"agnostic":true}`,
			wantCode:      "metadata.unknown_field",
			wantField:     "agnostic",
			wantErrString: "unknown field",
		},
		{
			name:          "mistaken top-level agents",
			data:          `{"schema_version":2,"agents":["codex"]}`,
			wantCode:      "metadata.unknown_field",
			wantField:     "agents",
			wantErrString: "unknown field",
		},
		{
			name:       "valid schema v1",
			data:       `{"source_type":"github","owner":"acme","repo":"skills","skill_path":"skills/review"}`,
			wantSchema: 1,
		},
		{
			name:       "legacy partial schema v1",
			data:       `{"source_type":"github","owner":""}`,
			wantSchema: 1,
		},
		{
			name:       "legacy schema v1 compatibility ids",
			data:       `{"schema_version":1,"compatibility":{"agents":["Claude","Claude"]}}`,
			wantSchema: 1,
		},
		{
			name:       "schema v2 with source and compatibility omitted",
			data:       `{"schema_version":2}`,
			wantSchema: 2,
		},
		{
			name:         "valid compatibility-only schema v2",
			data:         `{"schema_version":2,"compatibility":{"agnostic":true}}`,
			wantSchema:   2,
			wantAgnostic: true,
		},
		{
			name: "valid full github source",
			data: `{"schema_version":2,"source_type":"github","owner":"acme",` +
				`"repo":"skills","clone_url":"https://github.com/acme/skills.git",` +
				`"ref":"main","commit":"abc","skill_path":"skills/review",` +
				`"upstream_name":"review"}`,
			wantSchema: 2,
		},
		{
			name:          "partial source identity",
			data:          `{"schema_version":2,"source_type":"github","owner":"acme"}`,
			wantCode:      "metadata.source",
			wantField:     "repo",
			wantErrString: "repo",
		},
		{
			name:          "source field without source type",
			data:          `{"schema_version":2,"commit":"abc"}`,
			wantCode:      "metadata.source",
			wantField:     "source_type",
			wantErrString: "source_type",
		},
		{
			name: "empty github identity value",
			data: `{"schema_version":2,"source_type":"github","owner":null,` +
				`"repo":"skills","skill_path":"skills/review"}`,
			wantCode:      "metadata.source",
			wantField:     "owner",
			wantErrString: "owner",
		},
		{
			name:          "null git identity value",
			data:          `{"schema_version":2,"source_type":"git","clone_url":null,"skill_path":"skills/review"}`,
			wantCode:      "metadata.source",
			wantField:     "clone_url",
			wantErrString: "clone_url",
		},
		{
			name:          "unknown source type",
			data:          `{"schema_version":2,"source_type":"archive"}`,
			wantCode:      "metadata.source",
			wantField:     "source_type",
			wantErrString: "unknown source type",
		},
		{
			name:          "compatibility exclusivity",
			data:          `{"schema_version":2,"compatibility":{"agnostic":true,"agents":["codex"]}}`,
			wantCode:      "metadata.compatibility",
			wantField:     "compatibility",
			wantErrString: "exactly one",
		},
		{
			name:          "empty agents",
			data:          `{"schema_version":2,"compatibility":{"agents":[]}}`,
			wantCode:      "metadata.compatibility",
			wantField:     "compatibility.agents",
			wantErrString: "at least one agent",
		},
		{
			name:          "explicit null compatibility",
			data:          `{"schema_version":2,"compatibility":null}`,
			wantCode:      "metadata.compatibility",
			wantField:     "compatibility",
			wantErrString: "must not be null",
		},
		{
			name:          "invalid agent id",
			data:          `{"schema_version":2,"compatibility":{"agents":["Claude"]}}`,
			wantCode:      "metadata.compatibility",
			wantField:     "compatibility.agents",
			wantErrString: "invalid agent id",
		},
		{
			name:          "duplicate agent id",
			data:          `{"schema_version":2,"compatibility":{"agents":["codex","codex"]}}`,
			wantCode:      "metadata.compatibility",
			wantField:     "compatibility.agents",
			wantErrString: "duplicate agent id",
		},
		{
			name:          "unsupported schema",
			data:          `{"schema_version":3}`,
			wantCode:      "metadata.schema",
			wantField:     "schema_version",
			wantErrString: "unsupported source metadata schema version 3",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := DecodeSourceMetadata([]byte(test.data))
			if test.wantCode == "" {
				if err != nil {
					t.Fatalf("DecodeSourceMetadata() error = %v", err)
				}
				if got.SchemaVersion != test.wantSchema {
					t.Fatalf("schema version = %d, want %d", got.SchemaVersion, test.wantSchema)
				}
				if test.wantAgnostic && (got.Compatibility == nil || !got.Compatibility.Agnostic) {
					t.Fatalf("compatibility = %#v, want agnostic", got.Compatibility)
				}
				return
			}

			if err == nil || !strings.Contains(err.Error(), test.wantErrString) {
				t.Fatalf("error = %v, want error containing %q", err, test.wantErrString)
			}
			var metadataErr *MetadataError
			if !errors.As(err, &metadataErr) {
				t.Fatalf("error type = %T, want *MetadataError", err)
			}
			if metadataErr.Code != test.wantCode || metadataErr.Field != test.wantField {
				t.Fatalf(
					"metadata error = {Code: %q, Field: %q}, want {Code: %q, Field: %q}",
					metadataErr.Code,
					metadataErr.Field,
					test.wantCode,
					test.wantField,
				)
			}
		})
	}
}

func TestDecodeSourceMetadataTracksSourceMemberPresence(t *testing.T) {
	fields := []string{
		"source_type",
		"owner",
		"repo",
		"clone_url",
		"ref",
		"commit",
		"skill_path",
		"upstream_name",
	}
	values := []struct {
		name string
		json string
	}{
		{name: "empty string", json: `""`},
		{name: "null", json: "null"},
	}

	for _, field := range fields {
		for _, value := range values {
			t.Run(field+" "+value.name, func(t *testing.T) {
				data := fmt.Sprintf(`{"schema_version":2,%q:%s}`, field, value.json)
				_, err := DecodeSourceMetadata([]byte(data))
				var metadataErr *MetadataError
				if !errors.As(err, &metadataErr) {
					t.Fatalf("error = %v, want *MetadataError", err)
				}
				if metadataErr.Code != "metadata.source" || metadataErr.Field != "source_type" {
					t.Fatalf(
						"metadata error = {Code: %q, Field: %q}, want source_type error",
						metadataErr.Code,
						metadataErr.Field,
					)
				}
			})
		}
	}
}

func TestReadSourceMetadataIgnoresSkillFrontmatterCompatibility(t *testing.T) {
	dir := t.TempDir()
	skill := "---\nname: review\ndescription: Review code\ncompatibility: Designed for Claude Code\n---\nBody\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(skill), 0o644); err != nil {
		t.Fatal(err)
	}
	data := []byte(`{"schema_version":2,"compatibility":{"agnostic":true}}`)
	if err := os.WriteFile(filepath.Join(dir, MetadataFile), data, 0o644); err != nil {
		t.Fatal(err)
	}

	got, ok, err := ReadSourceMetadata(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || got.Compatibility == nil || !got.Compatibility.Agnostic {
		t.Fatalf("metadata = %#v, ok = %v; want agnostic compatibility", got, ok)
	}
}

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

func TestWriteSourceMetadataRoundTripsCompatibilityOnly(t *testing.T) {
	dir := t.TempDir()
	meta := SourceMetadata{
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
	if got.SchemaVersion != metadataSchemaV2 {
		t.Fatalf("schema version = %d, want %d", got.SchemaVersion, metadataSchemaV2)
	}
	if got.Compatibility == nil {
		t.Fatal("compatibility = nil, want agents profile")
	}
	wantAgents := []string{"claude", "codex"}
	if !reflect.DeepEqual(got.Compatibility.Agents, wantAgents) {
		t.Fatalf("compatibility agents = %#v, want %#v", got.Compatibility.Agents, wantAgents)
	}
}

func TestWriteSourceMetadataRejectsInvalidSourceIdentityBeforeWriting(t *testing.T) {
	tests := []struct {
		name             string
		meta             SourceMetadata
		existingMetadata []byte
		wantError        string
	}{
		{
			name: "partial GitHub identity does not create metadata",
			meta: SourceMetadata{
				SourceType: SourceTypeGitHub,
				Owner:      "acme",
			},
			wantError: "source metadata field \"repo\" is required",
		},
		{
			name: "unknown source type does not replace metadata",
			meta: SourceMetadata{
				SourceType: "registry",
			},
			existingMetadata: []byte("existing metadata\n"),
			wantError:        `unknown source type "registry"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			metadataPath := filepath.Join(dir, MetadataFile)
			if tt.existingMetadata != nil {
				if err := os.WriteFile(metadataPath, tt.existingMetadata, 0o644); err != nil {
					t.Fatal(err)
				}
			}

			err := WriteSourceMetadata(dir, tt.meta)
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("error = %v, want %q", err, tt.wantError)
			}
			if !strings.Contains(err.Error(), "encode source metadata") {
				t.Fatalf("error = %v, want encode context", err)
			}

			got, readErr := os.ReadFile(metadataPath)
			switch {
			case tt.existingMetadata == nil && !errors.Is(readErr, os.ErrNotExist):
				t.Fatalf("read metadata error = %v, want not exist", readErr)
			case tt.existingMetadata != nil && readErr != nil:
				t.Fatal(readErr)
			case tt.existingMetadata != nil && !bytes.Equal(got, tt.existingMetadata):
				t.Fatalf("metadata = %q, want unchanged %q", got, tt.existingMetadata)
			}
		})
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
	right := SourceMetadata{SourceType: SourceTypeGitHub, Owner: "vercel-labs", Repo: "skills", SkillPath: `skills\svelte-coder`}
	if !left.SameIdentity(right) {
		t.Fatalf("expected same identity: %#v %#v", left, right)
	}
}

func TestSourceIdentityMatchesSameGitSkill(t *testing.T) {
	left := SourceMetadata{SourceType: SourceTypeGit, CloneURL: "https://example.com/skills.git", SkillPath: "skills/svelte-coder"}
	right := SourceMetadata{SourceType: SourceTypeGit, CloneURL: "https://example.com/skills.git", SkillPath: `skills\svelte-coder`}
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
