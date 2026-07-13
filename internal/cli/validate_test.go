package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/InkyQuill/x-skills/internal/validation"
)

func TestValidateRequiresPath(t *testing.T) {
	var out bytes.Buffer
	err := executeValidate(t, []string{"validate"}, &out)
	if err == nil || !strings.Contains(err.Error(), "requires at least 1 arg") {
		t.Fatalf("err = %v, want missing path error", err)
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", out.String())
	}
}

func TestValidateAcceptsRepeatedLocations(t *testing.T) {
	skill := writeValidationSkill(t, t.TempDir(), "portable", "portable", "Portable skill.")
	metadata := `{"schema_version":2,"compatibility":{"agents":["claude","codex"]}}`
	if err := os.WriteFile(filepath.Join(skill, ".x-skills.json"), []byte(metadata), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	err := executeValidate(t, []string{
		"validate", skill,
		"--at", "project:claude",
		"--at", "project:codex",
	}, &out)
	if err != nil {
		t.Fatalf("validate repeated --at: %v\n%s", err, out.String())
	}
	if !strings.HasSuffix(out.String(), "1 skills, 0 errors, 0 warnings\n") {
		t.Fatalf("stdout = %q, want successful summary", out.String())
	}
}

func TestValidateWarningsReturnSuccess(t *testing.T) {
	skill := writeValidationSkill(t, t.TempDir(), "directory-name", "declared-name", "Portable skill.")

	var out bytes.Buffer
	if err := executeValidate(t, []string{"validate", skill}, &out); err != nil {
		t.Fatalf("validate warning: %v\n%s", err, out.String())
	}
	output := out.String()
	if !strings.Contains(output, "warning skill.identity_mismatch") {
		t.Fatalf("stdout missing warning:\n%s", output)
	}
	if !strings.HasSuffix(output, "1 skills, 0 errors, 1 warnings\n") {
		t.Fatalf("stdout missing summary:\n%s", output)
	}
}

func TestValidateErrorsPrintCompleteHumanReportBeforeFailure(t *testing.T) {
	skill := writeValidationSkill(t, t.TempDir(), "broken", "Bad Name", "")

	var out bytes.Buffer
	err := executeValidate(t, []string{"validate", skill}, &out)
	if err == nil || err.Error() != "validation failed" {
		t.Fatalf("err = %v, want validation failed", err)
	}
	output := out.String()
	for _, code := range []string{
		validation.CodeNameInvalid,
		validation.CodeDescriptionRequired,
	} {
		if !strings.Contains(output, "error "+code) {
			t.Errorf("stdout missing %s:\n%s", code, output)
		}
	}
	if count := strings.Count(output, skill+"\n"); count != 1 {
		t.Errorf("path heading count = %d, want 1:\n%s", count, output)
	}
	if !strings.HasSuffix(output, "1 skills, 2 errors, 0 warnings\n") {
		t.Errorf("stdout missing complete summary:\n%s", output)
	}
}

func TestValidateJSONMatchesValidationReport(t *testing.T) {
	skill := writeValidationSkill(t, t.TempDir(), "broken", "Bad Name", "Portable skill.")

	var out bytes.Buffer
	err := executeValidate(t, []string{"--json", "validate", skill}, &out)
	if err == nil {
		t.Fatal("err = nil, want validation failure")
	}
	var got validation.Report
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("decode json: %v\n%s", err, out.String())
	}
	expected := validation.ValidatePaths([]string{skill}, validation.Options{})
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("report = %#v, want %#v", got, expected)
	}
}

func TestValidateValidJSONHasEmptyDiagnosticsArray(t *testing.T) {
	skill := writeValidationSkill(t, t.TempDir(), "portable", "portable", "Portable skill.")

	var out bytes.Buffer
	if err := executeValidate(t, []string{"validate", skill, "--json"}, &out); err != nil {
		t.Fatalf("validate: %v\n%s", err, out.String())
	}
	var report validation.Report
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("decode json: %v\n%s", err, out.String())
	}
	if !report.Valid {
		t.Fatalf("valid = false: %#v", report)
	}
	if report.Diagnostics == nil || len(report.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v, want initialized empty slice", report.Diagnostics)
	}
	if !bytes.Contains(out.Bytes(), []byte(`"diagnostics": []`)) {
		t.Fatalf("json diagnostics is not an array:\n%s", out.String())
	}
}

func TestValidateRejectsUnknownLocation(t *testing.T) {
	skill := writeValidationSkill(t, t.TempDir(), "portable", "portable", "Portable skill.")

	var out bytes.Buffer
	err := executeValidate(t, []string{"validate", skill, "--at", "project:bogus"}, &out)
	if err == nil || !strings.Contains(err.Error(), `unknown --at location "project:bogus"; run x-skills list-roots`) {
		t.Fatalf("err = %v, want actionable unknown location", err)
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", out.String())
	}
}

func executeValidate(t *testing.T, args []string, out *bytes.Buffer) error {
	t.Helper()
	home := t.TempDir()
	project := t.TempDir()
	argv := append([]string{"--home", home, "--project-root", project}, args...)
	return Execute(argv, strings.NewReader(""), out, &bytes.Buffer{})
}

func writeValidationSkill(t *testing.T, root, directoryName, declaredName, description string) string {
	t.Helper()
	dir := filepath.Join(root, directoryName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	document := "---\nname: " + declaredName + "\ndescription: " + description + "\n---\n# Instructions\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(document), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}
