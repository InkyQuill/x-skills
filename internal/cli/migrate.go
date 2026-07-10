package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/InkyQuill/x-skills/internal/actions"
	"github.com/spf13/cobra"
)

type activeRootOptions struct {
	at []string
}

type mutationFailure struct {
	name string
	err  error
}

type mutationSkipped struct {
	name   string
	reason string
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

			results, failures, skipped := migrateNames(cmd, rootOptions, args, opts)
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

			writeMutationSummary(cmd.OutOrStdout(), "migrated", results, failures, skipped)
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
	cmd.Flags().StringArrayVar(&opts.at, "at", nil, "managed root location; repeat for multiple locations")
}

func (o activeRootOptions) validateFilter() error {
	return nil
}

func migrateNames(
	cmd *cobra.Command,
	rootOptions *options,
	names []string,
	opts activeRootOptions,
) ([]actions.MutationResult, []mutationFailure, []mutationSkipped) {
	cfg := rootOptions.config()
	location, locationErr := optionalOneLocation(cfg, opts.at)
	var results []actions.MutationResult
	var failures []mutationFailure
	var skipped []mutationSkipped
	for _, name := range names {
		if locationErr != nil {
			failures = append(failures, mutationFailure{name: name, err: locationErr})
			continue
		}
		skill, err := chooseActiveSkill(cmd, rootOptions, cfg, name, "migrate", location)
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
			skipped = append(skipped, mutationSkipped{name: name, reason: "cancelled"})
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
	return results, failures, skipped
}

func writeMutationSummary(
	out io.Writer,
	successLabel string,
	results []actions.MutationResult,
	failures []mutationFailure,
	skipped []mutationSkipped,
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
	if len(skipped) > 0 {
		names := make([]string, 0, len(skipped))
		for _, item := range skipped {
			names = append(names, item.name)
		}
		_, _ = fmt.Fprintf(out, "skipped: %s\n", strings.Join(names, ", "))
	}
}
