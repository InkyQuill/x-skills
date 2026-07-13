# TUI Startup, Version, and Installers Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restore safe semantic row styling, render slow startup work visibly and asynchronously, show build/update badges, and make release and development installation reliably replace an existing binary.

**Architecture:** `internal/buildinfo` is the sole boundary for injected versions, semantic comparison, and best-effort GitHub release discovery. The CLI injects build information and an update checker into a Bubble Tea model whose initial filesystem load and update check run as independent commands. GoReleaser becomes the sole GitHub release publisher after semantic-release creates the version tag.

**Tech Stack:** Go 1.26, Cobra, Bubble Tea, Lip Gloss, `golang.org/x/mod/semver`, POSIX shell, PowerShell, GoReleaser v2, semantic-release with `@semantic-release/exec`, GitHub Actions.

## Global Constraints

- Preserve Linux, macOS, and Windows compatibility.
- Default source/development version is exactly `dev`; release display is `v<semver>`.
- Update checks are asynchronous, stable-release-only, and silent on failure.
- `tui.New` performs no filesystem or network I/O.
- Unicode markers: managed `●`, unmanaged `○`, broken `×`, unchecked `◇`, checked `◆`.
- ASCII markers: managed `+`, unmanaged `?`, broken `x`, unchecked `[ ]`, checked `[x]`.
- `NO_COLOR` retains meaningful shapes and explicit badge text.
- Rows preserve SGR styling while removing OSC, DCS, non-SGR CSI, and unsafe controls.
- Re-running either installer replaces the destination and reports the replacement.
- Never mutate an occupied `xs` shortcut.
- GoReleaser is the only GitHub release/artifact publisher.
- Leave the unrelated untracked `.claude/` directory untouched.

## File Structure

New focused units:

- `internal/buildinfo/buildinfo.go` and `buildinfo_test.go`: version normalization/comparison.
- `internal/buildinfo/latest.go` and `latest_test.go`: GitHub latest-release resolution.
- `internal/cli/version.go` and `version_test.go`: `x-skills version`.
- `scripts/install-dev.sh` and `scripts/install-dev.ps1`: local development overwrite.

Existing boundaries remain in place: CLI dependency construction stays in `internal/cli`, Bubble Tea
state stays in `internal/tui`, canonical ANSI helpers stay in `internal/tui/ui`, and release
packaging stays in `.goreleaser.yaml` plus `.github/workflows/release.yml`.

---

### Task 1: Build information and version command

**Files:**
- Create: `internal/buildinfo/buildinfo.go`
- Create: `internal/buildinfo/buildinfo_test.go`
- Create: `internal/cli/version.go`
- Create: `internal/cli/version_test.go`
- Modify: `internal/cli/root.go` (`options`, construction, command registration)
- Modify: `go.mod`, `go.sum`

**Interfaces:**
- Produces: `buildinfo.New(string) Info`, `Current() Info`, `Info.Display() string`,
  `Info.IsRelease() bool`, `Info.NewerStable(string) (string, bool)`.
- Produces: `newVersionCommand(buildinfo.Info) *cobra.Command`.

- [ ] **Step 1: Add semver and write the failing table tests**

Run `go get golang.org/x/mod/semver`, then create tests covering empty/`dev`, prefixed and
unprefixed stable releases, malformed versions, prereleases, equal/older/newer remote versions, and
a development build. The core table is:

```go
func TestInfoNewerStable(t *testing.T) {
	tests := []struct {
		name, current, latest, want string
		available                   bool
	}{
		{name: "newer", current: "v1.2.3", latest: "v1.3.0", want: "v1.3.0", available: true},
		{name: "missing prefix", current: "1.2.3", latest: "1.2.4", want: "v1.2.4", available: true},
		{name: "equal", current: "v1.2.3", latest: "v1.2.3"},
		{name: "older", current: "v1.2.3", latest: "v1.1.9"},
		{name: "dev", current: "dev", latest: "v9.0.0"},
		{name: "prerelease", current: "v1.2.3", latest: "v1.3.0-rc.1"},
		{name: "invalid", current: "v1.2.3", latest: "main"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, available := New(tt.current).NewerStable(tt.latest)
			if got != tt.want || available != tt.available {
				t.Fatalf("NewerStable(%q) = (%q, %t), want (%q, %t)",
					t.latest, got, available, tt.want, tt.available)
			}
		})
	}
}
```

- [ ] **Step 2: Verify the build-info tests fail**

Run: `go test ./internal/buildinfo`

Expected: FAIL because `Info` and `New` do not exist.

- [ ] **Step 3: Implement `internal/buildinfo/buildinfo.go`**

```go
package buildinfo

import (
	"strings"

	"golang.org/x/mod/semver"
)

var version = "dev"

type Info struct{ version string }

func Current() Info { return New(version) }

func New(raw string) Info {
	normalized := normalizeStable(raw)
	if normalized == "" {
		normalized = "dev"
	}
	return Info{version: normalized}
}

func (i Info) Display() string {
	if i.version == "" {
		return "dev"
	}
	return i.version
}

func (i Info) IsRelease() bool { return i.Display() != "dev" }

func (i Info) NewerStable(raw string) (string, bool) {
	latest := normalizeStable(raw)
	if !i.IsRelease() || latest == "" || semver.Compare(latest, i.version) <= 0 {
		return "", false
	}
	return latest, true
}

func normalizeStable(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" || value == "dev" {
		return ""
	}
	if !strings.HasPrefix(value, "v") {
		value = "v" + value
	}
	if !semver.IsValid(value) || semver.Prerelease(value) != "" {
		return ""
	}
	return value
}
```

- [ ] **Step 4: Verify build-info tests pass**

Run: `go test ./internal/buildinfo`

Expected: PASS.

- [ ] **Step 5: Write the failing CLI test**

```go
func TestVersionCommandPrintsBuildVersion(t *testing.T) {
	var stdout bytes.Buffer
	cmd := newVersionCommand(buildinfo.New("1.2.3"))
	cmd.SetOut(&stdout)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got, want := stdout.String(), "v1.2.3\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}
```

Run: `go test ./internal/cli -run TestVersionCommandPrintsBuildVersion -count=1`

Expected: FAIL because `newVersionCommand` is undefined.

- [ ] **Step 6: Implement and register `version`**

```go
func newVersionCommand(info buildinfo.Info) *cobra.Command {
	return &cobra.Command{
		Use: "version", Short: "Print the x-skills version", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), info.Display())
			return err
		},
	}
}
```

Add `buildInfo buildinfo.Info` to `options`, initialize it with `buildinfo.Current()`, and register
`newVersionCommand(opts.buildInfo)` in `root.AddCommand`.

- [ ] **Step 7: Verify tests and linker injection**

```bash
go test ./internal/buildinfo ./internal/cli
tmp="$(mktemp -d)"
go build -ldflags '-X github.com/InkyQuill/x-skills/internal/buildinfo.version=1.2.3' -o "$tmp/x-skills" ./cmd/x-skills
test "$("$tmp/x-skills" version)" = "v1.2.3"
rm -rf "$tmp"
```

Expected: all assertions pass.

- [ ] **Step 8: Commit**

```bash
git add go.mod go.sum internal/buildinfo internal/cli/root.go internal/cli/version.go internal/cli/version_test.go
git commit -m "feat: add build version information"
```

---

### Task 2: Latest stable release checker

**Files:**
- Create: `internal/buildinfo/latest.go`
- Create: `internal/buildinfo/latest_test.go`

**Interfaces:**
- Produces: `LatestReleaseChecker.LatestRelease(context.Context) (string, error)`.
- Produces: `NewGitHubReleaseChecker(*http.Client) LatestReleaseChecker`.

- [ ] **Step 1: Write failing HTTP tests**

Use `httptest.Server` for: `/releases/latest` redirecting to `/releases/tag/v1.4.0`, an OK response
whose final URL lacks `/releases/tag/`, a non-200 response, and a handler blocked until context
cancellation. Assert the successful case returns `v1.4.0` and every other case returns an error.

- [ ] **Step 2: Verify failure**

Run: `go test ./internal/buildinfo -run GitHubReleaseChecker -count=1`

Expected: FAIL because the checker API is missing.

- [ ] **Step 3: Implement `internal/buildinfo/latest.go`**

```go
package buildinfo

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const latestReleaseURL = "https://github.com/InkyQuill/x-skills/releases/latest"

type LatestReleaseChecker interface {
	LatestRelease(context.Context) (string, error)
}

type githubReleaseChecker struct {
	endpoint string
	client   *http.Client
}

func NewGitHubReleaseChecker(client *http.Client) LatestReleaseChecker {
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Second}
	}
	return githubReleaseChecker{endpoint: latestReleaseURL, client: client}
}

func (c githubReleaseChecker) LatestRelease(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("create latest release request: %w", err)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("resolve latest release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("resolve latest release: status %s", resp.Status)
	}
	return releaseTagFromURL(resp.Request.URL)
}

func releaseTagFromURL(value *url.URL) (string, error) {
	const marker = "/releases/tag/"
	index := strings.LastIndex(value.EscapedPath(), marker)
	if index < 0 {
		return "", fmt.Errorf("latest release response has no release tag: %s", value)
	}
	tag, err := url.PathUnescape(value.EscapedPath()[index+len(marker):])
	if err != nil || tag == "" || strings.Contains(tag, "/") {
		return "", fmt.Errorf("latest release response has invalid release tag: %s", value)
	}
	return tag, nil
}
```

Tests use an unexported constructor literal `githubReleaseChecker{endpoint: server.URL, client:
server.Client()}` to avoid production network calls.

- [ ] **Step 4: Verify and commit**

```bash
go test ./internal/buildinfo -count=1
git add internal/buildinfo/latest.go internal/buildinfo/latest_test.go
git commit -m "feat: check latest stable release"
```

Expected: PASS and commit succeeds.

---

### Task 3: TUI version/update badges and asynchronous startup

**Files:**
- Modify: `internal/cli/root.go`, `internal/cli/tui.go`
- Modify: `internal/tui/options.go`, `internal/tui/model.go`, `internal/tui/views.go`, `internal/tui/styles.go`
- Modify: `internal/tui/model_test.go`, `internal/tui/render_test.go`, `internal/tui/animation_test.go`
- Modify: only those other `internal/tui/*_test.go` fixtures that require scanned filesystem data

**Interfaces:**
- Consumes: `buildinfo.Info` and `buildinfo.LatestReleaseChecker`.
- Produces: `Options.BuildInfo`, `Options.LatestReleaseChecker`, internal `Options.loadData`.
- Produces: cancellable `startupLoadCmd`, `updateCheckCmd`, and token-guarded result messages.

- [ ] **Step 1: Add failing badge tests**

Add a fake checker in `model_test.go`:

```go
type staticLatestRelease struct{ version string }

func (s staticLatestRelease) LatestRelease(context.Context) (string, error) {
	return s.version, nil
}
```

Add header tests that construct `buildinfo.New("1.2.3")`, set `m.latestRelease = "v1.3.0"`, and
require `x-skills`, `v1.2.3`, `update v1.3.0`, and `A:Active`. Add a development case requiring
`x-skills  dev` and no `update` copy even if `latestRelease` is populated. Add an 80-column case
requiring `lipgloss.Width(renderHeader(m, 80)) <= 80` and intact ANSI according to
`tuiui.SanitizeANSI`.

- [ ] **Step 2: Add a failing immediate-startup test**

```go
func TestNewReturnsBeforeInitialDataLoadCompletes(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	loader := func(context.Context, config.Config) ([]ActiveGroup, []repo.Skill, []doctor.Issue, map[string][]string, error) {
		close(started)
		<-release
		return nil, []repo.Skill{{Name: "loaded"}}, nil, map[string][]string{}, nil
	}

	m := New(config.Default(t.TempDir(), t.TempDir()), Options{loadData: loader})
	select {
	case <-started:
		t.Fatal("loader ran during New")
	default:
	}
	if got := plain(m.View()); !strings.Contains(got, "Loading skills data…") {
		t.Fatalf("initial view missing loading state:\n%s", got)
	}

	results := make(chan tea.Msg, 1)
	go func() { results <- m.startupLoadCmd() }()
	<-started
	close(release)
	updated, _ := m.Update(<-results)
	m = mustModel(t, updated)
	if m.reloadInFlight || len(m.repo) != 1 || m.repo[0].Name != "loaded" {
		t.Fatalf("repo after startup = %#v", m.repo)
	}
}
```

Add a second loader that waits on `ctx.Done()`; send `ctrl+c` and require cancellation within one
second. While the loader is blocked, send `?` and require a non-nil help modal. Extend
`TestStaleReloadResultIgnored` so the initial token cannot overwrite a later refresh.

- [ ] **Step 3: Verify focused tests fail**

Run:

```bash
go test ./internal/tui -run 'Test(Header|NewReturns|QuitCancels|StaleReload)' -count=1
```

Expected: FAIL because options, badge fields, loader injection, and loading state are absent.

- [ ] **Step 4: Define injectable options**

`internal/tui/options.go` must define:

```go
type dataLoader func(context.Context, config.Config) ([]ActiveGroup, []repo.Skill, []doctor.Issue, map[string][]string, error)

type Options struct {
	ASCII                bool
	BuildInfo            buildinfo.Info
	LatestReleaseChecker buildinfo.LatestReleaseChecker
	loadData             dataLoader
}

func defaultOptions() Options {
	return Options{BuildInfo: buildinfo.Current(), loadData: loadTUIData}
}
```

Merge provided non-zero dependencies over defaults in `New`; never replace the default loader with
nil.

- [ ] **Step 5: Prepare cancellable commands without doing I/O**

Add these model fields:

```go
buildInfo       buildinfo.Info
latestRelease   string
startupLoadCmd  tea.Cmd
updateCheckCmd  tea.Cmd
reloadCancel    context.CancelFunc
updateCancel    context.CancelFunc
```

Delete `reloadSynchronously()` from `New`. Before returning, call preparation methods that only
create contexts/closures:

```go
m.startupLoadCmd = m.beginReloadWithStatus(false)
if m.opts.LatestReleaseChecker != nil && m.buildInfo.IsRelease() {
	ctx, cancel := context.WithCancel(context.Background())
	m.updateCancel = cancel
	m.updateCheckCmd = m.latestReleaseCmd(ctx)
}
return m
```

Change `reloadCmd` to capture `ctx`, `m.opts.loadData`, config, and token. Implement
`beginReloadWithStatus(report bool)` to cancel the previous load, increment `reloadToken`, set
`reloadInFlight`, create a new context/cancel pair, and return the command. `beginReload()` delegates
with `true`; initial loading delegates with `false`.

- [ ] **Step 6: Batch startup work and apply messages**

```go
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.startupLoadCmd, m.updateCheckCmd}
	if m.animationsEnabled() {
		cmds = append(cmds, animationTick())
	}
	return tea.Batch(cmds...)
}
```

`latestReleaseCmd` returns an empty `latestReleaseMsg` on any error. Its `Update` case calls
`m.buildInfo.NewerStable(msg.version)` and stores only an available version. Matching reload results
clear `reloadCancel`; `ctrl+c` invokes both cancel functions before `tea.Quit`.

Update `animation_test.go`: ASCII `Init` is no longer nil because data loading always starts, so
assert both modes return startup work. Keep `TestAnimationTickAdvancesOnlyWhenUnicode` as the direct
contract that Unicode schedules the next animation tick while ASCII returns no animation command;
do not assume `tea.Batch` always returns `tea.BatchMsg`, because it compacts a single command.

- [ ] **Step 7: Render loading and header badges**

Before the `No items.` branch in `renderListPanel`:

```go
if m.reloadInFlight && len(rows) == 0 {
	rows = []string{accentStyle.Render(m.pulseDiamond() + " Loading skills data…")}
}
```

In `renderHeader`:

```go
titleParts := []string{
	titleStyle.Render(m.pulseDiamond() + " x-skills"),
	versionStyle.Render(m.buildInfo.Display()),
}
if m.latestRelease != "" {
	titleParts = append(titleParts, updateStyle.Render("update "+m.latestRelease))
}
title := strings.Join(titleParts, "  ") + "  " + strings.Join(tabs, " ")
return tuiui.TruncateANSI(title, width)
```

Define muted `versionStyle` and accented `updateStyle` in `styles.go`; unset their colors/backgrounds
inside the existing `NO_COLOR` initialization.

- [ ] **Step 8: Inject production dependencies**

Add `latestReleaseChecker buildinfo.LatestReleaseChecker` to CLI `options`, initialize it with
`buildinfo.NewGitHubReleaseChecker(nil)`, and pass:

```go
tui.Options{
	ASCII: opts.ascii,
	BuildInfo: rootOptions.buildInfo,
	LatestReleaseChecker: rootOptions.latestReleaseChecker,
}
```

- [ ] **Step 9: Adapt filesystem-backed tests deliberately**

Add this test helper:

```go
func newLoadedModel(t *testing.T, cfg config.Config, opts ...Options) Model {
	t.Helper()
	m := New(cfg, opts...)
	updated, _ := m.Update(m.startupLoadCmd())
	return mustModel(t, updated)
}
```

Use it only where a test creates skills on disk and immediately expects discovered `active`, `repo`,
or `issues`. Tests that directly populate model slices continue using `New`.

- [ ] **Step 10: Verify and commit**

```bash
go test ./internal/buildinfo ./internal/cli ./internal/tui -count=1
git add internal/cli/root.go internal/cli/tui.go internal/tui
git commit -m "feat: render versioned tui while skills load"
```

Expected: PASS with no GitHub traffic from tests and no synchronous scan in `New`.

---

### Task 4: Restore safe row colors and approved markers

**Files:**
- Modify: `internal/tui/views.go`, `symbols.go`, `modal_help.go`
- Modify: `internal/tui/views_security_test.go`, `rows_test.go`, `filter_test.go`, `render_test.go`

**Interfaces:**
- Consumes: `tui/ui.SanitizeANSI` and `TruncateANSI`.
- Removes: `symbols.CountPrefix` and its stale help line.

- [ ] **Step 1: Strengthen the row security test**

Replace the current test with one whose segment contains red SGR plus OSC hyperlink and clear-screen
CSI. For unselected, focused, and selected rows, require `\x1b[31mred\x1b[0m` to survive while
`\x1b]`, BEL, and `\x1b[2J` are absent.

- [ ] **Step 2: Change expected marker contracts first**

Use these exact Unicode test cases:

```go
{name: "unicode managed", status: actions.StatusManaged, wantChip: "● managed", wantMarker: "●"},
{name: "unicode unmanaged", status: actions.StatusUnmanaged, wantChip: "○ unmanaged", wantMarker: "○"},
{name: "unicode broken", status: actions.StatusBroken, wantChip: "× broken", wantMarker: "×"},
```

Update row substrings from the unmanaged diamond to `○`. Add a help test rejecting both
`group count badge` and `×N`.

- [ ] **Step 3: Verify regression tests fail**

Run:

```bash
go test ./internal/tui -run 'Test(SelectableRow|StatusRenderers|RenderActiveRows|HelpDoesNot)' -count=1
```

Expected: FAIL because SGR is stripped, old markers remain, and stale help is present.

- [ ] **Step 4: Replace blanket stripping with canonical sanitizing**

In `selectableRow` and `joinRowSegments`, replace every row-segment `ansi.Strip` call with
`tuiui.SanitizeANSI`. Preserve this unselected branch:

```go
return tuiui.TruncateANSI(joinRowSegments(segments, lipgloss.NoColor{}), width)
```

For focused/selected segments use:

```go
if segment.render != nil {
	text = tuiui.SanitizeANSI(segment.render(background))
} else {
	text = tuiui.SanitizeANSI(text)
}
```

Remove the `ansi` import from `views.go` after `rg -n 'ansi\.' internal/tui/views.go` returns no
matches.

- [ ] **Step 5: Apply marker and dead-state cleanup**

Set Unicode `Managed: "●"`, `Unmanaged: "○"`, `Broken: "×"`. Keep approved ASCII markers. Remove
`CountPrefix` from the type and constructors, and remove the group-count help line.

- [ ] **Step 6: Verify color and no-color modes, then commit**

```bash
env -u NO_COLOR go test ./internal/tui -run 'Test(SelectableRow|Status|RenderActiveRows|FilterCursor|Help)' -count=1
NO_COLOR=1 go test ./internal/tui -run 'Test(StatusRowsDistinguishableWithoutColor|HelpDoesNot)' -count=1
go test ./internal/tui -count=1
git add internal/tui
git commit -m "fix: restore semantic tui row styling"
```

Expected: all commands PASS; safe SGR survives and unsafe terminal controls do not.

---

### Task 5: Idempotent release and development installers

**Files:**
- Modify: `scripts/install.sh`, `scripts/install.ps1`
- Create: `scripts/install-dev.sh`, `scripts/install-dev.ps1`
- Modify: `cmd/x-skills/docs_test.go`
- Modify: `README.md`, `CONTRIBUTING.md`

**Interfaces:**
- Consumes: `internal/buildinfo.version` linker variable.
- Preserves: `X_SKILLS_INSTALL_DIR`, `X_SKILLS_VERSION`, and safe `xs` behavior.

- [ ] **Step 1: Add failing configuration assertions**

Extend `TestReleaseAndInstallerConfiguration` to require both dev scripts. Require Unix scripts to
contain `existing x-skills found`, `replacing it`, `mv -f`, `X_SKILLS_INSTALL_DIR`, and the build-info
linker path. Require PowerShell scripts to contain the same copy and
`[System.IO.File]::Replace`. Require dev scripts to inject literal `dev`.

Run: `go test ./cmd/x-skills -run TestReleaseAndInstallerConfiguration -count=1`

Expected: FAIL.

- [ ] **Step 2: Stage and replace in `install.sh`**

Add `need install` and `need mv`. Initialize `staged=""`, clean it in the trap, and replace direct
installation with:

```sh
target="$INSTALL_DIR/$BIN_NAME"
staged="$INSTALL_DIR/.${BIN_NAME}.install.$$"
if [ -e "$target" ] || [ -L "$target" ]; then
  log "existing $BIN_NAME found at $target; replacing it"
fi
install -m 0755 "$tmp/$BIN_NAME" "$staged"
mv -f "$staged" "$target"
staged=""
install_xs_link "$target"
```

Staging inside `INSTALL_DIR` keeps rename on the destination filesystem.

For hermetic integration tests, allow an undocumented `X_SKILLS_DOWNLOAD_URL` override after
validating it with `case "$X_SKILLS_DOWNLOAD_URL" in http://*|https://*) ;; *) fail ... ;; esac`.
When absent, retain the existing latest/pinned GitHub URL construction exactly.

- [ ] **Step 3: Stage and replace in `install.ps1`**

After extraction validation, use:

```powershell
$installedExe = Join-Path $InstallDir "$BinName.exe"
$stagedExe = Join-Path $InstallDir ".$BinName.install.$PID.exe"
Copy-Item -Force $exe $stagedExe
try {
    if (Test-Path $installedExe) {
        Write-Step "existing $BinName found at $installedExe; replacing it"
        [System.IO.File]::Replace($stagedExe, $installedExe, $null)
    } else {
        [System.IO.File]::Move($stagedExe, $installedExe)
    }
} catch [System.IO.IOException] {
    throw "replace $installedExe failed; close any running x-skills process and retry: $($_.Exception.Message)"
} finally {
    Remove-Item -Force $stagedExe -ErrorAction SilentlyContinue
}
Install-XsShortcut
```

Support the same undocumented `$env:X_SKILLS_DOWNLOAD_URL` override only when
`[System.Uri]::TryCreate(..., [System.UriKind]::Absolute, ...)` succeeds and the scheme is `http` or
`https`; otherwise throw before downloading. Keep current GitHub URL selection when it is absent.

- [ ] **Step 4: Create `install-dev.sh`**

Reuse the Unix destination, staging, replacement copy, cleanup, and safe shortcut functions. Resolve
the checkout with:

```sh
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
REPO_ROOT=$(dirname "$SCRIPT_DIR")
cd "$REPO_ROOT"
```

Build with:

```sh
go build \
  -ldflags '-X github.com/InkyQuill/x-skills/internal/buildinfo.version=dev' \
  -o "$tmp/$BIN_NAME" \
  ./cmd/x-skills
```

Then `chmod +x scripts/install-dev.sh`.

- [ ] **Step 5: Create `install-dev.ps1`**

Reuse the PowerShell replacement and shortcut functions. Build from `$RepoRoot` with:

```powershell
& go build -ldflags "-X github.com/InkyQuill/x-skills/internal/buildinfo.version=dev" -o $builtExe ./cmd/x-skills
if ($LASTEXITCODE -ne 0) {
    throw "go build failed with exit code $LASTEXITCODE"
}
```

Always restore location and remove the temporary build directory in `finally` blocks.

- [ ] **Step 6: Add a platform-aware development installer test**

Add `TestDevelopmentInstallerReplacesExistingBinary` to `cmd/x-skills/docs_test.go`. Set
`X_SKILLS_INSTALL_DIR` to `t.TempDir()`. Invoke `sh scripts/install-dev.sh` on Unix or
`powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts/install-dev.ps1` on Windows twice.
Require second output to contain both replacement phrases. Execute the installed binary with
`version`; require trimmed stdout `dev`. Do not mark this build test parallel.

Also add release-installer overwrite tests without network access. On Unix, prepend a temporary
directory containing a `curl` stub that copies a generated tarball to the argument following `-o`;
run `install.sh` twice with different fixture binaries. On Windows, start an `httptest.Server` that
serves generated zip fixtures and invoke `install.ps1` with its download URL redirected to the local
server through a test-only environment override. The second run on each platform must report
replacement and leave the second fixture at the destination. Keep the override undocumented and
reject it unless the environment variable contains an absolute `http://` or `https://` URL.

- [ ] **Step 7: Document and verify**

Add both dev commands to `README.md` and `CONTRIBUTING.md`, stating that they overwrite the normal
installation and report `dev`.

```bash
go test ./cmd/x-skills -run 'Test(ReleaseAndInstallerConfiguration|DevelopmentInstallerReplacesExistingBinary)' -count=1
```

Expected: PASS on Unix; Windows CI exercises PowerShell.

- [ ] **Step 8: Commit**

```bash
git add scripts/install.sh scripts/install.ps1 scripts/install-dev.sh scripts/install-dev.ps1 cmd/x-skills/docs_test.go README.md CONTRIBUTING.md
git commit -m "feat: add repeatable development install"
```

---

### Task 6: Publish versioned artifacts through GoReleaser

**Files:**
- Modify: `.goreleaser.yaml`
- Modify: `release.config.cjs`
- Modify: `.github/workflows/release.yml`
- Modify: `cmd/x-skills/docs_test.go`

**Interfaces:**
- Consumes: build-info linker variable and installer scripts.
- Produces: semantic-release tag followed by one GoReleaser-owned release.

- [ ] **Step 1: Add failing release-ownership assertions**

Require `.goreleaser.yaml` to contain the build-info `-X` flag, `release.extra_files`, and both
installer globs; reject `changelog:\n  disable: true`. Require `release.config.cjs` to contain
`@semantic-release/exec`, `publishCmd`, and `goreleaser release --clean`; reject
`@semantic-release/github`. Require the workflow to contain `install-only: true`; reject
`release --snapshot`.

Run: `go test ./cmd/x-skills -run TestReleaseAndInstallerConfiguration -count=1`

Expected: FAIL.

- [ ] **Step 2: Configure GoReleaser**

Add to the build:

```yaml
    ldflags:
      - >-
        -s -w
        -X github.com/InkyQuill/x-skills/internal/buildinfo.version={{ .Version }}
```

Make snapshot smoke-test version injection explicit:

```yaml
snapshot:
  version_template: >-
    {{- if index .Env "X_SKILLS_SNAPSHOT_VERSION" -}}
    {{- index .Env "X_SKILLS_SNAPSHOT_VERSION" -}}
    {{- else -}}
    {{- incpatch .Version }}-next
    {{- end -}}
```

Remove the disabled changelog and add:

```yaml
release:
  extra_files:
    - glob: scripts/install.sh
    - glob: scripts/install.ps1
```

- [ ] **Step 3: Make semantic-release delegate publication**

Replace `release.config.cjs` with:

```js
module.exports = {
  branches: ["main"],
  tagFormat: "v${version}",
  plugins: [
    "@semantic-release/commit-analyzer",
    [
      "@semantic-release/exec",
      { publishCmd: "goreleaser release --clean" },
    ],
  ],
};
```

- [ ] **Step 4: Install GoReleaser instead of building a snapshot**

Use this workflow step before Node setup:

```yaml
      - name: Install GoReleaser
        uses: goreleaser/goreleaser-action@v7
        with:
          distribution: goreleaser
          version: '~> v2'
          install-only: true
```

Run semantic-release with only required pinned-major packages:

```yaml
      - name: Release
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: >
          npx -y
          -p semantic-release@25
          -p @semantic-release/commit-analyzer
          -p @semantic-release/exec
          semantic-release
```

The exec child inherits the action-added `PATH` and `GITHUB_TOKEN`; semantic-release creates the tag
before its publish lifecycle invokes GoReleaser.

- [ ] **Step 5: Verify static and GoReleaser configuration**

```bash
go test ./cmd/x-skills -run TestReleaseAndInstallerConfiguration -count=1
goreleaser check
X_SKILLS_SNAPSHOT_VERSION=9.8.7 goreleaser release --snapshot --clean --skip=publish
```

Expected: PASS. The snapshot is only a local smoke build, never a release input.

- [ ] **Step 6: Inspect the snapshot build version and commit**

Locate the current-host executable below `dist`, run `version`, and require `v9.8.7`, proving the
GoReleaser `.Version` template reached the linker variable. Then:

```bash
git add .goreleaser.yaml release.config.cjs .github/workflows/release.yml cmd/x-skills/docs_test.go
git commit -m "fix: publish versioned release artifacts"
```

---

### Task 7: Integrated verification

**Files:**
- Modify only files implicated by a failing verification command.

**Interfaces:**
- Verifies all previous contracts; adds no public API.

- [ ] **Step 1: Format and inspect**

```bash
gofmt -w internal/buildinfo internal/cli internal/tui cmd/x-skills
git diff --check
git status --short
git diff --stat
```

Expected: no whitespace errors; `.claude/` remains untouched.

- [ ] **Step 2: Run full tests, race detector, and vet**

```bash
go test ./...
go test -race ./internal/tui/... ./internal/buildinfo/...
go vet ./...
```

Expected: all PASS.

- [ ] **Step 3: Verify color profiles independently**

```bash
env -u NO_COLOR go test ./internal/tui -run 'Test(Header|Status|SelectableRow)' -count=1
NO_COLOR=1 go test ./internal/tui -run 'Test(Header|StatusRowsDistinguishableWithoutColor)' -count=1
```

Expected: both PASS.

- [ ] **Step 4: Verify dev and release-labeled binaries**

```bash
tmp="$(mktemp -d)"
go build -o "$tmp/dev-x-skills" ./cmd/x-skills
test "$("$tmp/dev-x-skills" version)" = "dev"
go build -ldflags '-X github.com/InkyQuill/x-skills/internal/buildinfo.version=9.8.7' -o "$tmp/release-x-skills" ./cmd/x-skills
test "$("$tmp/release-x-skills" version)" = "v9.8.7"
rm -rf "$tmp"
```

Expected: both assertions pass.

- [ ] **Step 5: Manual TUI smoke test**

Run `go run ./cmd/x-skills tui` against the normal archive. Confirm the frame and loading copy appear
before rows, header shows `dev`, markers are `●`/`○`/`×`, row colors survive, quit works while
loading, and offline update failure remains silent.

- [ ] **Step 6: Record only necessary verification corrections**

If a prior step required corrections, stage the explicit corrected paths shown by `git status
--short`, review `git diff --cached`, and commit with `fix: address integrated verification findings`.
If no corrections were needed, do not create an empty commit.
