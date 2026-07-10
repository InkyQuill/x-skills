package cli

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/doctor"
	"github.com/InkyQuill/x-skills/internal/roots"
	"github.com/spf13/cobra"
)

type doctorOptions struct {
	at  []string
	fix bool
}

func newDoctorCommand(rootOptions *options) *cobra.Command {
	var opts doctorOptions
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose active skill root issues",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := rootOptions.config()
			locations, err := resolveOptionalLocations(cfg, opts.at)
			if err != nil {
				return err
			}
			filter := doctorFilterForLocations(locations)
			if opts.fix {
				if !rootOptions.yes {
					return fmt.Errorf("doctor fix requires confirmation; rerun with -y")
				}
				results, err := fixDoctorLocations(cfg, filter, locations)
				if err != nil && len(results) > 0 {
					writeDoctorFixResults(cmd.OutOrStdout(), results)
					return err
				}
				if err != nil {
					return err
				}
				writeDoctorFixResults(cmd.OutOrStdout(), results)
				return nil
			}

			issues, err := doctor.Diagnose(cfg, filter)
			if err != nil {
				return err
			}
			issues = filterDoctorIssuesByLocations(issues, locations)
			writeDoctorIssues(cmd.OutOrStdout(), issues)
			return nil
		},
	}
	cmd.Flags().StringArrayVar(&opts.at, "at", nil, "managed root location; repeat for multiple locations")
	cmd.Flags().BoolVar(&opts.fix, "fix", false, "apply safe fixes")
	return cmd
}

func doctorFilterForLocations(locations []roots.ActiveRoot) doctor.Filter {
	if len(locations) != 1 {
		return doctor.Filter{}
	}
	return doctor.Filter{
		Scope:  locations[0].Scope,
		Target: locations[0].Target,
	}
}

func fixDoctorLocations(cfg config.Config, filter doctor.Filter, locations []roots.ActiveRoot) ([]doctor.FixResult, error) {
	if len(locations) <= 1 {
		return doctor.Fix(cfg, doctor.FixOptions{Yes: true, Filter: filter})
	}
	issues, err := doctor.Diagnose(cfg, filter)
	if err != nil {
		return nil, err
	}
	return doctor.FixIssues(filterDoctorIssuesByLocations(issues, locations))
}

func filterDoctorIssuesByLocations(issues []doctor.Issue, locations []roots.ActiveRoot) []doctor.Issue {
	if len(locations) <= 1 {
		return issues
	}
	allowed := pathPrefixSet(locations)
	filtered := make([]doctor.Issue, 0, len(issues))
	for _, issue := range issues {
		for prefix := range allowed {
			if issue.Path == prefix || strings.HasPrefix(issue.Path, prefix+string(filepath.Separator)) {
				filtered = append(filtered, issue)
				break
			}
		}
	}
	return filtered
}

func writeDoctorIssues(out io.Writer, issues []doctor.Issue) {
	if len(issues) == 0 {
		_, _ = fmt.Fprintln(out, "No issues found.")
		return
	}

	_, _ = fmt.Fprintln(out, "Issues:")
	for _, issue := range issues {
		_, _ = fmt.Fprintf(
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
		_, _ = fmt.Fprintln(out, "No fixes applied.")
		return
	}

	_, _ = fmt.Fprintln(out, "Fixes:")
	for _, result := range results {
		_, _ = fmt.Fprintf(out, "%s  %s  %s\n", result.Action, result.Name, result.Path)
	}
}
