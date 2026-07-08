package remote

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSearchRejectsShortQuery(t *testing.T) {
	client := NewSearchClient("https://skills.sh/api/search", http.DefaultClient)
	_, err := client.Search(t.Context(), SearchRequest{Query: "s", Limit: 50})
	if err == nil {
		t.Fatal("expected short query error")
	}
}

func TestSearchRequestShapeAndResponse(t *testing.T) {
	var gotPath string
	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{{
				"name":        "svelte-coder",
				"description": "Svelte help.",
				"owner":       "vercel-labs",
				"repo":        "skills",
				"path":        "skills/svelte-coder",
				"installs":    812,
				"audit": map[string]any{
					"available": true,
					"alerts":    2,
				},
			}},
		})
	}))
	defer server.Close()

	client := NewSearchClient(server.URL, server.Client())
	results, err := client.Search(t.Context(), SearchRequest{Query: "svelte", Owner: "vercel-labs", Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/" {
		t.Fatalf("path = %q, want /", gotPath)
	}
	for _, want := range []string{"q=svelte", "owner=vercel-labs", "limit=50"} {
		if !strings.Contains(gotQuery, want) {
			t.Fatalf("query %q missing %q", gotQuery, want)
		}
	}
	if len(results) != 1 || results[0].Name != "svelte-coder" || results[0].Source() != "vercel-labs/skills" {
		t.Fatalf("results = %#v", results)
	}
	if results[0].Audit == nil || !results[0].Audit.Available || results[0].Audit.Alerts != 2 {
		t.Fatalf("audit = %#v", results[0].Audit)
	}
}
