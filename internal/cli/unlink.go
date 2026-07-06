package cli

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/InkyQuill/x-skills/internal/actions"
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

			results, failures := unlinkNamesWithOptions(cmd, rootOptions, args, opts)
			if len(results) == 0 && len(failures) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "cancelled")
				return nil
			}
			if len(args) == 1 && len(failures) == 0 {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", results[0].Status, results[0].Name)
				return nil
			}
			if len(args) == 1 && len(failures) == 1 {
				return failures[0].err
			}

			writeUnlinkSummary(cmd.OutOrStdout(), results, failures)
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
) ([]actions.MutationResult, []mutationFailure) {
	cfg := rootOptions.config()
	var results []actions.MutationResult
	var failures []mutationFailure
	for _, name := range names {
		skill, err := chooseActiveSkill(cmd, rootOptions, cfg, name, "unlink", opts.activeRootOptions)
		if err != nil {
			failures = append(failures, mutationFailure{name: name, err: err})
			continue
		}
		if rootOptions.no {
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
			continue
		}
		result, err := actions.Unlink(cfg, actions.UnlinkRequest{
			Name:            filepath.Base(skill.Path),
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
	}
	return results, failures
}

func writeUnlinkSummary(
	out io.Writer,
	results []actions.MutationResult,
	failures []mutationFailure,
) {
	fmt.Fprintln(out, "Summary:")
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
			fmt.Fprintf(out, "%s: %s\n", status, strings.Join(names, ", "))
		}
	}
	for _, failure := range failures {
		fmt.Fprintf(out, "failed: %s (%v)\n", failure.name, failure.err)
	}
}
