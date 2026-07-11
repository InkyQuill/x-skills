package cli

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/InkyQuill/x-skills/internal/compatibility"
	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/syncer"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

type syncChecklistOption struct {
	Label string
	Value string
}

var errSyncCancelled = errors.New("sync cancelled")

var syncInputIsTerminal = func(input io.Reader) bool {
	file, ok := input.(interface{ Fd() uintptr })
	return ok && term.IsTerminal(int(file.Fd()))
}

var runSyncChecklist = func(input io.Reader, output io.Writer, options []syncChecklistOption, defaults []string) ([]string, error) {
	selected := slices.Clone(defaults)
	defaultSet := make(map[string]bool, len(defaults))
	for _, value := range defaults {
		defaultSet[value] = true
	}
	huhOptions := make([]huh.Option[string], 0, len(options))
	for _, option := range options {
		huhOptions = append(huhOptions, huh.NewOption(option.Label, option.Value).Selected(defaultSet[option.Value]))
	}
	form := huh.NewForm(huh.NewGroup(
		huh.NewMultiSelect[string]().Title("Skills to sync").Options(huhOptions...).Value(&selected),
	)).WithInput(input).WithOutput(output)
	if err := form.Run(); err != nil {
		return nil, err
	}
	return selected, nil
}

var runSyncCompatibilityPrompt = defaultSyncCompatibilityPrompt

var runSyncConflictPrompt = defaultSyncConflictPrompt

func newSyncCommand(opts *options) *cobra.Command {
	var selectors []string
	var all bool
	var names []string
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Synchronize aggregate project skills into Skills Folders",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if all && len(names) > 0 {
				return fmt.Errorf("--all and --skill are mutually exclusive")
			}
			cfg := opts.config()
			destinations, err := resolveLocations(cfg, selectors)
			if err != nil {
				return err
			}
			for _, destination := range destinations {
				if destination.Scope != config.ScopeProject {
					return fmt.Errorf("sync destination %q is not a project Skills Folder", destination.Path)
				}
			}
			interactive := !opts.noInput && syncInputIsTerminal(cmd.InOrStdin())
			if !interactive && !all && len(names) == 0 {
				return fmt.Errorf("non-interactive sync requires --all or --skill")
			}
			groups, err := syncer.DiscoverContext(cmd.Context(), cfg, destinations)
			if err != nil {
				return err
			}
			selection, err := chooseSyncSelection(cmd, groups, all, names)
			if err != nil {
				if errors.Is(err, errSyncCancelled) || errors.Is(err, huh.ErrUserAborted) {
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), "sync cancelled")
					return nil
				}
				return err
			}
			acknowledged, err := acknowledgeIncompatibleSelection(cmd, groups, selection, interactive)
			if err != nil {
				return err
			}
			if !acknowledged {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "sync cancelled")
				return nil
			}
			plan, err := syncer.PreflightContext(cmd.Context(), cfg, groups, destinations, selection, nil)
			if err != nil {
				return err
			}
			if len(plan.Conflicts) > 0 {
				if !interactive {
					return fmt.Errorf("sync conflict resolution requires an interactive terminal")
				}
				resolutions, err := promptSyncConflicts(cmd, plan.Conflicts)
				if err != nil {
					return err
				}
				plan, err = syncer.PreflightContext(cmd.Context(), cfg, groups, destinations, selection, resolutions)
				if err != nil {
					return err
				}
			}
			if plan.Cancelled {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "sync cancelled")
				return nil
			}
			printSyncPlan(cmd, plan)
			if !interactive && !opts.yes && !opts.no {
				return fmt.Errorf("sync requires confirmation; rerun with -y")
			}
			confirmed, err := confirm(cmd, opts, "Apply sync plan? [y/N] ", "sync requires confirmation; rerun with -y")
			if err != nil {
				return err
			}
			if !confirmed {
				return nil
			}
			result := syncer.Apply(cmd.Context(), cfg, plan)
			failures := []error{result.PlanError, result.ManifestError}
			for _, failed := range result.Failed {
				failures = append(failures, fmt.Errorf("sync %s: %w", failed.Name, failed.Err))
			}
			if result.Cancelled {
				failures = append(failures, fmt.Errorf("sync cancelled after %d completed skill(s)", len(result.Succeeded)))
			}
			if err := errors.Join(failures...); err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "synced: %d skill(s)\n", len(result.Succeeded))
			return err
		},
	}
	cmd.Flags().StringSliceVar(&selectors, "at", nil, "explicit project Skills Folder destination (repeatable)")
	cmd.Flags().BoolVar(&all, "all", false, "select every unique non-incompatible skill")
	cmd.Flags().StringArrayVar(&names, "skill", nil, "select an exact skill name (repeatable)")
	return cmd
}

func chooseSyncSelection(cmd *cobra.Command, groups []syncer.NameGroup, all bool, names []string) (syncer.Selection, error) {
	if all || len(names) > 0 {
		return selectSyncCandidates(groups, all, names)
	}
	options, defaults := syncChecklistOptions(groups)
	selected, err := runSyncChecklist(cmd.InOrStdin(), cmd.OutOrStdout(), options, defaults)
	if err != nil {
		return syncer.Selection{}, err
	}
	selectedSet := make(map[string]bool, len(selected))
	for _, value := range selected {
		selectedSet[value] = true
	}
	selection := syncer.Selection{VariantByName: make(map[string]string)}
	for _, group := range groups {
		value := groupSelectionValue(group)
		if !selectedSet[value] {
			continue
		}
		if len(group.Variants) == 1 {
			selection.CandidateIDs = append(selection.CandidateIDs, group.Variants[0].ID)
			continue
		}
		candidate, err := promptSyncVariant(cmd, group)
		if err != nil {
			return syncer.Selection{}, err
		}
		selection.VariantByName[group.Name] = candidate.ID
	}
	return selection, nil
}

func selectSyncCandidates(groups []syncer.NameGroup, all bool, names []string) (syncer.Selection, error) {
	selection := syncer.Selection{}
	requested := make(map[string]bool, len(names))
	for _, name := range names {
		requested[name] = true
	}
	for _, group := range groups {
		selected := all || requested[group.Name]
		if !selected {
			continue
		}
		delete(requested, group.Name)
		if len(group.Variants) != 1 {
			if all {
				continue
			}
			return syncer.Selection{}, fmt.Errorf("skill %q has multiple variants in Skills Folders: %s", group.Name, strings.Join(syncVariantSources(group), ", "))
		}
		candidate := group.Variants[0]
		if all && candidate.Compatibility.State == compatibility.StateIncompatible {
			continue
		}
		selection.CandidateIDs = append(selection.CandidateIDs, candidate.ID)
	}
	if len(requested) > 0 {
		missing := make([]string, 0, len(requested))
		for name := range requested {
			missing = append(missing, name)
		}
		slices.Sort(missing)
		return syncer.Selection{}, fmt.Errorf("skill not found: %q", missing[0])
	}
	return selection, nil
}

func syncChecklistOptions(groups []syncer.NameGroup) ([]syncChecklistOption, []string) {
	options := make([]syncChecklistOption, 0, len(groups))
	var defaults []string
	for _, group := range groups {
		value := groupSelectionValue(group)
		state, eligible := syncGroupCompatibility(group)
		label := fmt.Sprintf("%s  [%s]  %s", group.Name, strings.Join(syncVariantSources(group), ", "), state)
		options = append(options, syncChecklistOption{Label: label, Value: value})
		if eligible {
			defaults = append(defaults, value)
		}
	}
	return options, defaults
}

func syncGroupCompatibility(group syncer.NameGroup) (string, bool) {
	if len(group.Variants) == 1 {
		state := group.Variants[0].Compatibility.State
		return string(state), state != compatibility.StateIncompatible
	}
	states := make(map[compatibility.State]struct{})
	eligible := false
	for _, candidate := range group.Variants {
		state := candidate.Compatibility.State
		states[state] = struct{}{}
		if state != compatibility.StateIncompatible {
			eligible = true
		}
	}
	if !eligible {
		return "all incompatible", false
	}
	if len(states) > 1 {
		return "mixed compatibility", true
	}
	for state := range states {
		return "all " + string(state), true
	}
	return string(compatibility.StateUnknown), true
}

func groupSelectionValue(group syncer.NameGroup) string {
	if len(group.Variants) == 1 {
		return group.Variants[0].ID
	}
	return group.Name
}

func syncVariantSources(group syncer.NameGroup) []string {
	set := make(map[string]struct{})
	for _, candidate := range group.Variants {
		for _, occurrence := range candidate.Occurrences {
			label := occurrence.Root.Label
			if label == "" {
				label = occurrence.Root.Target
			}
			set[label] = struct{}{}
		}
	}
	labels := make([]string, 0, len(set))
	for label := range set {
		labels = append(labels, label)
	}
	slices.Sort(labels)
	return labels
}

func promptSyncVariant(cmd *cobra.Command, group syncer.NameGroup) (syncer.Candidate, error) {
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Choose source variant for %s:\n", group.Name)
	for index, candidate := range group.Variants {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %d. %s  %s\n", index+1, strings.Join(candidateSourceLabels(candidate), ", "), candidate.Compatibility.State)
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Select variant [1-%d]: ", len(group.Variants))
	answer, err := readPromptLine(cmd.InOrStdin())
	if err != nil {
		return syncer.Candidate{}, err
	}
	if strings.TrimSpace(answer) == "" {
		return syncer.Candidate{}, errSyncCancelled
	}
	index, err := readSelection(strings.NewReader(answer), len(group.Variants))
	if err != nil {
		return syncer.Candidate{}, err
	}
	return group.Variants[index], nil
}

func candidateSourceLabels(candidate syncer.Candidate) []string {
	return syncVariantSources(syncer.NameGroup{Variants: []syncer.Candidate{candidate}})
}

func acknowledgeIncompatibleSelection(cmd *cobra.Command, groups []syncer.NameGroup, selection syncer.Selection, interactive bool) (bool, error) {
	selected := make(map[string]bool, len(selection.CandidateIDs)+len(selection.VariantByName))
	for _, id := range selection.CandidateIDs {
		selected[id] = true
	}
	for _, id := range selection.VariantByName {
		selected[id] = true
	}
	for _, group := range groups {
		for _, candidate := range group.Variants {
			if !selected[candidate.ID] || candidate.Compatibility.State != compatibility.StateIncompatible {
				continue
			}
			printCompatibilityWarning(cmd.OutOrStdout(), candidate)
			if !interactive {
				continue
			}
			confirmed, err := runSyncCompatibilityPrompt(cmd.InOrStdin(), cmd.OutOrStdout(), candidate)
			if err != nil {
				return false, err
			}
			if !confirmed {
				return false, nil
			}
		}
	}
	return true, nil
}

func printCompatibilityWarning(output io.Writer, candidate syncer.Candidate) {
	_, _ = fmt.Fprintf(output, "warning: %s is incompatible with the selected destinations\n", candidate.Name)
	for _, reason := range candidate.Compatibility.Reasons {
		_, _ = fmt.Fprintf(output, "  %s\n", reason)
	}
}

func defaultSyncCompatibilityPrompt(input io.Reader, output io.Writer, candidate syncer.Candidate) (bool, error) {
	_, _ = fmt.Fprintf(output, "Sync incompatible skill %s anyway? [y/N] ", candidate.Name)
	answer, err := readPromptLine(input)
	if err != nil {
		return false, err
	}
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes", nil
}

func promptSyncConflicts(cmd *cobra.Command, conflicts []syncer.Conflict) ([]syncer.ConflictResolution, error) {
	resolutions := make([]syncer.ConflictResolution, 0, len(conflicts))
	for _, conflict := range conflicts {
		action, name, err := runSyncConflictPrompt(cmd.InOrStdin(), cmd.OutOrStdout(), conflict)
		if err != nil {
			return nil, err
		}
		resolutions = append(resolutions, syncer.ConflictResolution{DestinationPath: conflict.DestinationPath, PreserveAs: name, Action: action})
	}
	return resolutions, nil
}

func defaultSyncConflictPrompt(input io.Reader, output io.Writer, conflict syncer.Conflict) (string, string, error) {
	_, _ = fmt.Fprintf(output, "Conflict at %s:\n", conflict.DestinationPath)
	_, _ = fmt.Fprintln(output, "  1. replace destination and preserve its content")
	_, _ = fmt.Fprintln(output, "  2. keep destination and skip this link")
	_, _ = fmt.Fprintln(output, "  3. cancel sync")
	_, _ = fmt.Fprint(output, "Choose conflict action [1-3]: ")
	answer, err := readPromptLine(input)
	if err != nil {
		return "", "", err
	}
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "1", "r", "replace":
		_, _ = fmt.Fprintf(output, "Preserve conflicting skill as [%s]: ", conflict.SuggestedPreserveAs)
		name, err := readPromptLine(input)
		if err != nil {
			return "", "", err
		}
		name = strings.TrimSpace(name)
		if name == "" {
			name = conflict.SuggestedPreserveAs
		}
		return syncer.ConflictReplace, name, nil
	case "2", "k", "keep":
		return syncer.ConflictKeep, "", nil
	case "", "3", "c", "cancel":
		return syncer.ConflictCancel, "", nil
	default:
		return "", "", fmt.Errorf("invalid conflict action %q", answer)
	}
}

func printSyncPlan(cmd *cobra.Command, plan syncer.Plan) {
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "sync plan")
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  migrations: %d\n", len(plan.Migrations))
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  links: %d\n", len(plan.Links))
	for _, conflict := range plan.Conflicts {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  preserve: %s as %s\n", conflict.DestinationPath, conflict.Resolution.PreserveAs)
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  skipped: %d\n", len(plan.Skipped))
}
