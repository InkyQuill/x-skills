package syncer

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/InkyQuill/x-skills/internal/compatibility"
	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/remote"
	"github.com/InkyQuill/x-skills/internal/roots"
)

func TestDiscoverGroupsProjectOccurrencesAndExcludesDestinationsAndGlobals(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	home := t.TempDir()
	cfg := configuredTestConfig(t, project, home)
	agents := projectRoot(cfg, "agents")
	pi := projectRoot(cfg, "pi")
	opencode := projectRoot(cfg, "opencode")
	global := globalRoot(cfg, "pi")

	writeSkill(t, agents.Path, "shared", "frontmatter-name", "same")
	writeSkill(t, pi.Path, "shared", "frontmatter-name", "same")
	if err := os.MkdirAll(opencode.Path, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(agents.Path, "shared"), filepath.Join(opencode.Path, "shared")); err != nil {
		t.Fatal(err)
	}
	writeSkill(t, agents.Path, "divergent", "divergent", "one")
	writeSkill(t, opencode.Path, "divergent", "divergent", "two")
	writeSkill(t, pi.Path, "destination-only", "destination-only", "destination")
	writeSkill(t, global.Path, "global-only", "global-only", "global")

	groups, err := Discover(cfg, []roots.ActiveRoot{pi})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := groupNames(groups), []string{"divergent", "shared"}; !slices.Equal(got, want) {
		t.Fatalf("group names = %v, want %v", got, want)
	}
	if got := len(groups[0].Variants); got != 2 {
		t.Fatalf("divergent variants = %d, want 2", got)
	}
	if groups[0].Variants[0].Fingerprint > groups[0].Variants[1].Fingerprint {
		t.Fatal("divergent variants are not sorted by fingerprint")
	}
	shared := groups[1].Variants[0]
	if shared.Name != "shared" || len(shared.Occurrences) != 2 {
		t.Fatalf("shared candidate = %#v, want basename and two non-destination occurrences", shared)
	}
	if shared.ID != shared.Name+":"+shared.Fingerprint {
		t.Fatalf("candidate ID = %q, want name:fingerprint", shared.ID)
	}
}

func TestDiscoverAssessesCombinedDestinationConsumersFromArchiveMetadata(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	home := t.TempDir()
	cfg := configuredTestConfig(t, project, home)
	source := projectRoot(cfg, "agents")
	pi := projectRoot(cfg, "pi")
	opencode := projectRoot(cfg, "opencode")
	writeSkill(t, source.Path, "partial", "partial", "content")
	writeSkill(t, cfg.ArchiveSkillsRoot(), "partial", "partial", "content")
	if err := remote.WriteSourceMetadata(filepath.Join(cfg.ArchiveSkillsRoot(), "partial"), remote.SourceMetadata{
		Compatibility: &remote.CompatibilityProfile{Agents: []string{"pi"}},
	}); err != nil {
		t.Fatal(err)
	}

	groups, err := Discover(cfg, []roots.ActiveRoot{pi, opencode, pi})
	if err != nil {
		t.Fatal(err)
	}
	assessment := groups[0].Variants[0].Compatibility
	if assessment.State != compatibility.StatePartial || !assessment.Explicit {
		t.Fatalf("compatibility = %#v, want explicit partial", assessment)
	}
}

func configuredTestConfig(t *testing.T, project, home string) config.Config {
	t.Helper()
	configDir := filepath.Join(home, ".x-skills")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	contents := `version: 1
active_roots:
  - {scope: project, target: pi, path: .pi/skills, consumers: [pi]}
  - {scope: project, target: opencode, path: .opencode/skills, consumers: [opencode]}
  - {scope: global, target: pi, path: ~/.pi/skills, consumers: [pi]}
`
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadGlobal(config.Default(project, home))
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func projectRoot(cfg config.Config, target string) roots.ActiveRoot {
	for _, root := range roots.ActiveRoots(cfg, roots.Filter{Scope: config.ScopeProject}) {
		if root.Target == target {
			return root
		}
	}
	panic("missing project root " + target)
}

func globalRoot(cfg config.Config, target string) roots.ActiveRoot {
	for _, root := range roots.ActiveRoots(cfg, roots.Filter{Scope: config.ScopeGlobal}) {
		if root.Target == target {
			return root
		}
	}
	panic("missing global root " + target)
}

func writeSkill(t *testing.T, root, basename, metadataName, body string) {
	t.Helper()
	dir := filepath.Join(root, basename)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	contents := "---\nname: " + metadataName + "\ndescription: test\n---\n" + body + "\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func groupNames(groups []NameGroup) []string {
	names := make([]string, len(groups))
	for i := range groups {
		names[i] = groups[i].Name
	}
	return names
}
