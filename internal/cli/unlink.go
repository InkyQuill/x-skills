package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/InkyQuill/x-skills/internal/actions"
	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/manifest"
	"github.com/spf13/cobra"
)

type unlinkOptions struct {
	activeRootOptions
	deleteUnmanaged bool
}

func newUnlinkCommand(rootOptions *options) *cobra.Command {
	var opts unlinkOptions
	cmd := &cobra.Command{
		Use:   "unlink NAME [NAME...]",
		Short: "Remove active skills from a target root",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.validate(); err != nil {
				return err
			}

			results, failures, skipped := unlinkNamesWithOptions(cmd, rootOptions, args, opts)
			if len(results) == 0 && len(failures) == 0 && len(skipped) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "cancelled")
				return nil
			}
			if len(args) == 1 && len(failures) == 0 && len(skipped) == 0 {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", results[0].Status, results[0].Name)
				return nil
			}
			if len(args) == 1 && len(skipped) == 1 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "cancelled")
				return nil
			}
			if len(args) == 1 && len(failures) == 1 {
				return failures[0].err
			}

			writeUnlinkSummary(cmd.OutOrStdout(), results, failures, skipped)
			if len(failures) > 0 {
				return fmt.Errorf("unlink failed for %d skill(s)", len(failures))
			}
			return nil
		},
	}
	addActiveRootFlags(cmd, &opts.activeRootOptions)
	cmd.Flags().BoolVar(&opts.deleteUnmanaged, "delete-unmanaged", false, "delete unmanaged active directories instead of archiving them")
	return cmd
}

func (o unlinkOptions) validate() error {
	return o.validateFilter()
}

func unlinkNamesWithOptions(
	cmd *cobra.Command,
	rootOptions *options,
	names []string,
	opts unlinkOptions,
) ([]actions.MutationResult, []mutationFailure, []mutationSkipped) {
	cfg := rootOptions.config()
	location, locationErr := optionalOneLocation(cfg, opts.at)
	var results []actions.MutationResult
	var failures []mutationFailure
	var skipped []mutationSkipped
	projectMutated := false
	for _, name := range names {
		if locationErr != nil {
			failures = append(failures, mutationFailure{name: name, err: locationErr})
			continue
		}
		skill, err := chooseActiveSkill(cmd, rootOptions, cfg, name, "unlink", location)
		if err != nil {
			failures = append(failures, mutationFailure{name: name, err: err})
			continue
		}
		if rootOptions.no {
			skipped = append(skipped, mutationSkipped{name: name, reason: "cancelled"})
			continue
		}

		confirmed := false
		deleteUnmanaged := opts.deleteUnmanaged
		if skill.Status == actions.StatusUnmanaged && !opts.deleteUnmanaged {
			confirmed, deleteUnmanaged, err = chooseUnmanagedUnlinkAction(cmd, rootOptions, skill)
			if err != nil {
				failures = append(failures, mutationFailure{name: name, err: err})
				continue
			}
		} else {
			prompt := fmt.Sprintf("Unlink %s %s skill %q? [y/N]: ", skill.Root.Scope, skill.Root.Target, name)
			noInputErr := "unlink requires confirmation; rerun with -y"
			if skill.Status == actions.StatusUnmanaged && opts.deleteUnmanaged {
				prompt = fmt.Sprintf("Remove unmanaged %s %s skill %q without archiving? [y/N]: ", skill.Root.Scope, skill.Root.Target, name)
				noInputErr = "unmanaged delete requires confirmation; rerun with --delete-unmanaged -y"
			}
			confirmed, err = confirm(
				cmd,
				rootOptions,
				prompt,
				noInputErr,
			)
			if err != nil {
				failures = append(failures, mutationFailure{name: name, err: err})
				continue
			}
		}
		if !confirmed {
			skipped = append(skipped, mutationSkipped{name: name, reason: "cancelled"})
			continue
		}
		result, err := actions.Unlink(cfg, actions.UnlinkRequest{
			Name:            skill.Identity,
			Scope:           skill.Root.Scope,
			Target:          skill.Root.Target,
			Confirmed:       true,
			DeleteUnmanaged: deleteUnmanaged,
		})
		if err != nil {
			failures = append(failures, mutationFailure{name: name, err: err})
			continue
		}
		results = append(results, result)
		if skill.Root.Scope == config.ScopeProject {
			projectMutated = true
		}
	}
	if projectMutated {
		if _, err := manifest.ReconcileLocal(cfg); err != nil {
			failures = append(failures, mutationFailure{name: "local manifest", err: fmt.Errorf("skill mutation succeeded but local manifest reconciliation failed: %w", err)})
		}
	}
	return results, failures, skipped
}

func writeUnlinkSummary(
	out io.Writer,
	results []actions.MutationResult,
	failures []mutationFailure,
	skipped []mutationSkipped,
) {
	_, _ = fmt.Fprintln(out, "Summary:")
	for _, status := range []string{
		actions.ResultRemovedActiveLink,
		actions.ResultRemovedUnmanagedLink,
		actions.ResultRemovedUnmanaged,
		actions.ResultMigratedUnlinked,
	} {
		var names []string
		for _, result := range results {
			if result.Status == status {
				names = append(names, result.Name)
			}
		}
		if len(names) > 0 {
			_, _ = fmt.Fprintf(out, "%s: %s\n", status, strings.Join(names, ", "))
		}
	}
	for _, failure := range failures {
		_, _ = fmt.Fprintf(out, "failed: %s (%v)\n", failure.name, failure.err)
	}
	if len(skipped) > 0 {
		names := make([]string, 0, len(skipped))
		for _, item := range skipped {
			names = append(names, item.name)
		}
		_, _ = fmt.Fprintf(out, "skipped: %s\n", strings.Join(names, ", "))
	}
}
