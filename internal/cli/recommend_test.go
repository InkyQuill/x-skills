package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/manifest"
	"github.com/InkyQuill/x-skills/internal/remote"
)

func TestRecommendAndUnrecommendCommands(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	archive := makeSkill(t, cfg.ArchiveSkillsRoot(), "svelte-coder", "Svelte.")
	if err := remote.WriteSourceMetadata(archive, remote.SourceMetadata{SourceType: remote.SourceTypeGitHub, Owner: "owner", Repo: "skills", SkillPath: "skills/svelte-coder"}); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := Execute([]string{"--home", home, "--project-root", project, "recommend", "svelte-coder"}, strings.NewReader(""), &out, &bytes.Buffer{}); err != nil {
		t.Fatalf("recommend command error = %v", err)
	}
	if !strings.Contains(out.String(), "recommended svelte-coder") {
		t.Fatalf("recommend output = %q", out.String())
	}

	out.Reset()
	if err := Execute([]string{"--home", home, "--project-root", project, "unrecommend", "svelte-coder"}, strings.NewReader(""), &out, &bytes.Buffer{}); err != nil {
		t.Fatalf("unrecommend command error = %v", err)
	}
	if !strings.Contains(out.String(), "removed svelte-coder from project recommendations") {
		t.Fatalf("unrecommend output = %q", out.String())
	}
	recommended, err := manifest.LoadRecommended(filepath.Clean(project))
	if err != nil {
		t.Fatal(err)
	}
	if len(recommended.Skills) != 0 {
		t.Fatalf("recommended skills = %#v, want empty", recommended.Skills)
	}
}

func TestRecommendCommandRequiresNames(t *testing.T) {
	err := Execute([]string{"recommend"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "requires at least 1 arg") {
		t.Fatalf("recommend error = %v", err)
	}
}
