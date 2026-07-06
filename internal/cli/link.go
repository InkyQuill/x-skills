package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/InkyQuill/x-skills/internal/actions"
	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/spf13/cobra"
)

func newLinkCommand(rootOptions *options) *cobra.Command {
	var opts activeRootOptions
	cmd := &cobra.Command{
		Use:   "link NAME [NAME...]",
		Short: "Link archived skills into an active root",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.validate(); err != nil {
				return err
			}

			scope := opts.scope()
			results, failures := linkNames(rootOptions.config(), args, scope, opts.target)
			if len(args) == 1 && len(failures) == 0 {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "linked %s %s: %s\n", scope, opts.target, results[0].Name)
				return nil
			}

			writeLinkSummary(cmd.OutOrStdout(), results, failures)
			if len(failures) > 0 {
				return fmt.Errorf("link failed for %d skill(s)", len(failures))
			}
			return nil
		},
	}

	addActiveRootFlags(cmd, &opts)

	return cmd
}

func linkNames(
	cfg config.Config,
	names []string,
	scope string,
	target string,
) ([]actions.MutationResult, []mutationFailure) {
	var results []actions.MutationResult
	var failures []mutationFailure
	for _, name := range names {
		result, err := actions.Link(cfg, actions.LinkRequest{Name: name, Scope: scope, Target: target})
		if err != nil {
			failures = append(failures, mutationFailure{name: name, err: err})
			continue
		}
		results = append(results, result)
	}
	return results, failures
}

func writeLinkSummary(
	out io.Writer,
	results []actions.MutationResult,
	failures []mutationFailure,
) {
	_, _ = fmt.Fprintln(out, "Summary:")
	if len(results) > 0 {
		names := make([]string, 0, len(results))
		for _, result := range results {
			names = append(names, result.Name)
		}
		_, _ = fmt.Fprintf(out, "linked: %s\n", strings.Join(names, ", "))
	}
	for _, failure := range failures {
		_, _ = fmt.Fprintf(out, "failed: %s (%v)\n", failure.name, failure.err)
	}
}
