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

type activeRootOptions struct {
	project bool
	global  bool
	target  string
}

type mutationFailure struct {
	name string
	err  error
}

func newMigrateCommand(rootOptions *options) *cobra.Command {
	var opts activeRootOptions
	cmd := &cobra.Command{
		Use:   "migrate NAME [NAME...]",
		Short: "Move unmanaged active skills into the archive",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.validate(); err != nil {
				return err
			}
			if !rootOptions.yes {
				return fmt.Errorf("migrate requires confirmation; rerun with -y")
			}

			scope := opts.scope()
			results, failures := migrateNames(rootOptions.config(), args, scope, opts.target, rootOptions.yes)
			if len(args) == 1 && len(failures) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "migrated %s %s: %s\n", scope, opts.target, results[0].Name)
				return nil
			}

			writeMutationSummary(cmd.OutOrStdout(), "migrated", results, failures)
			if len(failures) > 0 {
				return fmt.Errorf("migrate failed for %d skill(s)", len(failures))
			}
			return nil
		},
	}
	addActiveRootFlags(cmd, &opts)
	return cmd
}

func addActiveRootFlags(cmd *cobra.Command, opts *activeRootOptions) {
	cmd.Flags().BoolVar(&opts.project, "project", false, "use the project active root")
	cmd.Flags().BoolVar(&opts.global, "global", false, "use the global active root")
	cmd.Flags().StringVar(&opts.target, "target", "", "target to use: agents, claude, or codex")
}

func (o activeRootOptions) validate() error {
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

func (o activeRootOptions) scope() string {
	if o.project {
		return config.ScopeProject
	}
	return config.ScopeGlobal
}

func migrateNames(
	cfg config.Config,
	names []string,
	scope string,
	target string,
	confirmed bool,
) ([]actions.MutationResult, []mutationFailure) {
	var results []actions.MutationResult
	var failures []mutationFailure
	for _, name := range names {
		result, err := actions.Migrate(cfg, actions.MigrateRequest{
			Name:      name,
			Scope:     scope,
			Target:    target,
			Confirmed: confirmed,
		})
		if err != nil {
			failures = append(failures, mutationFailure{name: name, err: err})
			continue
		}
		results = append(results, result)
	}
	return results, failures
}

func writeMutationSummary(
	out io.Writer,
	successLabel string,
	results []actions.MutationResult,
	failures []mutationFailure,
) {
	fmt.Fprintln(out, "Summary:")
	if len(results) > 0 {
		names := make([]string, 0, len(results))
		for _, result := range results {
			names = append(names, result.Name)
		}
		fmt.Fprintf(out, "%s: %s\n", successLabel, strings.Join(names, ", "))
	}
	for _, failure := range failures {
		fmt.Fprintf(out, "failed: %s (%v)\n", failure.name, failure.err)
	}
}
