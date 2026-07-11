package cli

import (
	"fmt"
	"strings"

	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/manifest"
	"github.com/spf13/cobra"
)

func newRestoreCommand(opts *options) *cobra.Command {
	var selectors []string
	var full bool
	cmd := &cobra.Command{
		Use:   "restore",
		Short: "Restore project skills into explicit Skills Folders",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := opts.config()
			destinations, err := resolveLocations(cfg, selectors)
			if err != nil {
				return err
			}
			for _, destination := range destinations {
				if destination.Scope != config.ScopeProject {
					return fmt.Errorf("restore destination %q is not a project Skills Folder", destination.Path)
				}
			}
			plan, err := manifest.PlanRestore(cmd.Context(), cfg, manifest.RestoreRequest{Destinations: destinations, Full: full})
			if err != nil {
				return err
			}
			defer plan.Close()
			printRestorePlan(cmd, plan)
			if err := resolveRestoreConflicts(cmd, opts, &plan); err != nil {
				return err
			}
			confirmed, err := confirm(cmd, opts, "Apply restore plan? [y/N] ", "restore requires confirmation; rerun with -y")
			if err != nil {
				return err
			}
			if !confirmed {
				return nil
			}
			result, err := manifest.ApplyRestore(cmd.Context(), cfg, plan)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "restored: %d links, %d migrations, %d removals\n", len(result.Additions), len(result.Normalizations), len(result.Removals))
			return err
		},
	}
	cmd.Flags().StringSliceVar(&selectors, "at", nil, "explicit project Skills Folder destination (repeatable)")
	cmd.Flags().BoolVar(&full, "full", false, "exactly reconcile selected project Skills Folders")
	return cmd
}

func printRestorePlan(cmd *cobra.Command, plan manifest.RestorePlan) {
	printRestoreGroup(cmd, "available", restoreAvailableNames(plan.Available))
	unavailable := make([]string, 0, len(plan.Unavailable))
	for _, skill := range plan.Unavailable {
		unavailable = append(unavailable, skill.Name+"  "+skill.Reason)
	}
	printRestoreGroup(cmd, "unavailable", unavailable)
	printRestoreGroup(cmd, "links", restoreChangeLines(plan.Additions))
	migrations := append(restoreChangeLines(plan.Normalizations), restoreChangeLinesByKind(plan.Removals, manifest.ChangeMigrate)...)
	printRestoreGroup(cmd, "migrations", migrations)
	printRestoreGroup(cmd, "removals", restoreChangeLinesWithoutKind(plan.Removals, manifest.ChangeMigrate))
	if plan.RemovalsBlocked {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "warning: unavailable skills block destructive migrations and removals")
	}
}

func printRestoreGroup(cmd *cobra.Command, title string, lines []string) {
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), title)
	if len(lines) == 0 {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "  (none)")
		return
	}
	for _, line := range lines {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "  "+line)
	}
}

func restoreAvailableNames(skills []manifest.PlannedSkill) []string {
	lines := make([]string, 0, len(skills))
	for _, skill := range skills {
		lines = append(lines, skill.Name)
	}
	return lines
}

func restoreChangeLines(changes []manifest.Change) []string {
	lines := make([]string, 0, len(changes))
	for _, change := range changes {
		lines = append(lines, change.Name+"  "+change.Destination.Label)
	}
	return lines
}

func restoreChangeLinesByKind(changes []manifest.Change, kind string) []string {
	lines := []string{}
	for _, change := range changes {
		if change.Kind == kind {
			lines = append(lines, change.Name+"  "+change.Destination.Label)
		}
	}
	return lines
}

func restoreChangeLinesWithoutKind(changes []manifest.Change, kind string) []string {
	lines := []string{}
	for _, change := range changes {
		if change.Kind != kind {
			lines = append(lines, change.Name+"  "+change.Destination.Label)
		}
	}
	return lines
}

func resolveRestoreConflicts(cmd *cobra.Command, opts *options, plan *manifest.RestorePlan) error {
	for _, conflict := range plan.Conflicts {
		if opts.noInput {
			return fmt.Errorf("migration conflict for %q requires an archive name; rerun interactively", conflict.Name)
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Archive name for preserving %s [%s]: ", conflict.Path, conflict.SuggestedName)
		name, err := readPromptLine(cmd.InOrStdin())
		if err != nil {
			return err
		}
		name = strings.TrimSpace(name)
		if name == "" {
			name = conflict.SuggestedName
		}
		setRestoreArchiveName(plan, conflict.Path, name)
	}
	return nil
}

func setRestoreArchiveName(plan *manifest.RestorePlan, path, name string) {
	sets := [][]manifest.Change{plan.Normalizations, plan.Removals}
	for _, changes := range sets {
		for i := range changes {
			if changes[i].Path == path && changes[i].Kind == manifest.ChangeMigrate {
				changes[i].ArchiveName = name
			}
		}
	}
}
