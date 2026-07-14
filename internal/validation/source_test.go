package validation

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/InkyQuill/x-skills/internal/roots"
)

func TestValidatePathsClassifiesAndAggregatesInputs(t *testing.T) {
	base := t.TempDir()
	direct := makeSkill(t, base, "direct", validSkill("direct"))
	collection := filepath.Join(base, "collection")
	if err := os.Mkdir(collection, 0o755); err != nil {
		t.Fatal(err)
	}
	makeSkill(t, collection, "alpha", validSkill("alpha"))
	makeSkill(t, collection, "broken", "---\nname: broken\n---\nBody\n")
	nested := filepath.Join(collection, "group")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	makeSkill(t, nested, "grandchild", validSkill("grandchild"))
	empty := filepath.Join(base, "empty")
	if err := os.Mkdir(empty, 0o755); err != nil {
		t.Fatal(err)
	}
	unrelated := filepath.Join(base, "notes.txt")
	writeFile(t, unrelated, "notes")
	missing := filepath.Join(base, "missing")

	report := ValidatePaths([]string{
		filepath.Join(direct, "SKILL.md"),
		direct,
		collection,
		empty,
		unrelated,
		missing,
	}, Options{})

	if report.Summary.Skills != 3 {
		t.Fatalf("skills = %d, want 3", report.Summary.Skills)
	}
	for _, code := range []string{
		CodeDescriptionRequired,
		CodeCollectionEmpty,
		CodeInputUnsupported,
		CodeInputMissing,
	} {
		assertDiagnostic(t, report, LevelError, code, "")
	}
	for i := 1; i < len(report.Diagnostics); i++ {
		previous := diagnosticSortKey(report.Diagnostics[i-1])
		current := diagnosticSortKey(report.Diagnostics[i])
		if previous > current {
			t.Fatalf("diagnostics not sorted at %d: %q > %q", i, previous, current)
		}
	}
}

func TestValidatePathsDeduplicatesCanonicalSkillDirectories(t *testing.T) {
	base := t.TempDir()
	skill := makeSkill(t, base, "portable", validSkill("portable"))
	alias := filepath.Join(base, "alias")
	if err := os.Symlink(skill, alias); err != nil {
		t.Fatal(err)
	}

	report := ValidatePaths([]string{skill, alias, filepath.Join(alias, "SKILL.md")}, Options{})
	if !report.Valid || report.Summary.Skills != 1 || len(report.Diagnostics) != 0 {
		t.Fatalf("report = %#v, want one valid canonical skill", report)
	}
}

func TestValidatePathsRejectsDirectoryNamedSkillMarkdown(t *testing.T) {
	path := filepath.Join(t.TempDir(), "SKILL.md")
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	report := ValidatePaths([]string{path}, Options{})
	assertDiagnostic(t, report, LevelError, CodeInputUnsupported, "")
	if report.Summary.Skills != 0 {
		t.Fatalf("skills = %d, want 0", report.Summary.Skills)
	}
}

func TestValidatePathsDoesNotDeduplicateEmptyInputWithCurrentDirectory(t *testing.T) {
	skill := makeSkill(t, t.TempDir(), "portable", validSkill("portable"))
	t.Chdir(skill)

	report := ValidatePaths([]string{"", " ", "."}, Options{})
	assertDiagnostic(t, report, LevelError, CodeInputUnsupported, "")
	if report.Summary.Skills != 1 {
		t.Fatalf("skills = %d, want current directory skill counted once", report.Summary.Skills)
	}
	if report.Summary.Errors != 1 {
		t.Fatalf("errors = %d, want duplicate empty inputs reported once", report.Summary.Errors)
	}
}

func TestImmediateSkillDirsReturnsReadDirError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "collection.txt")
	writeFile(t, path, "not a directory")

	_, err := immediateSkillDirs(path)
	if err == nil {
		t.Fatal("immediateSkillDirs() error = nil, want os.ReadDir error")
	}
}

func TestImmediateSkillDirReturnsStatError(t *testing.T) {
	_, err := isImmediateSkillDir(string([]byte{0}))
	if err == nil {
		t.Fatal("isImmediateSkillDir() error = nil, want os.Stat error")
	}
}

func TestAddSkillPathReturnsCanonicalizationError(t *testing.T) {
	paths := map[string]string{}
	err := addSkillPath(paths, filepath.Join(t.TempDir(), "missing"))
	if err == nil {
		t.Fatal("addSkillPath() error = nil, want canonicalization error")
	}
	if len(paths) != 0 {
		t.Fatalf("paths = %#v, want no silently added path", paths)
	}
}

func TestValidatePathsTranslatesMetadataErrors(t *testing.T) {
	tests := []struct {
		name      string
		metadata  string
		wantCode  string
		wantField string
	}{
		{name: "invalid json", metadata: "{", wantCode: CodeMetadataInvalid},
		{
			name: "unknown field", metadata: `{"schema_version":2,"vendor":true}`,
			wantCode: CodeMetadataUnknownField, wantField: "vendor",
		},
		{name: "trailing json", metadata: `{"schema_version":2} {}`, wantCode: CodeMetadataTrailingJSON},
		{name: "schema", metadata: `{"schema_version":3}`, wantCode: CodeMetadataSchema, wantField: "schema_version"},
		{
			name: "source", metadata: `{"schema_version":2,"source_type":"archive"}`,
			wantCode: CodeMetadataSource, wantField: "source_type",
		},
		{
			name:     "compatibility",
			metadata: `{"schema_version":2,"compatibility":{"agents":[]}}`,
			wantCode: CodeMetadataCompatibility, wantField: "compatibility.agents",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			skill := makeSkill(t, t.TempDir(), "portable", validSkill("portable"))
			writeFile(t, filepath.Join(skill, ".x-skills.json"), test.metadata)
			report := ValidatePaths([]string{skill}, Options{})
			diagnostic := assertDiagnostic(t, report, LevelError, test.wantCode, test.wantField)
			assertEquivalentPath(t, diagnostic.Path, skill)
		})
	}
}

func TestValidatePathsChecksCompatibilityAgainstSuppliedRootConsumerUnion(t *testing.T) {
	skill := makeSkill(t, t.TempDir(), "portable", validSkill("portable"))
	metadata := `{"schema_version":2,"compatibility":{"agents":["goose"]}}`
	writeFile(t, filepath.Join(skill, ".x-skills.json"), metadata)

	withoutRoots := ValidatePaths([]string{skill}, Options{})
	if !withoutRoots.Valid {
		t.Fatalf("without roots = %#v, want portable validity", withoutRoots)
	}

	withRoots := ValidatePaths([]string{skill}, Options{Roots: []roots.ActiveRoot{
		{Consumers: []string{"codex"}},
		{Consumers: []string{"claude", "codex"}},
	}})
	diagnostic := assertDiagnostic(
		t,
		withRoots,
		LevelError,
		CodeCompatibilityConsumer,
		"compatibility.agents",
	)
	if diagnostic.Message == "" {
		t.Fatal("consumer diagnostic message is empty")
	}
}

func TestValidatePathsRequiresEveryDeclaredAgentInSuppliedConsumerUnion(t *testing.T) {
	skill := makeSkill(t, t.TempDir(), "portable", validSkill("portable"))
	metadata := `{"schema_version":2,"compatibility":{"agents":["claude","codex"]}}`
	writeFile(t, filepath.Join(skill, ".x-skills.json"), metadata)

	partialRoots := ValidatePaths([]string{skill}, Options{Roots: []roots.ActiveRoot{
		{Consumers: []string{"codex"}},
	}})
	assertDiagnostic(
		t,
		partialRoots,
		LevelError,
		CodeCompatibilityConsumer,
		"compatibility.agents",
	)

	unionRoots := ValidatePaths([]string{skill}, Options{Roots: []roots.ActiveRoot{
		{Consumers: []string{"codex"}},
		{Consumers: []string{"claude"}},
	}})
	if !unionRoots.Valid {
		t.Fatalf("union roots report = %#v, want valid", unionRoots)
	}
}

func TestValidatePathsChecksMembershipWhenSuppliedRootHasNoConsumers(t *testing.T) {
	skill := makeSkill(t, t.TempDir(), "portable", validSkill("portable"))
	metadata := `{"schema_version":2,"compatibility":{"agents":["codex"]}}`
	writeFile(t, filepath.Join(skill, ".x-skills.json"), metadata)

	report := ValidatePaths([]string{skill}, Options{Roots: []roots.ActiveRoot{{}}})
	assertDiagnostic(
		t,
		report,
		LevelError,
		CodeCompatibilityConsumer,
		"compatibility.agents",
	)
}

func TestValidatePathsReturnsInitializedEmptyDiagnostics(t *testing.T) {
	skill := makeSkill(t, t.TempDir(), "portable", validSkill("portable"))
	report := ValidatePaths([]string{skill}, Options{})
	if report.Diagnostics == nil {
		t.Fatal("Diagnostics = nil, want empty JSON array")
	}
}

func makeSkill(t *testing.T, parent, name, document string) string {
	t.Helper()
	dir := filepath.Join(parent, name)
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "SKILL.md"), document)
	return dir
}

func validSkill(name string) string {
	return "---\nname: " + name + "\ndescription: Portable skill\n---\nBody\n"
}

func diagnosticSortKey(diagnostic Diagnostic) string {
	return diagnostic.Path + "\x00" + string(diagnostic.Level) + "\x00" + diagnostic.Code +
		"\x00" + diagnostic.Field + "\x00" + diagnostic.RelatedPath + "\x00" + diagnostic.Message
}
