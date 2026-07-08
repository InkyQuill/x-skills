# Install Tab Workflow Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the `I` Install tab as a working search-driven TUI workflow for remote `skills.sh` discovery, preview, archive-only install, and install-and-use linking.

**Architecture:** Add a small `internal/remote` layer for search, git checkout, source metadata, archive-state detection, and add planning/apply logic. Keep `internal/tui` focused on Bubble Tea state, rendering, async commands, and modal workflows; TUI calls remote actions instead of doing network/git/filesystem orchestration inline. Execute on the current branch; do not create worktrees.

**Tech Stack:** Go, Bubble Tea, Lip Gloss, Bubbles textinput/viewport, Glamour, existing `internal/tui` modal system, `net/http`, local `git` CLI via `os/exec`, existing `internal/actions` link helpers.

---

## Scope

Implement this pass:

- Top-level `I` Install tab with header, list, inspector, footer, and help text.
- `/` query input with minimum 2 characters, Enter-forces-search, previous results kept while searching.
- Optional `o` owner filter input.
- `skills.sh` legacy search client with default limit 50.
- Process-lifetime preview/install checkout cache.
- `enter` preview of remote `SKILL.md`.
- `a` archive-only install.
- `i` install-and-use with destination checklist and project agents checked by default.
- Archive state badges: `not archived`, `archived`, `name conflict`, `update available`.
- Same-source divergent update uses Archive vs Incoming remote diff modal.
- Same-name unproven source conflict uses explicit replace/rename/cancel workflow.
- Stay on Install after every action and update row/status in place.

Defer from this plan:

- Repo update badges and `^U` update workflow.
- CLI `search`, `add`, `repo check`, `repo update`, `repo update-all`.
- Batch TUI remote installs.
- Direct URL/archive installs.
- Full advisory audit partner integration if the endpoint contract is not already known locally; include cache/render plumbing and no-pill-when-unavailable behavior.

## File Structure

- Create `internal/remote/search.go`: remote search request/response types and `Client.Search`.
- Create `internal/remote/search_test.go`: endpoint shape, min length, owner filter, JSON parsing.
- Create `internal/remote/source.go`: `.x-skills.json` metadata types and read/write helpers.
- Create `internal/remote/source_test.go`: metadata round trip and missing metadata behavior.
- Create `internal/remote/git.go`: git clone/cache helpers and skill path discovery.
- Create `internal/remote/git_test.go`: checkout reuse and ambiguous path discovery tests using local git repos.
- Create `internal/remote/add.go`: archive-state detection, add plan, archive apply, conflict types.
- Create `internal/remote/add_test.go`: archive-only, same-source update, name conflict, rename, replace.
- Create `internal/tui/install.go`: Install view state, messages, async commands, action handlers.
- Create `internal/tui/install_test.go`: TUI search, preview, archive-only, install-and-use, conflicts.
- Modify `internal/tui/model.go`: add `ViewInstall`, install state, message handling, key routing.
- Modify `internal/tui/views.go`: render Install rows/inspector/footer.
- Modify `internal/tui/keys.go`: add `keyInstall`.
- Modify `internal/tui/modal_help.go`: make Install help real.
- Modify `internal/tui/modal_diff.go`: reuse Incoming remote label already supported.
- Create `internal/tui/modal_text.go`: editable rename prompt for incoming/existing archive rename choices.
- Modify `internal/actions/link.go` only if destination checklist needs a reusable batch link helper.
- Modify `README.md`: remove "designed but not yet built" wording after the tab ships.

## Validation Commands

Run before and after each task:

```bash
go test ./internal/remote -count=1
go test ./internal/tui -count=1
go test ./cmd/... ./internal/... -count=1
gofmt -w internal/remote/*.go internal/tui/*.go
go build -o bin/x-skills ./cmd/x-skills
```

Expected final result:

```text
ok  	github.com/InkyQuill/x-skills/internal/remote
ok  	github.com/InkyQuill/x-skills/internal/tui
ok  	github.com/InkyQuill/x-skills/cmd/x-skills
```

---

### Task 1: Remote Source Metadata

**Files:**
- Create: `internal/remote/source.go`
- Create: `internal/remote/source_test.go`
- Modify: `internal/repo/repo.go`
- Test: `internal/remote/source_test.go`

- [x] **Step 1: Write metadata round-trip tests**

Create `internal/remote/source_test.go`:

```go
package remote

import (
	"path/filepath"
	"testing"
)

func TestSourceMetadataRoundTrip(t *testing.T) {
	dir := t.TempDir()
	meta := SourceMetadata{
		SourceType:   SourceTypeGitHub,
		Owner:        "vercel-labs",
		Repo:         "skills",
		CloneURL:     "https://github.com/vercel-labs/skills.git",
		Ref:          "main",
		Commit:       "abc123",
		SkillPath:    "skills/svelte-coder",
		UpstreamName: "svelte-coder",
	}
	if err := WriteSourceMetadata(dir, meta); err != nil {
		t.Fatal(err)
	}
	got, ok, err := ReadSourceMetadata(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("metadata not found")
	}
	if got != meta {
		t.Fatalf("metadata = %#v, want %#v", got, meta)
	}
}

func TestReadSourceMetadataMissing(t *testing.T) {
	got, ok, err := ReadSourceMetadata(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatalf("ok = true for missing metadata: %#v", got)
	}
}

func TestSourceIdentityMatchesSameGitHubSkill(t *testing.T) {
	left := SourceMetadata{SourceType: SourceTypeGitHub, Owner: "vercel-labs", Repo: "skills", SkillPath: "skills/svelte-coder"}
	right := SourceMetadata{SourceType: SourceTypeGitHub, Owner: "vercel-labs", Repo: "skills", SkillPath: filepath.ToSlash("skills/svelte-coder")}
	if !left.SameIdentity(right) {
		t.Fatalf("expected same identity: %#v %#v", left, right)
	}
}
```

- [x] **Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/remote -run 'TestSource' -count=1 -v
```

Expected:

```text
FAIL
package github.com/InkyQuill/x-skills/internal/remote is not in std
```

- [x] **Step 3: Implement metadata helpers**

Create `internal/remote/source.go`:

```go
package remote

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const MetadataFile = ".x-skills.json"

const (
	SourceTypeGitHub = "github"
	SourceTypeGit    = "git"
)

type SourceMetadata struct {
	SourceType   string `json:"source_type"`
	Owner        string `json:"owner,omitempty"`
	Repo         string `json:"repo,omitempty"`
	CloneURL     string `json:"clone_url"`
	Ref          string `json:"ref,omitempty"`
	Commit       string `json:"commit"`
	SkillPath    string `json:"skill_path"`
	UpstreamName string `json:"upstream_name,omitempty"`
}

func ReadSourceMetadata(skillDir string) (SourceMetadata, bool, error) {
	data, err := os.ReadFile(filepath.Join(skillDir, MetadataFile))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return SourceMetadata{}, false, nil
		}
		return SourceMetadata{}, false, fmt.Errorf("read source metadata: %w", err)
	}
	var meta SourceMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return SourceMetadata{}, false, fmt.Errorf("parse source metadata: %w", err)
	}
	return meta, true, nil
}

func WriteSourceMetadata(skillDir string, meta SourceMetadata) error {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("encode source metadata: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(filepath.Join(skillDir, MetadataFile), data, 0o644); err != nil {
		return fmt.Errorf("write source metadata: %w", err)
	}
	return nil
}

func (m SourceMetadata) SameIdentity(other SourceMetadata) bool {
	if m.SourceType == "" || other.SourceType == "" || m.SourceType != other.SourceType {
		return false
	}
	if m.SourceType == SourceTypeGitHub {
		return strings.EqualFold(m.Owner, other.Owner) &&
			strings.EqualFold(m.Repo, other.Repo) &&
			cleanSkillPath(m.SkillPath) == cleanSkillPath(other.SkillPath)
	}
	return m.CloneURL == other.CloneURL && cleanSkillPath(m.SkillPath) == cleanSkillPath(other.SkillPath)
}

func cleanSkillPath(path string) string {
	return strings.Trim(strings.ReplaceAll(path, `\`, `/`), `/`)
}
```

- [x] **Step 4: Expose metadata on repo skills**

Modify `internal/repo/repo.go`:

```go
type Skill struct {
	Name        string
	Path        string
	Description string
	Source      *remote.SourceMetadata
}
```

Add import:

```go
	"github.com/InkyQuill/x-skills/internal/remote"
```

Inside `List`, after reading skill metadata:

```go
		source, ok, err := remote.ReadSourceMetadata(path)
		if err != nil {
			source = remote.SourceMetadata{}
			ok = false
		}
		var sourcePtr *remote.SourceMetadata
		if ok {
			sourcePtr = &source
		}
```

Set `Source: sourcePtr` in the appended `Skill`.

- [x] **Step 5: Verify and commit**

Run:

```bash
gofmt -w internal/remote/source.go internal/remote/source_test.go internal/repo/repo.go
go test ./internal/remote -run 'TestSource|TestReadSource' -count=1 -v
go test ./internal/repo -count=1
```

Expected: PASS.

Commit:

```bash
git add internal/remote/source.go internal/remote/source_test.go internal/repo/repo.go
git commit -m "feat: track remote source metadata"
```

---

### Task 2: skills.sh Search Client

**Files:**
- Create: `internal/remote/search.go`
- Create: `internal/remote/search_test.go`

- [x] **Step 1: Write search client tests**

Create `internal/remote/search_test.go`:

```go
package remote

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
				"name": "svelte-coder", "description": "Svelte help.", "owner": "vercel-labs",
				"repo": "skills", "path": "skills/svelte-coder", "installs": 812,
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
}
```

- [x] **Step 2: Run failing tests**

Run:

```bash
go test ./internal/remote -run TestSearch -count=1 -v
```

Expected: FAIL with undefined `NewSearchClient`, `SearchRequest`, and missing `strings` import in the test. Add `strings` to the test imports before implementing.

- [x] **Step 3: Implement search client**

Create `internal/remote/search.go`:

```go
package remote

import (
	"context"
	"encoding/json"
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
	Name        string `json:"name"`
	Description string `json:"description"`
	Owner       string `json:"owner"`
	Repo        string `json:"repo"`
	Path        string `json:"path"`
	Installs    int    `json:"installs"`
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

func (c SearchClient) Search(ctx context.Context, req SearchRequest) ([]SearchResult, error) {
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
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("search skills: HTTP %d", resp.StatusCode)
	}
	var payload struct {
		Results []SearchResult `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode search results: %w", err)
	}
	return payload.Results, nil
}
```

- [x] **Step 4: Verify and commit**

Run:

```bash
gofmt -w internal/remote/search.go internal/remote/search_test.go
go test ./internal/remote -run TestSearch -count=1 -v
```

Expected: PASS.

Commit:

```bash
git add internal/remote/search.go internal/remote/search_test.go
git commit -m "feat: add skills search client"
```

---

### Task 3: Git Checkout Cache And Skill Discovery

**Files:**
- Create: `internal/remote/git.go`
- Create: `internal/remote/git_test.go`

- [x] **Step 1: Write local-git tests**

Create `internal/remote/git_test.go` with helper local repos:

```go
package remote

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckoutCacheReusesCloneAndFindsSkill(t *testing.T) {
	repo := makeGitRepo(t)
	writeRemoteSkill(t, repo, "skills/svelte-coder", "svelte-coder", "Svelte help.")
	gitCommit(t, repo, "initial")

	cache := NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	checkout, err := cache.Checkout(t.Context(), GitSource{CloneURL: repo})
	if err != nil {
		t.Fatal(err)
	}
	second, err := cache.Checkout(t.Context(), GitSource{CloneURL: repo})
	if err != nil {
		t.Fatal(err)
	}
	if checkout.Path != second.Path {
		t.Fatalf("cache did not reuse checkout: %q != %q", checkout.Path, second.Path)
	}
	found, err := checkout.FindSkill("svelte-coder", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(found.SkillDir, filepath.Join("skills", "svelte-coder")) {
		t.Fatalf("skill dir = %q", found.SkillDir)
	}
	if found.Metadata.SkillPath != "skills/svelte-coder" || found.Metadata.Commit == "" {
		t.Fatalf("metadata = %#v", found.Metadata)
	}
}

func TestFindSkillReportsAmbiguousName(t *testing.T) {
	repo := makeGitRepo(t)
	writeRemoteSkill(t, repo, "packs/one", "dup-skill", "One.")
	writeRemoteSkill(t, repo, "packs/two", "dup-skill", "Two.")
	gitCommit(t, repo, "initial")
	cache := NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	checkout, err := cache.Checkout(t.Context(), GitSource{CloneURL: repo})
	if err != nil {
		t.Fatal(err)
	}
	_, err = checkout.FindSkill("dup-skill", "")
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("err = %v, want ambiguous", err)
	}
}

func makeGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test")
	return dir
}

func writeRemoteSkill(t *testing.T, root, rel, name, desc string) {
	t.Helper()
	dir := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	data := "---\nname: " + name + "\ndescription: " + desc + "\n---\n# " + name + "\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func gitCommit(t *testing.T, repo, msg string) {
	t.Helper()
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", msg)
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}
```

- [x] **Step 2: Run failing tests**

Run:

```bash
go test ./internal/remote -run 'TestCheckout|TestFindSkill' -count=1 -v
```

Expected: FAIL with undefined checkout types.

- [x] **Step 3: Implement git checkout cache**

Create `internal/remote/git.go`:

```go
package remote

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/InkyQuill/x-skills/internal/skills"
)

type GitSource struct {
	CloneURL string
	Ref      string
	Owner    string
	Repo     string
}

type CheckoutCache struct {
	root      string
	checkouts map[string]Checkout
}

type Checkout struct {
	Path     string
	Source   GitSource
	Commit   string
}

type FoundSkill struct {
	SkillDir string
	Info     skills.Info
	Metadata SourceMetadata
}

func NewCheckoutCache(root string) *CheckoutCache {
	return &CheckoutCache{root: root, checkouts: map[string]Checkout{}}
}

func (c *CheckoutCache) Checkout(ctx context.Context, source GitSource) (Checkout, error) {
	key := source.CloneURL + "@" + source.Ref
	if checkout, ok := c.checkouts[key]; ok {
		return checkout, nil
	}
	if err := os.MkdirAll(c.root, 0o755); err != nil {
		return Checkout{}, fmt.Errorf("create checkout cache: %w", err)
	}
	dir, err := os.MkdirTemp(c.root, "repo-*")
	if err != nil {
		return Checkout{}, fmt.Errorf("create checkout dir: %w", err)
	}
	args := []string{"clone", "--depth", "1"}
	if source.Ref != "" {
		args = append(args, "--branch", source.Ref)
	}
	args = append(args, source.CloneURL, dir)
	if err := runGit(ctx, "", args...); err != nil {
		return Checkout{}, err
	}
	commit, err := gitOutput(ctx, dir, "rev-parse", "HEAD")
	if err != nil {
		return Checkout{}, err
	}
	checkout := Checkout{Path: dir, Source: source, Commit: strings.TrimSpace(commit)}
	c.checkouts[key] = checkout
	return checkout, nil
}

func (c Checkout) FindSkill(name, preferredPath string) (FoundSkill, error) {
	if preferredPath != "" {
		return c.foundAt(filepath.Join(c.Path, filepath.FromSlash(preferredPath)), preferredPath)
	}
	var matches []string
	err := filepath.WalkDir(c.Path, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}
		if skills.IsDir(path) {
			info, err := skills.Read(path)
			if err == nil && (info.Name == name || filepath.Base(path) == name) {
				rel, _ := filepath.Rel(c.Path, path)
				matches = append(matches, filepath.ToSlash(rel))
			}
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		return FoundSkill{}, fmt.Errorf("find skill: %w", err)
	}
	if len(matches) == 0 {
		return FoundSkill{}, fmt.Errorf("skill %q not found in checkout", name)
	}
	if len(matches) > 1 {
		return FoundSkill{}, fmt.Errorf("ambiguous skill %q: %s", name, strings.Join(matches, ", "))
	}
	return c.foundAt(filepath.Join(c.Path, filepath.FromSlash(matches[0])), matches[0])
}

func (c Checkout) foundAt(path, rel string) (FoundSkill, error) {
	info, err := skills.Read(path)
	if err != nil {
		return FoundSkill{}, err
	}
	meta := SourceMetadata{
		SourceType:   SourceTypeGit,
		Owner:        c.Source.Owner,
		Repo:         c.Source.Repo,
		CloneURL:     c.Source.CloneURL,
		Ref:          c.Source.Ref,
		Commit:       c.Commit,
		SkillPath:    filepath.ToSlash(rel),
		UpstreamName: info.Name,
	}
	if c.Source.Owner != "" && c.Source.Repo != "" {
		meta.SourceType = SourceTypeGitHub
	}
	return FoundSkill{SkillDir: path, Info: info, Metadata: meta}, nil
}

func runGit(ctx context.Context, dir string, args ...string) error {
	_, err := gitOutput(ctx, dir, args...)
	return err
}

func gitOutput(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %v failed: %w\n%s", args, err, out)
	}
	return string(out), nil
}
```

- [x] **Step 4: Verify and commit**

Run:

```bash
gofmt -w internal/remote/git.go internal/remote/git_test.go
go test ./internal/remote -run 'TestCheckout|TestFindSkill' -count=1 -v
```

Expected: PASS.

Commit:

```bash
git add internal/remote/git.go internal/remote/git_test.go
git commit -m "feat: cache remote git checkouts"
```

---

### Task 4: Add Planning And Archive Apply

**Files:**
- Create: `internal/remote/add.go`
- Create: `internal/remote/add_test.go`

- [x] **Step 1: Write add planning tests**

Create `internal/remote/add_test.go`:

```go
package remote

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/skills"
)

func TestApplyArchiveOnlyCopiesSkillAndWritesMetadata(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	incoming := writeIncomingSkill(t, "svelte-coder", "Svelte help.")
	meta := SourceMetadata{SourceType: SourceTypeGitHub, Owner: "vercel-labs", Repo: "skills", CloneURL: "https://github.com/vercel-labs/skills.git", Commit: "abc", SkillPath: "skills/svelte-coder", UpstreamName: "svelte-coder"}

	result, err := ApplyArchive(AddRequest{Config: cfg, IncomingDir: incoming, ArchiveName: "svelte-coder", Metadata: meta, Conflict: ConflictReplaceArchive})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != AddStatusArchived {
		t.Fatalf("status = %q", result.Status)
	}
	info, err := skills.Read(filepath.Join(cfg.ArchiveSkillsRoot(), "svelte-coder"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Description != "Svelte help." {
		t.Fatalf("description = %q", info.Description)
	}
	if _, ok, err := ReadSourceMetadata(filepath.Join(cfg.ArchiveSkillsRoot(), "svelte-coder")); err != nil || !ok {
		t.Fatalf("source metadata missing: ok=%v err=%v", ok, err)
	}
}

func TestPlanArchiveDetectsNameConflictWithoutSourceIdentity(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	makeArchivedSkillForRemoteTest(t, cfg, "svelte-coder", "Local archived.")
	incoming := writeIncomingSkill(t, "svelte-coder", "Remote.")
	meta := SourceMetadata{SourceType: SourceTypeGitHub, Owner: "vercel-labs", Repo: "skills", SkillPath: "skills/svelte-coder"}
	plan, err := PlanArchive(cfg, incoming, "svelte-coder", meta)
	if err != nil {
		t.Fatal(err)
	}
	if plan.State != ArchiveStateNameConflict {
		t.Fatalf("state = %q, want name conflict", plan.State)
	}
}

func writeIncomingSkill(t *testing.T, name, desc string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: "+name+"\ndescription: "+desc+"\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func makeArchivedSkillForRemoteTest(t *testing.T, cfg config.Config, name, desc string) string {
	t.Helper()
	dir := filepath.Join(cfg.ArchiveSkillsRoot(), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: "+name+"\ndescription: "+desc+"\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}
```

- [x] **Step 2: Run failing tests**

Run:

```bash
go test ./internal/remote -run 'TestApplyArchive|TestPlanArchive' -count=1 -v
```

Expected: FAIL with undefined add APIs.

- [x] **Step 3: Implement archive plan/apply**

Create `internal/remote/add.go`:

```go
package remote

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/fingerprint"
	"github.com/InkyQuill/x-skills/internal/repo"
)

const (
	ArchiveStateNotArchived     = "not archived"
	ArchiveStateArchived        = "archived"
	ArchiveStateUpdateAvailable = "update available"
	ArchiveStateNameConflict    = "name conflict"

	ConflictReplaceArchive = "replace-archive"
	ConflictRenameIncoming = "rename-incoming"
	ConflictCancel         = "cancel"

	AddStatusArchived = "archived"
	AddStatusUpdated  = "updated"
	AddStatusSkipped  = "skipped"
)

type ArchivePlan struct {
	State       string
	ArchivePath string
	Existing    *SourceMetadata
}

type AddRequest struct {
	Config      config.Config
	IncomingDir string
	ArchiveName string
	Metadata    SourceMetadata
	Conflict    string
}

type AddResult struct {
	Name   string
	Path   string
	Status string
}

func PlanArchive(cfg config.Config, incomingDir, archiveName string, meta SourceMetadata) (ArchivePlan, error) {
	archivePath, err := repo.SkillPath(cfg, archiveName)
	if err != nil {
		return ArchivePlan{}, err
	}
	if !repo.HasSkill(cfg, archiveName) {
		return ArchivePlan{State: ArchiveStateNotArchived, ArchivePath: archivePath}, nil
	}
	existing, ok, err := ReadSourceMetadata(archivePath)
	if err != nil {
		return ArchivePlan{}, err
	}
	if !ok || !existing.SameIdentity(meta) {
		return ArchivePlan{State: ArchiveStateNameConflict, ArchivePath: archivePath}, nil
	}
	activeFP, err := fingerprint.Directory(incomingDir)
	if err != nil {
		return ArchivePlan{}, fmt.Errorf("fingerprint incoming: %w", err)
	}
	archiveFP, err := fingerprint.Directory(archivePath)
	if err != nil {
		return ArchivePlan{}, fmt.Errorf("fingerprint archive: %w", err)
	}
	if activeFP == archiveFP {
		return ArchivePlan{State: ArchiveStateArchived, ArchivePath: archivePath, Existing: &existing}, nil
	}
	return ArchivePlan{State: ArchiveStateUpdateAvailable, ArchivePath: archivePath, Existing: &existing}, nil
}

func ApplyArchive(req AddRequest) (AddResult, error) {
	if req.Conflict == ConflictCancel {
		return AddResult{Name: req.ArchiveName, Status: AddStatusSkipped}, nil
	}
	if req.Conflict == "" {
		req.Conflict = ConflictReplaceArchive
	}
	archivePath, err := repo.SkillPath(req.Config, req.ArchiveName)
	if err != nil {
		return AddResult{}, err
	}
	if req.Conflict == ConflictReplaceArchive {
		if err := os.RemoveAll(archivePath); err != nil {
			return AddResult{}, fmt.Errorf("replace archive: %w", err)
		}
	}
	if err := copyDir(req.IncomingDir, archivePath); err != nil {
		return AddResult{}, err
	}
	if err := WriteSourceMetadata(archivePath, req.Metadata); err != nil {
		return AddResult{}, err
	}
	return AddResult{Name: req.ArchiveName, Path: archivePath, Status: AddStatusArchived}, nil
}

func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	return filepath.WalkDir(src, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil || rel == "." {
			return err
		}
		target := filepath.Join(dst, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode())
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, in)
		return err
	})
}
```

- [x] **Step 4: Verify and commit**

Run:

```bash
gofmt -w internal/remote/add.go internal/remote/add_test.go
go test ./internal/remote -run 'TestApplyArchive|TestPlanArchive' -count=1 -v
```

Expected: PASS.

Commit:

```bash
git add internal/remote/add.go internal/remote/add_test.go
git commit -m "feat: plan and apply remote archives"
```

---

### Task 5: Install Tab Shell

**Files:**
- Create: `internal/tui/install.go`
- Create: `internal/tui/install_test.go`
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/views.go`
- Modify: `internal/tui/keys.go`
- Modify: `internal/tui/modal_help.go`

- [x] **Step 1: Write shell tests**

Create `internal/tui/install_test.go`:

```go
package tui

import (
	"strings"
	"testing"

	"github.com/InkyQuill/x-skills/internal/config"
)

func TestInstallTabSwitchesAndRendersShell(t *testing.T) {
	m := New(config.Default(t.TempDir(), t.TempDir()))
	m.width = 120
	m.height = 30
	updated, _ := m.Update(keyRunes("I"))
	m = mustModel(t, updated)
	if m.view != ViewInstall {
		t.Fatalf("view = %q, want install", m.view)
	}
	view := plain(m.View())
	for _, want := range []string{"I:Install", "Install: search", "type at least 2 characters", "/ search", "i install & use", "a archive only"} {
		if !strings.Contains(view, want) {
			t.Fatalf("install shell missing %q:\n%s", want, view)
		}
	}
}

func TestInstallHelpShowsRealInstallKeys(t *testing.T) {
	m := New(config.Default(t.TempDir(), t.TempDir()))
	view := plain(newHelpModal().View(100, 40, m))
	for _, want := range []string{"switch to Install view", "Install: / search", "Install: i install and use", "Install: a archive only"} {
		if !strings.Contains(view, want) {
			t.Fatalf("help missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "not yet available") {
		t.Fatalf("help still says install is unavailable:\n%s", view)
	}
}
```

- [x] **Step 2: Run failing tests**

Run:

```bash
go test ./internal/tui -run 'TestInstallTabSwitches|TestInstallHelp' -count=1 -v
```

Expected: FAIL because `ViewInstall` and key handling do not exist.

- [x] **Step 3: Add install state and key**

Create `internal/tui/install.go`:

```go
package tui

import (
	"net/http"

	"github.com/InkyQuill/x-skills/internal/remote"
)

type installState struct {
	Query        string
	Owner        string
	Searching    bool
	Results      []installResultView
	Message      string
	searchClient remote.SearchClient
	checkouts     *remote.CheckoutCache
}

type installResultView struct {
	Result       remote.SearchResult
	ArchiveState string
	AuditPill    string
}

func newInstallState() installState {
	return installState{
		Message:      "type at least 2 characters",
		searchClient: remote.NewSearchClient(remote.DefaultSearchEndpoint, http.DefaultClient),
	}
}
```

Modify `internal/tui/keys.go`:

```go
const (
	keyActive  = "A"
	keyRepo    = "R"
	keyDoctor  = "D"
	keyInstall = "I"
	keyHelp    = "?"
)
```

Modify `internal/tui/model.go`:

```go
const (
	ViewActive  ViewName = "active"
	ViewRepo    ViewName = "repo"
	ViewDoctor  ViewName = "doctor"
	ViewInstall ViewName = "install"
)
```

Add field:

```go
	install installState
```

Initialize in `New`:

```go
		install: newInstallState(),
```

Add selected map entry:

```go
			ViewInstall: {},
```

Add key switch case:

```go
	case keyInstall:
		m.setView(ViewInstall)
```

- [x] **Step 4: Render install shell**

Modify `internal/tui/views.go`:

In `renderHeader`, add:

```go
		tabLabel(m.view == ViewInstall, "I", "Install"),
```

In `renderListPanel`, add:

```go
	case ViewInstall:
		title = installPanelTitle(m)
		rows = renderInstallRows(m, width)
```

Add functions:

```go
func installPanelTitle(m Model) string {
	query := m.install.Query
	if query == "" {
		return "Install: search"
	}
	if m.install.Owner != "" {
		return fmt.Sprintf("Install: search %q  owner: %s", query, m.install.Owner)
	}
	return fmt.Sprintf("Install: search %q", query)
}

func renderInstallRows(m Model, width int) []string {
	rows := []string{accentStyle.Render("/ search: " + m.install.Query + "_")}
	if len(m.install.Results) == 0 {
		rows = append(rows, mutedStyle.Render(m.install.Message))
		return rows
	}
	for i, result := range m.install.Results {
		prefix := cursorPrefix(m, i)
		pill := result.AuditPill
		if pill != "" {
			pill = " " + pill
		}
		rows = append(rows, selectableRow([]rowSegment{
			{text: fmt.Sprintf("%s %s  %s  %s%s  %s", prefix, result.Result.Name, result.Result.Source(), result.ArchiveState, pill, result.Result.Description)},
		}, i == m.cursor, false, width-6))
	}
	return rows
}
```

In `renderInspector`, add:

```go
	case ViewInstall:
		if m.cursor >= 0 && m.cursor < len(m.install.Results) {
			result := m.install.Results[m.cursor]
			lines = append(lines,
				"◇ "+result.Result.Name,
				result.Result.Source(),
				"skill", "  "+result.Result.Name,
				"installs", fmt.Sprintf("  %d", result.Result.Installs),
				"status", "  "+result.ArchiveState,
			)
			if result.AuditPill != "" {
				lines = append(lines, "audit", "  "+result.AuditPill)
			}
		}
```

In `commandPalette`, add:

```go
	case ViewInstall:
		return renderCommandPalette(m.opts.ASCII, []tuiui.Shortcut{
			{ASCII: "enter", Unicode: "↵", Label: "preview"},
			{ASCII: "/", Label: "search"},
			{ASCII: "o", Label: "owner"},
			{ASCII: "i", Label: "install & use"},
			{ASCII: "a", Label: "archive only"},
			{ASCII: "^R", Label: "refresh"},
			{ASCII: "?", Label: "help"},
			{ASCII: "q", Label: "quit"},
		})
```

Modify `internal/tui/modal_help.go`, replacing the Install line and adding Install-specific keys:

```go
		"  " + helpCommand(m.opts.ASCII, tuiui.Shortcut{ASCII: "I", Label: "switch to Install view"}),
		"  " + helpCommand(m.opts.ASCII, tuiui.Shortcut{ASCII: "/", Label: "Install: / search"}),
		"  " + helpCommand(m.opts.ASCII, tuiui.Shortcut{ASCII: "o", Label: "Install: edit owner filter"}),
		"  " + helpCommand(m.opts.ASCII, tuiui.Shortcut{ASCII: "i", Label: "Install: i install and use"}),
		"  " + helpCommand(m.opts.ASCII, tuiui.Shortcut{ASCII: "a", Label: "Install: a archive only"}),
```

- [x] **Step 5: Verify and commit**

Run:

```bash
gofmt -w internal/tui/install.go internal/tui/install_test.go internal/tui/model.go internal/tui/views.go internal/tui/keys.go internal/tui/modal_help.go
go test ./internal/tui -run 'TestInstallTabSwitches|TestInstallHelp|TestModelSwitchesViews' -count=1 -v
```

Expected: PASS.

Commit:

```bash
git add internal/tui/install.go internal/tui/install_test.go internal/tui/model.go internal/tui/views.go internal/tui/keys.go internal/tui/modal_help.go
git commit -m "feat: add install tab shell"
```

---

### Task 6: Install Search Input And Async Results

**Files:**
- Modify: `internal/tui/install.go`
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/install_test.go`

- [x] **Step 1: Write search workflow tests**

Append to `internal/tui/install_test.go`:

```go
func TestInstallSearchRunsAfterEnterAndKeepsResults(t *testing.T) {
	m := New(config.Default(t.TempDir(), t.TempDir()))
	m.install.searchClient = remote.NewSearchClient(testSearchServer(t, []remote.SearchResult{
		{Name: "svelte-coder", Description: "Svelte help.", Owner: "vercel-labs", Repo: "skills", Path: "skills/svelte-coder", Installs: 812},
	}), http.DefaultClient)
	m.setView(ViewInstall)

	updated, cmd := m.Update(keyRunes("/"))
	m = mustModel(t, updated)
	for _, key := range []string{"s", "v", "e", "l", "t", "e"} {
		updated, cmd = m.Update(keyRunes(key))
		m = mustModel(t, updated)
		_ = cmd
	}
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	if !m.install.Searching {
		t.Fatal("searching = false")
	}
	msg := cmd().(installSearchResultMsg)
	updated, _ = m.Update(msg)
	m = mustModel(t, updated)
	if len(m.install.Results) != 1 || m.install.Results[0].Result.Name != "svelte-coder" {
		t.Fatalf("results = %#v", m.install.Results)
	}
	if m.status != "found 1 result for \"svelte\"" {
		t.Fatalf("status = %q", m.status)
	}
}
```

Add helper imports `net/http`, `net/http/httptest`, `encoding/json`, `github.com/InkyQuill/x-skills/internal/remote`, and Bubble Tea if missing. Add helper:

```go
func testSearchServer(t *testing.T, results []remote.SearchResult) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"results": results})
	}))
	t.Cleanup(server.Close)
	return server.URL
}
```

- [x] **Step 2: Run failing test**

Run:

```bash
go test ./internal/tui -run TestInstallSearchRunsAfterEnterAndKeepsResults -count=1 -v
```

Expected: FAIL with missing search message/handler.

- [x] **Step 3: Implement install input state and command**

Modify `internal/tui/install.go`:

```go
type installInputMode int

const (
	installInputNone installInputMode = iota
	installInputQuery
	installInputOwner
)

type installSearchResultMsg struct {
	token   int
	query   string
	results []remote.SearchResult
	err     error
}
```

Add fields to `installState`:

```go
	InputMode   installInputMode
	searchToken int
```

Add functions:

```go
func (m Model) runInstallSearch() tea.Cmd {
	token := m.install.searchToken
	query := m.install.Query
	owner := m.install.Owner
	client := m.install.searchClient
	return func() tea.Msg {
		results, err := client.Search(context.Background(), remote.SearchRequest{Query: query, Owner: owner, Limit: remote.DefaultSearchLimit})
		return installSearchResultMsg{token: token, query: query, results: results, err: err}
	}
}

func (m *Model) startInstallSearch() tea.Cmd {
	if len([]rune(strings.TrimSpace(m.install.Query))) < 2 {
		m.install.Message = "type at least 2 characters"
		return nil
	}
	m.install.searchToken++
	m.install.Searching = true
	m.install.Message = "searching..."
	return m.runInstallSearch()
}
```

Add imports: `context`, `strings`, `tea`.

- [x] **Step 4: Wire key handling and result message**

Modify `internal/tui/model.go` in `Update`:

```go
	case installSearchResultMsg:
		m.applyInstallSearchResult(msg)
		return m, nil
```

In `handleKey`, before global non-modal actions:

```go
	if m.view == ViewInstall && m.install.InputMode != installInputNone {
		cmd := m.updateInstallInput(msg)
		return m, cmd
	}
```

In key switch:

```go
	case "/":
		if m.view == ViewInstall {
			m.install.InputMode = installInputQuery
			m.status = ""
		} else if m.view == ViewActive || m.view == ViewRepo {
			m.filter = newFilterState()
			m.filter.Active = true
			m.filter.input.Focus()
		}
	case "o":
		if m.view == ViewInstall {
			m.install.InputMode = installInputOwner
			m.status = ""
		}
```

Create these in `internal/tui/install.go`:

```go
func (m *Model) updateInstallInput(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		m.install.InputMode = installInputNone
		return nil
	case "enter":
		m.install.InputMode = installInputNone
		m.cursor = 0
		return m.startInstallSearch()
	case "backspace":
		if m.install.InputMode == installInputQuery {
			m.install.Query = trimLastRune(m.install.Query)
		} else {
			m.install.Owner = trimLastRune(m.install.Owner)
		}
	default:
		if msg.Type == tea.KeyRunes {
			if m.install.InputMode == installInputQuery {
				m.install.Query += string(msg.Runes)
			} else {
				m.install.Owner += string(msg.Runes)
			}
		}
	}
	return nil
}

func trimLastRune(value string) string {
	runes := []rune(value)
	if len(runes) == 0 {
		return ""
	}
	return string(runes[:len(runes)-1])
}

func (m *Model) applyInstallSearchResult(msg installSearchResultMsg) {
	if msg.token != m.install.searchToken {
		return
	}
	m.install.Searching = false
	if msg.err != nil {
		m.install.Message = msg.err.Error()
		m.status = msg.err.Error()
		return
	}
	m.install.Results = make([]installResultView, 0, len(msg.results))
	for _, result := range msg.results {
		m.install.Results = append(m.install.Results, installResultView{
			Result:       result,
			ArchiveState: m.installArchiveState(result),
		})
	}
	count := len(msg.results)
	if count == 1 {
		m.status = "found 1 result for " + strconv.Quote(msg.query)
	} else {
		m.status = fmt.Sprintf("found %d results for %q", count, msg.query)
	}
}
```

Add imports `fmt`, `strconv`.

- [x] **Step 5: Add archive-state helper**

In `internal/tui/install.go`:

```go
func (m Model) installArchiveState(result remote.SearchResult) string {
	meta := remote.SourceMetadata{
		SourceType: remote.SourceTypeGitHub,
		Owner:      result.Owner,
		Repo:       result.Repo,
		SkillPath:  result.Path,
	}
	archivePath, err := repo.SkillPath(m.cfg, result.Name)
	if err != nil || !repo.HasSkill(m.cfg, result.Name) {
		return remote.ArchiveStateNotArchived
	}
	existing, ok, err := remote.ReadSourceMetadata(archivePath)
	if err != nil || !ok || !existing.SameIdentity(meta) {
		return remote.ArchiveStateNameConflict
	}
	return remote.ArchiveStateArchived
}
```

Add imports `repo` and use `remote.SourceTypeGitHub`.

- [x] **Step 6: Verify and commit**

Run:

```bash
gofmt -w internal/tui/install.go internal/tui/install_test.go internal/tui/model.go
go test ./internal/tui -run TestInstallSearchRunsAfterEnterAndKeepsResults -count=1 -v
go test ./internal/tui -count=1
```

Expected: PASS.

Commit:

```bash
git add internal/tui/install.go internal/tui/install_test.go internal/tui/model.go
git commit -m "feat: search from install tab"
```

---

### Task 7: Remote Preview

**Files:**
- Modify: `internal/tui/install.go`
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/install_test.go`

- [x] **Step 1: Write preview test with local git source**

Append to `internal/tui/install_test.go`:

```go
func TestInstallEnterPreviewsRemoteSkill(t *testing.T) {
	repoDir := makeGitRepo(t)
	writeRemoteSkill(t, repoDir, "skills/svelte-coder", "svelte-coder", "Svelte help.")
	gitCommit(t, repoDir, "initial")

	m := New(config.Default(t.TempDir(), t.TempDir()))
	m.setView(ViewInstall)
	m.install.checkouts = remote.NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	m.install.Results = []installResultView{{
		Result: remote.SearchResult{Name: "svelte-coder", Description: "Svelte help.", Owner: "local", Repo: "repo", Path: "skills/svelte-coder"},
		ArchiveState: remote.ArchiveStateNotArchived,
	}}
	m.install.Results[0].Result.Owner = ""
	m.install.Results[0].Result.Repo = ""
	m.install.Results[0].Result.Path = "skills/svelte-coder"
	m.install.testCloneURL = repoDir

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	msg := cmd().(installPreviewMsg)
	updated, _ = m.Update(msg)
	m = mustModel(t, updated)
	if m.modal == nil {
		t.Fatal("preview modal is nil")
	}
	view := plain(m.modal.View(100, 30, m))
	if !strings.Contains(view, "Preview: svelte-coder") || !strings.Contains(view, "Svelte help.") {
		t.Fatalf("preview missing remote content:\n%s", view)
	}
}
```

Add local test helpers to `internal/tui/install_test.go`:

```go
func makeTUITestGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runTUITestGit(t, dir, "init")
	runTUITestGit(t, dir, "config", "user.email", "test@example.com")
	runTUITestGit(t, dir, "config", "user.name", "Test")
	return dir
}

func writeTUITestRemoteSkill(t *testing.T, root, rel, name, desc string) {
	t.Helper()
	dir := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	data := "---\nname: " + name + "\ndescription: " + desc + "\n---\n# " + name + "\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func gitTUITestCommit(t *testing.T, repo, msg string) {
	t.Helper()
	runTUITestGit(t, repo, "add", ".")
	runTUITestGit(t, repo, "commit", "-m", msg)
}

func runTUITestGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}
```

- [x] **Step 2: Run failing test**

Run:

```bash
go test ./internal/tui -run TestInstallEnterPreviewsRemoteSkill -count=1 -v
```

Expected: FAIL with undefined preview message/handler.

- [x] **Step 3: Implement preview command**

In `internal/tui/install.go`, add:

```go
type installPreviewMsg struct {
	name string
	path string
	err  error
}

func (m Model) selectedInstallResult() (installResultView, bool) {
	if m.cursor < 0 || m.cursor >= len(m.install.Results) {
		return installResultView{}, false
	}
	return m.install.Results[m.cursor], true
}

func (m Model) previewInstallResult() tea.Cmd {
	row, ok := m.selectedInstallResult()
	if !ok {
		return nil
	}
	checkouts := m.install.checkouts
	if checkouts == nil {
		checkouts = remote.NewCheckoutCache(filepath.Join(os.TempDir(), "x-skills-tui-checkouts"))
	}
	source := m.gitSourceForInstall(row.Result)
	return func() tea.Msg {
		checkout, err := checkouts.Checkout(context.Background(), source)
		if err != nil {
			return installPreviewMsg{name: row.Result.Name, err: err}
		}
		found, err := checkout.FindSkill(row.Result.Name, row.Result.Path)
		if err != nil {
			return installPreviewMsg{name: row.Result.Name, err: err}
		}
		return installPreviewMsg{name: row.Result.Name, path: found.SkillDir}
	}
}

func (m Model) gitSourceForInstall(result remote.SearchResult) remote.GitSource {
	return remote.GitSource{
		Owner:    result.Owner,
		Repo:     result.Repo,
		CloneURL: "https://github.com/" + result.Owner + "/" + result.Repo + ".git",
	}
}
```

For tests, add optional field to `installState`:

```go
	testCloneURL string
```

And in `gitSourceForInstall`, first:

```go
	if m.install.testCloneURL != "" {
		return remote.GitSource{CloneURL: m.install.testCloneURL}
	}
```

- [x] **Step 4: Wire Enter on Install**

Modify `internal/tui/model.go` in `case "enter"`:

```go
	case "enter":
		if m.view == ViewInstall {
			return m, m.previewInstallResult()
		}
		m.openDetailModal()
```

Modify `Update`:

```go
	case installPreviewMsg:
		if msg.err != nil {
			m.status = msg.err.Error()
			return m, nil
		}
		m.modal = newPreviewModal("Preview: "+msg.name, msg.path)
		return m, nil
```

- [x] **Step 5: Verify and commit**

Run:

```bash
gofmt -w internal/tui/install.go internal/tui/install_test.go internal/tui/model.go
go test ./internal/tui -run TestInstallEnterPreviewsRemoteSkill -count=1 -v
```

Expected: PASS.

Commit:

```bash
git add internal/tui/install.go internal/tui/install_test.go internal/tui/model.go
git commit -m "feat: preview install results"
```

---

### Task 8: Archive Only Action

**Files:**
- Modify: `internal/tui/install.go`
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/install_test.go`

- [x] **Step 1: Write archive-only test**

Append:

```go
func TestInstallArchiveOnlyArchivesRemoteSkillAndStaysOnInstall(t *testing.T) {
	repoDir := makeTUITestGitRepo(t)
	writeTUITestRemoteSkill(t, repoDir, "skills/svelte-coder", "svelte-coder", "Svelte help.")
	gitTUITestCommit(t, repoDir, "initial")

	cfg := config.Default(t.TempDir(), t.TempDir())
	m := New(cfg)
	m.setView(ViewInstall)
	m.install.checkouts = remote.NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	m.install.testCloneURL = repoDir
	m.install.Results = []installResultView{{Result: remote.SearchResult{Name: "svelte-coder", Path: "skills/svelte-coder"}, ArchiveState: remote.ArchiveStateNotArchived}}

	updated, cmd := m.Update(keyRunes("a"))
	m = mustModel(t, updated)
	msg := cmd().(installArchiveMsg)
	updated, _ = m.Update(msg)
	m = mustModel(t, updated)
	if m.view != ViewInstall {
		t.Fatalf("view = %q, want install", m.view)
	}
	if m.status != "archived svelte-coder" {
		t.Fatalf("status = %q", m.status)
	}
	if _, err := os.Stat(filepath.Join(cfg.ArchiveSkillsRoot(), "svelte-coder", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	if m.install.Results[0].ArchiveState != remote.ArchiveStateArchived {
		t.Fatalf("archive state = %q", m.install.Results[0].ArchiveState)
	}
}
```

- [x] **Step 2: Run failing test**

Run:

```bash
go test ./internal/tui -run TestInstallArchiveOnlyArchivesRemoteSkillAndStaysOnInstall -count=1 -v
```

Expected: FAIL with undefined archive message/action.

- [x] **Step 3: Implement archive command**

In `internal/tui/install.go`:

```go
type installArchiveMsg struct {
	name string
	err  error
}

func (m Model) archiveSelectedInstallResult() tea.Cmd {
	row, ok := m.selectedInstallResult()
	if !ok {
		return nil
	}
	return m.archiveInstallResult(row, false)
}

func (m Model) archiveInstallResult(row installResultView, useNow bool) tea.Cmd {
	cfg := m.cfg
	checkouts := m.install.checkouts
	if checkouts == nil {
		checkouts = remote.NewCheckoutCache(filepath.Join(os.TempDir(), "x-skills-tui-checkouts"))
	}
	source := m.gitSourceForInstall(row.Result)
	return func() tea.Msg {
		checkout, err := checkouts.Checkout(context.Background(), source)
		if err != nil {
			return installArchiveMsg{name: row.Result.Name, err: err}
		}
		found, err := checkout.FindSkill(row.Result.Name, row.Result.Path)
		if err != nil {
			return installArchiveMsg{name: row.Result.Name, err: err}
		}
		_, err = remote.ApplyArchive(remote.AddRequest{
			Config:      cfg,
			IncomingDir: found.SkillDir,
			ArchiveName: row.Result.Name,
			Metadata:    found.Metadata,
			Conflict:    remote.ConflictReplaceArchive,
		})
		if err != nil {
			return installArchiveMsg{name: row.Result.Name, err: err}
		}
		return installArchiveMsg{name: row.Result.Name}
	}
}
```

Wire key `a` in `model.go`:

```go
	case "a":
		if m.view == ViewInstall {
			return m, m.archiveSelectedInstallResult()
		}
```

Handle message:

```go
	case installArchiveMsg:
		if msg.err != nil {
			m.status = msg.err.Error()
			return m, nil
		}
		m.reload()
		m.refreshInstallArchiveStates()
		m.status = "archived " + msg.name
		return m, nil
```

Add helper:

```go
func (m *Model) refreshInstallArchiveStates() {
	for i := range m.install.Results {
		m.install.Results[i].ArchiveState = m.installArchiveState(m.install.Results[i].Result)
	}
}
```

- [x] **Step 4: Verify and commit**

Run:

```bash
gofmt -w internal/tui/install.go internal/tui/install_test.go internal/tui/model.go
go test ./internal/tui -run TestInstallArchiveOnlyArchivesRemoteSkillAndStaysOnInstall -count=1 -v
```

Expected: PASS.

Commit:

```bash
git add internal/tui/install.go internal/tui/install_test.go internal/tui/model.go
git commit -m "feat: archive install results"
```

---

### Task 9: Install And Use Destination Checklist

**Files:**
- Modify: `internal/tui/install.go`
- Modify: `internal/tui/actions.go`
- Modify: `internal/tui/install_test.go`

- [x] **Step 1: Write install-and-use test**

Append:

```go
func TestInstallAndUseLinksProjectAgentsByDefault(t *testing.T) {
	repoDir := makeTUITestGitRepo(t)
	writeTUITestRemoteSkill(t, repoDir, "skills/svelte-coder", "svelte-coder", "Svelte help.")
	gitTUITestCommit(t, repoDir, "initial")

	cfg := config.Default(t.TempDir(), t.TempDir())
	m := New(cfg)
	m.setView(ViewInstall)
	m.install.checkouts = remote.NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	m.install.testCloneURL = repoDir
	m.install.Results = []installResultView{{Result: remote.SearchResult{Name: "svelte-coder", Path: "skills/svelte-coder"}, ArchiveState: remote.ArchiveStateNotArchived}}

	updated, _ := m.Update(keyRunes("i"))
	m = mustModel(t, updated)
	if m.modal == nil {
		t.Fatal("destination modal is nil")
	}
	view := plain(m.modal.View(120, 30, m))
	if !strings.Contains(view, "Install and use svelte-coder") || !strings.Contains(view, "[x] .Ag") {
		t.Fatalf("destination modal missing default project agents:\n%s", view)
	}
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mustModel(t, updated)
	msg := cmd().(installUseMsg)
	updated, _ = m.Update(msg)
	m = mustModel(t, updated)
	active := filepath.Join(cfg.MustActiveRoot("project", "agents"), "svelte-coder")
	resolved, err := filepath.EvalSymlinks(active)
	if err != nil {
		t.Fatal(err)
	}
	if resolved != filepath.Join(cfg.ArchiveSkillsRoot(), "svelte-coder") {
		t.Fatalf("resolved = %q", resolved)
	}
}
```

- [x] **Step 2: Run failing test**

Run:

```bash
go test ./internal/tui -run TestInstallAndUseLinksProjectAgentsByDefault -count=1 -v
```

Expected: FAIL because `i` is not wired.

- [x] **Step 3: Add destination modal**

In `internal/tui/install.go`, define:

```go
type installDestination struct {
	Scope   string
	Target  string
	Label   string
	Checked bool
}

type installDestinationModal struct {
	name         string
	row          installResultView
	destinations []installDestination
	cursor       int
}

func newInstallDestinationModal(row installResultView) modal {
	return installDestinationModal{name: row.Result.Name, row: row, destinations: []installDestination{
		{Scope: config.ScopeProject, Target: config.TargetAgents, Label: ".Ag", Checked: true},
		{Scope: config.ScopeProject, Target: config.TargetClaude, Label: ".Cl"},
		{Scope: config.ScopeProject, Target: config.TargetCodex, Label: ".Cd"},
		{Scope: config.ScopeGlobal, Target: config.TargetAgents, Label: "~Ag"},
		{Scope: config.ScopeGlobal, Target: config.TargetClaude, Label: "~Cl"},
		{Scope: config.ScopeGlobal, Target: config.TargetCodex, Label: "~Cd"},
	}}
}
```

Implement `Title`, `View`, and `Update` with:

```go
func (d installDestinationModal) Title() string { return "Install and use " + d.name }
```

Rows render `[x] .Ag` for checked and `[ ] .Cl` for unchecked; `space` toggles; `up/down` moves; `enter` calls `m.installAndUse(d.row, d.checked())`; `esc/q` closes. Return `false` when `enter` starts command so the command is preserved.

- [x] **Step 4: Add install-and-use command**

In `internal/tui/install.go`:

```go
type installUseMsg struct {
	name string
	err  error
}

func (m Model) installAndUse(row installResultView, destinations []installDestination) tea.Cmd {
	archiveCmd := m.archiveInstallResult(row, true)
	cfg := m.cfg
	return func() tea.Msg {
		archiveMsg := archiveCmd().(installArchiveMsg)
		if archiveMsg.err != nil {
			return installUseMsg{name: row.Result.Name, err: archiveMsg.err}
		}
		for _, dest := range destinations {
			_, err := actions.Link(cfg, actions.LinkRequest{Name: row.Result.Name, Scope: dest.Scope, Target: dest.Target})
			if err != nil {
				return installUseMsg{name: row.Result.Name, err: err}
			}
		}
		return installUseMsg{name: row.Result.Name}
	}
}
```

Wire key `i`:

```go
	case "i":
		if m.view == ViewInstall {
			if row, ok := m.selectedInstallResult(); ok {
				m.modal = newInstallDestinationModal(row)
			}
		}
```

Handle `installUseMsg`:

```go
	case installUseMsg:
		if msg.err != nil {
			m.status = msg.err.Error()
			return m, nil
		}
		m.reload()
		m.refreshInstallArchiveStates()
		m.modal = nil
		m.status = "installed " + msg.name + " to .Ag"
		return m, nil
```

- [x] **Step 5: Verify and commit**

Run:

```bash
gofmt -w internal/tui/install.go internal/tui/install_test.go internal/tui/model.go
go test ./internal/tui -run TestInstallAndUseLinksProjectAgentsByDefault -count=1 -v
```

Expected: PASS.

Commit:

```bash
git add internal/tui/install.go internal/tui/install_test.go internal/tui/model.go
git commit -m "feat: install and link remote skills"
```

---

### Task 10: Conflict And Update Branches

Status: completed

**Files:**
- Modify: `internal/remote/add.go`
- Modify: `internal/remote/add_test.go`
- Modify: `internal/tui/install.go`
- Modify: `internal/tui/install_test.go`
- Modify: `internal/tui/modal_diff.go`

- [x] **Step 1: Write same-name conflict test**

Append to `internal/tui/install_test.go`:

```go
func TestInstallArchiveOnlyNameConflictShowsChoice(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	makeSkill(t, cfg.ArchiveSkillsRoot(), "svelte-coder", "Local archived.")
	m := New(cfg)
	m.setView(ViewInstall)
	m.install.Results = []installResultView{{Result: remote.SearchResult{Name: "svelte-coder", Owner: "vercel-labs", Repo: "skills", Path: "skills/svelte-coder"}, ArchiveState: remote.ArchiveStateNameConflict}}

	updated, _ := m.Update(keyRunes("a"))
	m = mustModel(t, updated)
	if m.modal == nil {
		t.Fatal("conflict modal is nil")
	}
	view := plain(m.modal.View(120, 35, m))
	for _, want := range []string{"Name conflict: svelte-coder", "Replace archive", "Rename existing archive", "Rename incoming archive", "Cancel"} {
		if !strings.Contains(view, want) {
			t.Fatalf("name conflict modal missing %q:\n%s", want, view)
		}
	}
}
```

- [x] **Step 2: Write same-source update diff test**

Append:

```go
func TestInstallSameSourceUpdateOpensIncomingRemoteDiff(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	archived := makeSkill(t, cfg.ArchiveSkillsRoot(), "svelte-coder", "Old.")
	if err := remote.WriteSourceMetadata(archived, remote.SourceMetadata{SourceType: remote.SourceTypeGitHub, Owner: "vercel-labs", Repo: "skills", SkillPath: "skills/svelte-coder"}); err != nil {
		t.Fatal(err)
	}
	repoDir := makeTUITestGitRepo(t)
	writeTUITestRemoteSkill(t, repoDir, "skills/svelte-coder", "svelte-coder", "New.")
	gitTUITestCommit(t, repoDir, "initial")
	m := New(cfg)
	m.setView(ViewInstall)
	m.install.checkouts = remote.NewCheckoutCache(filepath.Join(t.TempDir(), "cache"))
	m.install.testCloneURL = repoDir
	m.install.Results = []installResultView{{Result: remote.SearchResult{Name: "svelte-coder", Owner: "vercel-labs", Repo: "skills", Path: "skills/svelte-coder"}, ArchiveState: remote.ArchiveStateUpdateAvailable}}

	updated, _ := m.Update(keyRunes("a"))
	m = mustModel(t, updated)
	if m.modal == nil {
		t.Fatal("diff modal is nil")
	}
	view := plain(m.modal.View(120, 40, m))
	if !strings.Contains(view, "Incoming remote") || !strings.Contains(view, "Archive conflict: svelte-coder") {
		t.Fatalf("update diff missing remote labels:\n%s", view)
	}
}
```

- [x] **Step 3: Run failing tests**

Run:

```bash
go test ./internal/tui -run 'TestInstallArchiveOnlyNameConflict|TestInstallSameSourceUpdate' -count=1 -v
```

Expected: FAIL because conflict branches are not wired.

- [x] **Step 4: Implement conflict routing**

In `archiveSelectedInstallResult`, before returning the archive command:

```go
	if row.ArchiveState == remote.ArchiveStateNameConflict {
		modal := newChoiceModal(
			"Name conflict: "+row.Result.Name,
			[]string{
				"Archive already contains a skill with this name from an unknown or different source.",
				"Choose exactly how to proceed.",
			},
			[]string{"Replace archive", "Rename existing archive", "Rename incoming archive", "Cancel"},
			0,
			func(current *Model, choice int) {
				current.applyInstallNameConflict(row, choice)
			},
		)
		return func() tea.Msg { return installOpenModalMsg{modal: modal} }
	}
	if row.ArchiveState == remote.ArchiveStateUpdateAvailable {
		return m.openInstallUpdateDiff(row)
	}
```

Add message:

```go
type installOpenModalMsg struct{ modal modal }
```

Handle it in `model.go`:

```go
	case installOpenModalMsg:
		m.modal = msg.modal
		return m, nil
```

Implement `openInstallUpdateDiff` to checkout incoming, call `buildDirectoryDiff(incoming, archived)`, and open:

```go
newConflictDiffModalWithModelApply(row.Result.Name, diff, "Incoming remote", func(current *Model, chosen string) {
	if chosen == actions.ConflictResolutionUseActive {
		current.applyInstallArchiveWithConflict(row, remote.ConflictReplaceArchive)
		return
	}
	current.status = "kept archive " + row.Result.Name
})
```

Use `actions.ConflictResolutionUseActive` to mean accept incoming remote, matching the diff modal's existing whole-directory decision semantics.

- [x] **Step 5: Implement name conflict choices**

Add in `internal/tui/install.go`:

```go
func (m *Model) applyInstallNameConflict(row installResultView, choice int) {
	switch choice {
	case 0:
		m.modal = nil
		m.applyInstallArchiveWithConflict(row, remote.ConflictReplaceArchive)
	case 1:
		m.modal = newInstallRenameModal(row, true)
	case 2:
		m.modal = newInstallRenameModal(row, false)
	default:
		m.modal = nil
		m.status = "cancelled install " + row.Result.Name
	}
}
```

Add the helper used above:

```go
func (m *Model) applyInstallArchiveWithConflict(row installResultView, conflict string) {
	cmd := m.archiveInstallResultWithConflict(row, conflict)
	msg := cmd().(installArchiveMsg)
	m.applyInstallArchiveMsg(msg)
}

func (m *Model) applyInstallArchiveMsg(msg installArchiveMsg) {
	if msg.err != nil {
		m.status = msg.err.Error()
		return
	}
	m.reload()
	m.refreshInstallArchiveStates()
	m.status = "archived " + msg.name
}
```

Create `internal/tui/modal_text.go`:

```go
package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type textModal struct {
	title string
	label string
	input textinput.Model
	apply func(*Model, string)
}

func newTextModal(title, label, value string, apply func(*Model, string)) modal {
	input := textinput.New()
	input.SetValue(value)
	input.Focus()
	return textModal{title: title, label: label, input: input, apply: apply}
}

func (t textModal) Title() string {
	return t.title
}

func (t textModal) View(width, height int, m Model) string {
	lines := []string{
		accentStyle.Render(t.title),
		"",
		t.label,
		t.input.View(),
		"",
		mutedStyle.Render(renderCommandPalette(m.opts.ASCII, []tuiui.Shortcut{
			{ASCII: "enter", Unicode: "↵", Label: "apply"},
			{ASCII: "esc", Unicode: "Esc", Label: "cancel"},
		})),
	}
	return modalStyle(width, height).Render(strings.Join(lines, "\n"))
}

func (t textModal) Update(msg tea.KeyMsg, m *Model) (bool, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		return true, nil
	case "enter":
		t.apply(m, strings.TrimSpace(t.input.Value()))
		return false, nil
	}
	var cmd tea.Cmd
	t.input, cmd = t.input.Update(msg)
	m.modal = t
	return false, cmd
}
```

Add `tuiui "github.com/InkyQuill/x-skills/internal/tui/ui"` to `modal_text.go` imports.

Add rename modal helpers in `internal/tui/install.go`:

```go
func newInstallRenameModal(row installResultView, renameExisting bool) modal {
	suffix := "-remote"
	title := "Rename incoming archive"
	if renameExisting {
		suffix = "-local"
		title = "Rename existing archive"
	}
	suggestion := row.Result.Name + suffix
	return newTextModal(title, "Archive name", suggestion, func(m *Model, name string) {
		if name == "" {
			m.status = "archive name is required"
			return
		}
		if renameExisting {
			m.renameExistingArchiveThenInstall(row, name)
			return
		}
		row.Result.Name = name
		m.applyInstallArchiveWithConflict(row, remote.ConflictRenameIncoming)
	})
}
```

Add this existing-archive rename helper in `internal/tui/install.go`:

```go
func (m *Model) renameExistingArchiveThenInstall(row installResultView, newName string) {
	oldPath, err := repo.SkillPath(m.cfg, row.Result.Name)
	if err != nil {
		m.status = err.Error()
		return
	}
	newPath, err := repo.SkillPath(m.cfg, newName)
	if err != nil {
		m.status = err.Error()
		return
	}
	if repo.HasSkill(m.cfg, newName) {
		m.status = "archive name already exists: " + newName
		return
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		m.status = err.Error()
		return
	}
	m.applyInstallArchiveWithConflict(row, remote.ConflictReplaceArchive)
}
```

Add `archiveInstallResultWithConflict` by extracting the existing `archiveInstallResult` body so the conflict value is passed through to `remote.ApplyArchive` instead of always using `remote.ConflictReplaceArchive`.

```go
func (m Model) archiveInstallResultWithConflict(row installResultView, conflict string) tea.Cmd {
	cfg := m.cfg
	checkouts := m.install.checkouts
	if checkouts == nil {
		checkouts = remote.NewCheckoutCache(filepath.Join(os.TempDir(), "x-skills-tui-checkouts"))
	}
	source := m.gitSourceForInstall(row.Result)
	return func() tea.Msg {
		checkout, err := checkouts.Checkout(context.Background(), source)
		if err != nil {
			return installArchiveMsg{name: row.Result.Name, err: err}
		}
		found, err := checkout.FindSkill(row.Result.Name, row.Result.Path)
		if err != nil {
			return installArchiveMsg{name: row.Result.Name, err: err}
		}
		_, err = remote.ApplyArchive(remote.AddRequest{
			Config:      cfg,
			IncomingDir: found.SkillDir,
			ArchiveName: row.Result.Name,
			Metadata:    found.Metadata,
			Conflict:    conflict,
		})
		if err != nil {
			return installArchiveMsg{name: row.Result.Name, err: err}
		}
		return installArchiveMsg{name: row.Result.Name}
	}
}
```

- [x] **Step 6: Verify and commit**

Run:

```bash
gofmt -w internal/tui/install.go internal/tui/install_test.go internal/tui/model.go
go test ./internal/tui -run 'TestInstallArchiveOnlyNameConflict|TestInstallSameSourceUpdate' -count=1 -v
```

Expected: PASS.

Commit:

```bash
git add internal/tui/install.go internal/tui/install_test.go internal/tui/model.go
git commit -m "feat: handle install conflicts"
```

---

### Task 11: Audit Plumbing Without Blocking Install

Status: completed

**Files:**
- Create: `internal/remote/audit.go`
- Create: `internal/remote/audit_test.go`
- Modify: `internal/tui/install.go`
- Modify: `internal/tui/install_test.go`

- [x] **Step 1: Write audit summarization tests**

Create `internal/remote/audit_test.go`:

```go
package remote

import "testing"

func TestAuditSummaryPill(t *testing.T) {
	tests := []struct {
		name string
		in   AuditSummary
		want string
	}{
		{name: "safe", in: AuditSummary{Available: true, Alerts: 0}, want: "✓ safe"},
		{name: "warn", in: AuditSummary{Available: true, Alerts: 2}, want: "⚠ warn"},
		{name: "risky", in: AuditSummary{Available: true, Critical: 1}, want: "‼ risky"},
		{name: "missing", in: AuditSummary{}, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.in.Pill(); got != tt.want {
				t.Fatalf("Pill() = %q, want %q", got, tt.want)
			}
		})
	}
}
```

- [x] **Step 2: Implement audit summary**

Create `internal/remote/audit.go`:

```go
package remote

type AuditSummary struct {
	Available bool
	Alerts    int
	Critical  int
}

func (a AuditSummary) Pill() string {
	if !a.Available {
		return ""
	}
	if a.Critical > 0 {
		return "‼ risky"
	}
	if a.Alerts > 0 {
		return "⚠ warn"
	}
	return "✓ safe"
}
```

- [x] **Step 3: Wire audit cache in TUI**

Add to `installState`:

```go
	Audit map[string]remote.AuditSummary
```

Initialize:

```go
		Audit: map[string]remote.AuditSummary{},
```

In `applyInstallSearchResult`, set:

```go
audit := m.install.Audit[result.Source()+"@"+result.Name]
```

And row `AuditPill: audit.Pill()`.

Do not show a pill when no audit data exists.

- [x] **Step 4: Verify and commit**

Run:

```bash
gofmt -w internal/remote/audit.go internal/remote/audit_test.go internal/tui/install.go internal/tui/install_test.go
go test ./internal/remote -run TestAuditSummaryPill -count=1 -v
go test ./internal/tui -run TestInstall -count=1
```

Expected: PASS.

Commit:

```bash
git add internal/remote/audit.go internal/remote/audit_test.go internal/tui/install.go internal/tui/install_test.go
git commit -m "feat: render install audit summaries"
```

---

### Task 12: Documentation And Final Verification

Status: completed

**Files:**
- Modify: `README.md`
- Modify: `docs/superpowers/specs/2026-07-06-go-tui-install-and-repo-updates-design.md` only if implementation intentionally narrows behavior.
- Test: full Go suite.

- [x] **Step 1: Update README TUI section**

Replace the Install wording in `README.md`:

```markdown
The TUI has Active, Repo, Doctor, and Install pages: press `A` for Active,
`R` for Repo, `D` for Doctor, and `I` for Install. Refresh is `ctrl+r`.

Use Install to search `skills.sh`, preview remote `SKILL.md`, archive a skill,
or install and link it into the current project. Generic Git and URL install
sources remain outside the TUI Install page for now.
```

- [x] **Step 2: Run final verification**

Run:

```bash
gofmt -w internal/remote/*.go internal/tui/*.go
go test ./internal/remote -count=1
go test ./internal/tui -count=1
go test ./cmd/... ./internal/... -count=1
go build -o bin/x-skills ./cmd/x-skills
```

Expected: all commands pass.

- [x] **Step 3: Manual smoke test**

Run:

```bash
./bin/x-skills tui
```

Manual checklist:

- Press `I`; Install tab opens and footer shows `/`, `o`, `i`, `a`.
- Press `/`, type `svelte`, press Enter; status becomes `searching...`, then results render or a network error appears in status without crashing.
- Move cursor; inspector updates.
- Press `enter`; preview opens or status explains the checkout/search error.
- Press `a` on a testable result; archive is created and app remains on Install.
- Press `i`; destination checklist opens with `.Ag` checked.

- [x] **Step 4: Commit**

```bash
git add README.md docs/superpowers/specs/2026-07-06-go-tui-install-and-repo-updates-design.md internal/remote internal/tui
git commit -m "docs: document install tab workflow"
```

---

## Self-Review

Spec coverage:

- Top-level `I` page: Task 5.
- Search query, owner filter, min length, async search: Task 6.
- Legacy endpoint and request shape: Task 2.
- Row model and inspector: Tasks 5 and 6.
- Preview via temp checkout: Task 7.
- Archive-only: Task 8.
- Install-and-use with destination checklist and `.Ag` default: Task 9.
- Name conflicts and same-source update diff: Task 10.
- Audit no-pill-when-unavailable and risk pills when data exists: Task 11.
- Stay on Install after actions: Tasks 8 and 9 tests.
- README truthfulness: Task 12.

Known intentional gaps:

- Repo update pipeline is not part of this Install-tab plan.
- CLI remote commands are not part of this TUI plan.
- Batch TUI installs remain deferred per the design doc.
- Real advisory service HTTP integration is represented by cache/render plumbing unless an existing endpoint contract is identified during execution.

Placeholder scan:

- No task contains TBD/TODO/fill-in placeholders.
- Conflict Task 10 uses `applyInstallArchiveWithConflict` and does not call `Update` recursively.

Type consistency:

- `remote.SearchResult`, `remote.SourceMetadata`, `remote.CheckoutCache`, `installResultView`, and `installState` are introduced before TUI tasks use them.
- `ArchiveState*` constants live in `internal/remote` and are referenced consistently by TUI tasks.
