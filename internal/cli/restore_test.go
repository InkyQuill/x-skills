package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/fingerprint"
	"github.com/InkyQuill/x-skills/internal/manifest"
)

func TestRestoreRequiresExplicitProjectDestination(t *testing.T) {
	err := Execute([]string{"restore"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "at least one --at") {
		t.Fatalf("restore error = %v", err)
	}

	home, project := t.TempDir(), t.TempDir()
	err = Execute([]string{"--home", home, "--project-root", project, "restore", "--at", "~Ag"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "project Skills Folder") {
		t.Fatalf("global restore error = %v", err)
	}
}

func TestRestoreAdditiveLeavesExtrasAndPrintsGroupedPlan(t *testing.T) {
	home, project := t.TempDir(), t.TempDir()
	cfg := config.Default(project, home)
	writeRestoreCLISkill(t, cfg, "wanted")
	makeSkill(t, cfg.MustActiveRoot(config.ScopeProject, config.TargetAgents), "extra", "Extra.")

	var out bytes.Buffer
	err := Execute([]string{"--home", home, "--project-root", project, "-y", "restore", "--at", ".Ag"}, strings.NewReader(""), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	for _, group := range []string{"available", "unavailable", "links", "migrations", "removals"} {
		if !strings.Contains(out.String(), group) {
			t.Fatalf("restore output missing %q:\n%s", group, out.String())
		}
	}
	if _, err := os.Stat(filepath.Join(cfg.MustActiveRoot(config.ScopeProject, config.TargetAgents), "extra")); err != nil {
		t.Fatalf("additive restore removed extra: %v", err)
	}
}

func TestRestoreFullPrintsPlanAndRequiresConfirmation(t *testing.T) {
	home, project := t.TempDir(), t.TempDir()
	cfg := config.Default(project, home)
	makeSkill(t, cfg.MustActiveRoot(config.ScopeProject, config.TargetAgents), "extra", "Extra.")

	var out bytes.Buffer
	err := Execute([]string{"--home", home, "--project-root", project, "--no-input", "restore", "--full", "--at", ".Ag"}, strings.NewReader(""), &out, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "requires confirmation") {
		t.Fatalf("full restore error = %v", err)
	}
	if !strings.Contains(out.String(), "migrations\n  extra") {
		t.Fatalf("full restore output = %q", out.String())
	}
}

func TestRestoreConflictRequiresExplicitRenameEvenWithYes(t *testing.T) {
	home, project := t.TempDir(), t.TempDir()
	cfg := config.Default(project, home)
	writeRestoreCLISkill(t, cfg, "wanted")
	makeSkill(t, cfg.MustActiveRoot(config.ScopeProject, config.TargetAgents), "wanted", "Different.")

	err := Execute([]string{"--home", home, "--project-root", project, "--no-input", "-y", "restore", "--at", ".Ag"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "requires an archive name") {
		t.Fatalf("restore conflict error = %v", err)
	}
}

func writeRestoreCLISkill(t *testing.T, cfg config.Config, name string) {
	t.Helper()
	archive := makeSkill(t, cfg.ArchiveSkillsRoot(), name, "Archived.")
	fp, err := fingerprint.Directory(archive)
	if err != nil {
		t.Fatal(err)
	}
	if err := manifest.WriteLocal(cfg.ProjectRoot, manifest.Manifest{Version: 1, Skills: []manifest.Skill{{Name: name, Source: manifest.Source{Type: manifest.SourceArchive}, Fingerprint: fp}}}); err != nil {
		t.Fatal(err)
	}
}
