package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/InkyQuill/x-skills/internal/builtin"
	"github.com/InkyQuill/x-skills/internal/config"
)

func TestDoctorFixBuiltInsNonInteractiveDoesNotGuessDestination(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	var out bytes.Buffer
	if err := Execute([]string{"--home", home, "--project-root", project, "-y", "doctor", "--fix"}, strings.NewReader(""), &out, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "archived but inactive") {
		t.Fatalf("output missing archive-only status:\n%s", out.String())
	}
	catalog, _ := builtin.List()
	cfg := config.Default(project, home)
	for _, skill := range catalog {
		if _, err := os.Stat(filepath.Join(cfg.ArchiveSkillsRoot(), skill.Name)); err != nil {
			t.Fatalf("archive %s: %v", skill.Name, err)
		}
		for _, target := range []string{config.TargetAgents, config.TargetClaude, config.TargetCodex} {
			if _, err := os.Lstat(filepath.Join(cfg.MustActiveRoot(config.ScopeGlobal, target), skill.Name)); !os.IsNotExist(err) {
				t.Fatalf("doctor guessed %s destination for %s: %v", target, skill.Name, err)
			}
		}
	}
}

func TestDoctorFixBuiltInsLinksOnlyExplicitGlobalDestination(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	var out bytes.Buffer
	err := Execute([]string{"--home", home, "--project-root", project, "-y", "doctor", "--fix", "--at", "global:agents"}, strings.NewReader(""), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "archived and linked") {
		t.Fatalf("output missing linked status:\n%s", out.String())
	}
}

func TestDoctorFixBuiltInsRejectsProjectDestination(t *testing.T) {
	err := Execute([]string{"--home", t.TempDir(), "--project-root", t.TempDir(), "-y", "doctor", "--fix", "--at", "project:agents"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "global") {
		t.Fatalf("error = %v, want global destination rejection", err)
	}
}

func TestDoctorFixBuiltInsRejectsProjectDestinationWithBrokenSymlink(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	root := filepath.Join(project, ".agents", "skills")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	broken := filepath.Join(root, "broken")
	if err := os.Symlink(filepath.Join(home, "missing"), broken); err != nil {
		t.Fatal(err)
	}
	err := Execute([]string{"--home", home, "--project-root", project, "-y", "doctor", "--fix", "--at", "project:agents"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "global") {
		t.Fatalf("error = %v, want global destination rejection", err)
	}
	if _, statErr := os.Lstat(broken); statErr != nil {
		t.Fatalf("broken symlink mutated before destination validation: %v", statErr)
	}
}

func TestDoctorFixBuiltInsInteractiveShowsGlobalChecklistWithAgentsPreselected(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	var out bytes.Buffer
	err := Execute([]string{"--home", home, "--project-root", project, "doctor", "--fix"}, strings.NewReader("\n"), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"[x] ~Ag", "[ ] ~Cl", "[ ] ~Cd", "Archive only"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("interactive checklist missing %q:\n%s", want, out.String())
		}
	}
}

func TestDoctorReportsAndFixesBrokenSymlink(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	root := filepath.Join(project, ".claude", "skills")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "chapter-drafter")
	if err := os.Symlink(filepath.Join(home, "missing"), link); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	err := Execute([]string{"--home", home, "--project-root", project, "doctor"}, strings.NewReader(""), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "broken") || !strings.Contains(out.String(), "chapter-drafter") {
		t.Fatalf("doctor output:\n%s", out.String())
	}
	if !strings.Contains(out.String(), link) || !strings.Contains(out.String(), "resolve symlink") {
		t.Fatalf("doctor output missing path or reason:\n%s", out.String())
	}

	out.Reset()
	err = Execute([]string{"--home", home, "--project-root", project, "-y", "doctor", "--fix"}, strings.NewReader(""), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(link); !os.IsNotExist(err) {
		t.Fatalf("link still exists or unexpected err: %v", err)
	}
	if !strings.Contains(out.String(), "removed") {
		t.Fatalf("fix output:\n%s", out.String())
	}
}

func TestDoctorFixRejectsProjectScopeBeforeMutatingBrokenLinks(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	projectRoot := filepath.Join(project, ".claude", "skills")
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	projectLink := filepath.Join(projectRoot, "project-broken")
	if err := os.Symlink(filepath.Join(home, "missing-project"), projectLink); err != nil {
		t.Fatal(err)
	}

	globalRoot := filepath.Join(home, ".claude", "skills")
	if err := os.MkdirAll(globalRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	globalLink := filepath.Join(globalRoot, "global-broken")
	if err := os.Symlink(filepath.Join(home, "missing-global"), globalLink); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	err := Execute([]string{"--home", home, "--project-root", project, "-y", "doctor", "--fix", "--at", ".Cl"}, strings.NewReader(""), &out, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "global") {
		t.Fatalf("error = %v, want global destination rejection", err)
	}
	if _, err := os.Lstat(projectLink); err != nil {
		t.Fatalf("project link changed: %v", err)
	}
	if _, err := os.Lstat(globalLink); err != nil {
		t.Fatalf("global link was changed: %v", err)
	}
}

func TestDoctorFixWithoutYesReturnsErrorAndDoesNotMutate(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	root := filepath.Join(project, ".claude", "skills")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "chapter-drafter")
	if err := os.Symlink(filepath.Join(home, "missing"), link); err != nil {
		t.Fatal(err)
	}

	err := Execute([]string{"--home", home, "--project-root", project, "doctor", "--fix"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected confirmation error")
	}
	if !strings.Contains(err.Error(), "requires confirmation") {
		t.Fatalf("error = %q, want confirmation", err)
	}
	info, statErr := os.Lstat(link)
	if statErr != nil {
		t.Fatalf("link was removed: %v", statErr)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("mode = %v, want symlink", info.Mode())
	}
}
