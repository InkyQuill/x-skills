package actions

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/pathidentity"
	"github.com/InkyQuill/x-skills/internal/repo"
)

const (
	ResultLinked        = "linked"
	ResultAlreadyLinked = "already_linked"
)

type LinkRequest struct {
	Name   string
	Scope  string
	Target string
}

type MutationResult struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Status string `json:"status"`
}

func Link(cfg config.Config, req LinkRequest) (MutationResult, error) {
	mutationMu.Lock()
	defer mutationMu.Unlock()

	paths, err := mutationPaths(cfg, req.Name, req.Scope, req.Target)
	if err != nil {
		return MutationResult{}, err
	}
	if !repo.HasSkill(cfg, req.Name) {
		return MutationResult{}, fmt.Errorf("repo skill %q not found", req.Name)
	}

	destination := paths.active
	if _, err := os.Lstat(destination); err == nil {
		matches, matchErr := existingLinkMatches(destination, paths.archived)
		if matchErr != nil {
			return MutationResult{}, fmt.Errorf("inspect destination %q: %w", destination, matchErr)
		}
		if matches {
			return MutationResult{
				Name:   req.Name,
				Path:   destination,
				Status: ResultAlreadyLinked,
			}, nil
		}
		return MutationResult{}, fmt.Errorf("destination exists: %s", destination)
	} else if !os.IsNotExist(err) {
		return MutationResult{}, fmt.Errorf("inspect destination %q: %w", destination, err)
	}

	if err := os.MkdirAll(paths.activeRoot, 0o755); err != nil {
		return MutationResult{}, fmt.Errorf("create active root %q: %w", paths.activeRoot, err)
	}
	if err := os.Symlink(paths.archived, destination); err != nil {
		if os.IsExist(err) {
			return MutationResult{}, fmt.Errorf("destination exists: %s", destination)
		}
		return MutationResult{}, fmt.Errorf("link %q to %q: %w", req.Name, destination, err)
	}

	return MutationResult{Name: req.Name, Path: destination, Status: ResultLinked}, nil
}

func existingLinkMatches(destination, archivePath string) (bool, error) {
	info, err := os.Lstat(destination)
	if err != nil {
		return false, err
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return false, nil
	}
	target, err := os.Readlink(destination)
	if err != nil {
		return false, err
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(destination), target)
	}
	return pathidentity.EquivalentE(target, archivePath)
}
