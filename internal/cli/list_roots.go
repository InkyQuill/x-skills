package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/spf13/cobra"
)

type listRootsPayload struct {
	Roots []listRootEntry `json:"roots"`
}

type listRootEntry struct {
	Location string `json:"location"`
	Scope    string `json:"scope"`
	Target   string `json:"target"`
	Label    string `json:"label"`
	Path     string `json:"path"`
	Builtin  bool   `json:"builtin"`
	Enabled  bool   `json:"enabled"`
}

func newListRootsCommand(rootOptions *options) *cobra.Command {
	return &cobra.Command{
		Use:   "list-roots",
		Short: "List managed active skill roots",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := rootOptions.configE()
			if err != nil {
				return err
			}
			return writeListRoots(cmd.OutOrStdout(), cfg.ManagedRoots(), rootOptions.json)
		},
	}
}

func writeListRoots(out io.Writer, roots []config.ManagedRoot, asJSON bool) error {
	payload := listRootsPayload{Roots: make([]listRootEntry, 0, len(roots))}
	for _, root := range roots {
		if !root.Enabled {
			continue
		}
		payload.Roots = append(payload.Roots, listRootEntry{
			Location: root.Location(),
			Scope:    root.Scope,
			Target:   root.Target,
			Label:    root.Label,
			Path:     root.Path,
			Builtin:  root.Builtin,
			Enabled:  root.Enabled,
		})
	}
	if asJSON {
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		return encoder.Encode(payload)
	}
	for _, root := range payload.Roots {
		if _, err := fmt.Fprintf(out, "%-8s  %-18s  %s\n", root.Label, root.Location, root.Path); err != nil {
			return err
		}
	}
	return nil
}
