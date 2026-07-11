package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListRootsShowsConfiguredRoots(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	configPath := filepath.Join(home, ".x-skills", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("active_roots:\n  - scope: project\n    target: opencode\n    path: .opencode/skills\n    label: .Oc\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	err := Execute([]string{"--home", home, "--project-root", project, "list-roots"}, strings.NewReader(""), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	output := out.String()
	if !strings.Contains(output, ".Oc") ||
		!strings.Contains(output, "project:opencode") ||
		!strings.Contains(output, filepath.Join(project, ".opencode", "skills")) {
		t.Fatalf("output missing configured root:\n%s", output)
	}
}

func TestListRootsJSON(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	var out bytes.Buffer
	err := Execute([]string{"--home", home, "--project-root", project, "--json", "list-roots"}, strings.NewReader(""), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Roots []struct {
			Location  string   `json:"location"`
			Scope     string   `json:"scope"`
			Target    string   `json:"target"`
			Label     string   `json:"label"`
			Path      string   `json:"path"`
			Consumers []string `json:"consumers"`
			Builtin   bool     `json:"builtin"`
			Enabled   bool     `json:"enabled"`
		} `json:"roots"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("decode json: %v\n%s", err, out.String())
	}
	if len(payload.Roots) != 6 {
		t.Fatalf("len(roots) = %d, want 6", len(payload.Roots))
	}
	if payload.Roots[0].Location == "" || payload.Roots[0].Path == "" {
		t.Fatalf("first root = %#v", payload.Roots[0])
	}
	if len(payload.Roots[0].Consumers) == 0 {
		t.Fatalf("first root consumers = %#v, want configured consumer ids", payload.Roots[0].Consumers)
	}
}

func TestListRootsJSONAfterSubcommand(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	var out bytes.Buffer
	err := Execute([]string{"--home", home, "--project-root", project, "list-roots", "--json"}, strings.NewReader(""), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Roots []struct {
			Location string `json:"location"`
			Path     string `json:"path"`
		} `json:"roots"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("decode json: %v\n%s", err, out.String())
	}
	if len(payload.Roots) != 6 {
		t.Fatalf("len(roots) = %d, want 6", len(payload.Roots))
	}
	if payload.Roots[0].Location == "" || payload.Roots[0].Path == "" {
		t.Fatalf("first root = %#v", payload.Roots[0])
	}
}

func TestListRootsRejectsInvalidConfig(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	configPath := filepath.Join(home, ".x-skills", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("version: 99\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	err := Execute([]string{"--home", home, "--project-root", project, "list-roots"}, strings.NewReader(""), &out, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "unsupported version 99") {
		t.Fatalf("err = %v, want unsupported version", err)
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", out.String())
	}
}
