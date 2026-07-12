package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/InkyQuill/x-skills/internal/actions"
	"github.com/InkyQuill/x-skills/internal/roots"
	"github.com/spf13/cobra"
)

type listOptions struct {
	at []string
}

func newListCommand(rootOptions *options) *cobra.Command {
	var opts listOptions
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List active skills",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := rootOptions.config()
			locations, err := resolveOptionalLocations(cfg, opts.at)
			if err != nil {
				return err
			}
			filter := scanFilterForLocations(locations)
			skills, err := actions.ScanActive(cfg, filter)
			if err != nil {
				return err
			}
			skills = filterSkillsByLocations(skills, locations)
			return writeList(cmd.OutOrStdout(), skills, filter)
		},
	}

	cmd.Flags().StringArrayVar(&opts.at, "at", nil, "managed root location; repeat for multiple locations")

	return cmd
}

func scanFilterForLocations(locations []roots.ActiveRoot) actions.ScanFilter {
	if len(locations) != 1 {
		return actions.ScanFilter{}
	}
	return actions.ScanFilter{
		Scope:  locations[0].Scope,
		Target: locations[0].Target,
	}
}

func filterSkillsByLocations(skills []actions.ActiveSkill, locations []roots.ActiveRoot) []actions.ActiveSkill {
	if len(locations) <= 1 {
		return skills
	}
	allowed := locationSet(locations)
	filtered := make([]actions.ActiveSkill, 0, len(skills))
	for _, skill := range skills {
		if allowed[locationKey(skill.Root)] {
			filtered = append(filtered, skill)
		}
	}
	return filtered
}

func writeList(out io.Writer, skills []actions.ActiveSkill, filter actions.ScanFilter) error {
	if len(skills) == 0 {
		_, err := fmt.Fprintln(out, "No active skills found.")
		return err
	}

	for _, root := range rootsForSkills(skills, filter) {
		if _, err := fmt.Fprintf(out, "%s %s (%s)\n", strings.ToUpper(root.Scope), root.Target, root.Label); err != nil {
			return err
		}
		for _, skill := range skills {
			if skill.Root.Scope != root.Scope || skill.Root.Target != root.Target {
				continue
			}
			if skill.Status == actions.StatusBroken {
				if _, err := fmt.Fprintf(out, "  %s  %s  %s\n", skill.Name, skill.Status, skill.Reason); err != nil {
					return err
				}
				continue
			}
			if _, err := fmt.Fprintf(out, "  %s  %s  %s\n", skill.Name, skill.Status, skill.Description); err != nil {
				return err
			}
		}
	}
	return nil
}

func rootsForSkills(skills []actions.ActiveSkill, filter actions.ScanFilter) []roots.ActiveRoot {
	seen := map[string]bool{}
	var result []roots.ActiveRoot
	for _, skill := range skills {
		key := skill.Root.Scope + "\x00" + skill.Root.Target
		if seen[key] {
			continue
		}
		if filter.Scope != "" && skill.Root.Scope != filter.Scope {
			continue
		}
		if filter.Target != "" && skill.Root.Target != filter.Target {
			continue
		}
		seen[key] = true
		result = append(result, skill.Root)
	}
	return result
}
