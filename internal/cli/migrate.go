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
			if err := opts.validateFilter(); err != nil {
				return err
			}

			results, failures := migrateNames(cmd, rootOptions, args, opts)
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

func (o activeRootOptions) validateFilter() error {
	if o.project && o.global {
		return fmt.Errorf("choose at most one of --project or --global")
	}
	if o.target != "" && !slices.Contains(config.Targets, o.target) {
		return fmt.Errorf("unknown target %q", o.target)
	}
	return nil
}

func (o activeRootOptions) scopeFilter() string {
	switch {
	case o.project:
		return config.ScopeProject
	case o.global:
		return config.ScopeGlobal
	default:
		return ""
	}
}

func migrateNames(
	cmd *cobra.Command,
	rootOptions *options,
	names []string,
	opts activeRootOptions,
) ([]actions.MutationResult, []mutationFailure) {
	cfg := rootOptions.config()
	var results []actions.MutationResult
	var failures []mutationFailure
	for _, name := range names {
		skill, err := chooseActiveSkill(cmd, rootOptions, cfg, name, "migrate", opts)
		if err != nil {
			failures = append(failures, mutationFailure{name: name, err: err})
			continue
		}
		ok, err := confirm(
			cmd,
			rootOptions,
			fmt.Sprintf("Migrate %s %s skill %q into repo? [y/N]: ", skill.Root.Scope, skill.Root.Target, name),
			"migrate requires confirmation; rerun with -y",
		)
		if err != nil {
			failures = append(failures, mutationFailure{name: name, err: err})
			continue
		}
		if !ok {
			continue
		}
		result, err := actions.Migrate(cfg, actions.MigrateRequest{
			Name:      name,
			Scope:     skill.Root.Scope,
			Target:    skill.Root.Target,
			Confirmed: true,
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
	_, _ = fmt.Fprintln(out, "Summary:")
	if len(results) > 0 {
		names := make([]string, 0, len(results))
		for _, result := range results {
			names = append(names, result.Name)
		}
		_, _ = fmt.Fprintf(out, "%s: %s\n", successLabel, strings.Join(names, ", "))
	}
	for _, failure := range failures {
		_, _ = fmt.Fprintf(out, "failed: %s (%v)\n", failure.name, failure.err)
	}
}
