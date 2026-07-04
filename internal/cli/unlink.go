package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/InkyQuill/x-skills/internal/actions"
	"github.com/InkyQuill/x-skills/internal/config"
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
			if !rootOptions.yes {
				return fmt.Errorf("unlink requires confirmation; rerun with -y")
			}

			scope := opts.scope()
			results, failures := unlinkNames(
				rootOptions.config(),
				args,
				scope,
				opts.target,
				rootOptions.yes,
				opts.deleteUnmanaged,
			)
			if len(args) == 1 && len(failures) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "%s %s %s: %s\n", results[0].Status, scope, opts.target, results[0].Name)
				return nil
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

func unlinkNames(
	cfg config.Config,
	names []string,
	scope string,
	target string,
	confirmed bool,
	deleteUnmanaged bool,
) ([]actions.MutationResult, []mutationFailure) {
	var results []actions.MutationResult
	var failures []mutationFailure
	for _, name := range names {
		result, err := actions.Unlink(cfg, actions.UnlinkRequest{
			Name:            name,
			Scope:           scope,
			Target:          target,
			Confirmed:       confirmed,
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
