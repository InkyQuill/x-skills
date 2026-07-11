package compatibility

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

var (
	agentMentionPattern = regexp.MustCompile(`(?i)\b(claude|codex|opencode|crush|pi)\b`)
	claudeToolPattern   = regexp.MustCompile(`(?i)\b(?:must|always|required to)\s+(?:use|call|invoke)\s+(?:the\s+)?(?:AskUserQuestion|EnterPlanMode|ExitPlanMode|Task tool)\b`)
	hookKeyPattern      = regexp.MustCompile(`(?im)^\s*hooks?\s*:`)
)

type inference struct {
	Confidence Confidence
	Agents     []string
	Reasons    []string
}

func infer(skillDir string) (inference, error) {
	files, err := inferenceFiles(skillDir)
	if err != nil {
		return inference{}, err
	}

	result := inference{Confidence: ConfidenceUnknown}
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			return inference{}, fmt.Errorf("read compatibility input %s: %w", path, err)
		}
		content := string(data)
		if strings.Contains(content, "$CLAUDE_PROJECT_DIR") {
			result.Confidence = ConfidenceHigh
			result.Agents = []string{"claude"}
			result.Reasons = append(result.Reasons, "uses the Claude-only $CLAUDE_PROJECT_DIR runtime variable")
		}
		if claudeToolPattern.MatchString(content) {
			result.Confidence = ConfidenceHigh
			result.Agents = []string{"claude"}
			result.Reasons = append(result.Reasons, "mandates a Claude-only tool")
		}
		if isClaudeHookMetadata(skillDir, path, content) {
			result.Confidence = ConfidenceHigh
			result.Agents = []string{"claude"}
			result.Reasons = append(result.Reasons, "declares Claude-only hook configuration")
		}
		if result.Confidence != ConfidenceHigh && agentMentionPattern.MatchString(content) {
			result.Confidence = ConfidenceLow
			result.Reasons = append(result.Reasons, "mentions an agent without exclusive executable semantics")
		}
	}
	result.Agents = sortedUnique(result.Agents)
	result.Reasons = unique(result.Reasons)
	return result, nil
}

func inferenceFiles(skillDir string) ([]string, error) {
	files := []string{filepath.Join(skillDir, "SKILL.md")}
	agentsDir := filepath.Join(skillDir, "agents")
	err := filepath.WalkDir(agentsDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		switch strings.ToLower(filepath.Ext(path)) {
		case ".json", ".yaml", ".yml":
			files = append(files, path)
		}
		return nil
	})
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("scan agent metadata: %w", err)
	}
	slices.Sort(files)
	return files, nil
}

func isClaudeHookMetadata(skillDir, path, content string) bool {
	relative, err := filepath.Rel(filepath.Join(skillDir, "agents"), path)
	if err != nil || strings.HasPrefix(relative, "..") {
		return false
	}
	name := strings.ToLower(filepath.ToSlash(relative))
	return strings.Contains(name, "claude") && hookKeyPattern.MatchString(content)
}

func unique(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if !slices.Contains(result, value) {
			result = append(result, value)
		}
	}
	return result
}
