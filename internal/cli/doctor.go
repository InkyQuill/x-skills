package cli

import (
	"bufio"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
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
				archiveOnlyBuiltIns := len(opts.at) == 0
				if !rootOptions.yes {
					if len(opts.at) > 0 || rootOptions.noInput || rootOptions.no {
						return fmt.Errorf("doctor fix requires confirmation; rerun with -y")
					}
					locations, archiveOnlyBuiltIns, err = promptDoctorBuiltInDestinations(cmd, cfg)
					if err != nil {
						return fmt.Errorf("doctor fix requires confirmation; rerun with -y")
					}
					filter = doctorFilterForLocations(locations)
				}
				results, err := fixDoctorLocations(cfg, filter, locations, archiveOnlyBuiltIns)
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

func promptDoctorBuiltInDestinations(cmd *cobra.Command, cfg config.Config) ([]roots.ActiveRoot, bool, error) {
	globalRoots := roots.ActiveRoots(cfg, roots.Filter{Scope: config.ScopeGlobal})
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Select global Skills Folders for built-in skills (comma-separated):")
	if len(globalRoots) == 0 {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "  1. [x] Archive only")
		_, _ = fmt.Fprint(cmd.OutOrStdout(), "Choice [default 1]: ")
		line, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
		if err != nil {
			return nil, false, err
		}
		if choice := strings.TrimSpace(line); choice != "" && choice != "1" {
			return nil, false, fmt.Errorf("invalid destination choice %q", choice)
		}
		return nil, true, nil
	}
	defaultIndex := 0
	for i, root := range globalRoots {
		if root.Target == config.TargetAgents {
			defaultIndex = i
			break
		}
	}
	for i, root := range globalRoots {
		checked := " "
		if i == defaultIndex {
			checked = "x"
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %d. [%s] %s\n", i+1, checked, root.Label)
	}
	archiveIndex := len(globalRoots) + 1
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %d. [ ] Archive only\nChoice [default %d]: ", archiveIndex, defaultIndex+1)
	line, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	if err != nil {
		return nil, false, err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return []roots.ActiveRoot{globalRoots[defaultIndex]}, false, nil
	}
	var selected []roots.ActiveRoot
	for _, field := range strings.Split(line, ",") {
		index, parseErr := strconv.Atoi(strings.TrimSpace(field))
		if parseErr != nil || index < 1 || index > archiveIndex {
			return nil, false, fmt.Errorf("invalid destination choice %q", field)
		}
		if index == archiveIndex {
			if len(strings.Split(line, ",")) != 1 {
				return nil, false, fmt.Errorf("archive only cannot be combined with Skills Folders")
			}
			return nil, true, nil
		}
		selected = append(selected, globalRoots[index-1])
	}
	return selected, false, nil
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

func fixDoctorLocations(cfg config.Config, filter doctor.Filter, locations []roots.ActiveRoot, archiveOnlyBuiltIns bool) ([]doctor.FixResult, error) {
	issues, err := doctor.Diagnose(cfg, filter)
	if err != nil {
		return nil, err
	}
	filtered := issues
	if len(locations) > 1 {
		filtered = filterDoctorIssuesByLocations(issues, locations)
	}
	destinations := locations
	if archiveOnlyBuiltIns {
		destinations = nil
	}
	for _, issue := range issues {
		if issue.Kind != doctor.KindMissingBuiltIn && issue.Kind != doctor.KindInactiveBuiltIn {
			continue
		}
		if err := doctor.ValidateBuiltInDestinations(destinations); err != nil {
			return nil, err
		}
		break
	}
	results, err := doctor.FixIssues(filtered)
	if err != nil {
		return results, err
	}
	builtInResults, err := doctor.FixBuiltIns(cfg, issues, doctor.FixOptions{
		BuiltInDestinations: destinations,
		ArchiveOnlyBuiltIns: archiveOnlyBuiltIns,
	})
	return append(results, builtInResults...), err
}

func filterDoctorIssuesByLocations(issues []doctor.Issue, locations []roots.ActiveRoot) []doctor.Issue {
	if len(locations) <= 1 {
		return issues
	}
	allowed := pathPrefixSet(locations)
	filtered := make([]doctor.Issue, 0, len(issues))
	for _, issue := range issues {
		if issue.Kind == doctor.KindRecommendedManifestUntracked || issue.Kind == doctor.KindLocalManifestTracked || issue.Kind == doctor.KindSkillsFolderTracked {
			filtered = append(filtered, issue)
			continue
		}
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
