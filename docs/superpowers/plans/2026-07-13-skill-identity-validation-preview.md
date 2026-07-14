# Skill Identity, Validation, and Preview Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make skill identity unambiguous, add strict reusable validation and raw remote previews, and make local and remote TUI previews responsive and useful.

**Architecture:** Filesystem identity becomes a first-class field throughout scanning, reconciliation, CLI output, and TUI grouping; declared frontmatter remains metadata only. A focused validation package composes strict `SKILL.md` parsing with strict source-metadata decoding, while one UI-agnostic remote resolver serves both Cobra and Bubble Tea preview consumers.

**Tech Stack:** Go 1.24, Cobra, Bubble Tea, Lip Gloss, Glamour, YAML v3, standard-library JSON/Git/filesystem APIs, existing `internal/pathidentity` and remote checkout cache.

## Global Constraints

- Keep the manifest schema unchanged; manifest skill `name` remains archive/directory identity.
- Keep unknown/vendor-specific `SKILL.md` frontmatter keys valid.
- Treat `.x-skills.json.compatibility` as structured x-skills metadata; do not interpret the unrelated free-text `SKILL.md` frontmatter field with the same spelling.
- Preserve schema-v1 source metadata; schema-v2 compatibility-only metadata is valid.
- Do not recurse below immediate children when validating a collection.
- Do not render Markdown or ANSI in `x-skills preview`; default to exactly the first 50 raw lines.
- Do not scrape skills.sh or require live network access in tests.
- Keep preview cancellation independent from install/archive mutation cancellation.
- Preserve existing destination-exists errors for files, directories, and wrong-target symlinks; never replace them.
- Run cross-platform-safe path tests and preserve LF/CRLF behavior.

---

## File Structure

### New files

- `internal/validation/validation.go`: public validation options, report, summary, diagnostic, level, and stable code types.
- `internal/validation/skill.go`: input classification, shallow collection traversal, canonical deduplication, and `SKILL.md` checks.
- `internal/validation/skill_test.go`: portable skill validation, path classification, aggregation, and deduplication tests.
- `internal/validation/source.go`: bridge from source metadata errors and compatibility membership checks into typed diagnostics.
- `internal/validation/source_test.go`: strict metadata and `--at` consumer-membership tests.
- `internal/cli/validate.go`: Cobra command, `--at`, human/JSON rendering, and exit behavior.
- `internal/cli/validate_test.go`: CLI contract and JSON schema tests.
- `internal/remote/preview.go`: UI-independent repository checkout and skill-document resolver.
- `internal/remote/preview_test.go`: local-Git resolver, ambiguity, cancellation, and cache tests.
- `internal/cli/preview.go`: raw line slicing and Cobra preview command.
- `internal/cli/preview_test.go`: plain/JSON output and argument validation tests.
- `internal/tui/modal_remote_preview.go`: loading/error rendering and transition helpers for Search preview.

### Existing files with focused changes

- `internal/skills/skill.go`: expose strict document parsing without basename fallback; retain current discovery fallback only where callers explicitly need it.
- `internal/actions/scan.go`, `internal/repo/repo.go`: emit separate `Identity` and `DeclaredName` fields.
- `internal/manifest/reconcile.go`: use only filesystem identity.
- `internal/actions/link.go`, `internal/cli/link.go`: recognize and report an already-correct link.
- `internal/remote/source.go`: strict JSON decoding and shared structural validation.
- `internal/cli/list.go`, `internal/cli/repo.go`: honor root `--json` and annotate divergent names in human output.
- `internal/tui/{model.go,install.go,rows.go,filter.go,views.go,inspector.go,modal_detail.go,modal_preview.go}`: consume explicit identity and implement local/remote preview behavior.
- `skills/{x-port-skill,x-find-skills,x-manage-skills}/SKILL.md`: correct executable workflows one skill at a time.
- `docs/cli.md`, `docs/tui.md`: document commands, JSON, key behavior, and loading/cancellation.

---

### Task 1: Separate Filesystem Identity from Declared Metadata

**Files:**
- Modify: `internal/skills/skill.go`
- Modify: `internal/skills/skill_test.go`
- Modify: `internal/actions/scan.go`
- Modify: `internal/actions/scan_test.go`
- Modify: `internal/repo/repo.go`
- Modify: `internal/repo/repo_test.go`
- Modify: `internal/cli/prompt.go`
- Modify: `internal/cli/list.go`
- Modify: `internal/cli/restore.go`
- Modify: `internal/cli/restore_test.go`
- Modify: `internal/cli/migrate_unlink_test.go`
- Modify: `internal/syncer/plan.go`
- Modify: `internal/syncer/plan_test.go`
- Modify: `internal/tui/rows.go`
- Modify: `internal/tui/rows_test.go`
- Modify: `internal/tui/filter.go`
- Modify: `internal/tui/filter_test.go`
- Modify: `internal/tui/inspector.go`
- Modify: `internal/tui/inspector_test.go`
- Modify: `internal/tui/actions.go`
- Modify: `internal/tui/actions_test.go`
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/model_test.go`
- Modify: `internal/tui/modal_detail.go`
- Modify: `internal/tui/modal_sync.go`
- Modify: `internal/tui/restore.go`
- Modify: `internal/tui/sync.go`
- Modify: `internal/tui/sync_test.go`
- Modify: `internal/tui/views.go`

**Interfaces:**
- Produces: `skills.ParseDocument(data []byte) (skills.Document, error)` with no basename fallback.
- Produces: `actions.ActiveSkill.Identity string` and `actions.ActiveSkill.DeclaredName string`.
- Produces: `repo.Skill.Identity string` and `repo.Skill.DeclaredName string`.
- Produces: `tui.ActiveGroup.Identity string` and `tui.ActiveGroup.DeclaredName string`.
- Consumes later: every filesystem lookup and mutation uses `Identity`; display/filter matching may additionally use `DeclaredName`.

- [ ] **Step 1: Write strict parsing and scan/repo identity tests**

Add focused cases equivalent to:

```go
func TestParseDocumentDoesNotInventDeclaredName(t *testing.T) {
	got, err := ParseDocument([]byte("---\ndescription: usable\n---\nbody\n"))
	require.NoError(t, err)
	assert.Empty(t, got.DeclaredName)
	assert.Equal(t, "usable", got.Description)
}

func TestScanKeepsIdentitySeparateFromDeclaredName(t *testing.T) {
	// Create <root>/composition-patterns/SKILL.md declaring
	// name: vercel-composition-patterns.
	assert.Equal(t, "composition-patterns", got.Identity)
	assert.Equal(t, "vercel-composition-patterns", got.DeclaredName)
}

func TestRepoListKeepsIdentitySeparateFromDeclaredName(t *testing.T) {
	assert.Equal(t, "composition-patterns", got.Identity)
	assert.Equal(t, "vercel-composition-patterns", got.DeclaredName)
}
```

Also cover an unreadable/malformed document: the active entry retains its basename identity and has an empty declared name. This test must call the new strict parser path, not `skills.Read`, because `skills.Read` currently falls back to the basename.

- [ ] **Step 2: Run focused tests and capture the expected failure**

Run:

```bash
go test ./internal/skills ./internal/actions ./internal/repo
```

Expected: FAIL because `ParseDocument`, `Identity`, and `DeclaredName` do not exist and current records conflate the names.

- [ ] **Step 3: Add the strict document boundary and explicit record fields**

Implement these types and boundary in `internal/skills/skill.go`:

```go
type Document struct {
	DeclaredName string
	Description  string
	Body         string
}

func ParseDocument(data []byte) (Document, error)
```

`ParseDocument` must parse actual YAML frontmatter and return exactly what was declared. It must never substitute `filepath.Base`. Keep `Read(path)` compatible for remote discovery callers that currently depend on its effective-name fallback, but implement it by calling the parser and applying fallback only inside `Read`.

Replace ambiguous record fields:

```go
type ActiveSkill struct {
	Identity     string
	DeclaredName string
	Root         config.Root
	Path         string
	Status       Status
	Description  string
	Reason       string
}

type Skill struct {
	Identity     string
	DeclaredName string
	Path         string
	Description  string
	Source       *remote.SourceMetadata
}

type ActiveGroup struct {
	ID           string
	Identity     string
	DeclaredName string
	Status       string
	Description  string
	Chips        []string
	Aliases      []string
	Members      []actions.ActiveSkill
	Reason       string
	Fingerprint  string
}
```

In scanners and repo listing, derive `Identity` from the directory entry and obtain `DeclaredName` only from `ParseDocument`.

- [ ] **Step 4: Update grouping, matching, filters, inspectors, and compile-time consumers**

Use these rules everywhere:

```go
func matchesSkill(selector string, skill actions.ActiveSkill) bool {
	return selector == skill.Identity ||
		(skill.DeclaredName != "" && selector == skill.DeclaredName)
}
```

For an Active group, choose primary identity by preferring a managed occurrence; if no occurrence is managed, sort occurrence identities and select the first. Preserve other occurrence identities as aliases. Do not add a differing declared name to aliases. Show a differing declared name in the inspector as metadata.

- [ ] **Step 5: Run focused and package-wide tests**

Run:

```bash
gofmt -w internal/skills internal/actions internal/repo internal/cli internal/tui
go test ./internal/skills ./internal/actions ./internal/repo ./internal/cli ./internal/tui
```

Expected: PASS, including identity/declared-name, deterministic grouping, and selector ambiguity cases.

- [ ] **Step 6: Commit**

```bash
git add internal/skills internal/actions internal/repo internal/cli internal/tui
git commit -m "refactor: separate skill identity from declared name"
```

### Task 2: Reconcile Manifests Exclusively by Filesystem Identity

**Files:**
- Modify: `internal/manifest/reconcile.go`
- Modify: `internal/manifest/reconcile_test.go`
- Modify: `internal/cli/repo_link_test.go`
- Modify: `internal/cli/migrate_unlink_test.go`
- Modify: `internal/cli/sync_test.go`
- Modify: `internal/cli/restore_test.go`

**Interfaces:**
- Consumes: `actions.ActiveSkill.Identity` from Task 1.
- Produces: local manifest records and archive paths keyed only by occurrence identity.

- [ ] **Step 1: Write the reconciliation regression**

Create an archive directory `composition-patterns` whose `SKILL.md` declares `vercel-composition-patterns`, activate it, and assert:

```go
result, err := manifest.ReconcileLocal(cfg)
require.NoError(t, err)
assert.Equal(t, "composition-patterns", result.Skills[0].Name)
```

Add table-driven CLI integration cases showing the same mismatched skill already active while each of link, unlink, migrate, sync, and restore mutates another skill. Each command must complete and reconcile successfully.

- [ ] **Step 2: Run the failing regression**

Run:

```bash
go test ./internal/manifest ./internal/cli -run 'Reconcile|DeclaredName|IdentityMismatch'
```

Expected: FAIL because reconciliation still builds the archive path from declared metadata.

- [ ] **Step 3: Replace every reconciliation name use with identity**

In `planLocalReconciliation`, use:

```go
identity := occurrence.Identity
archivePath := filepath.Join(cfg.ArchiveSkillsRoot(), identity)
```

Use `identity` for recommendation exclusion, deduplication, divergent-identity errors, manifest `Skill.Name`, archive path construction, and affected-skill error text. Do not reference `DeclaredName` anywhere in reconciliation.

- [ ] **Step 4: Run all mutation regressions**

Run:

```bash
gofmt -w internal/manifest internal/cli
go test ./internal/manifest ./internal/cli
```

Expected: PASS for isolated reconciliation and all five post-mutation command paths.

- [ ] **Step 5: Commit**

```bash
git add internal/manifest internal/cli
git commit -m "fix: reconcile manifests by skill identity"
```

### Task 3: Add Machine-Readable List and Repo Output

**Files:**
- Modify: `internal/cli/list.go`
- Modify: `internal/cli/list_test.go`
- Modify: `internal/cli/repo.go`
- Modify: `internal/cli/repo_link_test.go`

**Interfaces:**
- Consumes: explicit identity fields from Task 1 and existing root `rootOptions.json`.
- Produces: JSON arrays whose empty representation is `[]`, plus human divergence annotation.

- [ ] **Step 1: Add human and JSON contract tests**

Cover matching and divergent names, empty results, and parsing stdout with `json.Unmarshal`. Assert representative fields:

```go
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
```

For Repo, assert identity, optional differing declared name, description, archive path, and typed source metadata. Assert matching names omit `declared_name`, and human output contains `(declared: vercel-composition-patterns)` only on divergence.

- [ ] **Step 2: Run tests to verify current behavior fails**

Run:

```bash
go test ./internal/cli -run 'List|Repo.*JSON|Declared'
```

Expected: FAIL because both commands ignore `--json` and emit human rows.

- [ ] **Step 3: Split rendering by output mode**

Follow the established `list-roots`/`search` pattern:

```go
if rootOptions.json {
	return writeListJSON(cmd.OutOrStdout(), records)
}
return writeListHuman(cmd.OutOrStdout(), records)
```

Construct non-nil empty slices (`make([]listRecord, 0, len(skills))`) so JSON is `[]`. Keep ANSI/styled strings out of JSON. Use identity as the human primary label and append the declared annotation only when non-empty and different.

- [ ] **Step 4: Run focused tests**

Run:

```bash
gofmt -w internal/cli/list.go internal/cli/list_test.go internal/cli/repo.go internal/cli/repo_link_test.go
go test ./internal/cli -run 'List|Repo'
```

Expected: PASS for human, JSON, and empty-array cases.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/list.go internal/cli/list_test.go internal/cli/repo.go internal/cli/repo_link_test.go
git commit -m "feat: add JSON list and repo output"
```

### Task 4: Make Linking Idempotent Without Replacing Destinations

**Files:**
- Modify: `internal/actions/link.go`
- Modify: `internal/actions/link_test.go`
- Modify: `internal/cli/link.go`
- Modify: `internal/cli/repo_link_test.go`

**Interfaces:**
- Consumes: `pathidentity.EquivalentE(left, right string) (bool, error)`.
- Produces: `actions.ResultLinked = "linked"` and `actions.ResultAlreadyLinked = "already_linked"`, rendered as `linked`/`already linked` in human output.

- [ ] **Step 1: Add action-level no-op and safety tests**

Cover absolute and relative correct symlinks, platform-equivalent paths, a wrong-target symlink, regular file, directory, unreadable/uninspectable link, and absence of mutation. The successful no-op assertion must include:

```go
before, err := os.Lstat(destination)
require.NoError(t, err)
result, err := Link(cfg, request)
require.NoError(t, err)
assert.Equal(t, ResultAlreadyLinked, result.Status)
after, err := os.Lstat(destination)
require.NoError(t, err)
assert.Equal(t, before.ModTime(), after.ModTime())
```

- [ ] **Step 2: Add CLI summary and JSON tests**

Run one batch with a new link and one already-correct link. Assert separate human lines:

```text
linked: first-skill
already linked: second-skill
```

Assert JSON result statuses are `linked` and `already_linked`, and that project reconciliation still runs after the no-op.

- [ ] **Step 3: Run tests to verify failure**

Run:

```bash
go test ./internal/actions ./internal/cli -run 'Link.*Already|Link.*Existing|Link.*JSON'
```

Expected: FAIL with the current `destination exists` behavior.

- [ ] **Step 4: Inspect an existing destination before erroring**

Implement a focused helper:

```go
func existingLinkMatches(destination, archivePath string) (bool, error) {
	info, err := os.Lstat(destination)
	if err != nil {
		return false, err
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return false, nil
	}
	target, err := os.Readlink(destination)
	if err != nil {
		return false, err
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(destination), target)
	}
	return pathidentity.EquivalentE(target, archivePath)
}
```

Call it only after `Lstat` says the destination exists. Return `ResultAlreadyLinked` on equality; a newly created link returns `ResultLinked`. Preserve the existing error and filesystem state for every other existing destination.

- [ ] **Step 5: Render distinct batch states and validate**

Run:

```bash
gofmt -w internal/actions/link.go internal/actions/link_test.go internal/cli/link.go internal/cli/repo_link_test.go
go test ./internal/actions ./internal/cli -run Link
```

Expected: PASS, including relative target and no-replacement cases.

- [ ] **Step 6: Commit**

```bash
git add internal/actions/link.go internal/actions/link_test.go internal/cli/link.go internal/cli/repo_link_test.go
git commit -m "feat: make link idempotent"
```

### Task 5: Make Source Metadata Decoding Strict and Reusable

**Files:**
- Modify: `internal/remote/source.go`
- Modify: `internal/remote/source_test.go`

**Interfaces:**
- Produces: `remote.DecodeSourceMetadata(data []byte) (remote.SourceMetadata, error)` using strict JSON decoding.
- Produces: `remote.ValidateSourceMetadata(metadata remote.SourceMetadata) error` for structural/schema rules.
- Produces: typed `remote.MetadataError` with `Code` and `Field`, allowing validators to preserve stable diagnostics without parsing error strings.
- Preserves: `remote.ReadSourceMetadata(path string)` as the production filesystem entry point.

- [ ] **Step 1: Write strict decoding and schema tests**

Add table cases for unknown fields, trailing JSON values, mistaken top-level `agnostic`/`agents`, valid schema v1, valid compatibility-only v2, valid full GitHub source, partial source identity, unknown source type, compatibility exclusivity, empty agents, invalid agent ID, and duplicate agent ID.

Representative assertions:

```go
_, err := DecodeSourceMetadata([]byte(`{"schema_version":2,"agnostic":true}`))
require.ErrorContains(t, err, "unknown field")

got, err := DecodeSourceMetadata([]byte(
	`{"schema_version":2,"compatibility":{"agnostic":true}}`,
))
require.NoError(t, err)
assert.True(t, got.Compatibility.Agnostic)
```

Also assert `SKILL.md` frontmatter such as `compatibility: "Designed for Claude Code"` is irrelevant to this decoder; it is neither read nor rejected here.

- [ ] **Step 2: Run source metadata tests and verify failure**

Run:

```bash
go test ./internal/remote -run 'Source|Compatibility|UnknownField'
```

Expected: FAIL because `json.Unmarshal` silently ignores unknown keys and structural checks are incomplete.

- [ ] **Step 3: Implement strict single-document decoding**

Use a decoder with unknown-field rejection and EOF verification:

```go
type MetadataError struct {
	Code  string
	Field string
	Err   error
}

func (e *MetadataError) Error() string { return e.Err.Error() }
func (e *MetadataError) Unwrap() error { return e.Err }

func DecodeSourceMetadata(data []byte) (SourceMetadata, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	var metadata SourceMetadata
	if err := decoder.Decode(&metadata); err != nil {
		return SourceMetadata{}, fmt.Errorf("decode source metadata: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return SourceMetadata{}, errors.New("decode source metadata: multiple JSON values")
		}
		return SourceMetadata{}, fmt.Errorf("decode source metadata trailer: %w", err)
	}
	if err := ValidateSourceMetadata(metadata); err != nil {
		return SourceMetadata{}, err
	}
	return metadata, nil
}
```

Wrap decoder unknown-field errors with code `metadata.unknown_field`, trailing data with `metadata.trailing_json`, schema errors with `metadata.schema`, source identity errors with `metadata.source`, and compatibility errors with `metadata.compatibility`; populate `Field` whenever the failing member is known. Preserve the existing missing-schema-to-v1 normalization. For schema v2, allow compatibility without provenance. If any source identity field is present, require `source_type` plus every field required by that type. Enforce `^[a-z][a-z0-9-]*$`, uniqueness, non-empty agent list, and exactly one of `agnostic: true` or agents.

- [ ] **Step 4: Route ordinary reads through the strict boundary**

Change `ReadSourceMetadata` to read bytes, call `DecodeSourceMetadata`, and wrap errors with the `.x-skills.json` path. Do not keep a permissive alternate read path.

- [ ] **Step 5: Run the complete remote suite**

Run:

```bash
gofmt -w internal/remote/source.go internal/remote/source_test.go
go test ./internal/remote
```

Expected: PASS; unknown keys now fail in validation and all normal reads while v1 and compatibility-only v2 remain accepted.

- [ ] **Step 6: Commit**

```bash
git add internal/remote/source.go internal/remote/source_test.go
git commit -m "fix: validate source metadata strictly"
```

### Task 6: Build the Reusable Validation Engine

**Files:**
- Create: `internal/validation/validation.go`
- Create: `internal/validation/skill.go`
- Create: `internal/validation/skill_test.go`
- Create: `internal/validation/source.go`
- Create: `internal/validation/source_test.go`

**Interfaces:**
- Consumes: strict `skills.ParseDocument`, `remote.ReadSourceMetadata`, configured roots/consumer IDs, and canonical path identity.
- Produces:

```go
type Options struct {
	Roots []roots.ActiveRoot
}

type Level string

const (
	LevelError   Level = "error"
	LevelWarning Level = "warning"
)

type Diagnostic struct {
	Path        string `json:"path"`
	Level       Level  `json:"level"`
	Code        string `json:"code"`
	Message     string `json:"message"`
	Field       string `json:"field,omitempty"`
	RelatedPath string `json:"related_path,omitempty"`
}

type Summary struct {
	Skills   int `json:"skills"`
	Errors   int `json:"errors"`
	Warnings int `json:"warnings"`
}

type Report struct {
	Valid       bool         `json:"valid"`
	Summary     Summary      `json:"summary"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

func ValidatePaths(paths []string, opts Options) Report
```

- [ ] **Step 1: Test deterministic input classification and aggregation**

Add table-driven fixtures for:

- direct `SKILL.md`;
- direct skill directory;
- collection with immediate child skills;
- nested grandchildren that must not be traversed;
- missing path;
- unrelated regular file;
- empty collection;
- overlapping/direct duplicate inputs;
- mixed valid and invalid children.

Assert canonical deduplication, lexicographically stable diagnostic order, one skill count per canonical skill directory, and collection-wide error aggregation.

- [ ] **Step 2: Test the portable `SKILL.md` core**

Cover LF and CRLF delimiters, malformed YAML, non-mapping YAML, missing/empty/wrong-type name and description, invalid kebab case, leading/trailing/consecutive hyphens, name length 65, description length 1025, angle brackets, empty body, allowed vendor fields, and identity mismatch warning.

Use exact stable codes:

```go
const (
	CodeInputMissing          = "input.missing"
	CodeInputUnsupported      = "input.unsupported"
	CodeCollectionEmpty       = "collection.empty"
	CodeFrontmatterMalformed  = "skill.frontmatter_malformed"
	CodeNameRequired          = "skill.name_required"
	CodeNameInvalid           = "skill.name_invalid"
	CodeDescriptionRequired   = "skill.description_required"
	CodeDescriptionInvalid    = "skill.description_invalid"
	CodeBodyEmpty             = "skill.body_empty"
	CodeIdentityMismatch      = "skill.identity_mismatch"
	CodeMetadataInvalid       = "metadata.invalid"
	CodeMetadataUnknownField  = "metadata.unknown_field"
	CodeMetadataTrailingJSON  = "metadata.trailing_json"
	CodeMetadataSchema        = "metadata.schema"
	CodeMetadataSource        = "metadata.source"
	CodeMetadataCompatibility = "metadata.compatibility"
	CodeCompatibilityConsumer = "compatibility.unknown_consumer"
)
```

Mismatch must be `warning`; every malformed portable-core rule must be `error`.

- [ ] **Step 3: Test metadata diagnostics and optional consumer membership**

Create roots whose consumer IDs are `claude` and `codex`. Assert:

```go
withoutAt := ValidatePaths([]string{skill}, Options{})
assert.True(t, withoutAt.Valid) // structurally valid "goose" stays portable

withAt := ValidatePaths([]string{skill}, Options{
	Roots: []roots.ActiveRoot{projectRoot},
})
assertDiagnostic(t, withAt, LevelError,
	CodeCompatibilityConsumer, "compatibility.agents")
```

Repeated locations use the union of the supplied enabled-root consumer IDs. Location selectors are resolved by Cobra before this package is called; structural compatibility errors always apply, even when `Roots` is empty.

- [ ] **Step 4: Run the new tests and verify failure**

Run:

```bash
go test ./internal/validation
```

Expected: FAIL because the package and types do not exist.

- [ ] **Step 5: Implement classification, validation, and reporting**

Use `os.Stat`/`filepath.Base` to classify inputs. Resolve direct `SKILL.md` to its parent. For a collection, inspect only immediate child directories containing `SKILL.md`. Canonicalize paths with the same path-identity strategy used elsewhere, deduplicate before validation, sort inputs and diagnostics, and continue after independent errors.

Parse YAML into a map-capable representation that distinguishes missing/wrong-typed fields. Apply:

```go
var portableName = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
```

Permit lowercase letters, digits, and single interior hyphens exactly as `quick_validate.py` does, including a leading digit. Count characters with `utf8.RuneCountInString`, not bytes. Treat trimmed empty body as an error. Allow all unknown frontmatter keys.

If `.x-skills.json` exists, call the strict production reader. Translate `remote.MetadataError.Code` and `.Field` directly into the diagnostic; use `metadata.invalid` only for I/O or otherwise unclassified metadata failures. When `Roots` is non-empty, validate agent membership against the union of their `Consumers` fields.

- [ ] **Step 6: Run and review the validation package**

Run:

```bash
gofmt -w internal/validation
go test ./internal/validation
go test ./internal/remote ./internal/skills
```

Expected: PASS with deterministic JSON-ready reports and no regression in shared parsers.

- [ ] **Step 7: Commit**

```bash
git add internal/validation
git commit -m "feat: add skill validation engine"
```

### Task 7: Expose `x-skills validate`

**Files:**
- Create: `internal/cli/validate.go`
- Create: `internal/cli/validate_test.go`
- Modify: `internal/cli/root.go`

**Interfaces:**
- Consumes: `validation.ValidatePaths(paths []string, opts validation.Options) validation.Report` and existing `resolveLocations`.
- Produces: `x-skills validate PATH... [--at LOCATION...] [--json]`.

- [ ] **Step 1: Write CLI behavior tests**

Using the existing root-command test helper, assert:

- one or more paths are required;
- `--at` is repeatable;
- warnings-only output returns success;
- errors print the complete report before returning nonzero;
- human output groups by path and ends `N skills, E errors, W warnings`;
- `--json` decodes exactly as `validation.Report`;
- valid output contains `"valid": true` and non-nil `diagnostics: []`;
- unknown `--at` selectors fail actionably.

- [ ] **Step 2: Run tests and verify command absence**

Run:

```bash
go test ./internal/cli -run Validate
```

Expected: FAIL because `validate` is not registered.

- [ ] **Step 3: Implement the Cobra command and renderers**

Use:

```go
type validateOptions struct {
	locations []string
}

func newValidateCommand(rootOptions *options) *cobra.Command
func writeValidationHuman(w io.Writer, report validation.Report) error
func writeValidationJSON(w io.Writer, report validation.Report) error
```

Set `Args: cobra.MinimumNArgs(1)`, bind `--at` with `StringArrayVar`, resolve non-empty selectors with `resolveLocations`, pass the resulting roots into validation, use the root `--json` option, and render before returning a sentinel/typed validation-failed error when `report.Valid` is false. Keep selector/usage errors distinct from validation findings and avoid printing the report twice through Cobra.

- [ ] **Step 4: Run focused and root integration tests**

Run:

```bash
gofmt -w internal/cli/validate.go internal/cli/validate_test.go internal/cli/root.go
go test ./internal/cli -run 'Validate|Root'
```

Expected: PASS for human/JSON formatting and exit semantics.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/validate.go internal/cli/validate_test.go internal/cli/root.go
git commit -m "feat: add skill validation command"
```

### Task 8: Add a Shared Remote Resolver and Raw CLI Preview

**Files:**
- Create: `internal/remote/preview.go`
- Create: `internal/remote/preview_test.go`
- Create: `internal/cli/preview.go`
- Create: `internal/cli/preview_test.go`
- Modify: `internal/cli/root.go`

**Interfaces:**
- Consumes: `remote.CheckoutCache`, `remote.GitSource`, `remote.Checkout`, and existing exact/path/frontmatter skill lookup.
- Produces:

```go
type PreviewRequest struct {
	Source        GitSource
	Name          string
	PreferredPath string
}

type PreviewResult struct {
	Repository    string
	RequestedName string
	SkillDir      string
	SkillPath     string
	SkillMD       []byte
	Commit        string
}

func ResolvePreview(
	ctx context.Context,
	cache *CheckoutCache,
	request PreviewRequest,
) (PreviewResult, error)
```

- [ ] **Step 1: Write resolver tests using local Git fixtures**

Create temporary repositories with committed skill directories and test exact name, preferred path, frontmatter-name fallback, missing skill, ambiguous name, malformed/unreadable document, cancelled context, checkout failure, and repeated lookup through the same cache. Assert `SkillPath` is repository-relative and `SkillMD` contains the original bytes including frontmatter.

- [ ] **Step 2: Run resolver tests and verify failure**

Run:

```bash
go test ./internal/remote -run Preview
```

Expected: FAIL because `ResolvePreview` does not exist.

- [ ] **Step 3: Implement the UI-independent resolver**

Reuse the existing checkout and `FindSkillContext` behavior rather than reimplementing matching. Check `ctx.Err()` at boundary points, read `SKILL.md` only after an unambiguous match, and wrap errors so callers can distinguish cancellation, checkout, missing, and ambiguity through `errors.Is`/`errors.As` or existing typed errors. Do no printing, styling, or Bubble Tea work here.

- [ ] **Step 4: Write raw CLI preview tests**

Inject/configure a local repository source in CLI tests and assert:

```go
func TestFirstLinesReturnsRawPrefix(t *testing.T) {
	content := []byte("one\ntwo\nthree\n")
	got, returned, truncated := firstLines(content, 2)
	assert.Equal(t, []byte("one\ntwo\n"), got)
	assert.Equal(t, 2, returned)
	assert.True(t, truncated)
}
```

Cover the default 50 lines, `--lines 1`, zero/negative rejection, a short file, a file without final newline, exact-limit content, no heading/ANSI/Glamour/truncation marker, and JSON fields `repository`, `requested_skill`, `skill_path`, `commit`, `content`, `returned_lines`, `requested_lines`, and `truncated`.

- [ ] **Step 5: Run CLI tests and verify command absence**

Run:

```bash
go test ./internal/cli -run Preview
```

Expected: FAIL because the preview command and `firstLines` do not exist.

- [ ] **Step 6: Implement and register `preview`**

Use:

```go
type previewOptions struct {
	lines int
}

func newPreviewCommand(rootOptions *options) *cobra.Command
func firstLines(content []byte, limit int) ([]byte, int, bool)
```

Set `Use: "preview OWNER/REPO SKILL"`, `Args: cobra.ExactArgs(2)`, and default `--lines 50`. Parse `OWNER/REPO` through the existing GitHub source parser. Plain stdout writes only the raw prefix and adds a final newline only when required for clean terminal termination; JSON preserves the returned raw content exactly as a string. Resolver errors must emit no partial successful document.

- [ ] **Step 7: Validate resolver and CLI together**

Run:

```bash
gofmt -w internal/remote/preview.go internal/remote/preview_test.go internal/cli/preview.go internal/cli/preview_test.go internal/cli/root.go
go test ./internal/remote ./internal/cli -run Preview
```

Expected: PASS without network access and without Markdown rendering.

- [ ] **Step 8: Commit**

```bash
git add internal/remote/preview.go internal/remote/preview_test.go internal/cli/preview.go internal/cli/preview_test.go internal/cli/root.go
git commit -m "feat: add remote skill preview command"
```

### Task 9: Replace Redundant TUI Details with Responsive Preview Modals

**Files:**
- Create: `internal/tui/modal_remote_preview.go`
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/install.go`
- Modify: `internal/tui/install_test.go`
- Modify: `internal/tui/model_test.go`
- Modify: `internal/tui/model_inflight_test.go`
- Modify: `internal/tui/modal_detail.go`
- Modify: `internal/tui/modal_preview.go`
- Modify: `internal/tui/modal_test.go`
- Modify: `internal/tui/views.go`
- Modify: `internal/tui/render_test.go`
- Modify: `internal/tui/keys.go` if help text needs to distinguish preview/detail behavior

**Interfaces:**
- Consumes: `remote.ResolvePreview` and `remote.PreviewResult` from Task 8; existing local preview modal, animation frame/tick, preview token, and checkout cache.
- Produces: local Active/Repo Enter behavior and a dedicated cancellable Search preview lifecycle.

- [ ] **Step 1: Test local Enter and `p` routing**

Add key-update cases asserting:

```go
// Active and Repo
assertPreviewModal(t, updateWithKey(model, tea.KeyEnter))
assertPreviewModal(t, updateWithRune(model, 'p'))

// Doctor
assertDoctorDetailModal(t, updateWithKey(model, tea.KeyEnter))
```

Assert the local preview still supports raw/rendered toggle, scrolling, constrained layout, and inline read errors. Assert Active/Repo detail constructors are no longer reachable.

- [ ] **Step 2: Test immediate remote loading state and animation**

Inject a blocking preview resolver. In the same update that handles Enter, assert the modal is open, has `previewLoading == true`, contains repository/skill labels, and returns a command. Feed an animation tick and assert the indicator frame changes, including ASCII fallback when Unicode is disabled.

- [ ] **Step 3: Test remote completion, error, and races**

Cover:

- matching success replaces the loading body in the same modal with the existing rendered preview;
- matching error keeps the modal open and shows an actionable message;
- Escape calls the dedicated cancel function, closes quietly, and invalidates the token;
- cancellation result does not reopen or display an error;
- late results after cursor movement, leaving Search, opening a new preview, or closing the modal are ignored;
- preview cancellation does not call or clear install/archive cancellation;
- a second preview of the same repository uses the existing checkout cache.

- [ ] **Step 4: Run TUI preview tests and verify failure**

Run:

```bash
go test ./internal/tui -run 'Preview|Detail|Install.*Modal|Inflight'
```

Expected: FAIL because Enter still opens Active/Repo details and Search does not enter a visible loading state synchronously.

- [ ] **Step 5: Route Active/Repo Enter to the existing local preview**

In key handling, make Enter call the same local preview function as `p` for Active and Repo. Retain the detail path only for Doctor. Remove redundant Active/Repo detail constructors and their obsolete rendering branches/tests; keep shared Doctor detail infrastructure.

- [ ] **Step 6: Add an independent remote preview lifecycle**

Add model state with explicit separation:

```go
type previewResolver func(
	context.Context,
	*remote.CheckoutCache,
	remote.PreviewRequest,
) (remote.PreviewResult, error)

// model fields
previewCancel   context.CancelFunc
previewLoading  bool
resolvePreview  previewResolver
```

On Search Enter, cancel/invalidate any previous preview, increment `previewToken`, create a new context, open the loading modal immediately, and return a command that invokes the resolver. Include token and result/error in the message. Do not reuse `operationCancel` or any mutation context.

- [ ] **Step 7: Implement loading, success, error, and cancellation rendering**

`modal_remote_preview.go` should render the existing animation frame plus repository/skill label while loading, then hand successful bytes to the normal preview content/rendering path. Errors replace loading content but keep Escape available. Escape and leaving Search call a helper that cancels, clears state, increments/invalidate the token, and closes without surfacing `context.Canceled`.

- [ ] **Step 8: Run focused and race tests**

Run:

```bash
gofmt -w internal/tui
go test ./internal/tui -run 'Preview|Detail|Install.*Modal|Inflight'
go test -race ./internal/tui
```

Expected: PASS; loading is visible before checkout completes and stale/cancelled messages cannot alter the modal.

- [ ] **Step 9: Commit**

```bash
git add internal/tui
git commit -m "feat: show responsive skill preview modals"
```

### Task 10: Correct and Forward-Test the Three Bundled Skills

**Files:**
- Modify: `skills/x-port-skill/SKILL.md`
- Modify: `skills/x-find-skills/SKILL.md`
- Modify: `skills/x-manage-skills/SKILL.md`

**Interfaces:**
- Consumes: the built `x-skills validate` and `x-skills preview` commands from Tasks 7 and 8.
- Produces: three independently tested and committed agent workflows.

**Required discipline:** Use `superpowers:writing-skills` for each skill separately. Do not edit the next skill until the current skill has completed baseline failure, minimal edit, forward test, `x-skills validate`, diff review, and commit.

#### Task 10A: Fix `x-port-skill`

- [ ] **Step 1: Run and record the RED baseline**

Give a fresh agent the current `x-port-skill` plus a fixture skill that must become agent-agnostic and ask it to produce `.x-skills.json`. Record that it writes top-level `agnostic`/`agents` or cannot name an exact validator. Preserve the transcript/result outside the tracked skill file while testing.

- [ ] **Step 2: Make the minimum workflow correction**

Require exactly these nested shapes:

```json
{"schema_version": 2, "compatibility": {"agnostic": true}}
```

or:

```json
{"schema_version": 2, "compatibility": {"agents": ["claude", "codex"]}}
```

State that all existing source/provenance fields remain intact. Replace the vague validator reference with:

```bash
x-skills validate <staged-skill> --at <destination> --json
```

Repeat the command after applying the approved diff. Preserve the existing explicit-approval/sandbox rule for bundled scripts.

- [ ] **Step 3: Run the GREEN scenario and automated validation**

Run the same fresh-agent scenario and assert the produced JSON nests compatibility and names the exact command. Then run:

```bash
go run ./cmd/x-skills validate skills/x-port-skill --json
```

Expected: exit 0, `"valid": true`, no errors.

- [ ] **Step 4: Review and commit only this skill**

```bash
git diff --check -- skills/x-port-skill/SKILL.md
git diff -- skills/x-port-skill/SKILL.md
git add skills/x-port-skill/SKILL.md
git commit -m "fix: correct port skill compatibility metadata"
```

#### Task 10B: Make `x-find-skills` Tolerate Sparse Registry Data

- [ ] **Step 1: Run and record the RED baseline**

Give a fresh agent a representative skills.sh result containing only `name`, `source`, and install count. Ask it for a consequential recommendation. Record whether it invents description/audit data, stops because fields are absent, or recommends without reading `SKILL.md`.

- [ ] **Step 2: Add the available-data recipe**

State that registry `description`, `path`, and `audit` are optional. Initial ranking uses only available name, source, and installs. Before a consequential recommendation or install, require:

```bash
x-skills preview owner/repo skill
```

Final relevance is based on the returned `SKILL.md` content. Keep the existing archive-first add/link commands exact.

- [ ] **Step 3: Run the GREEN scenario and automated validation**

Repeat the sparse-result scenario and assert no missing fields are invented and preview is used before recommendation. Then run:

```bash
go run ./cmd/x-skills validate skills/x-find-skills --json
```

Expected: exit 0, `"valid": true`, no errors.

- [ ] **Step 4: Review and commit only this skill**

```bash
git diff --check -- skills/x-find-skills/SKILL.md
git diff -- skills/x-find-skills/SKILL.md
git add skills/x-find-skills/SKILL.md
git commit -m "fix: make skill discovery tolerate sparse registry data"
```

#### Task 10C: Make `x-manage-skills` Untrack Guidance Safe

- [ ] **Step 1: Run and record the RED baseline**

Give a fresh agent doctor output that says a skills folder is ignored and suggests `git rm -r --cached`, in a fixture orchestration repository with 300 tracked files below that folder. Ask it to finish cleanup. Record whether it proposes/runs the recursive untrack without inspecting scope or obtaining explicit confirmation.

- [ ] **Step 2: Add an accurate conditional safety gate**

State that `doctor --fix -y` adds ignore entries and may print a manual Git follow-up; it does not execute `git rm --cached`. Before any suggested recursive untrack, require both:

```bash
git status --short -- <skills-folder>
git ls-files -- <skills-folder> | wc -l
```

Require explicit user confirmation before staging a large untrack operation. Call out root orchestration repositories and submodules with historically tracked skill trees.

- [ ] **Step 3: Run the GREEN scenario and automated validation**

Repeat the 300-file scenario and assert the agent reports scope/count and pauses for confirmation without running the command. Then run:

```bash
go run ./cmd/x-skills validate skills/x-manage-skills --json
```

Expected: exit 0, `"valid": true`, no errors.

- [ ] **Step 4: Review and commit only this skill**

```bash
git diff --check -- skills/x-manage-skills/SKILL.md
git diff -- skills/x-manage-skills/SKILL.md
git add skills/x-manage-skills/SKILL.md
git commit -m "docs: make managed skill cleanup safer"
```

### Task 11: Document the Contracts and Run Integrated Verification

**Files:**
- Modify: `docs/cli.md`
- Modify: `docs/tui.md`
- Modify: `cmd/x-skills/docs_test.go`

**Interfaces:**
- Consumes: all prior tasks.
- Produces: user documentation and final cross-package evidence.

- [ ] **Step 1: Add documentation assertions if the project checks docs**

Extend existing docs tests to require the literal commands and key behavior:

```text
x-skills validate PATH... [--at LOCATION...] [--json]
x-skills preview OWNER/REPO SKILL [--lines N] [--json]
```

Require docs to mention identity versus declared name, `already_linked`, Active/Repo Enter preview, Doctor Enter detail, Search loading animation, and Escape cancellation.

- [ ] **Step 2: Run docs tests and verify missing documentation**

Run:

```bash
go test ./cmd/x-skills -run 'Docs|Documentation'
```

Expected: FAIL because the newly added literal contract assertions are not documented yet.

- [ ] **Step 3: Update CLI and TUI documentation**

In `docs/cli.md`, document list/repo JSON arrays and fields, identity annotation, idempotent link statuses, validate path classification/rules/exit behavior, strict nested `.x-skills.json` compatibility, and raw preview defaults/options. Explicitly distinguish structured `.x-skills.json.compatibility` from free-text `SKILL.md` frontmatter compatibility.

In `docs/tui.md`, document Active/Repo Enter and `p`, Doctor Enter, immediate Search loading modal, animation, Escape cancellation, error-in-modal behavior, and cache reuse.

- [ ] **Step 4: Format and inspect all changes**

Run:

```bash
gofmt -w cmd internal
git diff --check
git status --short
```

Expected: no formatting or whitespace errors; only intended source, test, skill, and documentation files are changed. Do not add the unrelated `.claude/` directory.

- [ ] **Step 5: Run focused race and static checks**

Run:

```bash
go test -race ./internal/tui ./internal/remote ./internal/validation
go vet ./...
```

Expected: both commands exit 0.

- [ ] **Step 6: Run the full suite and CLI smoke checks**

Run:

```bash
go test ./...
go run ./cmd/x-skills version
go run ./cmd/x-skills validate skills/x-port-skill skills/x-find-skills skills/x-manage-skills --json
go run ./cmd/x-skills preview --help
```

Expected: full suite exits 0; version prints current build information; validation reports three skills, zero errors, and valid true; preview help shows `OWNER/REPO SKILL` and default 50 lines.

- [ ] **Step 7: Commit documentation or any test-only contract updates**

```bash
git add docs/cli.md docs/tui.md cmd/x-skills/docs_test.go
git commit -m "docs: document skill validation and previews"
```

- [ ] **Step 8: Verify cross-platform CI before merge**

Push/open the chosen branch only when authorized, then require the existing Linux, macOS, and Windows test matrix to pass. Pay particular attention to relative symlink/path identity, CRLF frontmatter, cancellation races, and CLI golden/help tests. Do not replace local-Git fixtures with live GitHub calls to make CI pass.

## Final Acceptance Checklist

- [ ] A directory/frontmatter mismatch survives scan and every post-mutation reconciliation path.
- [ ] All filesystem behavior keys on identity; declared name is display/filter metadata only.
- [ ] `list --json` and `repo --json` return typed arrays, including `[]` when empty.
- [ ] Correct existing links return `already_linked`; every other existing destination remains untouched and errors.
- [ ] `validate` aggregates portable skill and strict metadata diagnostics and honors repeated `--at`.
- [ ] Unknown `.x-skills.json` fields fail in both validation and ordinary metadata reads.
- [ ] CLI preview returns raw first-50-line content by default and structured JSON on request.
- [ ] Active/Repo Enter previews local skill text; Doctor Enter still shows issue details.
- [ ] Search Enter opens an animated modal immediately; Escape cancels quietly and stale results are ignored.
- [ ] Each bundled skill passed its own RED/GREEN scenario, validation, review, and commit.
- [ ] Full tests, focused race tests, vet, and all cross-platform CI jobs pass.
