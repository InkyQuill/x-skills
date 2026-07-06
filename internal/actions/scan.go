package actions

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/repo"
	"github.com/InkyQuill/x-skills/internal/roots"
	"github.com/InkyQuill/x-skills/internal/skills"
	"github.com/InkyQuill/x-skills/internal/symlinkcheck"
)

const (
	StatusManaged   = "managed"
	StatusUnmanaged = "unmanaged"
	StatusBroken    = "broken"
)

type ActiveSkill struct {
	Name        string
	Root        roots.ActiveRoot
	Path        string
	Status      string
	Description string
	Reason      string
}

type ScanFilter struct {
	Scope  string
	Target string
}

func ScanActive(cfg config.Config, filter ScanFilter) ([]ActiveSkill, error) {
	activeRoots := roots.ActiveRoots(cfg, roots.Filter{
		Scope:  filter.Scope,
		Target: filter.Target,
	})

	var found []ActiveSkill
	for _, root := range activeRoots {
		activeSkills, err := scanRoot(cfg, root)
		if err != nil {
			return nil, err
		}
		found = append(found, activeSkills...)
	}

	return found, nil
}

func scanRoot(cfg config.Config, root roots.ActiveRoot) ([]ActiveSkill, error) {
	entries, err := os.ReadDir(root.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("scan active root %q: %w", root.Path, err)
	}

	var found []ActiveSkill
	for _, entry := range entries {
		activePath := filepath.Join(root.Path, entry.Name())
		info, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("inspect active skill %q: %w", activePath, err)
		}

		if info.Mode()&os.ModeSymlink != 0 {
			found = append(found, scanSymlink(cfg, root, activePath, entry.Name()))
			continue
		}
		if !entry.IsDir() || !skills.IsDir(activePath) {
			continue
		}

		skill, err := skills.Read(activePath)
		if err != nil {
			return nil, fmt.Errorf("read active skill %q: %w", activePath, err)
		}
		found = append(found, ActiveSkill{
			Name:        skill.Name,
			Root:        root,
			Path:        activePath,
			Status:      StatusUnmanaged,
			Description: skill.Description,
		})
	}

	return found, nil
}

func scanSymlink(cfg config.Config, root roots.ActiveRoot, activePath, name string) ActiveSkill {
	classification := classifySymlink(cfg, activePath, name)
	if classification.status == StatusBroken {
		return brokenSkill(root, activePath, name, classification.reason)
	}

	skill, err := skills.Read(classification.resolvedPath)
	if err != nil {
		return brokenSkill(root, activePath, name, fmt.Sprintf("read target metadata: %v", err))
	}

	return ActiveSkill{
		Name:        skill.Name,
		Root:        root,
		Path:        activePath,
		Status:      classification.status,
		Description: skill.Description,
	}
}

type symlinkClassification struct {
	status       string
	reason       string
	resolvedPath string
}

func classifySymlink(cfg config.Config, activePath, name string) symlinkClassification {
	result := symlinkcheck.ValidateSkillTarget(activePath)
	if result.Broken {
		return symlinkClassification{status: StatusBroken, reason: result.Reason}
	}

	status := StatusUnmanaged
	repoPath, err := repo.SkillPath(cfg, name)
	if err == nil {
		if resolvedRepoPath, err := filepath.EvalSymlinks(repoPath); err == nil {
			repoPath = resolvedRepoPath
		}
	}
	if err == nil {
		if samePath(result.ResolvedPath, repoPath) {
			status = StatusManaged
		}
	}

	return symlinkClassification{
		status:       status,
		resolvedPath: result.ResolvedPath,
	}
}

func brokenSkill(root roots.ActiveRoot, path, name, reason string) ActiveSkill {
	return ActiveSkill{
		Name:   name,
		Root:   root,
		Path:   path,
		Status: StatusBroken,
		Reason: reason,
	}
}

func samePath(a, b string) bool {
	absA, err := filepath.Abs(a)
	if err == nil {
		a = absA
	}
	absB, err := filepath.Abs(b)
	if err == nil {
		b = absB
	}
	return filepath.Clean(a) == filepath.Clean(b)
}
