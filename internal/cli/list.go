package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/InkyQuill/x-skills/internal/actions"
	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/roots"
	"github.com/spf13/cobra"
)

type listOptions struct {
	project bool
	global  bool
	target  string
}

func newListCommand(rootOptions *options) *cobra.Command {
	var opts listOptions
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List active skills",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.validate(); err != nil {
				return err
			}
			filter := opts.scanFilter()
			skills, err := actions.ScanActive(rootOptions.config(), filter)
			if err != nil {
				return err
			}
			return writeList(cmd.OutOrStdout(), skills, filter)
		},
	}

	cmd.Flags().BoolVar(&opts.project, "project", false, "show project active skills")
	cmd.Flags().BoolVar(&opts.global, "global", false, "show global active skills")
	cmd.Flags().StringVar(&opts.target, "target", "", "filter by target: agents, claude, or codex")

	return cmd
}

func (o listOptions) validate() error {
	if o.project && o.global {
		return fmt.Errorf("choose at most one of --project or --global")
	}
	if o.target == "" {
		return nil
	}
	for _, target := range config.Targets {
		if o.target == target {
			return nil
		}
	}
	return fmt.Errorf("unknown target %q", o.target)
}

func (o listOptions) scanFilter() actions.ScanFilter {
	var scope string
	switch {
	case o.project && !o.global:
		scope = config.ScopeProject
	case o.global && !o.project:
		scope = config.ScopeGlobal
	}

	return actions.ScanFilter{
		Scope:  scope,
		Target: o.target,
	}
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
