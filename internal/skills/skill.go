package skills

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	scanner := bufio.NewScanner(strings.NewReader(content))
	if !scanner.Scan() || strings.TrimSpace(scanner.Text()) != "---" {
		return "", ""
	}

	var name string
	var description string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "---" {
			break
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}

		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		switch strings.TrimSpace(key) {
		case "name":
			name = value
		case "description":
			description = value
		}
	}

	return name, description
}
