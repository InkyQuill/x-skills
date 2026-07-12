package remote

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const DefaultSearchEndpoint = "https://skills.sh/api/search"
const DefaultSearchLimit = 50

type SearchRequest struct {
	Query string
	Owner string
	Limit int
}

type SearchResult struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	ID          string        `json:"id,omitempty"`
	SourceSlug  string        `json:"source,omitempty"`
	Owner       string        `json:"owner"`
	Repo        string        `json:"repo"`
	Path        string        `json:"path"`
	Ref         string        `json:"ref,omitempty"`
	Installs    int           `json:"installs"`
	Audit       *AuditSummary `json:"audit,omitempty"`
}

func (r SearchResult) Source() string {
	if r.Owner == "" || r.Repo == "" {
		return ""
	}
	return r.Owner + "/" + r.Repo
}

type SearchClient struct {
	endpoint string
	http     *http.Client
}

func NewSearchClient(endpoint string, httpClient *http.Client) SearchClient {
	if endpoint == "" {
		endpoint = DefaultSearchEndpoint
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return SearchClient{endpoint: endpoint, http: httpClient}
}

func (c SearchClient) Search(ctx context.Context, req SearchRequest) (results []SearchResult, err error) {
	query := strings.TrimSpace(req.Query)
	if len([]rune(query)) < 2 {
		return nil, fmt.Errorf("search query must be at least 2 characters")
	}
	limit := req.Limit
	if limit <= 0 || limit > DefaultSearchLimit {
		limit = DefaultSearchLimit
	}
	u, err := url.Parse(c.endpoint)
	if err != nil {
		return nil, fmt.Errorf("parse search endpoint: %w", err)
	}
	values := u.Query()
	values.Set("q", query)
	values.Set("limit", strconv.Itoa(limit))
	if owner := strings.TrimSpace(req.Owner); owner != "" {
		values.Set("owner", owner)
	}
	u.RawQuery = values.Encode()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create search request: %w", err)
	}
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("search skills: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close search response: %w", closeErr))
		}
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("search skills: HTTP %d", resp.StatusCode)
	}
	var payload struct {
		Results []SearchResult `json:"results"`
		Skills  []SearchResult `json:"skills"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode search results: %w", err)
	}
	if len(payload.Skills) > 0 {
		results = make([]SearchResult, 0, len(payload.Skills))
		for _, result := range payload.Skills {
			normalized, ok := normalizeSkillsAPIResult(result)
			if ok {
				results = append(results, normalized)
			}
		}
		return results, nil
	}
	return payload.Results, nil
}

func normalizeSkillsAPIResult(result SearchResult) (SearchResult, bool) {
	result.Name = strings.TrimSpace(result.Name)
	result.ID = strings.TrimSpace(result.ID)
	if result.Name == "" || result.ID == "" {
		return SearchResult{}, false
	}
	source := strings.TrimSpace(result.SourceSlug)
	if source == "" {
		source = strings.TrimSpace(result.ID)
		if slash := strings.LastIndex(source, "/"); slash >= 0 {
			source = source[:slash]
		}
	}
	parts := strings.SplitN(source, "/", 2)
	if len(parts) == 2 {
		result.Owner = parts[0]
		result.Repo = parts[1]
	}
	if result.Path == "" && source != "" {
		result.Path = strings.TrimPrefix(result.ID, source+"/")
	}
	return result, true
}
