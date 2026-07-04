package repo

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/skills"
)

type Skill struct {
	Name        string
	Path        string
	Description string
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

		info, err := skills.Read(path)
		if err != nil {
			return nil, fmt.Errorf("read repo skill %q: %w", entry.Name(), err)
		}
		found = append(found, Skill{
			Name:        info.Name,
			Path:        info.Path,
			Description: info.Description,
		})
	}

	sort.Slice(found, func(i, j int) bool {
		return found[i].Name < found[j].Name
	})

	return found, nil
}

func SkillPath(cfg config.Config, name string) string {
	return filepath.Join(cfg.ArchiveSkillsRoot(), name)
}
