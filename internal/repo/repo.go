package repo

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/remote"
	"github.com/InkyQuill/x-skills/internal/skills"
)

type Skill struct {
	Identity     string
	DeclaredName string
	Path         string
	Description  string
	Source       *remote.SourceMetadata
}

var readSkill = func(path string) (skills.Document, error) {
	data, err := os.ReadFile(filepath.Join(path, "SKILL.md"))
	if err != nil {
		return skills.Document{}, err
	}
	return skills.ParseDocument(data)
}

func List(cfg config.Config) ([]Skill, error) {
	root := cfg.ArchiveSkillsRoot()
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list repo skills: %w", err)
	}

	var found []Skill
	for _, entry := range entries {
		path := filepath.Join(root, entry.Name())
		if !entry.IsDir() || !skills.IsDir(path) {
			continue
		}

		info, err := readSkill(path)
		if err != nil {
			continue
		}
		source, ok, err := remote.ReadSourceMetadata(path)
		if err != nil {
			source = remote.SourceMetadata{}
			ok = false
		}
		var sourcePtr *remote.SourceMetadata
		if ok {
			sourcePtr = &source
		}
		found = append(found, Skill{
			Identity:     entry.Name(),
			DeclaredName: info.DeclaredName,
			Path:         path,
			Description:  info.Description,
			Source:       sourcePtr,
		})
	}

	sort.Slice(found, func(i, j int) bool {
		return found[i].Identity < found[j].Identity
	})

	return found, nil
}

func SkillPath(cfg config.Config, name string) (string, error) {
	if err := ValidateName(name); err != nil {
		return "", err
	}
	return filepath.Join(cfg.ArchiveSkillsRoot(), name), nil
}

func DeleteSkill(cfg config.Config, name string) (string, error) {
	path, err := SkillPath(cfg, name)
	if err != nil {
		return "", err
	}
	if err := os.RemoveAll(path); err != nil {
		return "", fmt.Errorf("delete repo skill %q: %w", path, err)
	}
	return path, nil
}

func ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("invalid skill name: %q", name)
	}
	if filepath.IsAbs(name) || name == "." || name == ".." || filepath.Clean(name) != name {
		return fmt.Errorf("invalid skill name: %q", name)
	}
	if strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("invalid skill name: %q", name)
	}
	return nil
}

func HasSkill(cfg config.Config, name string) bool {
	path, err := SkillPath(cfg, name)
	if err != nil {
		return false
	}
	return skills.IsDir(path)
}
