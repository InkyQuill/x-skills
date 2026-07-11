package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/InkyQuill/x-skills/internal/actions"
	"github.com/InkyQuill/x-skills/internal/compatibility"
	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/roots"
	"github.com/InkyQuill/x-skills/internal/syncer"
)

func TestSyncRequiresExplicitProjectDestination(t *testing.T) {
	err := Execute([]string{"sync", "--all"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "at least one --at") {
		t.Fatalf("sync error = %v", err)
	}

	home, project := t.TempDir(), t.TempDir()
	err = Execute([]string{"--home", home, "--project-root", project, "sync", "--all", "--at", "~Ag"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "project Skills Folder") {
		t.Fatalf("global sync error = %v", err)
	}
}

func TestSyncNonTTYRequiresExplicitSelection(t *testing.T) {
	home, project := t.TempDir(), t.TempDir()
	err := Execute([]string{"--home", home, "--project-root", project, "sync", "--at", ".Ag"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "requires --all or --skill") {
		t.Fatalf("sync error = %v", err)
	}
}

func TestSyncNonTTYRequiresExplicitConfirmation(t *testing.T) {
	home, project := t.TempDir(), t.TempDir()
	cfg := config.Default(project, home)
	makeSkill(t, cfg.MustActiveRoot(config.ScopeProject, config.TargetClaude), "one", "One.")
	err := Execute([]string{"--home", home, "--project-root", project, "sync", "--at", ".Ag", "--skill", "one"}, strings.NewReader("y\n"), &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "requires confirmation; rerun with -y") {
		t.Fatalf("sync error = %v", err)
	}
}

func TestSyncAllAndSkillAreMutuallyExclusive(t *testing.T) {
	err := Execute([]string{"sync", "--at", ".Ag", "--all", "--skill", "one"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "--all and --skill are mutually exclusive") {
		t.Fatalf("sync error = %v", err)
	}
}

func TestSyncAllSelectsUniqueNonIncompatibleCandidates(t *testing.T) {
	groups := []syncer.NameGroup{
		{Name: "compatible", Variants: []syncer.Candidate{{ID: "compatible:a", Name: "compatible", Compatibility: compatibility.Assessment{State: compatibility.StateCompatible}}}},
		{Name: "partial", Variants: []syncer.Candidate{{ID: "partial:b", Name: "partial", Compatibility: compatibility.Assessment{State: compatibility.StatePartial}}}},
		{Name: "unknown", Variants: []syncer.Candidate{{ID: "unknown:c", Name: "unknown", Compatibility: compatibility.Assessment{State: compatibility.StateUnknown}}}},
		{Name: "incompatible", Variants: []syncer.Candidate{{ID: "incompatible:d", Name: "incompatible", Compatibility: compatibility.Assessment{State: compatibility.StateIncompatible}}}},
		{Name: "divergent", Variants: []syncer.Candidate{{ID: "divergent:e", Name: "divergent"}, {ID: "divergent:f", Name: "divergent"}}},
	}

	selection, err := selectSyncCandidates(groups, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(selection.CandidateIDs, ",")
	if got != "compatible:a,partial:b,unknown:c" {
		t.Fatalf("selected = %q", got)
	}
}

func TestSyncSkillSelectsExactNamesAndRejectsDivergentVariants(t *testing.T) {
	groups := []syncer.NameGroup{
		{Name: "one", Variants: []syncer.Candidate{{ID: "one:a", Name: "one"}}},
		{Name: "two", Variants: []syncer.Candidate{
			{ID: "two:b", Name: "two", Occurrences: []actions.ActiveSkill{{Root: roots.ActiveRoot{Target: config.TargetClaude}}}},
			{ID: "two:c", Name: "two", Occurrences: []actions.ActiveSkill{{Root: roots.ActiveRoot{Target: config.TargetCodex}}}},
		}},
	}
	selection, err := selectSyncCandidates(groups, false, []string{"one"})
	if err != nil || len(selection.CandidateIDs) != 1 || selection.CandidateIDs[0] != "one:a" {
		t.Fatalf("selection = %#v, err = %v", selection, err)
	}
	_, err = selectSyncCandidates(groups, false, []string{"two"})
	if err == nil || !strings.Contains(err.Error(), "claude") || !strings.Contains(err.Error(), "codex") {
		t.Fatalf("divergent error = %v", err)
	}
	_, err = selectSyncCandidates(groups, false, []string{"missing"})
	if err == nil || !strings.Contains(err.Error(), `skill not found: "missing"`) {
		t.Fatalf("missing error = %v", err)
	}
}

func TestSyncInteractiveDefaultsAndLabels(t *testing.T) {
	groups := []syncer.NameGroup{
		{Name: "good", Variants: []syncer.Candidate{{ID: "good:a", Name: "good", Compatibility: compatibility.Assessment{State: compatibility.StateCompatible}}}},
		{Name: "bad", Variants: []syncer.Candidate{{ID: "bad:b", Name: "bad", Compatibility: compatibility.Assessment{State: compatibility.StateIncompatible}}}},
	}
	options, defaults := syncChecklistOptions(groups)
	if len(options) != 2 || !strings.Contains(options[0].Label, "compatible") || !strings.Contains(options[1].Label, "incompatible") {
		t.Fatalf("options = %#v", options)
	}
	if len(defaults) != 1 || defaults[0] != "good:a" {
		t.Fatalf("defaults = %#v", defaults)
	}
}

func TestSyncYesDoesNotResolveVariantOrDestinationConflict(t *testing.T) {
	home, project := t.TempDir(), t.TempDir()
	cfg := config.Default(project, home)
	makeSkill(t, cfg.MustActiveRoot(config.ScopeProject, config.TargetClaude), "same", "Claude.")
	makeSkill(t, cfg.MustActiveRoot(config.ScopeProject, config.TargetCodex), "same", "Codex.")

	err := Execute([]string{"--home", home, "--project-root", project, "-y", "sync", "--at", ".Ag", "--skill", "same"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "multiple variants") {
		t.Fatalf("variant error = %v", err)
	}

	makeSkill(t, cfg.MustActiveRoot(config.ScopeProject, config.TargetClaude), "conflict", "Source.")
	makeSkill(t, cfg.MustActiveRoot(config.ScopeProject, config.TargetAgents), "conflict", "Destination.")
	err = Execute([]string{"--home", home, "--project-root", project, "-y", "sync", "--at", ".Ag", "--skill", "conflict"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "conflict resolution requires an interactive terminal") {
		t.Fatalf("conflict error = %v", err)
	}
}

func TestSyncAppliesResolvedPlanAfterConfirmation(t *testing.T) {
	home, project := t.TempDir(), t.TempDir()
	cfg := config.Default(project, home)
	makeSkill(t, cfg.MustActiveRoot(config.ScopeProject, config.TargetClaude), "one", "One.")

	var out bytes.Buffer
	err := Execute([]string{"--home", home, "--project-root", project, "-y", "sync", "--at", ".Ag", "--skill", "one"}, strings.NewReader(""), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	active := filepath.Join(cfg.MustActiveRoot(config.ScopeProject, config.TargetAgents), "one")
	if info, err := os.Lstat(active); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("active skill is not a link: info=%v err=%v", info, err)
	}
	if !strings.Contains(out.String(), "synced: 1 skill") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestSyncInteractiveConflictPromptsForPreserveNameAndConfirmation(t *testing.T) {
	home, project := t.TempDir(), t.TempDir()
	cfg := config.Default(project, home)
	makeSkill(t, cfg.MustActiveRoot(config.ScopeProject, config.TargetClaude), "one", "Source.")
	destination := makeSkill(t, cfg.MustActiveRoot(config.ScopeProject, config.TargetAgents), "one", "Destination.")

	previous := syncInputIsTerminal
	syncInputIsTerminal = func(io.Reader) bool { return true }
	t.Cleanup(func() { syncInputIsTerminal = previous })
	previousChecklist := runSyncChecklist
	runSyncChecklist = func(_ io.Reader, _ io.Writer, _ []syncChecklistOption, defaults []string) ([]string, error) {
		return defaults, nil
	}
	t.Cleanup(func() { runSyncChecklist = previousChecklist })

	var out bytes.Buffer
	err := Execute([]string{"--home", home, "--project-root", project, "sync", "--at", ".Ag"}, strings.NewReader("one-local\ny\n"), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{destination, "Preserve", "one-local", "Apply sync plan?"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output missing %q:\n%s", want, out.String())
		}
	}
}
