package validation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidatePathsPortableSkillDocument(t *testing.T) {
	tests := []struct {
		name      string
		front     string
		body      string
		dirName   string
		wantCode  string
		wantLevel Level
	}{
		{name: "lf document", front: "name: portable\ndescription: Portable skill", body: "# Body\n"},
		{name: "crlf document", front: "name: portable\r\ndescription: Portable skill", body: "# Body\r\n"},
		{
			name: "vendor fields",
			front: "name: portable\ndescription: Portable skill\nlicense: MIT\n" +
				"compatibility: Designed for Codex\nmetadata:\n  vendor: value",
			body: "Body\n",
		},
		{
			name:      "missing closing delimiter",
			front:     "name: portable\ndescription: Portable skill\nBody",
			wantCode:  CodeFrontmatterMalformed,
			wantLevel: LevelError,
		},
		{name: "malformed yaml", front: "name: [", body: "Body\n", wantCode: CodeFrontmatterMalformed, wantLevel: LevelError},
		{
			name:      "non mapping yaml",
			front:     "- portable\n- skill",
			body:      "Body\n",
			wantCode:  CodeFrontmatterMalformed,
			wantLevel: LevelError,
		},
		{
			name: "missing name", front: "description: Portable skill", body: "Body\n",
			wantCode: CodeNameRequired, wantLevel: LevelError,
		},
		{
			name: "empty name", front: "name: '  '\ndescription: Portable skill", body: "Body\n",
			wantCode: CodeNameRequired, wantLevel: LevelError,
		},
		{
			name: "wrong type name", front: "name: [portable]\ndescription: Portable skill", body: "Body\n",
			wantCode: CodeNameRequired, wantLevel: LevelError,
		},
		{
			name: "uppercase name", front: "name: Portable\ndescription: Portable skill", body: "Body\n",
			wantCode: CodeNameInvalid, wantLevel: LevelError,
		},
		{
			name: "padded name", front: "name: ' portable '\ndescription: Portable skill", body: "Body\n",
			wantCode: CodeNameInvalid, wantLevel: LevelError,
		},
		{
			name: "leading hyphen", front: "name: -portable\ndescription: Portable skill", body: "Body\n",
			wantCode: CodeNameInvalid, wantLevel: LevelError,
		},
		{
			name: "trailing hyphen", front: "name: portable-\ndescription: Portable skill", body: "Body\n",
			wantCode: CodeNameInvalid, wantLevel: LevelError,
		},
		{
			name: "consecutive hyphens", front: "name: portable--skill\ndescription: Portable skill", body: "Body\n",
			wantCode: CodeNameInvalid, wantLevel: LevelError,
		},
		{
			name: "leading digit allowed", front: "name: 2portable\ndescription: Portable skill",
			body: "Body\n", dirName: "2portable",
		},
		{
			name:  "name too long",
			front: "name: " + strings.Repeat("a", 65) + "\ndescription: Portable skill",
			body:  "Body\n", wantCode: CodeNameInvalid, wantLevel: LevelError,
		},
		{
			name: "missing description", front: "name: portable", body: "Body\n",
			wantCode: CodeDescriptionRequired, wantLevel: LevelError,
		},
		{
			name: "empty description", front: "name: portable\ndescription: '  '", body: "Body\n",
			wantCode: CodeDescriptionRequired, wantLevel: LevelError,
		},
		{
			name: "wrong type description", front: "name: portable\ndescription: [skill]", body: "Body\n",
			wantCode: CodeDescriptionRequired, wantLevel: LevelError,
		},
		{
			name:  "description too long",
			front: "name: portable\ndescription: " + strings.Repeat("a", 1025),
			body:  "Body\n", wantCode: CodeDescriptionInvalid, wantLevel: LevelError,
		},
		{
			name:  "quoted description whitespace counts toward limit",
			front: "name: portable\ndescription: ' " + strings.Repeat("a", 1023) + " '",
			body:  "Body\n", wantCode: CodeDescriptionInvalid, wantLevel: LevelError,
		},
		{
			name: "description angle bracket", front: "name: portable\ndescription: Use <portable>",
			body: "Body\n", wantCode: CodeDescriptionInvalid, wantLevel: LevelError,
		},
		{
			name: "empty body", front: "name: portable\ndescription: Portable skill", body: " \r\n\t",
			wantCode: CodeBodyEmpty, wantLevel: LevelError,
		},
		{
			name: "identity mismatch", front: "name: other\ndescription: Portable skill", body: "Body\n",
			wantCode: CodeIdentityMismatch, wantLevel: LevelWarning,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dirName := test.dirName
			if dirName == "" {
				dirName = "portable"
			}
			skill := filepath.Join(t.TempDir(), dirName)
			if err := os.Mkdir(skill, 0o755); err != nil {
				t.Fatal(err)
			}
			newline := "\n"
			if strings.Contains(test.front, "\r\n") {
				newline = "\r\n"
			}
			document := "---" + newline + test.front + newline + "---" + newline + test.body
			if test.name == "missing closing delimiter" {
				document = "---\n" + test.front
			}
			writeFile(t, filepath.Join(skill, "SKILL.md"), document)

			report := ValidatePaths([]string{skill}, Options{})
			if report.Summary.Skills != 1 {
				t.Fatalf("skills = %d, want 1", report.Summary.Skills)
			}
			if test.wantCode == "" {
				if !report.Valid || len(report.Diagnostics) != 0 {
					t.Fatalf("report = %#v, want valid without diagnostics", report)
				}
				return
			}
			diagnostic := assertDiagnostic(t, report, test.wantLevel, test.wantCode, "")
			if diagnostic.Path != skill {
				t.Fatalf("diagnostic path = %q, want skill directory %q", diagnostic.Path, skill)
			}
			if test.wantLevel == LevelError && report.Valid {
				t.Fatal("Valid = true, want false")
			}
			if test.wantLevel == LevelWarning && !report.Valid {
				t.Fatal("Valid = false for warning-only report")
			}
		})
	}
}

func TestValidateSkillAggregatesMetadataWhenSkillDocumentReadFails(t *testing.T) {
	skill := filepath.Join(t.TempDir(), "portable")
	if err := os.Mkdir(skill, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(skill, "SKILL.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(skill, ".x-skills.json"), "{")

	diagnostics := validateSkill(skill, nil, false)
	report := Report{Diagnostics: diagnostics}
	assertDiagnostic(t, report, LevelError, CodeFrontmatterMalformed, "")
	assertDiagnostic(t, report, LevelError, CodeMetadataInvalid, "")
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertDiagnostic(t *testing.T, report Report, level Level, code, field string) Diagnostic {
	t.Helper()
	for _, diagnostic := range report.Diagnostics {
		fieldMatches := field == "" || diagnostic.Field == field
		if diagnostic.Level == level && diagnostic.Code == code && fieldMatches {
			return diagnostic
		}
	}
	t.Fatalf("diagnostic (%s, %s, %s) missing from %#v", level, code, field, report.Diagnostics)
	return Diagnostic{}
}
