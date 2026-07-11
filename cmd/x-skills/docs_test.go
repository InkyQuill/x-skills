package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDocumentationDescribesSupportedDistribution(t *testing.T) {
	t.Parallel()

	repoRoot := filepath.Join("..", "..")
	readFile := func(t *testing.T, name string) string {
		t.Helper()

		content, err := os.ReadFile(filepath.Join(repoRoot, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		return string(content)
	}

	readme := readFile(t, "README.md")
	for _, required := range []string{
		"Go implementation",
		"mkdir -p ~/bin",
		"go build -o ~/bin/x-skills ./cmd/x-skills",
		"go run ./cmd/x-skills list",
		"x-skills add owner/repo@skill",
	} {
		if !strings.Contains(readme, required) {
			t.Errorf("README.md must contain %q", required)
		}
	}

	maintainedDocs := map[string]string{
		"CONTEXT.md":            readFile(t, "CONTEXT.md"),
		"docs/cli.md":           readFile(t, "docs/cli.md"),
		"docs/tui.md":           readFile(t, "docs/tui.md"),
		"docs/remote-skills.md": readFile(t, "docs/remote-skills.md"),
	}
	for _, link := range []string{
		"[CLI guide](docs/cli.md)",
		"[TUI guide](docs/tui.md)",
		"[Remote skills guide](docs/remote-skills.md)",
	} {
		if !strings.Contains(readme, link) {
			t.Errorf("README.md must link to %q", link)
		}
	}
	allMaintainedDocs := strings.Join([]string{
		maintainedDocs["CONTEXT.md"],
		maintainedDocs["docs/cli.md"],
		maintainedDocs["docs/tui.md"],
		maintainedDocs["docs/remote-skills.md"],
	}, "\n")
	for _, concept := range []string{
		"managed skill",
		"skills folder",
		".x-skills.yaml",
		".x-skills.local.yaml",
		"--at",
		"current-page selection",
		"source identity",
		"archive state",
	} {
		if !strings.Contains(strings.ToLower(allMaintainedDocs), strings.ToLower(concept)) {
			t.Errorf("maintained documentation must describe %q", concept)
		}
	}

	llms := readFile(t, "llms.txt")
	for _, required := range []string{
		"mkdir -p bin",
		"go build -o bin/x-skills ./cmd/x-skills",
		"x-skills add <source>",
	} {
		if !strings.Contains(llms, required) {
			t.Errorf("llms.txt must contain %q", required)
		}
	}

	for _, name := range []string{
		"install.sh",
		"pyproject.toml",
		"uv.lock",
		filepath.Join("tests", "test_cli.py"),
		filepath.Join("tests", "test_interactive.py"),
		filepath.Join("tests", "test_install_docs.py"),
		filepath.Join("src", "x_skills"),
	} {
		if _, err := os.Stat(filepath.Join(repoRoot, name)); !os.IsNotExist(err) {
			t.Errorf("retired distribution artifact %s still exists", name)
		}
	}

	liveDocs := map[string]string{
		"README.md": readme,
		"llms.txt":  llms,
	}
	for name, content := range liveDocs {
		for _, retired := range []string{
			"install.sh",
			"go install github.com/InkyQuill/x-skills@latest",
			"add-github",
			"add-url",
			"repo add-github",
			"repo add-url",
			"Python/Textual source remains",
			"historical, non-distributed Python prototype",
			"uv tool install",
		} {
			if strings.Contains(content, retired) {
				t.Errorf("%s contains retired distribution token %q", name, retired)
			}
		}
	}
}
