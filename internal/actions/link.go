package actions

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

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
	if err := repo.ValidateName(req.Name); err != nil {
		return MutationResult{}, err
	}
	if !slices.Contains(config.Scopes, req.Scope) {
		return MutationResult{}, fmt.Errorf("unknown scope %q", req.Scope)
	}
	if !slices.Contains(config.Targets, req.Target) {
		return MutationResult{}, fmt.Errorf("unknown target %q", req.Target)
	}
	if !repo.HasSkill(cfg, req.Name) {
		return MutationResult{}, fmt.Errorf("repo skill %q not found", req.Name)
	}

	root := cfg.ActiveRoot(req.Scope, req.Target)
	destination := filepath.Join(root, req.Name)
	if _, err := os.Lstat(destination); err == nil {
		return MutationResult{}, fmt.Errorf("destination exists: %s", destination)
	} else if !os.IsNotExist(err) {
		return MutationResult{}, fmt.Errorf("inspect destination %q: %w", destination, err)
	}

	if err := os.MkdirAll(root, 0o755); err != nil {
		return MutationResult{}, fmt.Errorf("create active root %q: %w", root, err)
	}
	source := repo.SkillPath(cfg, req.Name)
	if err := os.Symlink(source, destination); err != nil {
		if os.IsExist(err) {
			return MutationResult{}, fmt.Errorf("destination exists: %s", destination)
		}
		return MutationResult{}, fmt.Errorf("link %q to %q: %w", req.Name, destination, err)
	}

	return MutationResult{Name: req.Name, Path: destination}, nil
}
