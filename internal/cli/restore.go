package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/manifest"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var restoreInputIsTerminal = func(input io.Reader) bool {
	file, ok := input.(interface{ Fd() uintptr })
	return ok && term.IsTerminal(int(file.Fd()))
}

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
			plan, err := manifest.PlanRestore(cmd.Context(), cfg, manifest.RestoreRequest{
				Destinations: destinations,
				Full:         full,
			})
			if err != nil {
				return err
			}
			defer plan.Close()
			printRestorePlan(cmd, plan)
			if err := resolveRestoreConflicts(cmd, opts, &plan); err != nil {
				return err
			}
			if len(plan.Conflicts) > 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "final restore plan")
				printRestorePlan(cmd, plan)
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
			migrations, removals := restoreResultChangeCounts(result)
			_, err = fmt.Fprintf(
				cmd.OutOrStdout(),
				"restored: %d links, %d migrations, %d removals\n",
				len(result.Additions),
				migrations,
				removals,
			)
			return err
		},
	}
	cmd.Flags().StringSliceVar(&selectors, "at", nil, "explicit project Skills Folder destination (repeatable)")
	cmd.Flags().BoolVar(&full, "full", false, "exactly reconcile selected project Skills Folders")
	return cmd
}

func restoreResultChangeCounts(result manifest.RestoreResult) (migrations, removals int) {
	changes := append(append([]manifest.Change{}, result.Normalizations...), result.Removals...)
	for _, change := range changes {
		if change.Kind == manifest.ChangeMigrate {
			migrations++
			continue
		}
		if change.Kind == manifest.ChangeRemove {
			removals++
		}
	}
	return migrations, removals
}

func printRestorePlan(cmd *cobra.Command, plan manifest.RestorePlan) {
	printRestoreGroup(cmd, "available", restoreAvailableNames(plan.Available))
	unavailable := make([]string, 0, len(plan.Unavailable))
	for _, skill := range plan.Unavailable {
		unavailable = append(unavailable, skill.Name+"  "+skill.Reason)
	}
	printRestoreGroup(cmd, "unavailable", unavailable)
	printRestoreGroup(cmd, "links", restoreChangeLines(plan.Additions))
	migrations := restoreChangeLinesByKind(plan.Normalizations, manifest.ChangeMigrate)
	migrations = append(migrations, restoreChangeLinesByKind(plan.Removals, manifest.ChangeMigrate)...)
	printRestoreGroup(cmd, "migrations", migrations)
	removals := restoreChangeLinesWithoutKind(plan.Normalizations, manifest.ChangeMigrate)
	removals = append(removals, restoreChangeLinesWithoutKind(plan.Removals, manifest.ChangeMigrate)...)
	printRestoreGroup(cmd, "removals", removals)
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
		lines = append(lines, restoreChangeLine(change))
	}
	return lines
}

func restoreChangeLinesByKind(changes []manifest.Change, kind string) []string {
	lines := []string{}
	for _, change := range changes {
		if change.Kind == kind {
			lines = append(lines, restoreChangeLine(change))
		}
	}
	return lines
}

func restoreChangeLinesWithoutKind(changes []manifest.Change, kind string) []string {
	lines := []string{}
	for _, change := range changes {
		if change.Kind != kind {
			lines = append(lines, restoreChangeLine(change))
		}
	}
	return lines
}

func resolveRestoreConflicts(cmd *cobra.Command, opts *options, plan *manifest.RestorePlan) error {
	if len(plan.Conflicts) > 0 && (opts.noInput || !restoreInputIsTerminal(cmd.InOrStdin())) {
		return fmt.Errorf("restore conflict resolution requires an interactive terminal")
	}
	for _, conflict := range plan.Conflicts {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Archive name for preserving %s [%s]: ", conflict.Path, conflict.SuggestedName)
		name, err := readPromptLine(cmd.InOrStdin())
		if err != nil {
			return err
		}
		name = strings.TrimSpace(name)
		if name == "" {
			name = conflict.SuggestedName
		}
		originalName := conflict.Name
		for _, changes := range [][]manifest.Change{plan.Normalizations, plan.Removals} {
			for _, change := range changes {
				if change.Path == conflict.Path && change.Kind == manifest.ChangeMigrate && name == change.Name {
					originalName = change.Name
				}
			}
		}
		if name == conflict.Name || name == originalName {
			return fmt.Errorf("preserve name must differ from %q", originalName)
		}
		setRestoreArchiveName(plan, conflict.Path, name)
	}
	return nil
}

func restoreChangeLine(change manifest.Change) string {
	line := change.Name + "  " + change.Destination.Label + "  " + change.Path
	if change.Kind == manifest.ChangeMigrate {
		archiveName := change.ArchiveName
		if archiveName == "" {
			archiveName = "(rename required)"
		}
		line += " -> archive:" + archiveName
	}
	return line
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
