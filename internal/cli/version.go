package cli

import (
	"fmt"

	"github.com/InkyQuill/x-skills/internal/buildinfo"
	"github.com/spf13/cobra"
)

func newVersionCommand(info buildinfo.Info) *cobra.Command {
	return &cobra.Command{
		Use:         "version",
		Short:       "Print the x-skills version",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{skipConfigAnnotation: ""},
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), info.Display())
			return err
		},
	}
}
