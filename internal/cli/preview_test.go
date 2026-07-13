package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestFirstLinesReturnsRawPrefix(t *testing.T) {
	content := []byte("one\ntwo\nthree\n")
	got, returned, truncated := firstLines(content, 2)
	if string(got) != "one\ntwo\n" {
		t.Fatalf("prefix = %q, want %q", got, "one\ntwo\n")
	}
	if returned != 2 {
		t.Fatalf("returned = %d, want 2", returned)
	}
	if !truncated {
		t.Fatal("truncated = false, want true")
	}
}

func TestFirstLinesHandlesLineEndingsAndLimits(t *testing.T) {
	tests := []struct {
		name              string
		content           string
		limit             int
		expected          string
		expectedReturned  int
		expectedTruncated bool
	}{
		{
			name:              "short final newline",
			content:           "one\ntwo\n",
			limit:             50,
			expected:          "one\ntwo\n",
			expectedReturned:  2,
			expectedTruncated: false,
		},
		{
			name:              "short without final newline",
			content:           "one\ntwo",
			limit:             50,
			expected:          "one\ntwo",
			expectedReturned:  2,
			expectedTruncated: false,
		},
		{
			name:              "exact limit final newline",
			content:           "one\ntwo\n",
			limit:             2,
			expected:          "one\ntwo\n",
			expectedReturned:  2,
			expectedTruncated: false,
		},
		{
			name:              "exact limit without final newline",
			content:           "one\ntwo",
			limit:             2,
			expected:          "one\ntwo",
			expectedReturned:  2,
			expectedTruncated: false,
		},
		{
			name:              "empty content",
			content:           "",
			limit:             1,
			expected:          "",
			expectedReturned:  0,
			expectedTruncated: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, returned, truncated := firstLines([]byte(tt.content), tt.limit)
			if string(got) != tt.expected {
				t.Fatalf("prefix = %q, want %q", got, tt.expected)
			}
			if returned != tt.expectedReturned {
				t.Fatalf("returned = %d, want %d", returned, tt.expectedReturned)
			}
			if truncated != tt.expectedTruncated {
				t.Fatalf("truncated = %t, want %t", truncated, tt.expectedTruncated)
			}
		})
	}
}

func TestPreviewDefaultsToFiftyRawLines(t *testing.T) {
	var document strings.Builder
	document.WriteString("---\nname: preview-skill\ndescription: Raw.\n---\n")
	for i := 1; i <= 52; i++ {
		fmt.Fprintf(&document, "line %02d\n", i)
	}
	content := []byte(document.String())
	expected, _, _ := expectedFirstLines(content, 50)

	out, err := executePreview(t, content, "preview", "owner/repo", "preview-skill")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, expected) {
		t.Fatalf("stdout = %q, want raw prefix %q", out, expected)
	}
	for _, unwanted := range [][]byte{
		[]byte("Preview:"),
		[]byte("truncated"),
		[]byte("\x1b["),
	} {
		if bytes.Contains(out, unwanted) {
			t.Fatalf("stdout contains synthetic or styled output %q: %q", unwanted, out)
		}
	}
}

func TestPreviewLinesOneAndTerminalCleanNewline(t *testing.T) {
	content := []byte("---\nname: preview-skill\n---\nbody without final newline")
	out, err := executePreview(
		t,
		content,
		"preview", "owner/repo", "preview-skill", "--lines", "1",
	)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "---\n" {
		t.Fatalf("stdout = %q, want first raw line", out)
	}

	out, err = executePreview(
		t,
		content,
		"preview", "owner/repo", "preview-skill", "--lines", "50",
	)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != string(content)+"\n" {
		t.Fatalf("stdout = %q, want content plus terminal-clean newline", out)
	}
}

func TestPreviewRejectsNonPositiveLinesWithoutOutput(t *testing.T) {
	content := []byte("---\nname: preview-skill\n---\nbody\n")
	for _, value := range []string{"0", "-1"} {
		t.Run(value, func(t *testing.T) {
			out, err := executePreview(
				t,
				content,
				"preview", "owner/repo", "preview-skill", "--lines", value,
			)
			if err == nil || !strings.Contains(err.Error(), "--lines must be positive") {
				t.Fatalf("error = %v, want positive-lines validation", err)
			}
			if len(out) != 0 {
				t.Fatalf("stdout = %q, want empty", out)
			}
		})
	}
}

func TestPreviewJSONContainsExactFieldsAndRawContent(t *testing.T) {
	content := []byte("---\nname: preview-skill\n---\nbody without final newline")
	out, err := executePreview(
		t,
		content,
		"--json", "preview", "owner/repo", "preview-skill", "--lines", "2",
	)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("decode JSON: %v\n%s", err, out)
	}
	if len(payload) != 8 {
		t.Fatalf("JSON fields = %#v, want exactly 8 fields", payload)
	}
	expected := map[string]any{
		"repository":      "owner/repo",
		"requested_skill": "preview-skill",
		"skill_path":      "skills/preview-skill/SKILL.md",
		"content":         "---\nname: preview-skill\n",
		"returned_lines":  float64(2),
		"requested_lines": float64(2),
		"truncated":       true,
	}
	for key, value := range expected {
		if payload[key] != value {
			t.Errorf("%s = %#v, want %#v", key, payload[key], value)
		}
	}
	commit, ok := payload["commit"].(string)
	if !ok || commit == "" {
		t.Errorf("commit = %#v, want non-empty string", payload["commit"])
	}
	if bytes.Contains(out, []byte("\x1b[")) {
		t.Fatalf("JSON contains ANSI styling: %q", out)
	}
}

func TestPreviewResolverErrorWritesNoPartialOutput(t *testing.T) {
	content := []byte("---\nname: other-skill\n---\nbody\n")
	out, err := executePreview(t, content, "preview", "owner/repo", "missing-skill")
	if err == nil {
		t.Fatal("preview returned nil error for missing skill")
	}
	if len(out) != 0 {
		t.Fatalf("stdout = %q, want empty", out)
	}
}

func expectedFirstLines(content []byte, limit int) ([]byte, int, bool) {
	lineEnd := 0
	for i, b := range content {
		if b != '\n' {
			continue
		}
		lineEnd++
		if lineEnd == limit {
			return content[:i+1], lineEnd, i+1 < len(content)
		}
	}
	returned := lineEnd
	if len(content) > 0 && content[len(content)-1] != '\n' {
		returned++
	}
	return content, returned, false
}

func executePreview(t *testing.T, content []byte, args ...string) ([]byte, error) {
	t.Helper()
	gitHome := t.TempDir()
	t.Setenv("HOME", gitHome)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(gitHome, ".config"))
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(gitHome, ".gitconfig"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_ALLOW_PROTOCOL", "file")

	repo := t.TempDir()
	runPreviewGit(t, repo, "init")
	runPreviewGit(t, repo, "config", "user.email", "test@example.com")
	runPreviewGit(t, repo, "config", "user.name", "Test")
	dir := filepath.Join(repo, "skills", "preview-skill")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), content, 0o644); err != nil {
		t.Fatal(err)
	}
	runPreviewGit(t, repo, "add", ".")
	runPreviewGit(t, repo, "commit", "-m", "initial")
	localRepoURL := "file://" + filepath.ToSlash(repo)
	runPreviewGit(
		t,
		"",
		"config",
		"--global",
		"url."+localRepoURL+".insteadOf",
		"https://github.com/owner/repo.git",
	)

	rootArgs := []string{"--home", t.TempDir(), "--project-root", t.TempDir()}
	rootArgs = append(rootArgs, args...)
	var stdout bytes.Buffer
	err := Execute(rootArgs, strings.NewReader(""), &stdout, &bytes.Buffer{})
	return stdout.Bytes(), err
}

func runPreviewGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}
