package manifest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	testFingerprintA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	testFingerprintB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

const recommendedFixture = `version: 1
skills:
  - name: using-svelte-5
    source:
      type: github
      repository: InkyQuill/x-skills
      path: skills/using-svelte-5
      ref: main
`

const localFixture = `version: 1
skills:
  - name: private-review
    source:
      type: archive
    fingerprint: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
`

func TestLoadRecommended(t *testing.T) {
	root := writeFixture(t, RecommendedFilename, recommendedFixture)

	got, err := LoadRecommended(root)
	if err != nil {
		t.Fatalf("LoadRecommended() error = %v", err)
	}
	if got.Version != 1 || len(got.Skills) != 1 {
		t.Fatalf("LoadRecommended() = %#v", got)
	}
	skill := got.Skills[0]
	if skill.Name != "using-svelte-5" || skill.Source.Type != SourceGitHub ||
		skill.Source.Repository != "InkyQuill/x-skills" || skill.Source.Path != "skills/using-svelte-5" ||
		skill.Source.Ref != "main" {
		t.Fatalf("LoadRecommended() skill = %#v", skill)
	}
}

func TestLoadLocal(t *testing.T) {
	root := writeFixture(t, LocalFilename, localFixture)

	got, err := LoadLocal(root)
	if err != nil {
		t.Fatalf("LoadLocal() error = %v", err)
	}
	if len(got.Skills) != 1 || got.Skills[0].Source.Type != SourceArchive ||
		got.Skills[0].Fingerprint != testFingerprintA {
		t.Fatalf("LoadLocal() = %#v", got)
	}
}

func TestLoadRejectsInvalidManifest(t *testing.T) {
	tests := []struct {
		name        string
		recommended bool
		contents    string
		want        string
	}{
		{name: "unknown field", contents: "version: 1\nunknown: true\nskills: []\n", want: "field unknown not found"},
		{name: "duplicate names", contents: "version: 1\nskills:\n  - name: same\n    source: {type: archive}\n    fingerprint: " + testFingerprintA + "\n  - name: same\n    source: {type: archive}\n    fingerprint: " + testFingerprintA + "\n", want: "duplicate skill name"},
		{name: "invalid name", contents: "version: 1\nskills:\n  - name: ../escape\n    source: {type: archive}\n", want: "invalid skill name"},
		{name: "unsupported version", contents: "version: 2\nskills: []\n", want: "unsupported manifest version"},
		{name: "archive recommended", recommended: true, contents: localFixture, want: "archive source"},
		{name: "archive fingerprint missing", contents: "version: 1\nskills:\n  - name: local\n    source: {type: archive}\n", want: "archive source requires a content fingerprint"},
		{name: "archive fingerprint malformed", contents: "version: 1\nskills:\n  - name: local\n    source: {type: archive}\n    fingerprint: not-a-fingerprint\n", want: "invalid content fingerprint"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filename := LocalFilename
			load := LoadLocal
			if tt.recommended {
				filename = RecommendedFilename
				load = LoadRecommended
			}
			root := writeFixture(t, filename, tt.contents)
			_, err := load(root)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("load error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestLoadLocalNormalizesArchiveFingerprint(t *testing.T) {
	root := writeFixture(t, LocalFilename, "version: 1\nskills:\n  - name: local\n    source: {type: archive}\n    fingerprint: SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA\n")

	got, err := LoadLocal(root)
	if err != nil {
		t.Fatalf("LoadLocal() error = %v", err)
	}
	if got.Skills[0].Fingerprint != strings.Repeat("a", 64) {
		t.Fatalf("fingerprint = %q, want normalized digest", got.Skills[0].Fingerprint)
	}
}

func TestWriteLocalRejectsInvalidArchiveFingerprint(t *testing.T) {
	tests := []struct {
		name        string
		fingerprint string
	}{
		{name: "missing"},
		{name: "malformed", fingerprint: "sha256:abc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := WriteLocal(t.TempDir(), Manifest{Version: 1, Skills: []Skill{{
				Name: "local", Source: Source{Type: SourceArchive}, Fingerprint: tt.fingerprint,
			}}})
			if err == nil || !strings.Contains(err.Error(), "fingerprint") {
				t.Fatalf("WriteLocal() error = %v, want fingerprint validation error", err)
			}
		})
	}
}

func TestWriteRecommendedIsDeterministicAndNormalizesPaths(t *testing.T) {
	root := t.TempDir()
	manifest := Manifest{Version: 1, Skills: []Skill{
		{Name: "zebra", Source: Source{Type: SourceGit, Repository: "https://example.test/repo.git", Path: `skills\\zebra`}},
		{Name: "Alpha", Source: Source{Type: SourceGitHub, Repository: "owner/repo", Path: `skills\\alpha`}},
	}}

	if err := WriteRecommended(root, manifest); err != nil {
		t.Fatalf("WriteRecommended() error = %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, RecommendedFilename))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if strings.Index(text, "name: Alpha") > strings.Index(text, "name: zebra") {
		t.Fatalf("skills not sorted case-insensitively:\n%s", text)
	}
	if strings.Contains(text, `\\`) || !strings.Contains(text, "path: skills/alpha") {
		t.Fatalf("paths not normalized:\n%s", text)
	}
	if strings.Contains(text, "fingerprint:") || strings.Contains(text, "ref:") {
		t.Fatalf("empty optional fields emitted:\n%s", text)
	}
	info, err := os.Stat(filepath.Join(root, RecommendedFilename))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("mode = %o, want 644", info.Mode().Perm())
	}

	got, err := LoadRecommended(root)
	if err != nil {
		t.Fatalf("round trip: %v", err)
	}
	if got.Skills[0].Source.Path != "skills/alpha" || got.Skills[1].Source.Path != "skills/zebra" {
		t.Fatalf("round trip = %#v", got)
	}
}

func TestWriteLocalRoundTrip(t *testing.T) {
	root := t.TempDir()
	want := Manifest{Version: 1, Skills: []Skill{{
		Name: "private-review", Source: Source{Type: SourceArchive}, Fingerprint: testFingerprintA,
	}}}
	if err := WriteLocal(root, want); err != nil {
		t.Fatalf("WriteLocal() error = %v", err)
	}
	got, err := LoadLocal(root)
	if err != nil {
		t.Fatalf("LoadLocal() error = %v", err)
	}
	if len(got.Skills) != 1 || got.Skills[0] != want.Skills[0] {
		t.Fatalf("round trip = %#v, want %#v", got, want)
	}
}

func writeFixture(t *testing.T, name, contents string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, name), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}
