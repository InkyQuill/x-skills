package cli

import (
	"encoding/json"
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

type listJSONRecord struct {
	Identity     string       `json:"identity"`
	DeclaredName string       `json:"declared_name,omitempty"`
	Description  string       `json:"description,omitempty"`
	Status       string       `json:"status"`
	Path         string       `json:"path"`
	Reason       string       `json:"reason,omitempty"`
	Root         listJSONRoot `json:"root"`
}

type listJSONRoot struct {
	Scope  string `json:"scope"`
	Target string `json:"target"`
	Label  string `json:"label"`
	Path   string `json:"path"`
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
			if rootOptions.json {
				return writeListJSON(cmd.OutOrStdout(), skills)
			}
			return writeListHuman(cmd.OutOrStdout(), skills, filter)
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

func writeListHuman(out io.Writer, skills []actions.ActiveSkill, filter actions.ScanFilter) error {
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
				if _, err := fmt.Fprintf(
					out,
					"  %s  %s  %s\n",
					skillDisplayName(skill.Identity, skill.DeclaredName),
					skill.Status,
					skill.Reason,
				); err != nil {
					return err
				}
				continue
			}
			if _, err := fmt.Fprintf(
				out,
				"  %s  %s  %s\n",
				skillDisplayName(skill.Identity, skill.DeclaredName),
				skill.Status,
				skill.Description,
			); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeListJSON(out io.Writer, skills []actions.ActiveSkill) error {
	records := make([]listJSONRecord, 0, len(skills))
	for _, skill := range skills {
		records = append(records, listJSONRecord{
			Identity:     skill.Identity,
			DeclaredName: differingDeclaredName(skill.Identity, skill.DeclaredName),
			Description:  skill.Description,
			Status:       skill.Status,
			Path:         skill.Path,
			Reason:       skill.Reason,
			Root: listJSONRoot{
				Scope:  skill.Root.Scope,
				Target: skill.Root.Target,
				Label:  skill.Root.Label,
				Path:   skill.Root.Path,
			},
		})
	}
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(records)
}

func skillDisplayName(identity, declaredName string) string {
	if differingDeclaredName(identity, declaredName) == "" {
		return identity
	}
	return fmt.Sprintf("%s (declared: %s)", identity, declaredName)
}

func differingDeclaredName(identity, declaredName string) string {
	if declaredName == "" || declaredName == identity {
		return ""
	}
	return declaredName
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
