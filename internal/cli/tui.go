package cli

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/InkyQuill/x-skills/internal/tui"
)

type tuiOptions struct {
	noInput bool
}

func newTUICommand(rootOptions *options) *cobra.Command {
	var opts tuiOptions
	cmd := &cobra.Command{
		Use:   "tui",
		Short: "Open the guided skill manager",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.noInput {
				return fmt.Errorf("tui requires an interactive terminal")
			}
			program := tea.NewProgram(tui.New(rootOptions.config()), tea.WithAltScreen())
			_, err := program.Run()
			return err
		},
	}
	cmd.Flags().BoolVar(&opts.noInput, "no-input", false, "fail instead of opening the interactive manager")
	return cmd
}
