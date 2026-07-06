package cli

import (
	"fmt"
	"io"
	"path/filepath"
	"slices"
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

			scope := opts.scopeFilter()
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

func (o unlinkOptions) validate() error {
	if o.project && o.global {
		return fmt.Errorf("choose at most one of --project or --global")
	}
	if o.target != "" && !slices.Contains(config.Targets, o.target) {
		return fmt.Errorf("unknown target %q", o.target)
	}
	return nil
}

func (o unlinkOptions) scopeFilter() string {
	switch {
	case o.project:
		return config.ScopeProject
	case o.global:
		return config.ScopeGlobal
	default:
		return ""
	}
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
	if scope == "" || target == "" {
		return unlinkMatchingNames(cfg, names, actions.ScanFilter{Scope: scope, Target: target}, confirmed, deleteUnmanaged)
	}
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

func unlinkMatchingNames(
	cfg config.Config,
	names []string,
	filter actions.ScanFilter,
	confirmed bool,
	deleteUnmanaged bool,
) ([]actions.MutationResult, []mutationFailure) {
	activeSkills, err := actions.ScanActive(cfg, filter)
	if err != nil {
		failures := make([]mutationFailure, 0, len(names))
		for _, name := range names {
			failures = append(failures, mutationFailure{name: name, err: err})
		}
		return nil, failures
	}
	wanted := map[string]bool{}
	for _, name := range names {
		wanted[name] = true
	}
	matched := map[string]bool{}
	var results []actions.MutationResult
	var failures []mutationFailure
	for _, skill := range activeSkills {
		requestName := filepath.Base(skill.Path)
		if !wanted[skill.Name] && !wanted[requestName] {
			continue
		}
		matched[skill.Name] = true
		matched[requestName] = true
		result, err := actions.Unlink(cfg, actions.UnlinkRequest{
			Name:            requestName,
			Scope:           skill.Root.Scope,
			Target:          skill.Root.Target,
			Confirmed:       confirmed,
			DeleteUnmanaged: deleteUnmanaged,
		})
		if err != nil {
			failures = append(failures, mutationFailure{name: requestName, err: err})
			continue
		}
		results = append(results, result)
	}
	for _, name := range names {
		if !matched[name] {
			failures = append(failures, mutationFailure{name: name, err: fmt.Errorf("active skill not found")})
		}
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
