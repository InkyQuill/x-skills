package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/InkyQuill/x-skills/internal/remote"
	"github.com/InkyQuill/x-skills/internal/repo"
	"github.com/spf13/cobra"
)

type repoJSONRecord struct {
	Identity     string                 `json:"identity"`
	DeclaredName string                 `json:"declared_name,omitempty"`
	Description  string                 `json:"description,omitempty"`
	Path         string                 `json:"path"`
	Source       *remote.SourceMetadata `json:"source,omitempty"`
}

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
			if rootOptions.json {
				return writeRepoJSON(cmd.OutOrStdout(), skills)
			}
			return writeRepoHuman(cmd.OutOrStdout(), skills)
		},
	}

	return cmd
}

func writeRepoHuman(out io.Writer, skills []repo.Skill) error {
	for _, skill := range skills {
		name := skillDisplayName(skill.Identity, skill.DeclaredName)
		if skill.Description == "" {
			if _, err := fmt.Fprintln(out, name); err != nil {
				return err
			}
			continue
		}
		if _, err := fmt.Fprintf(out, "%s  %s\n", name, skill.Description); err != nil {
			return err
		}
	}
	return nil
}

func writeRepoJSON(out io.Writer, skills []repo.Skill) error {
	records := make([]repoJSONRecord, 0, len(skills))
	for _, skill := range skills {
		records = append(records, repoJSONRecord{
			Identity:     skill.Identity,
			DeclaredName: differingDeclaredName(skill.Identity, skill.DeclaredName),
			Description:  skill.Description,
			Path:         skill.Path,
			Source:       skill.Source,
		})
	}
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(records)
}
