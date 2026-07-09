# CLI Add Parity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement native `x-skills add` as the remote-install command surface aligned with `npx skills add`, while preserving x-skills archive-first and link-by-default behavior.

**Architecture:** Keep Cobra thin. Put source parsing and checkout discovery in `internal/remote`, destination parsing in `internal/cli`, and reuse `remote.ApplyArchive` plus `actions.Link` for mutation behavior. Batch installs run sequentially and produce human summaries; mutation JSON remains out of scope per ADR 0014.

**Tech Stack:** Go, Cobra, local `git` transport, existing `internal/remote`, `internal/actions`, `internal/cli`.

---

## Context And Constraints

- ADR 0011: command shape is `x-skills add SOURCE [SKILL_NAME...]`.
- ADR 0005: add links to project `.Ag` by default unless `--no-link`.
- ADR 0006: `--to` accepts compact destinations (`.Ag`, `~Cl`, `project:codex`, `g:agents`, etc.).
- ADR 0018: generic Git source is `x-skills add --git CLONE_URL SKILL_NAME`.
- Spec says `search` remains discovery-only and should point users to `add`.
- Do not implement URL archive/direct `SKILL.md` installs; ADR 0008 defers those.
- Do not implement mutation JSON for `add`; unsupported root `--json` should produce a clear error when that feature exists.

## File Structure

- Create `internal/remote/source_parse.go`: parse GitHub shorthand, GitHub tree URLs, `owner/repo@skill`, and generic `--git`.
- Create `internal/remote/source_parse_test.go`: source parsing coverage.
- Modify `internal/remote/git.go`: add `ListSkillsContext` for `--all` and ambiguous source discovery.
- Modify `internal/remote/git_test.go`: discovery tests.
- Create `internal/cli/destination.go`: parse repeatable `--to` selectors into scope/target pairs.
- Create `internal/cli/destination_test.go`: selector coverage.
- Create `internal/cli/add.go`: Cobra command, batch pipeline, summaries, confirmation.
- Create `internal/cli/add_test.go`: CLI behavior tests.
- Modify `internal/cli/root.go`: register `newAddCommand`.
- Modify README only after tests pass, documenting `add` examples.

## Task 1: Source Grammar

**Files:**
- Create: `internal/remote/source_parse.go`
- Create: `internal/remote/source_parse_test.go`

- [ ] **Step 1: Write failing parser tests**

Add table tests covering:

```go
func TestParseAddSource(t *testing.T) {
	tests := []struct {
		name      string
		source    string
		gitURL    string
		ref       string
		want      remote.GitSource
		wantNames []string
		wantPath  string
	}{
		{
			name:      "github shorthand",
			source:    "vercel-labs/skills",
			want:      remote.GitSource{CloneURL: "https://github.com/vercel-labs/skills.git", Owner: "vercel-labs", Repo: "skills"},
			wantNames: nil,
		},
		{
			name:      "github shorthand with skill",
			source:    "vercel-labs/skills@next-best-practices",
			want:      remote.GitSource{CloneURL: "https://github.com/vercel-labs/skills.git", Owner: "vercel-labs", Repo: "skills"},
			wantNames: []string{"next-best-practices"},
		},
		{
			name:     "github tree url",
			source:   "https://github.com/vercel-labs/skills/tree/main/skills/next-best-practices",
			want:     remote.GitSource{CloneURL: "https://github.com/vercel-labs/skills.git", Owner: "vercel-labs", Repo: "skills", Ref: "main"},
			wantPath: "skills/next-best-practices",
		},
		{
			name:      "generic git",
			gitURL:    "https://gitlab.com/acme/skills.git",
			ref:       "release",
			want:      remote.GitSource{CloneURL: "https://gitlab.com/acme/skills.git", Ref: "release"},
			wantNames: nil,
		},
	}
	// Call ParseAddSource(source, gitURL, ref); assert source, names, path.
}
```

- [ ] **Step 2: Run parser tests and verify failure**

Run: `go test ./internal/remote -run TestParseAddSource -count=1`

Expected: compile failure because `ParseAddSource` does not exist.

- [ ] **Step 3: Implement parser**

Add:

```go
type AddSource struct {
	Source        GitSource
	Names         []string
	PreferredPath string
}

func ParseAddSource(sourceArg, gitURL, ref string) (AddSource, error)
```

Rules:
- `--git URL` means `sourceArg` is not a source; names come from positional args in CLI.
- `owner/repo` builds `https://github.com/owner/repo.git`.
- `owner/repo@skill` appends `skill` to `Names`.
- GitHub tree URL extracts owner, repo, ref segment, and preferred path after `/tree/<ref>/`.
- Reject direct file/archive URLs with an error mentioning URL installs are not supported yet.

- [ ] **Step 4: Run parser tests**

Run: `go test ./internal/remote -run TestParseAddSource -count=1`

Expected: PASS.

## Task 2: Discover One Or All Skills From A Checkout

**Files:**
- Modify: `internal/remote/git.go`
- Modify: `internal/remote/git_test.go`

- [ ] **Step 1: Write failing discovery tests**

Add tests:

```go
func TestCheckoutListSkillsFindsStandardAndNestedSkills(t *testing.T)
func TestCheckoutListSkillsSortsByName(t *testing.T)
func TestCheckoutFindSkillWithoutPreferredPathReturnsAmbiguousWhenDuplicateNames(t *testing.T)
```

Use existing `writeRemoteSkill` helpers. Expected returned metadata includes `SkillPath`, `UpstreamName`, source commit, and GitHub vs generic source type.

- [ ] **Step 2: Run discovery tests**

Run: `go test ./internal/remote -run 'TestCheckout(ListSkills|FindSkill)' -count=1`

Expected: failure for missing `ListSkillsContext`.

- [ ] **Step 3: Implement `ListSkillsContext`**

Add:

```go
func (c Checkout) ListSkillsContext(ctx context.Context) ([]FoundSkill, error)
```

Implementation:
- Walk `c.Path`.
- For each directory where `skills.IsDir(path)` is true, call `c.foundAt(path, rel)`.
- `filepath.SkipDir` after a skill directory so nested skill internals do not produce duplicates.
- Sort by `FoundSkill.Info.Name`, then `FoundSkill.Metadata.SkillPath`.
- Respect `ctx.Err()` before and during walk.

- [ ] **Step 4: Run remote tests**

Run: `go test ./internal/remote -count=1`

Expected: PASS.

## Task 3: Destination Selector Parser

**Files:**
- Create: `internal/cli/destination.go`
- Create: `internal/cli/destination_test.go`

- [ ] **Step 1: Write failing selector tests**

Cover:

```go
".Ag" -> project/agents
".Cl" -> project/claude
"~Cd" -> global/codex
"global:agents" -> global/agents
"g:cl" -> global/claude
"project:codex" -> project/codex
"Ag" -> project/agents
```

Also test invalid selectors produce a helpful error.

- [ ] **Step 2: Implement parser**

Add:

```go
type addDestination struct {
	Scope  string
	Target string
}

func parseAddDestinations(values []string) ([]addDestination, error)
func defaultAddDestinations(noLink bool, values []string) ([]addDestination, error)
```

Rules:
- If `--no-link`, destinations must be empty.
- If no `--to`, default is `project/agents`.
- Deduplicate repeated destinations while preserving order.

- [ ] **Step 3: Run selector tests**

Run: `go test ./internal/cli -run TestParseAddDestinations -count=1`

Expected: PASS.

## Task 4: Add Command Batch Pipeline

**Files:**
- Create: `internal/cli/add.go`
- Create: `internal/cli/add_test.go`
- Modify: `internal/cli/root.go`

- [ ] **Step 1: Write failing CLI tests**

Add integration-style tests using local git repos:

```go
func TestAddArchivesAndLinksDefaultProjectAgents(t *testing.T)
func TestAddNoLinkArchivesOnly(t *testing.T)
func TestAddToMultipleDestinations(t *testing.T)
func TestAddOwnerRepoAtSkillShorthand(t *testing.T)
func TestAddAllRequiresConfirmationWithoutYes(t *testing.T)
func TestAddAllArchivesEveryDiscoveredSkill(t *testing.T)
func TestAddConflictReturnsRerunHintWithoutReplace(t *testing.T)
func TestAddReplaceUpdatesSameNameArchive(t *testing.T)
```

Assertions:
- Archive directory exists under `cfg.ArchiveSkillsRoot()`.
- Default symlink exists in project Agents root.
- `--no-link` creates no active symlink.
- `--to .Cl --to ~Cd` creates both links.
- Conflicts do not silently overwrite.

- [ ] **Step 2: Run CLI tests and verify failure**

Run: `go test ./internal/cli -run TestAdd -count=1`

Expected: compile failure because `newAddCommand` does not exist.

- [ ] **Step 3: Implement command skeleton**

`newAddCommand(rootOptions *options) *cobra.Command`:

```go
Use:   "add SOURCE [SKILL_NAME...]",
Short: "Add remote skills to the archive and optionally link them",
Args:  cobra.MinimumNArgs(0),
```

Flags:
- `--git string`
- `--ref string`
- `--all`
- `--no-link`
- repeatable `--to`
- `--replace`
- `--archive-as string`
- `-y/--yes`, `-n/--no`, `--no-input` already root-level

Register in `root.AddCommand(newAddCommand(&opts), ...)`.

- [ ] **Step 4: Implement source/name resolution**

Rules:
- Without `--git`, first positional arg is `SOURCE`.
- With `--git`, positional args are skill names unless `--all`.
- Names from `owner/repo@skill` are prepended before explicit names.
- `--archive-as` only valid for a single selected skill.
- If no names and no `--all`, checkout and list skills; fail with discovered names and examples:

```text
multiple skills found; specify a name or use --all:
  x-skills add owner/repo code-review
  x-skills add owner/repo --all
```

- [ ] **Step 5: Implement archive and link per skill**

For each `FoundSkill`:
- `archiveName := found.Info.Name` unless `--archive-as` is set.
- `conflict := remote.ConflictArchiveOnly`; if `--replace`, use `remote.ConflictReplaceArchive`.
- Call `remote.ApplyArchive(remote.AddRequest{Config: cfg, IncomingDir: found.SkillDir, ArchiveName: archiveName, Metadata: found.Metadata, Conflict: conflict})`.
- If not `--no-link`, call `actions.Link` for each parsed destination.

- [ ] **Step 6: Implement summaries and exit behavior**

Single success:

```text
added: next-best-practices
linked: .Ag
```

Batch:

```text
Summary:
added: a, b
linked: a -> .Ag, b -> .Ag
skipped: c (already archived)
failed: d (archive conflict for d; rerun with --replace or inspect in tui)
```

If any failure exists, return `fmt.Errorf("add failed for %d skill(s)", len(failures))`.

- [ ] **Step 7: Run CLI tests**

Run: `go test ./internal/cli -run TestAdd -count=1`

Expected: PASS.

## Task 5: Search Output Hints

**Files:**
- Modify or create: `internal/cli/search.go`
- Test: `internal/cli/search_test.go`

- [ ] **Step 1: Check whether `search` exists**

Run: `rg -n "newSearchCommand|SearchClient|search QUERY" internal/cli`.

If it does not exist, create a small search command plan before implementation, using ADR 0003 and ADR 0014.

- [ ] **Step 2: Add add-shaped hints**

Human output for each result must include:

```text
Add and use: x-skills add OWNER/REPO NAME -y
Archive only: x-skills add OWNER/REPO NAME --no-link -y
```

- [ ] **Step 3: Run search tests**

Run: `go test ./internal/cli -run TestSearch -count=1`

Expected: PASS.

## Task 6: Verification And Commit

- [ ] **Step 1: Run focused tests**

Run:

```bash
go test ./internal/remote ./internal/actions ./internal/cli -count=1
```

Expected: PASS.

- [ ] **Step 2: Run binary test/build**

Run:

```bash
go test ./cmd/x-skills ./internal/... -count=1
go build -o bin/x-skills ./cmd/x-skills
./bin/x-skills add --help
```

Expected: tests/build pass, help shows `add SOURCE [SKILL_NAME...]`.

- [ ] **Step 3: Commit only scoped files**

Do not stage unrelated docs or prior dirty TUI work.

```bash
git add internal/remote/source_parse.go internal/remote/source_parse_test.go \
  internal/remote/git.go internal/remote/git_test.go \
  internal/cli/add.go internal/cli/add_test.go internal/cli/destination.go \
  internal/cli/destination_test.go internal/cli/root.go README.md
git commit -m "feat: add native remote install command"
```

## Self-Review Checklist

- `add` aligns with `npx skills add` mental model.
- `--all` exists and is confirmation-gated.
- Default link target is project `.Ag`.
- `--no-link` archives only.
- `--to` supports compact destination grammar.
- Generic `--git` records generic source metadata.
- URL installs remain unsupported with a clear error.
- Mutation JSON is not implemented.
