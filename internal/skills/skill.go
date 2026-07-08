package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Info struct {
	Name        string
	Path        string
	Description string
}

func Read(path string) (Info, error) {
	skillPath := filepath.Join(path, "SKILL.md")
	data, err := os.ReadFile(skillPath)
	if err != nil {
		return Info{}, fmt.Errorf("read skill metadata: %w", err)
	}

	name, description := parseFrontmatter(string(data))
	if name == "" {
		name = filepath.Base(path)
	}

	return Info{
		Name:        name,
		Path:        path,
		Description: description,
	}, nil
}

func IsDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return false
	}

	skillInfo, err := os.Stat(filepath.Join(path, "SKILL.md"))
	if err != nil {
		return false
	}
	return !skillInfo.IsDir()
}

func parseFrontmatter(content string) (string, string) {
	frontmatter, ok := frontmatterBlock(content)
	if !ok {
		return "", ""
	}

	var fields struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	}
	if err := yaml.Unmarshal([]byte(frontmatter), &fields); err != nil {
		return "", ""
	}

	return fields.Name, fields.Description
}

func frontmatterBlock(content string) (string, bool) {
	content = strings.TrimPrefix(content, "\ufeff")
	if !strings.HasPrefix(content, "---") {
		return "", false
	}
	afterStart := content[3:]
	if strings.HasPrefix(afterStart, "\r\n") {
		afterStart = afterStart[2:]
	} else if strings.HasPrefix(afterStart, "\n") {
		afterStart = afterStart[1:]
	} else {
		return "", false
	}
	for _, marker := range []string{"\r\n---\r\n", "\r\n---\n", "\n---\r\n", "\n---\n"} {
		if before, _, ok := strings.Cut(afterStart, marker); ok {
			return before, true
		}
	}
	if strings.HasSuffix(afterStart, "\r\n---") {
		return strings.TrimSuffix(afterStart, "\r\n---"), true
	}
	if strings.HasSuffix(afterStart, "\n---") {
		return strings.TrimSuffix(afterStart, "\n---"), true
	}
	return "", false
}
