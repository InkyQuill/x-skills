package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSearchPrintsResultsAndAddHints(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("q"); got != "next" {
			t.Fatalf("q = %q, want next", got)
		}
		if got := r.URL.Query().Get("limit"); got != "20" {
			t.Fatalf("limit = %q, want 20", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{{
				"name":        "next-best-practices",
				"description": "Next.js guidance.",
				"owner":       "vercel-labs",
				"repo":        "skills",
				"path":        "skills/next-best-practices",
				"installs":    42,
			}},
		})
	}))
	defer server.Close()

	var out bytes.Buffer
	err := Execute([]string{
		"search", "next",
		"--endpoint", server.URL,
		"--limit", "20",
	}, strings.NewReader(""), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}

	output := out.String()
	for _, want := range []string{
		"next-best-practices  vercel-labs/skills  42 installs",
		"Next.js guidance.",
		"x-skills add vercel-labs/skills@next-best-practices",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q:\n%s", want, output)
		}
	}
}

func TestSearchJSONPrintsResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{{
				"name":        "svelte-coder",
				"description": "Svelte help.",
				"owner":       "vercel-labs",
				"repo":        "skills",
				"path":        "skills/svelte-coder",
				"installs":    812,
			}},
		})
	}))
	defer server.Close()

	var out bytes.Buffer
	err := Execute([]string{
		"--json",
		"search", "svelte",
		"--endpoint", server.URL,
	}, strings.NewReader(""), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}

	var payload struct {
		Query   string `json:"query"`
		Results []struct {
			Name   string `json:"name"`
			Owner  string `json:"owner"`
			Repo   string `json:"repo"`
			AddCmd string `json:"add_command"`
		} `json:"results"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("decode json: %v\n%s", err, out.String())
	}
	if payload.Query != "svelte" || len(payload.Results) != 1 {
		t.Fatalf("payload = %#v", payload)
	}
	result := payload.Results[0]
	if result.Name != "svelte-coder" ||
		result.Owner != "vercel-labs" ||
		result.Repo != "skills" ||
		result.AddCmd != "x-skills add vercel-labs/skills@svelte-coder" {
		t.Fatalf("result = %#v", result)
	}
}

func TestSearchRequiresQuery(t *testing.T) {
	err := Execute([]string{"search"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "accepts 1 arg") {
		t.Fatalf("err = %v, want arg validation", err)
	}
}
