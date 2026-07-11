package manifest

import (
	"reflect"
	"testing"

	"github.com/InkyQuill/x-skills/internal/remote"
)

func TestEffectiveMergesCommittedAndLocalIntent(t *testing.T) {
	recommended := Manifest{Version: 1, Skills: []Skill{
		{Name: "recommended-only", Source: Source{Type: SourceGitHub, Repository: "owner/repo", Path: "skills/recommended-only"}},
		{
			Name:          "shared",
			Source:        Source{Type: SourceGitHub, Repository: "committed/repo", Path: "skills/shared", Ref: "main"},
			Compatibility: &remote.CompatibilityProfile{Agents: []string{"codex"}},
		},
	}}
	local := Manifest{Version: 1, Skills: []Skill{
		{Name: "local-only", Source: Source{Type: SourceArchive}, Fingerprint: "sha256:local"},
		{
			Name:          "shared",
			Source:        Source{Type: SourceGit, Repository: "https://example.test/local.git", Path: "shared"},
			Compatibility: &remote.CompatibilityProfile{Agnostic: true},
			Fingerprint:   "sha256:machine-local",
		},
	}}

	got, notices := Effective(recommended, local)

	if got.Version != 1 {
		t.Fatalf("Effective() version = %d, want 1", got.Version)
	}
	wantNames := []string{"local-only", "recommended-only", "shared"}
	gotNames := make([]string, len(got.Skills))
	for i, skill := range got.Skills {
		gotNames[i] = skill.Name
	}
	if !reflect.DeepEqual(gotNames, wantNames) {
		t.Fatalf("Effective() names = %v, want %v", gotNames, wantNames)
	}
	shared := got.Skills[2]
	if shared.Source != recommended.Skills[1].Source ||
		!reflect.DeepEqual(shared.Compatibility, recommended.Skills[1].Compatibility) ||
		shared.Fingerprint != recommended.Skills[1].Fingerprint {
		t.Fatalf("Effective() shared skill = %#v, want committed identity %#v", shared, recommended.Skills[1])
	}
	if len(notices) != 1 || notices[0].Skill != "shared" {
		t.Fatalf("Effective() notices = %#v, want one notice for shared", notices)
	}
}

func TestEffectiveDoesNotNoticeMatchingIdentity(t *testing.T) {
	skill := Skill{
		Name:          "shared",
		Source:        Source{Type: SourceGitHub, Repository: "owner/repo", Path: "skills/shared", Ref: "main"},
		Compatibility: &remote.CompatibilityProfile{Agents: []string{"claude", "codex"}},
		Fingerprint:   "sha256:same",
	}

	got, notices := Effective(
		Manifest{Version: 1, Skills: []Skill{skill}},
		Manifest{Version: 1, Skills: []Skill{skill}},
	)

	if len(notices) != 0 {
		t.Fatalf("Effective() notices = %#v, want none", notices)
	}
	if len(got.Skills) != 1 || !reflect.DeepEqual(got.Skills[0], skill) {
		t.Fatalf("Effective() = %#v, want %#v", got, skill)
	}
}

func TestEffectiveDoesNotMutateInputsOrAliasCompatibility(t *testing.T) {
	recommended := Manifest{Version: 1, Skills: []Skill{{
		Name: "Zulu", Source: Source{Type: SourceGitHub, Repository: "owner/repo", Path: "skills/Zulu"},
		Compatibility: &remote.CompatibilityProfile{Agents: []string{"codex", "claude"}},
	}}}
	local := Manifest{Version: 1, Skills: []Skill{{
		Name: "alpha", Source: Source{Type: SourceArchive}, Fingerprint: "sha256:alpha",
		Compatibility: &remote.CompatibilityProfile{Agents: []string{"gemini"}},
	}}}
	wantRecommended := cloneManifest(recommended)
	wantLocal := cloneManifest(local)

	got, _ := Effective(recommended, local)
	got.Skills[0].Compatibility.Agents[0] = "changed"
	got.Skills[1].Compatibility.Agents[0] = "changed"

	if !reflect.DeepEqual(recommended, wantRecommended) {
		t.Fatalf("Effective() mutated recommended: got %#v, want %#v", recommended, wantRecommended)
	}
	if !reflect.DeepEqual(local, wantLocal) {
		t.Fatalf("Effective() mutated local: got %#v, want %#v", local, wantLocal)
	}
}

func TestEffectiveKeepsExactCaseVariantsInDeterministicOrder(t *testing.T) {
	recommended := Manifest{Version: 1, Skills: []Skill{
		{Name: "alpha", Source: Source{Type: SourceGitHub, Repository: "owner/repo", Path: "skills/lower"}},
	}}
	local := Manifest{Version: 1, Skills: []Skill{
		{Name: "Alpha", Source: Source{Type: SourceArchive}, Fingerprint: "sha256:upper"},
	}}

	for range 20 {
		got, notices := Effective(recommended, local)
		if len(notices) != 0 {
			t.Fatalf("Effective() notices = %#v, want none for exact-name-distinct skills", notices)
		}
		if len(got.Skills) != 2 || got.Skills[0].Name != "Alpha" || got.Skills[1].Name != "alpha" {
			t.Fatalf("Effective() skills = %#v, want Alpha then alpha", got.Skills)
		}
	}
}
