package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/InkyQuill/x-skills/internal/actions"
	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/manifest"
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
			if rootOptions.json {
				if err := writeLinkJSON(cmd.OutOrStdout(), results); err != nil {
					return err
				}
				if len(failures) > 0 {
					return fmt.Errorf("link failed for %d skill(s)", len(failures))
				}
				return nil
			}
			if len(results) == 1 && len(failures) == 0 {
				_, _ = fmt.Fprintf(
					cmd.OutOrStdout(),
					"%s: %s\n",
					linkHumanStatus(results[0].Status),
					results[0].Name,
				)
				return nil
			}
			if len(args) == 1 && len(failures) == 1 && len(results) == 0 {
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
	projectMutated := false
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
			if root.Scope == config.ScopeProject {
				projectMutated = true
			}
		}
	}
	if projectMutated {
		if _, err := manifest.ReconcileLocal(cfg); err != nil {
			failures = append(failures, mutationFailure{name: "local manifest", err: fmt.Errorf("skill mutation succeeded but local manifest reconciliation failed: %w", err)})
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
	for _, status := range []string{actions.ResultLinked, actions.ResultAlreadyLinked} {
		names := make([]string, 0)
		for _, result := range results {
			if result.Status == status {
				names = append(names, result.Name)
			}
		}
		if len(names) > 0 {
			_, _ = fmt.Fprintf(out, "%s: %s\n", linkHumanStatus(status), strings.Join(names, ", "))
		}
	}
	for _, failure := range failures {
		_, _ = fmt.Fprintf(out, "failed: %s (%v)\n", failure.name, failure.err)
	}
}

func linkHumanStatus(status string) string {
	if status == actions.ResultAlreadyLinked {
		return "already linked"
	}
	return status
}

func writeLinkJSON(out io.Writer, results []actions.MutationResult) error {
	if results == nil {
		results = []actions.MutationResult{}
	}
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(results)
}
