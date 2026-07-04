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
			writeRepo(cmd.OutOrStdout(), skills)
			return nil
		},
	}

	return cmd
}

func writeRepo(out io.Writer, skills []repo.Skill) {
	for _, skill := range skills {
		if skill.Description == "" {
			fmt.Fprintln(out, skill.Name)
			continue
		}
		fmt.Fprintf(out, "%s  %s\n", skill.Name, skill.Description)
	}
}
