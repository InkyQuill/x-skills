package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/InkyQuill/x-skills/internal/remote"
	"github.com/spf13/cobra"
)

type searchOptions struct {
	owner    string
	limit    int
	endpoint string
}

type searchJSONResult struct {
	remote.SearchResult
	AddCommand string `json:"add_command"`
}

type searchJSONPayload struct {
	Query   string             `json:"query"`
	Owner   string             `json:"owner,omitempty"`
	Results []searchJSONResult `json:"results"`
}

func newSearchCommand(rootOptions *options) *cobra.Command {
	var opts searchOptions
	cmd := &cobra.Command{
		Use:   "search QUERY",
		Short: "Search skills.sh for remote skills",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSearch(cmd, rootOptions, opts, args[0])
		},
	}
	cmd.Flags().StringVar(&opts.owner, "owner", "", "filter by GitHub owner")
	cmd.Flags().IntVar(&opts.limit, "limit", remote.DefaultSearchLimit, "maximum number of results")
	cmd.Flags().StringVar(&opts.endpoint, "endpoint", "", "skills.sh search endpoint")
	if err := cmd.Flags().MarkHidden("endpoint"); err != nil {
		panic(err)
	}
	return cmd
}

func runSearch(cmd *cobra.Command, rootOptions *options, opts searchOptions, query string) error {
	client := remote.NewSearchClient(opts.endpoint, &http.Client{Timeout: 15 * time.Second})
	results, err := client.Search(cmd.Context(), remote.SearchRequest{
		Query: query,
		Owner: opts.owner,
		Limit: opts.limit,
	})
	if err != nil {
		return err
	}
	if rootOptions.json {
		return writeSearchJSON(cmd.OutOrStdout(), query, opts.owner, results)
	}
	return writeSearchHuman(cmd.OutOrStdout(), results)
}

func writeSearchHuman(out io.Writer, results []remote.SearchResult) error {
	if len(results) == 0 {
		_, err := fmt.Fprintln(out, "no skills found")
		return err
	}
	for _, result := range results {
		source := result.Source()
		if source == "" {
			source = "unknown source"
		}
		if _, err := fmt.Fprintf(
			out,
			"%s  %s  %d installs\n",
			result.Name,
			source,
			result.Installs,
		); err != nil {
			return err
		}
		if desc := strings.TrimSpace(result.Description); desc != "" {
			if _, err := fmt.Fprintf(out, "  %s\n", desc); err != nil {
				return err
			}
		}
		if addCmd := searchAddCommand(result); addCmd != "" {
			if _, err := fmt.Fprintf(out, "  %s\n", addCmd); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeSearchJSON(out io.Writer, query, owner string, results []remote.SearchResult) error {
	payload := searchJSONPayload{
		Query:   strings.TrimSpace(query),
		Owner:   strings.TrimSpace(owner),
		Results: make([]searchJSONResult, 0, len(results)),
	}
	for _, result := range results {
		payload.Results = append(payload.Results, searchJSONResult{
			SearchResult: result,
			AddCommand:   searchAddCommand(result),
		})
	}
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(payload)
}

func searchAddCommand(result remote.SearchResult) string {
	source := result.Source()
	if source == "" || strings.TrimSpace(result.Name) == "" {
		return ""
	}
	return "x-skills add " + source + "@" + result.Name
}
