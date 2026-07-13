package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
		"curl -fsSL https://raw.githubusercontent.com/InkyQuill/x-skills/main/scripts/install.sh | sh",
		"irm https://raw.githubusercontent.com/InkyQuill/x-skills/main/scripts/install.ps1 | iex",
		"mkdir -p ~/bin",
		"go build -o ~/bin/x-skills ./cmd/x-skills",
		"go run ./cmd/x-skills list",
		"x-skills add owner/repo@skill",
	} {
		if !strings.Contains(readme, required) {
			t.Errorf("README.md must contain %q", required)
		}
	}
	contributing := readFile(t, "CONTRIBUTING.md")
	for name, content := range map[string]string{
		"README.md":       readme,
		"CONTRIBUTING.md": contributing,
	} {
		for _, required := range []string{
			"scripts/install-dev.sh",
			"scripts/install-dev.ps1",
			"overwrite the normal installation",
			"reports `dev`",
		} {
			if !strings.Contains(content, required) {
				t.Errorf("%s must contain %q", name, required)
			}
		}
		if strings.Contains(content, "X_SKILLS_DOWNLOAD_URL") {
			t.Errorf("%s must not document the test-only download URL override", name)
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

func TestReleaseAndInstallerConfiguration(t *testing.T) {
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

	for _, name := range []string{
		filepath.Join("scripts", "install.sh"),
		filepath.Join("scripts", "install.ps1"),
		filepath.Join("scripts", "install-dev.sh"),
		filepath.Join("scripts", "install-dev.ps1"),
		".goreleaser.yaml",
		"release.config.cjs",
		filepath.Join(".github", "workflows", "ci.yml"),
		filepath.Join(".github", "workflows", "release.yml"),
	} {
		if _, err := os.Stat(filepath.Join(repoRoot, name)); err != nil {
			t.Fatalf("expected release artifact %s to exist: %v", name, err)
		}
	}

	for _, name := range []string{"install.sh", "install-dev.sh"} {
		installSH := readFile(t, filepath.Join("scripts", name))
		for _, required := range []string{
			"install_xs_link",
			"command -v xs",
			"existing $BIN_NAME found",
			"replacing it",
			"mv -f",
			"X_SKILLS_INSTALL_DIR",
			"github.com/InkyQuill/x-skills/internal/buildinfo.version",
		} {
			if !strings.Contains(installSH, required) {
				t.Errorf("scripts/%s must contain %q", name, required)
			}
		}
	}

	installSH := readFile(t, filepath.Join("scripts", "install.sh"))
	for _, required := range []string{
		"REPO=\"InkyQuill/x-skills\"",
		"tar -xzf",
		"https://github.com/${REPO}/releases/download/${version}/${asset}",
	} {
		if !strings.Contains(installSH, required) {
			t.Errorf("scripts/install.sh must contain %q", required)
		}
	}
	if installDevSH := readFile(t, filepath.Join("scripts", "install-dev.sh")); !strings.Contains(installDevSH, "version=dev") {
		t.Error("scripts/install-dev.sh must inject literal dev")
	}

	for _, name := range []string{"install.ps1", "install-dev.ps1"} {
		installPS := readFile(t, filepath.Join("scripts", name))
		for _, required := range []string{
			"Install-XsShortcut",
			"Get-Command xs",
			"existing $BinName found",
			"replacing it",
			"[System.IO.File]::Replace",
			"$backupExe",
			"Remove-Item -Force $backupExe",
			"close any running x-skills process and retry",
			"X_SKILLS_INSTALL_DIR",
			"github.com/InkyQuill/x-skills/internal/buildinfo.version",
		} {
			if !strings.Contains(installPS, required) {
				t.Errorf("scripts/%s must contain %q", name, required)
			}
		}
		if strings.Contains(installPS, "[System.IO.File]::Replace($stagedExe, $installedExe, $null)") {
			t.Errorf("scripts/%s must pass a legal backup path to File.Replace", name)
		}
	}

	installPS := readFile(t, filepath.Join("scripts", "install.ps1"))
	for _, required := range []string{
		"$Repo = \"InkyQuill/x-skills\"",
		"Expand-Archive",
		"https://github.com/$Repo/releases/download/$Version/$asset",
	} {
		if !strings.Contains(installPS, required) {
			t.Errorf("scripts/install.ps1 must contain %q", required)
		}
	}
	if installDevPS := readFile(t, filepath.Join("scripts", "install-dev.ps1")); !strings.Contains(installDevPS, "version=dev") {
		t.Error("scripts/install-dev.ps1 must inject literal dev")
	}

	goreleaser := readFile(t, ".goreleaser.yaml")
	for _, required := range []string{
		"goos:",
		"darwin",
		"linux",
		"windows",
		"goarch:",
		"amd64",
		"arm64",
		"- zip",
		"- tar.gz",
		"-X github.com/InkyQuill/x-skills/internal/buildinfo.version={{ .Version }}",
		"extra_files:",
		"glob: scripts/install.sh",
		"glob: scripts/install.ps1",
	} {
		if !strings.Contains(goreleaser, required) {
			t.Errorf(".goreleaser.yaml must contain %q", required)
		}
	}
	if strings.Contains(goreleaser, "changelog:\n  disable: true") {
		t.Error(".goreleaser.yaml must let GoReleaser generate the release changelog")
	}

	releaseConfig := readFile(t, "release.config.cjs")
	for _, required := range []string{
		"@semantic-release/commit-analyzer",
		"@semantic-release/exec",
		"publishCmd",
		"goreleaser release --clean",
	} {
		if !strings.Contains(releaseConfig, required) {
			t.Errorf("release.config.cjs must contain %q", required)
		}
	}
	if strings.Contains(releaseConfig, "@semantic-release/github") {
		t.Error("release.config.cjs must delegate release publication exclusively to GoReleaser")
	}
	if strings.Contains(releaseConfig, "@semantic-release/release-notes-generator") {
		t.Error("release.config.cjs must leave release notes to GoReleaser")
	}

	workflow := readFile(t, filepath.Join(".github", "workflows", "release.yml"))
	for _, required := range []string{
		"version: v2.17.0",
		"semantic-release@25.0.7",
		"@semantic-release/commit-analyzer@13.0.1",
		"@semantic-release/exec@7.1.0",
	} {
		if !strings.Contains(workflow, required) {
			t.Errorf("release workflow must pin %q", required)
		}
	}
	if err := validateReleaseWorkflow(workflow); err != nil {
		t.Error(err)
	}
	crlfWorkflow := strings.ReplaceAll(normalizeLineEndings(workflow), "\n", "\r\n")
	if err := validateReleaseWorkflow(crlfWorkflow); err != nil {
		t.Errorf("validate CRLF release workflow: %v", err)
	}
	invalidWorkflows := []struct {
		name    string
		content string
	}{
		{
			name: "wrong action major",
			content: strings.Replace(
				workflow,
				"goreleaser/goreleaser-action@v7",
				"goreleaser/goreleaser-action@v6",
				1,
			),
		},
		{
			name:    "semantic-release before GoReleaser",
			content: "          semantic-release\n" + workflow,
		},
	}
	for _, invalid := range invalidWorkflows {
		t.Run(invalid.name, func(t *testing.T) {
			if err := validateReleaseWorkflow(invalid.content); err == nil {
				t.Error("release workflow validation accepted invalid ordering or action version")
			}
		})
	}
	if strings.Contains(workflow, "release --snapshot") {
		t.Error("release workflow must not build and discard snapshot artifacts")
	}
}

func validateReleaseWorkflow(workflow string) error {
	workflow = normalizeLineEndings(workflow)
	actionIndex := strings.Index(workflow, "uses: goreleaser/goreleaser-action@v7")
	if actionIndex == -1 {
		return fmt.Errorf("release workflow must use goreleaser/goreleaser-action@v7")
	}
	installOnlyIndex := strings.Index(workflow, "install-only: true")
	if installOnlyIndex == -1 {
		return fmt.Errorf("release workflow must install GoReleaser without publishing")
	}
	semanticReleaseIndex := strings.Index(workflow, "          semantic-release\n")
	if semanticReleaseIndex == -1 {
		return fmt.Errorf("release workflow must invoke semantic-release")
	}
	if actionIndex >= installOnlyIndex || installOnlyIndex >= semanticReleaseIndex {
		return fmt.Errorf("release workflow must install GoReleaser before invoking semantic-release")
	}
	return nil
}

func normalizeLineEndings(value string) string {
	return strings.ReplaceAll(value, "\r\n", "\n")
}

func TestDevelopmentInstallerReplacesExistingBinary(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	installDir := t.TempDir()
	env := withEnv(os.Environ(), "X_SKILLS_INSTALL_DIR", installDir)
	shortcut := filepath.Join(installDir, "xs")
	if runtime.GOOS == "windows" {
		shortcut += ".cmd"
	}
	const shortcutContent = "occupied shortcut"
	if err := os.WriteFile(shortcut, []byte(shortcutContent), 0o755); err != nil {
		t.Fatalf("write occupied shortcut: %v", err)
	}

	var command func() *exec.Cmd
	var installed string
	if runtime.GOOS == "windows" {
		script := filepath.Join(repoRoot, "scripts", "install-dev.ps1")
		command = func() *exec.Cmd {
			return exec.Command(
				"powershell.exe",
				"-NoProfile",
				"-ExecutionPolicy",
				"Bypass",
				"-File",
				script,
			)
		}
		installed = filepath.Join(installDir, "x-skills.exe")
	} else {
		script := filepath.Join(repoRoot, "scripts", "install-dev.sh")
		command = func() *exec.Cmd { return exec.Command("sh", script) }
		installed = filepath.Join(installDir, "x-skills")
	}

	runCommand(t, command(), env)
	secondOutput := runCommand(t, command(), env)
	for _, required := range []string{"existing x-skills found at", "replacing it"} {
		if !strings.Contains(secondOutput, required) {
			t.Errorf("second development install output must contain %q; output:\n%s", required, secondOutput)
		}
	}

	versionOutput := runCommand(t, exec.Command(installed, "version"), env)
	if got := strings.TrimSpace(versionOutput); got != "dev" {
		t.Fatalf("installed development version = %q, want dev", got)
	}
	assertFileContent(t, shortcut, shortcutContent)
}

func TestReleaseInstallerReplacesExistingBinary(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	installDir := t.TempDir()
	env := withEnv(os.Environ(), "X_SKILLS_INSTALL_DIR", installDir)
	env = withEnv(env, "X_SKILLS_DOWNLOAD_URL", "http://fixture.invalid/x-skills")
	shortcut := filepath.Join(installDir, "xs")
	if runtime.GOOS == "windows" {
		shortcut += ".cmd"
	}
	const shortcutContent = "occupied shortcut"
	if err := os.WriteFile(shortcut, []byte(shortcutContent), 0o755); err != nil {
		t.Fatalf("write occupied shortcut: %v", err)
	}

	var command func() *exec.Cmd
	var installed string
	var firstFixture string
	var secondFixture string
	if runtime.GOOS == "windows" {
		script := filepath.Join(repoRoot, "scripts", "install.ps1")
		command = func() *exec.Cmd {
			return exec.Command(
				"powershell.exe",
				"-NoProfile",
				"-ExecutionPolicy",
				"Bypass",
				"-File",
				script,
			)
		}
		installed = filepath.Join(installDir, "x-skills.exe")
		firstFixture = serveZipFixture(t, []byte("first fixture"))
		secondFixture = serveZipFixture(t, []byte("second fixture"))
	} else {
		script := filepath.Join(repoRoot, "scripts", "install.sh")
		command = func() *exec.Cmd { return exec.Command("sh", script) }
		installed = filepath.Join(installDir, "x-skills")
		firstFixture = writeTarFixture(t, "first fixture")
		secondFixture = writeTarFixture(t, "second fixture")

		stubDir := t.TempDir()
		curlStub := filepath.Join(stubDir, "curl")
		stub := "#!/bin/sh\n" +
			"while [ \"$#\" -gt 0 ]; do\n" +
			"  if [ \"$1\" = -o ]; then cp \"$X_SKILLS_TEST_FIXTURE\" \"$2\"; exit; fi\n" +
			"  shift\n" +
			"done\n" +
			"exit 1\n"
		if err := os.WriteFile(curlStub, []byte(stub), 0o755); err != nil {
			t.Fatalf("write curl stub: %v", err)
		}
		env = withEnv(env, "PATH", stubDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	}

	if runtime.GOOS == "windows" {
		env = withEnv(env, "X_SKILLS_DOWNLOAD_URL", firstFixture)
	} else {
		env = withEnv(env, "X_SKILLS_TEST_FIXTURE", firstFixture)
	}
	runCommand(t, command(), env)

	if runtime.GOOS == "windows" {
		env = withEnv(env, "X_SKILLS_DOWNLOAD_URL", secondFixture)
	} else {
		env = withEnv(env, "X_SKILLS_TEST_FIXTURE", secondFixture)
	}
	secondOutput := runCommand(t, command(), env)
	for _, required := range []string{"existing x-skills found at", "replacing it"} {
		if !strings.Contains(secondOutput, required) {
			t.Errorf("second release install output must contain %q; output:\n%s", required, secondOutput)
		}
	}

	installedContent, err := os.ReadFile(installed)
	if err != nil {
		t.Fatalf("read installed fixture: %v", err)
	}
	want := "second fixture"
	if runtime.GOOS != "windows" {
		want = "#!/bin/sh\nprintf 'second fixture'\n"
	}
	if string(installedContent) != want {
		t.Fatalf("installed fixture = %q, want %q", installedContent, want)
	}
	assertFileContent(t, shortcut, shortcutContent)
}

func TestReleaseInstallerRejectsInvalidDownloadURL(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	env := withEnv(os.Environ(), "X_SKILLS_INSTALL_DIR", t.TempDir())
	env = withEnv(env, "X_SKILLS_DOWNLOAD_URL", "file:///tmp/x-skills")

	var command *exec.Cmd
	if runtime.GOOS == "windows" {
		command = exec.Command(
			"powershell.exe",
			"-NoProfile",
			"-ExecutionPolicy",
			"Bypass",
			"-File",
			filepath.Join(repoRoot, "scripts", "install.ps1"),
		)
	} else {
		command = exec.Command("sh", filepath.Join(repoRoot, "scripts", "install.sh"))
	}
	command.Env = env
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("installer accepted invalid download URL; output:\n%s", output)
	}
	if !strings.Contains(string(output), "must be an absolute http:// or https:// URL") {
		t.Fatalf("invalid URL error is not actionable; output:\n%s", output)
	}
}

func runCommand(t *testing.T, command *exec.Cmd, env []string) string {
	t.Helper()
	command.Env = env
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("run %s: %v\n%s", command.String(), err, output)
	}
	return string(output)
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(content) != want {
		t.Fatalf("%s content = %q, want %q", path, content, want)
	}
}

func withEnv(env []string, key, value string) []string {
	prefix := key + "="
	result := make([]string, 0, len(env)+1)
	for _, entry := range env {
		if !strings.HasPrefix(entry, prefix) {
			result = append(result, entry)
		}
	}
	return append(result, prefix+value)
}

func writeTarFixture(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "fixture.tar.gz")
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create tar fixture: %v", err)
	}
	gzipWriter := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gzipWriter)
	payload := []byte(fmt.Sprintf("#!/bin/sh\nprintf '%s'\n", content))
	header := &tar.Header{
		Name: "x-skills",
		Mode: 0o755,
		Size: int64(len(payload)),
	}
	if err := tarWriter.WriteHeader(header); err != nil {
		t.Fatalf("write tar header: %v", err)
	}
	if _, err := tarWriter.Write(payload); err != nil {
		t.Fatalf("write tar payload: %v", err)
	}
	for _, writer := range []struct {
		name   string
		closer io.Closer
	}{
		{name: "tar", closer: tarWriter},
		{name: "gzip", closer: gzipWriter},
		{name: "file", closer: file},
	} {
		if err := writer.closer.Close(); err != nil {
			t.Fatalf("close %s fixture writer: %v", writer.name, err)
		}
	}
	return path
}

func serveZipFixture(t *testing.T, content []byte) string {
	t.Helper()

	var archive bytes.Buffer
	zipWriter := zip.NewWriter(&archive)
	file, err := zipWriter.Create("x-skills.exe")
	if err != nil {
		t.Fatalf("create zip fixture entry: %v", err)
	}
	if _, err := file.Write(content); err != nil {
		t.Fatalf("write zip fixture: %v", err)
	}
	if err := zipWriter.Close(); err != nil {
		t.Fatalf("close zip fixture: %v", err)
	}
	payload := append([]byte(nil), archive.Bytes()...)
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = response.Write(payload)
	}))
	t.Cleanup(server.Close)
	return server.URL + "/x-skills.zip"
}
