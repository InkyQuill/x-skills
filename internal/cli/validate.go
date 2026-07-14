package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/InkyQuill/x-skills/internal/validation"
	"github.com/spf13/cobra"
)

var errValidationFailed = errors.New("validation failed")

type validateOptions struct {
	locations []string
}

func newValidateCommand(rootOptions *options) *cobra.Command {
	var opts validateOptions
	cmd := &cobra.Command{
		Use:   "validate PATH...",
		Short: "Validate skill files and directories",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := rootOptions.configE()
			if err != nil {
				return err
			}

			validationOptions := validation.Options{}
			if len(opts.locations) > 0 {
				locations, err := resolveLocations(cfg, opts.locations)
				if err != nil {
					return err
				}
				validationOptions.Roots = locations
			}

			report := validation.ValidatePaths(args, validationOptions)
			if rootOptions.json {
				err = writeValidationJSON(cmd.OutOrStdout(), report)
			} else {
				err = writeValidationHuman(cmd.OutOrStdout(), report)
			}
			if err != nil {
				return err
			}
			if !report.Valid {
				return errValidationFailed
			}
			return nil
		},
	}
	cmd.Flags().StringArrayVar(
		&opts.locations,
		"at",
		nil,
		"validate compatibility for a location (repeatable)",
	)
	return cmd
}

func writeValidationHuman(w io.Writer, report validation.Report) error {
	previousPath := ""
	for _, diagnostic := range report.Diagnostics {
		if diagnostic.Path != previousPath {
			if previousPath != "" {
				if _, err := fmt.Fprintln(w); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprintln(w, diagnostic.Path); err != nil {
				return err
			}
			previousPath = diagnostic.Path
		}
		if _, err := fmt.Fprintf(
			w,
			"  %s %s: %s\n",
			diagnostic.Level,
			diagnostic.Code,
			diagnostic.Message,
		); err != nil {
			return err
		}
		if diagnostic.Field != "" {
			if _, err := fmt.Fprintf(w, "    field: %s\n", diagnostic.Field); err != nil {
				return err
			}
		}
		if diagnostic.RelatedPath != "" {
			if _, err := fmt.Fprintf(w, "    related: %s\n", diagnostic.RelatedPath); err != nil {
				return err
			}
		}
	}
	if len(report.Diagnostics) > 0 {
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(
		w,
		"%d skills, %d errors, %d warnings\n",
		report.Summary.Skills,
		report.Summary.Errors,
		report.Summary.Warnings,
	)
	return err
}

func writeValidationJSON(w io.Writer, report validation.Report) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}
