package validation

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/InkyQuill/x-skills/internal/skills"
	"gopkg.in/yaml.v3"
)

var portableName = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func validateSkill(skillDir string, consumers []string, checkConsumers bool) []Diagnostic {
	skillPath := filepath.Join(skillDir, "SKILL.md")
	data, err := os.ReadFile(skillPath)
	if err != nil {
		diagnostics := []Diagnostic{{
			Path:    skillDir,
			Level:   LevelError,
			Code:    CodeFrontmatterMalformed,
			Message: fmt.Sprintf("cannot read SKILL.md: %v", err),
		}}
		return append(diagnostics, validateSource(skillDir, consumers, checkConsumers)...)
	}

	document, parseErr := skills.ParseDocument(data)
	frontmatter, body, hasFrontmatter := portableDocumentParts(string(data))
	if !hasFrontmatter {
		message := "SKILL.md must contain valid YAML frontmatter"
		return append([]Diagnostic{{
			Path:    skillDir,
			Level:   LevelError,
			Code:    CodeFrontmatterMalformed,
			Message: message,
		}}, validateSource(skillDir, consumers, checkConsumers)...)
	}

	var fields map[string]any
	if err := yaml.Unmarshal([]byte(frontmatter), &fields); err != nil || fields == nil {
		message := "skill frontmatter must be a YAML mapping"
		if err != nil {
			message = err.Error()
		}
		return append([]Diagnostic{{
			Path:    skillDir,
			Level:   LevelError,
			Code:    CodeFrontmatterMalformed,
			Message: message,
		}}, validateSource(skillDir, consumers, checkConsumers)...)
	}

	diagnostics := []Diagnostic{}
	if parseErr == nil {
		body = document.Body
	}
	name, isNameString := fields["name"].(string)
	if !isNameString || strings.TrimSpace(name) == "" {
		diagnostics = append(diagnostics, Diagnostic{
			Path:    skillDir,
			Level:   LevelError,
			Code:    CodeNameRequired,
			Message: "frontmatter name must be a nonempty string",
			Field:   "name",
		})
	} else {
		isNameValid := portableName.MatchString(name) && utf8.RuneCountInString(name) <= 64
		if !isNameValid {
			diagnostics = append(diagnostics, Diagnostic{
				Path:    skillDir,
				Level:   LevelError,
				Code:    CodeNameInvalid,
				Message: "frontmatter name must be at most 64 characters of lowercase letters, digits, and single interior hyphens",
				Field:   "name",
			})
		}
		if isNameValid && name != filepath.Base(skillDir) {
			diagnostics = append(diagnostics, Diagnostic{
				Path:        skillDir,
				Level:       LevelWarning,
				Code:        CodeIdentityMismatch,
				Message:     fmt.Sprintf("frontmatter name %q does not match directory name %q", name, filepath.Base(skillDir)),
				Field:       "name",
				RelatedPath: skillPath,
			})
		}
	}

	description, isDescriptionString := fields["description"].(string)
	if !isDescriptionString || strings.TrimSpace(description) == "" {
		diagnostics = append(diagnostics, Diagnostic{
			Path:    skillDir,
			Level:   LevelError,
			Code:    CodeDescriptionRequired,
			Message: "frontmatter description must be a nonempty string",
			Field:   "description",
		})
	} else if utf8.RuneCountInString(description) > 1024 || strings.ContainsAny(description, "<>") {
		diagnostics = append(diagnostics, Diagnostic{
			Path:    skillDir,
			Level:   LevelError,
			Code:    CodeDescriptionInvalid,
			Message: "frontmatter description must be at most 1024 characters and contain no angle brackets",
			Field:   "description",
		})
	}

	if strings.TrimSpace(body) == "" {
		diagnostics = append(diagnostics, Diagnostic{
			Path:    skillDir,
			Level:   LevelError,
			Code:    CodeBodyEmpty,
			Message: "SKILL.md body must not be empty",
		})
	}
	diagnostics = append(diagnostics, validateSource(skillDir, consumers, checkConsumers)...)
	return diagnostics
}

func portableDocumentParts(content string) (string, string, bool) {
	content = strings.TrimPrefix(content, "\ufeff")
	content = strings.ReplaceAll(content, "\r\n", "\n")
	if !strings.HasPrefix(content, "---\n") {
		return "", "", false
	}
	rest := strings.TrimPrefix(content, "---\n")
	frontmatter, body, ok := strings.Cut(rest, "\n---\n")
	if ok {
		return frontmatter, body, true
	}
	if strings.HasSuffix(rest, "\n---") {
		return strings.TrimSuffix(rest, "\n---"), "", true
	}
	return "", "", false
}
