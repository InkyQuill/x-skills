package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type listRecord struct {
	Identity     string `json:"identity"`
	DeclaredName string `json:"declared_name,omitempty"`
	Description  string `json:"description,omitempty"`
	Status       string `json:"status"`
	Path         string `json:"path"`
	Reason       string `json:"reason,omitempty"`
	Root         struct {
		Scope  string `json:"scope"`
		Target string `json:"target"`
		Label  string `json:"label"`
		Path   string `json:"path"`
	} `json:"root"`
}

func TestListShowsStatuses(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	archive := filepath.Join(home, ".x-skills", "skills")
	managed := makeSkill(t, archive, "managed-codex", "Managed codex skill.")
	root := filepath.Join(project, ".codex", "skills")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(managed, filepath.Join(root, "managed-codex")); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	err := Execute([]string{
		"--project-root", project,
		"--home", home,
		"list", "--at", "project:codex",
	}, strings.NewReader(""), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{"PROJECT codex", ".Cd", "managed-codex", "managed", "Managed codex skill."} {
		if !strings.Contains(text, want) {
			t.Fatalf("list output missing %q:\n%s", want, text)
		}
	}
}

func TestListShowsDeclaredNameOnlyWhenDifferent(t *testing.T) {
	home, project := t.TempDir(), t.TempDir()
	cfg := setupActiveIdentityMismatch(t, home, project)
	matching := makeSkill(t, cfg.ArchiveSkillsRoot(), "matching", "Matching.")
	if err := os.Symlink(
		matching,
		filepath.Join(cfg.MustActiveRoot("project", "agents"), "matching"),
	); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	err := Execute(
		[]string{"--home", home, "--project-root", project, "list", "--at", "project:agents"},
		strings.NewReader(""),
		&out,
		&bytes.Buffer{},
	)
	if err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, "composition-patterns (declared: vercel-composition-patterns)") {
		t.Fatalf("list output missing divergent declared name:\n%s", text)
	}
	if strings.Contains(text, "matching (declared:") {
		t.Fatalf("list output repeats matching declared name:\n%s", text)
	}
}

func TestListJSON(t *testing.T) {
	home, project := t.TempDir(), t.TempDir()
	setupActiveIdentityMismatch(t, home, project)

	var out bytes.Buffer
	err := Execute(
		[]string{"--home", home, "--project-root", project, "--json", "list", "--at", "project:agents"},
		strings.NewReader(""),
		&out,
		&bytes.Buffer{},
	)
	if err != nil {
		t.Fatal(err)
	}
	var records []listRecord
	if err := json.Unmarshal(out.Bytes(), &records); err != nil {
		t.Fatalf("unmarshal list JSON: %v\n%s", err, out.String())
	}
	if len(records) != 1 {
		t.Fatalf("records = %#v, want one", records)
	}
	record := records[0]
	if record.Identity != "composition-patterns" || record.DeclaredName != "vercel-composition-patterns" {
		t.Fatalf("identity fields = %q, %q", record.Identity, record.DeclaredName)
	}
	if record.Description != "Compose." || record.Status != "managed" {
		t.Fatalf("record = %#v", record)
	}
	if record.Path == "" || record.Root.Scope != "project" || record.Root.Target != "agents" ||
		record.Root.Label == "" || record.Root.Path == "" {
		t.Fatalf("record paths/root = %#v", record)
	}
	if strings.Contains(out.String(), "\x1b[") {
		t.Fatalf("list JSON contains ANSI styling: %q", out.String())
	}
}

func TestListJSONOmitsMatchingDeclaredName(t *testing.T) {
	home, project := t.TempDir(), t.TempDir()
	archive := makeSkill(t, filepath.Join(home, ".x-skills", "skills"), "matching", "Matching.")
	root := filepath.Join(project, ".agents", "skills")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(archive, filepath.Join(root, "matching")); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	err := Execute(
		[]string{"--home", home, "--project-root", project, "--json", "list", "--at", "project:agents"},
		strings.NewReader(""),
		&out,
		&bytes.Buffer{},
	)
	if err != nil {
		t.Fatal(err)
	}
	var raw []map[string]any
	if err := json.Unmarshal(out.Bytes(), &raw); err != nil {
		t.Fatalf("unmarshal list JSON: %v\n%s", err, out.String())
	}
	if len(raw) != 1 {
		t.Fatalf("records = %#v, want one", raw)
	}
	if _, ok := raw[0]["declared_name"]; ok {
		t.Fatalf("matching declared_name present: %#v", raw[0])
	}
}

func TestListJSONEmptyArray(t *testing.T) {
	var out bytes.Buffer
	err := Execute(
		[]string{"--home", t.TempDir(), "--project-root", t.TempDir(), "--json", "list"},
		strings.NewReader(""),
		&out,
		&bytes.Buffer{},
	)
	if err != nil {
		t.Fatal(err)
	}
	var records []listRecord
	if err := json.Unmarshal(out.Bytes(), &records); err != nil {
		t.Fatalf("unmarshal list JSON: %v\n%s", err, out.String())
	}
	if records == nil || len(records) != 0 {
		t.Fatalf("records = %#v, want non-nil empty slice", records)
	}
}

func TestListRejectsUnexpectedArgs(t *testing.T) {
	var out bytes.Buffer
	var stderr bytes.Buffer
	err := Execute([]string{"list", "unexpected"}, strings.NewReader(""), &out, &stderr)
	if err == nil {
		t.Fatal("expected error")
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", out.String())
	}
}

func TestListRejectsInvalidGlobalConfig(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	configPath := filepath.Join(home, ".x-skills", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("version: 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	var stderr bytes.Buffer
	err := Execute([]string{"--home", home, "--project-root", project, "list"}, strings.NewReader(""), &out, &stderr)
	if err == nil || !strings.Contains(err.Error(), "unsupported version 0") {
		t.Fatalf("err = %v, want unsupported version 0", err)
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", out.String())
	}
}

func TestListRejectsUnknownLocation(t *testing.T) {
	var out bytes.Buffer
	var stderr bytes.Buffer
	err := Execute([]string{"list", "--at", "project:bogus"}, strings.NewReader(""), &out, &stderr)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "unknown --at location") {
		t.Fatalf("error = %q, want unknown --at location", err)
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", out.String())
	}
}
