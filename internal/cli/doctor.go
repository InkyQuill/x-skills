package cli

import (
	"fmt"
	"io"
	"slices"

	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/doctor"
	"github.com/spf13/cobra"
)

type doctorOptions struct {
	project bool
	global  bool
	target  string
	fix     bool
}

func newDoctorCommand(rootOptions *options) *cobra.Command {
	var opts doctorOptions
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose active skill root issues",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.validate(); err != nil {
				return err
			}
			if opts.fix {
				if !rootOptions.yes {
					return fmt.Errorf("doctor fix requires confirmation; rerun with -y")
				}
				results, err := doctor.Fix(rootOptions.config(), doctor.FixOptions{
					Yes:    rootOptions.yes,
					Filter: opts.filter(),
				})
				if err != nil {
					return err
				}
				writeDoctorFixResults(cmd.OutOrStdout(), results)
				return nil
			}

			issues, err := doctor.Diagnose(rootOptions.config(), opts.filter())
			if err != nil {
				return err
			}
			writeDoctorIssues(cmd.OutOrStdout(), issues)
			return nil
		},
	}
	cmd.Flags().BoolVar(&opts.project, "project", false, "check project active roots")
	cmd.Flags().BoolVar(&opts.global, "global", false, "check global active roots")
	cmd.Flags().StringVar(&opts.target, "target", "", "filter by target: agents, claude, or codex")
	cmd.Flags().BoolVar(&opts.fix, "fix", false, "apply safe fixes")
	return cmd
}

func (o doctorOptions) validate() error {
	if o.target == "" {
		return nil
	}
	if !slices.Contains(config.Targets, o.target) {
		return fmt.Errorf("unknown target %q", o.target)
	}
	return nil
}

func (o doctorOptions) filter() doctor.Filter {
	var scope string
	switch {
	case o.project && !o.global:
		scope = config.ScopeProject
	case o.global && !o.project:
		scope = config.ScopeGlobal
	}
	return doctor.Filter{
		Scope:  scope,
		Target: o.target,
	}
}

func writeDoctorIssues(out io.Writer, issues []doctor.Issue) {
	if len(issues) == 0 {
		fmt.Fprintln(out, "No issues found.")
		return
	}

	fmt.Fprintln(out, "Issues:")
	for _, issue := range issues {
		fmt.Fprintf(
			out,
			"%s  %s  %s  %s\n  path: %s\n  reason: %s\n",
			issue.Location,
			issue.Name,
			issue.Kind,
			issue.SafeFix,
			issue.Path,
			issue.Reason,
		)
	}
}

func writeDoctorFixResults(out io.Writer, results []doctor.FixResult) {
	if len(results) == 0 {
		fmt.Fprintln(out, "No fixes applied.")
		return
	}

	fmt.Fprintln(out, "Fixes:")
	for _, result := range results {
		fmt.Fprintf(out, "%s  %s  %s\n", result.Action, result.Name, result.Path)
	}
}
