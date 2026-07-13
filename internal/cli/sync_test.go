package cli

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/InkyQuill/x-skills/internal/actions"
	"github.com/InkyQuill/x-skills/internal/compatibility"
	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/pathidentity"
	"github.com/InkyQuill/x-skills/internal/roots"
	"github.com/InkyQuill/x-skills/internal/syncer"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
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

func TestSyncDivergentChecklistDefaultsReflectVariantCompatibility(t *testing.T) {
	groups := []syncer.NameGroup{
		{Name: "blocked", Variants: []syncer.Candidate{
			{ID: "blocked:a", Name: "blocked", Compatibility: compatibility.Assessment{State: compatibility.StateIncompatible}},
			{ID: "blocked:b", Name: "blocked", Compatibility: compatibility.Assessment{State: compatibility.StateIncompatible}},
		}},
		{Name: "mixed", Variants: []syncer.Candidate{
			{ID: "mixed:a", Name: "mixed", Compatibility: compatibility.Assessment{State: compatibility.StateIncompatible}},
			{ID: "mixed:b", Name: "mixed", Compatibility: compatibility.Assessment{State: compatibility.StatePartial}},
		}},
	}
	options, defaults := syncChecklistOptions(groups)
	if len(defaults) != 1 || defaults[0] != "mixed" {
		t.Fatalf("defaults = %#v", defaults)
	}
	if !strings.Contains(options[0].Label, "all incompatible") || !strings.Contains(options[1].Label, "mixed") {
		t.Fatalf("options = %#v", options)
	}
}

func TestSyncExplicitIncompatibleSkillWarnsAndYesConfirmsNonTTY(t *testing.T) {
	home, project := t.TempDir(), t.TempDir()
	cfg := config.Default(project, home)
	writeSyncIncompatibleSkill(t, cfg.MustActiveRoot(config.ScopeProject, config.TargetClaude), "claude-only")

	var nonTTYOut bytes.Buffer
	err := Execute([]string{"--home", home, "--project-root", project, "-y", "sync", "--at", ".Ag", "--skill", "claude-only"}, strings.NewReader(""), &nonTTYOut, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("non-TTY incompatible sync: %v", err)
	}
	if !strings.Contains(nonTTYOut.String(), "incompatible") || !strings.Contains(nonTTYOut.String(), "$CLAUDE_PROJECT_DIR") {
		t.Fatalf("non-TTY warning output = %q", nonTTYOut.String())
	}
	active := filepath.Join(cfg.MustActiveRoot(config.ScopeProject, config.TargetAgents), "claude-only")
	if info, err := os.Lstat(active); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("confirmed incompatible skill is not linked: info=%v err=%v", info, err)
	}

	home, project = t.TempDir(), t.TempDir()
	cfg = config.Default(project, home)
	writeSyncIncompatibleSkill(t, cfg.MustActiveRoot(config.ScopeProject, config.TargetClaude), "claude-only")
	err = Execute([]string{"--home", home, "--project-root", project, "sync", "--at", ".Ag", "--skill", "claude-only"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "requires confirmation; rerun with -y") {
		t.Fatalf("unconfirmed incompatible error = %v", err)
	}

	previousTTY := syncInputIsTerminal
	syncInputIsTerminal = func(io.Reader) bool { return true }
	t.Cleanup(func() { syncInputIsTerminal = previousTTY })
	previousPrompt := runSyncCompatibilityPrompt
	runSyncCompatibilityPrompt = func(io.Reader, io.Writer, syncer.Candidate) (bool, error) { return false, nil }
	t.Cleanup(func() { runSyncCompatibilityPrompt = previousPrompt })
	var out bytes.Buffer
	err = Execute([]string{"--home", home, "--project-root", project, "sync", "--at", ".Ag", "--skill", "claude-only"}, strings.NewReader(""), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "incompatible") || !strings.Contains(out.String(), "$CLAUDE_PROJECT_DIR") {
		t.Fatalf("warning output = %q", out.String())
	}
	if _, err := os.Stat(filepath.Join(cfg.ArchiveSkillsRoot(), "claude-only")); !os.IsNotExist(err) {
		t.Fatalf("declined incompatible skill was archived: %v", err)
	}
}

func writeSyncIncompatibleSkill(t *testing.T, root, name string) {
	t.Helper()
	dir := makeSkill(t, root, name, "Claude-only workflow.")
	data := []byte("---\nname: " + name + "\ndescription: Claude-only workflow.\n---\n\nRead the project from $CLAUDE_PROJECT_DIR.\n")
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), data, 0o644); err != nil {
		t.Fatal(err)
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

func TestSyncReconcilesExistingDeclaredNameMismatchByIdentity(t *testing.T) {
	home, project := t.TempDir(), t.TempDir()
	cfg := setupActiveIdentityMismatch(t, home, project)
	makeSkill(t, cfg.MustActiveRoot(config.ScopeProject, config.TargetClaude), "other", "Other.")

	err := Execute(
		[]string{
			"--home", home,
			"--project-root", project,
			"-y", "sync",
			"--at", "project:codex",
			"--skill", "other",
		},
		strings.NewReader(""),
		&bytes.Buffer{},
		&bytes.Buffer{},
	)
	if err != nil {
		t.Fatal(err)
	}
	assertLocalManifestHasIdentity(t, cfg, "composition-patterns")
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
	err := Execute([]string{"--home", home, "--project-root", project, "sync", "--at", ".Ag"}, strings.NewReader("1\none-local\ny\n"), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	assertSyncOutputConflictPathEquivalent(t, out.String(), destination)
	for _, want := range []string{"Preserve", "one-local", "Apply sync plan?"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output missing %q:\n%s", want, out.String())
		}
	}
}

// assertSyncOutputConflictPathEquivalent accepts platform-specific canonical spellings.
func assertSyncOutputConflictPathEquivalent(t *testing.T, output, want string) {
	t.Helper()
	for _, line := range strings.Split(output, "\n") {
		path, ok := strings.CutPrefix(line, "Conflict at ")
		if !ok {
			continue
		}
		path = strings.TrimSuffix(path, ":")
		equivalent, err := pathidentity.EquivalentE(path, want)
		if err != nil {
			t.Fatalf("compare conflict path %q with %q: %v\n%s", path, want, err, output)
		}
		if !equivalent {
			t.Fatalf("conflict path = %q, want same location as %q\n%s", path, want, output)
		}
		return
	}
	t.Fatalf("output missing conflict path for %q:\n%s", want, output)
}

func TestSyncConflictChoicesReplaceKeepAndCancel(t *testing.T) {
	tests := []struct {
		name       string
		action     string
		preserveAs string
		wantOutput string
		wantLink   bool
	}{
		{name: "replace", action: syncer.ConflictReplace, preserveAs: "one-local", wantOutput: "one-local", wantLink: true},
		{name: "keep", action: syncer.ConflictKeep, wantOutput: "skipped: 1"},
		{name: "cancel", action: syncer.ConflictCancel, wantOutput: "sync cancelled"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home, project := t.TempDir(), t.TempDir()
			cfg := config.Default(project, home)
			makeSkill(t, cfg.MustActiveRoot(config.ScopeProject, config.TargetClaude), "one", "Source.")
			destination := makeSkill(t, cfg.MustActiveRoot(config.ScopeProject, config.TargetAgents), "one", "Destination.")

			previousTTY := syncInputIsTerminal
			syncInputIsTerminal = func(io.Reader) bool { return true }
			t.Cleanup(func() { syncInputIsTerminal = previousTTY })
			previousConflict := runSyncConflictPrompt
			runSyncConflictPrompt = func(io.Reader, io.Writer, syncer.Conflict) (string, string, error) {
				return tt.action, tt.preserveAs, nil
			}
			t.Cleanup(func() { runSyncConflictPrompt = previousConflict })

			var out bytes.Buffer
			err := Execute([]string{"--home", home, "--project-root", project, "-y", "sync", "--at", ".Ag", "--skill", "one"}, strings.NewReader(""), &out, &bytes.Buffer{})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out.String(), tt.wantOutput) {
				t.Fatalf("output missing %q: %s", tt.wantOutput, out.String())
			}
			info, err := os.Lstat(destination)
			if tt.wantLink && (err != nil || info.Mode()&os.ModeSymlink == 0) {
				t.Fatalf("replace destination not linked: info=%v err=%v", info, err)
			}
			if !tt.wantLink && (err != nil || info.Mode()&os.ModeSymlink != 0) {
				t.Fatalf("kept/cancelled destination changed: info=%v err=%v", info, err)
			}
		})
	}
}

func TestSyncConflictEOFAndPromptErrorsCancelWithoutReplacement(t *testing.T) {
	conflict := syncer.Conflict{DestinationPath: "/tmp/one", SuggestedPreserveAs: "one-local"}
	action, name, err := defaultSyncConflictPrompt(strings.NewReader(""), io.Discard, conflict)
	if err != nil || action != syncer.ConflictCancel || name != "" {
		t.Fatalf("EOF result = action %q name %q err %v", action, name, err)
	}
	wantErr := errors.New("form cancelled")
	previous := runSyncConflictPrompt
	runSyncConflictPrompt = func(io.Reader, io.Writer, syncer.Conflict) (string, string, error) { return "", "", wantErr }
	t.Cleanup(func() { runSyncConflictPrompt = previous })
	_, err = promptSyncConflicts(&cobra.Command{}, []syncer.Conflict{conflict})
	if !errors.Is(err, wantErr) {
		t.Fatalf("prompt error = %v", err)
	}
}

func TestSyncChecklistAndVariantCancellationAreClean(t *testing.T) {
	home, project := t.TempDir(), t.TempDir()
	cfg := config.Default(project, home)
	makeSkill(t, cfg.MustActiveRoot(config.ScopeProject, config.TargetClaude), "one", "One.")
	previousTTY := syncInputIsTerminal
	syncInputIsTerminal = func(io.Reader) bool { return true }
	t.Cleanup(func() { syncInputIsTerminal = previousTTY })
	previousChecklist := runSyncChecklist
	runSyncChecklist = func(io.Reader, io.Writer, []syncChecklistOption, []string) ([]string, error) {
		return nil, huh.ErrUserAborted
	}
	t.Cleanup(func() { runSyncChecklist = previousChecklist })
	var out bytes.Buffer
	if err := Execute([]string{"--home", home, "--project-root", project, "sync", "--at", ".Ag"}, strings.NewReader(""), &out, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "sync cancelled") {
		t.Fatalf("checklist cancellation output = %q", out.String())
	}
	if _, err := os.Stat(filepath.Join(cfg.ArchiveSkillsRoot(), "one")); !os.IsNotExist(err) {
		t.Fatalf("cancelled checklist mutated archive: %v", err)
	}

	group := syncer.NameGroup{Name: "one", Variants: []syncer.Candidate{{ID: "one:a"}, {ID: "one:b"}}}
	cmd := &cobra.Command{}
	cmd.SetIn(strings.NewReader(""))
	cmd.SetOut(io.Discard)
	_, err := promptSyncVariant(cmd, group)
	if !errors.Is(err, errSyncCancelled) {
		t.Fatalf("variant EOF error = %v", err)
	}
}
