package remote

import (
	"strings"
	"testing"
)

func TestParseAddSource(t *testing.T) {
	tests := []struct {
		name      string
		source    string
		gitURL    string
		ref       string
		want      GitSource
		wantNames []string
		wantPath  string
	}{
		{
			name:   "github shorthand",
			source: "vercel-labs/skills",
			want: GitSource{
				CloneURL: "https://github.com/vercel-labs/skills.git",
				Owner:    "vercel-labs",
				Repo:     "skills",
			},
		},
		{
			name:   "github shorthand with skill",
			source: "vercel-labs/skills@next-best-practices",
			want: GitSource{
				CloneURL: "https://github.com/vercel-labs/skills.git",
				Owner:    "vercel-labs",
				Repo:     "skills",
			},
			wantNames: []string{"next-best-practices"},
		},
		{
			name:   "github tree url",
			source: "https://github.com/vercel-labs/skills/tree/main/skills/next-best-practices",
			want: GitSource{
				CloneURL: "https://github.com/vercel-labs/skills.git",
				Owner:    "vercel-labs",
				Repo:     "skills",
				Ref:      "main",
			},
			wantPath: "skills/next-best-practices",
		},
		{
			name:   "github tree url with slash ref and skills path",
			source: "https://github.com/vercel-labs/skills/tree/feature/add-parity/skills/next-best-practices",
			want: GitSource{
				CloneURL: "https://github.com/vercel-labs/skills.git",
				Owner:    "vercel-labs",
				Repo:     "skills",
				Ref:      "feature/add-parity",
			},
			wantPath: "skills/next-best-practices",
		},
		{
			name:   "generic git",
			gitURL: "https://gitlab.com/acme/skills.git",
			ref:    "release",
			want: GitSource{
				CloneURL: "https://gitlab.com/acme/skills.git",
				Ref:      "release",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseAddSource(tt.source, tt.gitURL, tt.ref)
			if err != nil {
				t.Fatal(err)
			}
			if got.Source != tt.want {
				t.Fatalf("Source = %#v, want %#v", got.Source, tt.want)
			}
			if !sameStrings(got.Names, tt.wantNames) {
				t.Fatalf("Names = %#v, want %#v", got.Names, tt.wantNames)
			}
			if got.PreferredPath != tt.wantPath {
				t.Fatalf("PreferredPath = %q, want %q", got.PreferredPath, tt.wantPath)
			}
		})
	}
}

func TestParseAddSourceRejectsUnsupportedURLInstalls(t *testing.T) {
	tests := []string{
		"https://github.com/vercel-labs/skills/archive/refs/heads/main.zip",
		"https://raw.githubusercontent.com/vercel-labs/skills/main/SKILL.md",
		"https://example.com/SKILL.md",
		"file:///tmp/SKILL.md",
	}

	for _, source := range tests {
		t.Run(source, func(t *testing.T) {
			_, err := ParseAddSource(source, "", "")
			if err == nil {
				t.Fatal("expected unsupported URL install error")
			}
			if !strings.Contains(err.Error(), "URL installs are not supported yet") {
				t.Fatalf("error = %q, want unsupported URL installs", err)
			}
		})
	}
}

func sameStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
