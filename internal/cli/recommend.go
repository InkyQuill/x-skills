package cli

import (
	"fmt"
	"strings"

	"github.com/InkyQuill/x-skills/internal/manifest"
	"github.com/spf13/cobra"
)

func newRecommendCommand(rootOptions *options) *cobra.Command {
	return &cobra.Command{
		Use:   "recommend NAME...",
		Short: "Promote archived skills to project recommendations",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, names []string) error {
			if err := manifest.Recommend(rootOptions.config(), names); err != nil {
				return err
			}
			_, err := fmt.Fprintln(cmd.OutOrStdout(), "recommended "+strings.Join(names, ", "))
			return err
		},
	}
}

func newUnrecommendCommand(rootOptions *options) *cobra.Command {
	return &cobra.Command{
		Use:   "unrecommend NAME...",
		Short: "Remove skills from project recommendations",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, names []string) error {
			if err := manifest.Unrecommend(rootOptions.config(), names); err != nil {
				return err
			}
			_, err := fmt.Fprintln(cmd.OutOrStdout(), "removed "+strings.Join(names, ", ")+" from project recommendations")
			return err
		},
	}
}
