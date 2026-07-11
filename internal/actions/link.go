package actions

import (
	"fmt"
	"os"

	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/repo"
)

type LinkRequest struct {
	Name   string
	Scope  string
	Target string
}

type MutationResult struct {
	Name   string
	Path   string
	Status string
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

	return MutationResult{Name: req.Name, Path: destination}, nil
}
