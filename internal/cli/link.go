package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/InkyQuill/x-skills/internal/actions"
	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/repo"
	"github.com/InkyQuill/x-skills/internal/roots"
	"github.com/spf13/cobra"
)

func newLinkCommand(rootOptions *options) *cobra.Command {
	var opts activeRootOptions
	cmd := &cobra.Command{
		Use:   "link NAME [NAME...]",
		Short: "Link archived skills into an active root",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.validateFilter(); err != nil {
				return err
			}

			results, failures := linkNames(cmd, rootOptions, args, opts)
			if len(results) == 1 && len(failures) == 0 {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "linked: %s\n", results[0].Name)
				return nil
			}
			if len(args) == 1 && len(failures) == 1 {
				return failures[0].err
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
	cmd *cobra.Command,
	rootOptions *options,
	names []string,
	opts activeRootOptions,
) ([]actions.MutationResult, []mutationFailure) {
	cfg := rootOptions.config()
	locations, locationErr := resolveLinkLocations(cfg, opts.at)
	var results []actions.MutationResult
	var failures []mutationFailure
	for _, name := range names {
		if !repo.HasSkill(cfg, name) {
			failures = append(failures, mutationFailure{name: name, err: fmt.Errorf("repo skill %q not found", name)})
			continue
		}
		if locationErr != nil {
			failures = append(failures, mutationFailure{name: name, err: locationErr})
			continue
		}
		if len(locations) == 0 {
			root, err := chooseDestination(cmd, rootOptions, cfg, name, "link", nil)
			if err != nil {
				failures = append(failures, mutationFailure{name: name, err: err})
				continue
			}
			locations = append(locations, root)
		}
		for _, root := range locations {
			result, err := actions.Link(cfg, actions.LinkRequest{Name: name, Scope: root.Scope, Target: root.Target})
			if err != nil {
				failures = append(failures, mutationFailure{name: name, err: err})
				continue
			}
			results = append(results, result)
		}
	}
	return results, failures
}

func resolveLinkLocations(cfg config.Config, selectors []string) ([]roots.ActiveRoot, error) {
	if len(selectors) == 0 {
		return nil, nil
	}
	return resolveLocations(cfg, selectors)
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
