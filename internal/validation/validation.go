package validation

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/InkyQuill/x-skills/internal/pathidentity"
	"github.com/InkyQuill/x-skills/internal/roots"
)

var errUnsupportedInput = errors.New("unsupported validation input")

type Options struct {
	Roots []roots.ActiveRoot
}

type Level string

const (
	LevelError   Level = "error"
	LevelWarning Level = "warning"
)

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

func ValidatePaths(paths []string, opts Options) Report {
	diagnostics := []Diagnostic{}
	skillPaths := map[string]string{}
	seenInputs := map[string]struct{}{}

	sortedPaths := slices.Clone(paths)
	slices.Sort(sortedPaths)
	for _, path := range sortedPaths {
		if strings.TrimSpace(path) == "" {
			const emptyInputKey = "\x00empty-input"
			if _, exists := seenInputs[emptyInputKey]; exists {
				continue
			}
			seenInputs[emptyInputKey] = struct{}{}
			diagnostics = append(diagnostics, Diagnostic{
				Path:    "<empty>",
				Level:   LevelError,
				Code:    CodeInputUnsupported,
				Message: "input path must not be empty",
			})
			continue
		}

		inputPath, info, err := classifyInput(path)
		if err != nil {
			key := inputDiagnosticPath(path)
			if _, exists := seenInputs[key]; exists {
				continue
			}
			seenInputs[key] = struct{}{}
			diagnostics = append(
				diagnostics,
				pathErrorDiagnostic(key, "inspect input path", err),
			)
			continue
		}

		if _, exists := seenInputs[inputPath]; exists {
			continue
		}
		seenInputs[inputPath] = struct{}{}

		if !info.IsDir() {
			diagnostics = append(diagnostics, Diagnostic{
				Path:    inputPath,
				Level:   LevelError,
				Code:    CodeInputUnsupported,
				Message: "input file must be named SKILL.md",
			})
			continue
		}

		isSkill, err := isSkillDir(inputPath)
		if err != nil {
			diagnostics = append(
				diagnostics,
				pathErrorDiagnostic(inputPath, "inspect skill directory", err),
			)
			continue
		}
		if isSkill {
			if err := addSkillPath(skillPaths, inputPath); err != nil {
				diagnostics = append(
					diagnostics,
					pathErrorDiagnostic(inputPath, "canonicalize skill directory", err),
				)
			}
			continue
		}

		children, discoveryErr := immediateSkillDirs(inputPath)
		if discoveryErr != nil {
			diagnostics = append(
				diagnostics,
				pathErrorDiagnostic(inputPath, "inspect collection", discoveryErr),
			)
		}
		if len(children) == 0 {
			if discoveryErr == nil {
				diagnostics = append(diagnostics, Diagnostic{
					Path:    inputPath,
					Level:   LevelError,
					Code:    CodeCollectionEmpty,
					Message: "collection contains no immediate child skills",
				})
			}
			continue
		}
		for _, child := range children {
			if err := addSkillPath(skillPaths, child); err != nil {
				diagnostics = append(
					diagnostics,
					pathErrorDiagnostic(child, "canonicalize discovered skill", err),
				)
			}
		}
	}

	canonicalSkills := make([]string, 0, len(skillPaths))
	for canonical := range skillPaths {
		canonicalSkills = append(canonicalSkills, canonical)
	}
	slices.Sort(canonicalSkills)
	consumers := rootConsumers(opts.Roots)
	for _, skillDir := range canonicalSkills {
		diagnostics = append(diagnostics, validateSkill(skillDir, consumers, len(opts.Roots) > 0)...)
	}

	slices.SortFunc(diagnostics, func(a, b Diagnostic) int {
		return strings.Compare(diagnosticKey(a), diagnosticKey(b))
	})
	summary := Summary{Skills: len(canonicalSkills)}
	for _, diagnostic := range diagnostics {
		switch diagnostic.Level {
		case LevelError:
			summary.Errors++
		case LevelWarning:
			summary.Warnings++
		}
	}
	return Report{
		Valid:       summary.Errors == 0,
		Summary:     summary,
		Diagnostics: diagnostics,
	}
}

func classifyInput(path string) (string, os.FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", nil, err
	}
	canonical, err := pathidentity.Canonical(path)
	if err != nil {
		return "", nil, err
	}
	if filepath.Base(filepath.Clean(path)) == "SKILL.md" {
		if !info.Mode().IsRegular() {
			return "", nil, errUnsupportedInput
		}
		parent, err := pathidentity.Canonical(filepath.Dir(canonical))
		if err != nil {
			return "", nil, err
		}
		parentInfo, err := os.Stat(parent)
		return parent, parentInfo, err
	}
	return canonical, info, nil
}

func inputDiagnosticPath(path string) string {
	if canonical, err := pathidentity.CanonicalEntry(path); err == nil {
		return canonical
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return filepath.Clean(abs)
}

func isSkillDir(path string) (bool, error) {
	info, err := os.Stat(filepath.Join(path, "SKILL.md"))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("stat SKILL.md: %w", err)
	}
	return info.Mode().IsRegular(), nil
}

func immediateSkillDirs(collection string) ([]string, error) {
	entries, err := os.ReadDir(collection)
	if err != nil {
		return nil, fmt.Errorf("read collection: %w", err)
	}
	result := []string{}
	inspectionErrors := []error{}
	for _, entry := range entries {
		child := filepath.Join(collection, entry.Name())
		isSkill, err := isImmediateSkillDir(child)
		if err != nil {
			inspectionErrors = append(inspectionErrors, err)
			continue
		}
		if !isSkill {
			continue
		}
		result = append(result, child)
	}
	slices.Sort(result)
	return result, errors.Join(inspectionErrors...)
}

func isImmediateSkillDir(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, fmt.Errorf("stat collection child %q: %w", path, err)
	}
	if !info.IsDir() {
		return false, nil
	}
	isSkill, err := isSkillDir(path)
	if err != nil {
		return false, fmt.Errorf("inspect collection child %q: %w", path, err)
	}
	return isSkill, nil
}

func addSkillPath(paths map[string]string, path string) error {
	canonical, err := pathidentity.Canonical(path)
	if err != nil {
		return fmt.Errorf("canonicalize %q: %w", path, err)
	}
	paths[canonical] = canonical
	return nil
}

func rootConsumers(activeRoots []roots.ActiveRoot) []string {
	seen := map[string]struct{}{}
	for _, root := range activeRoots {
		for _, consumer := range root.Consumers {
			consumer = strings.ToLower(strings.TrimSpace(consumer))
			if consumer != "" {
				seen[consumer] = struct{}{}
			}
		}
	}
	result := make([]string, 0, len(seen))
	for consumer := range seen {
		result = append(result, consumer)
	}
	slices.Sort(result)
	return result
}

func diagnosticKey(diagnostic Diagnostic) string {
	return diagnostic.Path + "\x00" + string(diagnostic.Level) + "\x00" + diagnostic.Code +
		"\x00" + diagnostic.Field + "\x00" + diagnostic.RelatedPath + "\x00" + diagnostic.Message
}

func pathErrorDiagnostic(path, action string, err error) Diagnostic {
	code := CodeInputUnsupported
	if errors.Is(err, os.ErrNotExist) {
		code = CodeInputMissing
	}
	return Diagnostic{
		Path:    path,
		Level:   LevelError,
		Code:    code,
		Message: fmt.Sprintf("%s: %v", action, err),
	}
}
