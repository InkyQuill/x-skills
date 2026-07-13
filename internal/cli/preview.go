package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/InkyQuill/x-skills/internal/remote"
	"github.com/spf13/cobra"
)

type previewOptions struct {
	lines int
}

type previewJSON struct {
	Repository     string `json:"repository"`
	RequestedSkill string `json:"requested_skill"`
	SkillPath      string `json:"skill_path"`
	Commit         string `json:"commit"`
	Content        string `json:"content"`
	ReturnedLines  int    `json:"returned_lines"`
	RequestedLines int    `json:"requested_lines"`
	Truncated      bool   `json:"truncated"`
}

func newPreviewCommand(rootOptions *options) *cobra.Command {
	opts := previewOptions{lines: 50}
	cmd := &cobra.Command{
		Use:   "preview OWNER/REPO SKILL",
		Short: "Preview a remote skill document",
		Args:  cobra.ExactArgs(2),
		Annotations: map[string]string{
			skipConfigAnnotation: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.lines <= 0 {
				return fmt.Errorf("--lines must be positive")
			}
			parsed, err := remote.ParseAddSource(args[0], "", "")
			if err != nil {
				return err
			}
			isShorthand := parsed.Source.Owner != "" &&
				parsed.Source.Repo != "" &&
				parsed.Source.Ref == "" &&
				parsed.PreferredPath == "" &&
				len(parsed.Names) == 0
			if !isShorthand {
				return fmt.Errorf("repository must use OWNER/REPO shorthand")
			}

			cacheRoot, err := os.MkdirTemp("", "x-skills-preview-cache-*")
			if err != nil {
				return fmt.Errorf("create preview cache: %w", err)
			}
			defer func() {
				_ = os.RemoveAll(cacheRoot)
			}()

			result, err := remote.ResolvePreview(
				cmd.Context(),
				remote.NewCheckoutCache(cacheRoot),
				remote.PreviewRequest{
					Source: parsed.Source,
					Name:   args[1],
				},
			)
			if err != nil {
				return err
			}
			content, returned, truncated := firstLines(result.SkillMD, opts.lines)
			if rootOptions.json {
				return writePreviewJSON(
					cmd.OutOrStdout(),
					result,
					content,
					returned,
					opts.lines,
					truncated,
				)
			}
			return writeRawPreview(cmd.OutOrStdout(), content)
		},
	}
	cmd.Flags().IntVar(&opts.lines, "lines", opts.lines, "maximum raw lines to print")
	return cmd
}

func firstLines(content []byte, limit int) ([]byte, int, bool) {
	if limit <= 0 {
		return []byte{}, 0, len(content) > 0
	}
	lines := 0
	for i, b := range content {
		if b != '\n' {
			continue
		}
		lines++
		if lines == limit {
			return content[:i+1], lines, i+1 < len(content)
		}
	}
	if len(content) > 0 && content[len(content)-1] != '\n' {
		lines++
	}
	return content, lines, false
}

func writeRawPreview(out io.Writer, content []byte) error {
	if _, err := out.Write(content); err != nil {
		return fmt.Errorf("write preview: %w", err)
	}
	if len(content) == 0 || bytes.HasSuffix(content, []byte("\n")) {
		return nil
	}
	if _, err := io.WriteString(out, "\n"); err != nil {
		return fmt.Errorf("terminate preview output: %w", err)
	}
	return nil
}

func writePreviewJSON(
	out io.Writer,
	result remote.PreviewResult,
	content []byte,
	returnedLines int,
	requestedLines int,
	truncated bool,
) error {
	payload := previewJSON{
		Repository:     result.Repository,
		RequestedSkill: result.RequestedName,
		SkillPath:      result.SkillPath,
		Commit:         result.Commit,
		Content:        string(content),
		ReturnedLines:  returnedLines,
		RequestedLines: requestedLines,
		Truncated:      truncated,
	}
	if err := json.NewEncoder(out).Encode(payload); err != nil {
		return fmt.Errorf("encode preview JSON: %w", err)
	}
	return nil
}
