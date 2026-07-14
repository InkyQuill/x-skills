package cli

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/InkyQuill/x-skills/internal/actions"
	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/roots"
	"github.com/spf13/cobra"
)

func confirm(cmd *cobra.Command, opts *options, prompt, noInputErr string) (bool, error) {
	if opts.yes {
		return true, nil
	}
	if opts.no {
		return false, nil
	}
	if opts.noInput {
		return false, fmt.Errorf("%s", noInputErr)
	}
	_, _ = fmt.Fprint(cmd.OutOrStdout(), prompt)
	answer, err := readPromptLine(cmd.InOrStdin())
	if err != nil {
		return false, err
	}
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes", nil
}

func chooseDestination(
	cmd *cobra.Command,
	opts *options,
	cfg config.Config,
	name string,
	action string,
	location *roots.ActiveRoot,
) (roots.ActiveRoot, error) {
	if location != nil {
		return *location, nil
	}
	candidates := roots.ActiveRoots(cfg, roots.Filter{})
	if len(candidates) == 1 {
		return candidates[0], nil
	}
	if opts.noInput {
		return roots.ActiveRoot{}, fmt.Errorf("choose a destination:\n  %s", strings.Join(destinationCommands(action, name, candidates), "\n  "))
	}
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Select destination for "+action+":")
	for i, root := range candidates {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %d. %s %s  %s\n", i+1, root.Scope, root.Target, root.Path)
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Select destination for %s [1-%d]: ", action, len(candidates))
	index, err := readSelection(cmd.InOrStdin(), len(candidates))
	if err != nil {
		return roots.ActiveRoot{}, err
	}
	return candidates[index], nil
}

func chooseActiveSkill(
	cmd *cobra.Command,
	opts *options,
	cfg config.Config,
	name string,
	action string,
	location *roots.ActiveRoot,
) (actions.ActiveSkill, error) {
	filter := actions.ScanFilter{}
	if location != nil {
		filter.Scope = location.Scope
		filter.Target = location.Target
	}
	candidates, err := matchingActiveSkills(cfg, name, filter)
	if err != nil {
		return actions.ActiveSkill{}, err
	}
	if len(candidates) == 0 {
		return actions.ActiveSkill{}, fmt.Errorf("active skill not found: %s", name)
	}
	if len(candidates) == 1 {
		return candidates[0], nil
	}
	if opts.noInput {
		return actions.ActiveSkill{}, fmt.Errorf("multiple active skills named %q; choose one:\n  %s", name, strings.Join(activeSkillCommands(action, name, candidates), "\n  "))
	}
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Select skill to "+action+":")
	for i, skill := range candidates {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %d. %s %s  %s  %s\n", i+1, skill.Root.Scope, skill.Root.Target, skill.Path, skill.Status)
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Select skill to %s [1-%d]: ", action, len(candidates))
	index, err := readSelection(cmd.InOrStdin(), len(candidates))
	if err != nil {
		return actions.ActiveSkill{}, err
	}
	return candidates[index], nil
}

func chooseUnmanagedUnlinkAction(cmd *cobra.Command, opts *options, skill actions.ActiveSkill) (bool, bool, error) {
	if skill.Status != actions.StatusUnmanaged {
		return true, false, nil
	}
	if opts.yes {
		return true, false, nil
	}
	if opts.no {
		return false, false, nil
	}
	if opts.noInput {
		return false, false, fmt.Errorf(
			"unmanaged active skill %q requires a choice; archive then unlink with -y or remove without archiving with --delete-unmanaged -y",
			skill.Identity,
		)
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%q is an unmanaged active skill at %s.\n", skill.Identity, skill.Path)
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Choose unlink action:")
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "  1. archive in repo, then unlink")
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "  2. remove active copy without archiving")
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "  3. cancel")
	_, _ = fmt.Fprint(cmd.OutOrStdout(), "Select unlink action [1-3]: ")
	index, err := readSelection(cmd.InOrStdin(), 3)
	if err != nil {
		return false, false, err
	}
	switch index {
	case 0:
		return true, false, nil
	case 1:
		return true, true, nil
	default:
		return false, false, nil
	}
}

func matchingActiveSkills(cfg config.Config, name string, filter actions.ScanFilter) ([]actions.ActiveSkill, error) {
	skills, err := actions.ScanActive(cfg, filter)
	if err != nil {
		return nil, err
	}
	var matches []actions.ActiveSkill
	for _, skill := range skills {
		if matchesSkill(name, skill) {
			matches = append(matches, skill)
		}
	}
	return matches, nil
}

func matchesSkill(selector string, skill actions.ActiveSkill) bool {
	return selector == skill.Identity ||
		(skill.DeclaredName != "" && selector == skill.DeclaredName)
}

func destinationCommands(action, name string, candidates []roots.ActiveRoot) []string {
	commands := make([]string, 0, len(candidates))
	for _, root := range candidates {
		commands = append(commands, fmt.Sprintf("x-skills %s %s --at %s:%s", action, name, root.Scope, root.Target))
	}
	return commands
}

func activeSkillCommands(action, name string, candidates []actions.ActiveSkill) []string {
	commands := make([]string, 0, len(candidates))
	for _, skill := range candidates {
		commands = append(commands, fmt.Sprintf("x-skills %s %s --at %s:%s", action, name, skill.Root.Scope, skill.Root.Target))
	}
	return commands
}

func readSelection(in io.Reader, count int) (int, error) {
	line, err := readPromptLine(in)
	if err != nil {
		return 0, err
	}
	selected, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		return 0, fmt.Errorf("invalid selection")
	}
	if selected < 1 || selected > count {
		return 0, fmt.Errorf("selection out of range")
	}
	return selected - 1, nil
}

func readPromptLine(in io.Reader) (string, error) {
	var line []byte
	buf := make([]byte, 1)
	for {
		n, err := in.Read(buf)
		if n > 0 {
			if buf[0] == '\n' {
				return strings.TrimRight(string(line), "\r"), nil
			}
			line = append(line, buf[0])
		}
		if err != nil {
			if len(line) > 0 {
				return strings.TrimRight(string(line), "\r"), nil
			}
			if err == io.EOF {
				return "", nil
			}
			return "", err
		}
	}
}
