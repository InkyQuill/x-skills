package compatibility

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	agentMentionPattern   = regexp.MustCompile(`(?i)\b(claude|codex|opencode|crush|pi)\b`)
	claudeVariablePattern = regexp.MustCompile(`(?i)\b(?:read|use|set|export|run|open|resolve|inspect|load|write|change|cd)\b[^\n]*\$CLAUDE_PROJECT_DIR\b`)
	claudeToolPattern     = regexp.MustCompile("(?i)\\b(?:must(?:\\s+\\w+){0,3}|always|required\\s*:?)\\s+(?:use|call|invoke)\\s+(?:the\\s+)?`?(?:AskUserQuestion|EnterPlanMode|ExitPlanMode|Task tool)`?\\b")
	hookKeyPattern        = regexp.MustCompile(`(?i)^\s*hooks?\s*:`)
	negationPattern       = regexp.MustCompile(`(?i)\b(?:do not|don't|never|avoid|must not|not required)\b`)
	nonExecutablePattern  = regexp.MustCompile(`(?i)\b(?:example|counterexample|comparison|migration note)\s*:`)
	quotedTextPattern     = regexp.MustCompile(`"[^"]*"|'[^']*'`)
)

const (
	variableReason = "uses the Claude-only $CLAUDE_PROJECT_DIR runtime variable"
	toolReason     = "mandates a Claude-only tool"
	hookReason     = "declares Claude-only hook configuration"
	mentionReason  = "mentions an agent without exclusive executable semantics"
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

	var variableSignal bool
	var toolSignal bool
	var hookSignal bool
	var mentionSignal bool
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			return inference{}, fmt.Errorf("read compatibility input %s: %w", path, err)
		}
		content := string(data)
		for _, line := range executableLines(content) {
			variableSignal = variableSignal || claudeVariablePattern.MatchString(line)
			toolSignal = toolSignal || claudeToolPattern.MatchString(line)
			if isClaudeHookMetadata(skillDir, path) {
				hookSignal = hookSignal || hookKeyPattern.MatchString(line)
			}
		}
		mentionSignal = mentionSignal || agentMentionPattern.MatchString(content)
	}

	result := inference{Confidence: ConfidenceUnknown}
	if variableSignal || toolSignal || hookSignal {
		result.Confidence = ConfidenceHigh
		result.Agents = []string{"claude"}
		if variableSignal {
			result.Reasons = append(result.Reasons, variableReason)
		}
		if toolSignal {
			result.Reasons = append(result.Reasons, toolReason)
		}
		if hookSignal {
			result.Reasons = append(result.Reasons, hookReason)
		}
	} else if mentionSignal {
		result.Confidence = ConfidenceLow
		result.Reasons = []string{mentionReason}
	}
	return result, nil
}

func inferenceFiles(skillDir string) ([]string, error) {
	files := []string{filepath.Join(skillDir, "SKILL.md")}
	agentsDir := filepath.Join(skillDir, "agents")
	dirInfo, err := os.Lstat(agentsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return files, nil
		}
		return nil, fmt.Errorf("inspect agent metadata directory: %w", err)
	}
	if !dirInfo.IsDir() || dirInfo.Mode()&os.ModeSymlink != 0 {
		return files, nil
	}
	for _, name := range []string{"claude.yaml", "openai.yaml"} {
		path := filepath.Join(agentsDir, name)
		info, err := os.Lstat(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("inspect agent metadata %s: %w", path, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		files = append(files, path)
	}
	return files, nil
}

func isClaudeHookMetadata(skillDir, path string) bool {
	relative, err := filepath.Rel(filepath.Join(skillDir, "agents"), path)
	if err != nil || strings.HasPrefix(relative, "..") {
		return false
	}
	return strings.EqualFold(filepath.ToSlash(relative), "claude.yaml")
}

func executableLines(content string) []string {
	lines := make([]string, 0)
	inFence := false
	exampleSectionLevel := 0
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if level, title, ok := markdownHeading(trimmed); ok {
			if exampleSectionLevel > 0 && level <= exampleSectionLevel {
				exampleSectionLevel = 0
			}
			if strings.EqualFold(title, "example") || strings.EqualFold(title, "examples") {
				exampleSectionLevel = level
			}
			continue
		}
		if exampleSectionLevel > 0 {
			continue
		}
		if trimmed == "" ||
			strings.HasPrefix(trimmed, ">") || strings.HasPrefix(trimmed, "|") ||
			strings.HasPrefix(trimmed, `"`) || strings.Contains(trimmed, "://") ||
			nonExecutablePattern.MatchString(trimmed) || negationPattern.MatchString(trimmed) {
			continue
		}
		lines = append(lines, quotedTextPattern.ReplaceAllString(trimmed, ""))
	}
	return lines
}

func markdownHeading(line string) (int, string, bool) {
	level := 0
	for level < len(line) && level < 6 && line[level] == '#' {
		level++
	}
	if level == 0 || level == len(line) || line[level] != ' ' {
		return 0, "", false
	}
	title := strings.TrimSpace(strings.TrimRight(line[level+1:], "#"))
	return level, title, true
}
