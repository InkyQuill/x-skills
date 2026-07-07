package cli

import (
	"fmt"
	"io"

	"github.com/InkyQuill/x-skills/internal/repo"
	"github.com/spf13/cobra"
)

func newRepoCommand(rootOptions *options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repo",
		Short: "List archived skills",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			skills, err := repo.List(rootOptions.config())
			if err != nil {
				return err
			}
			return writeRepo(cmd.OutOrStdout(), skills)
		},
	}

	return cmd
}

func writeRepo(out io.Writer, skills []repo.Skill) error {
	for _, skill := range skills {
		if skill.Description == "" {
			if _, err := fmt.Fprintln(out, skill.Name); err != nil {
				return err
			}
			continue
		}
		if _, err := fmt.Fprintf(out, "%s  %s\n", skill.Name, skill.Description); err != nil {
			return err
		}
	}
	return nil
}
