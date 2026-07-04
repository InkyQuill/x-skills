package cli

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/InkyQuill/x-skills/internal/actions"
	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/spf13/cobra"
)

type linkOptions struct {
	project bool
	global  bool
	target  string
}

type linkFailure struct {
	name string
	err  error
}

func newLinkCommand(rootOptions *options) *cobra.Command {
	var opts linkOptions
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
				fmt.Fprintf(cmd.OutOrStdout(), "linked %s %s: %s\n", scope, opts.target, results[0].Name)
				return nil
			}

			writeLinkSummary(cmd.OutOrStdout(), results, failures)
			if len(failures) > 0 {
				return fmt.Errorf("link failed for %d skill(s)", len(failures))
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&opts.project, "project", false, "link into the project active root")
	cmd.Flags().BoolVar(&opts.global, "global", false, "link into the global active root")
	cmd.Flags().StringVar(&opts.target, "target", "", "target to link into: agents, claude, or codex")

	return cmd
}

func (o linkOptions) validate() error {
	if o.project == o.global {
		return fmt.Errorf("choose exactly one of --project or --global")
	}
	if o.target == "" {
		return fmt.Errorf("--target is required")
	}
	if !slices.Contains(config.Targets, o.target) {
		return fmt.Errorf("unknown target %q", o.target)
	}
	return nil
}

func (o linkOptions) scope() string {
	if o.project {
		return config.ScopeProject
	}
	return config.ScopeGlobal
}

func linkNames(
	cfg config.Config,
	names []string,
	scope string,
	target string,
) ([]actions.MutationResult, []linkFailure) {
	var results []actions.MutationResult
	var failures []linkFailure
	for _, name := range names {
		result, err := actions.Link(cfg, actions.LinkRequest{Name: name, Scope: scope, Target: target})
		if err != nil {
			failures = append(failures, linkFailure{name: name, err: err})
			continue
		}
		results = append(results, result)
	}
	return results, failures
}

func writeLinkSummary(
	out io.Writer,
	results []actions.MutationResult,
	failures []linkFailure,
) {
	fmt.Fprintln(out, "Summary:")
	if len(results) > 0 {
		names := make([]string, 0, len(results))
		for _, result := range results {
			names = append(names, result.Name)
		}
		fmt.Fprintf(out, "linked: %s\n", strings.Join(names, ", "))
	}
	for _, failure := range failures {
		fmt.Fprintf(out, "failed: %s (%v)\n", failure.name, failure.err)
	}
}
