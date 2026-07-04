package actions

import (
	"fmt"
	"os"

	"github.com/InkyQuill/x-skills/internal/config"
)

type UnlinkRequest struct {
	Name            string
	Scope           string
	Target          string
	Confirmed       bool
	DeleteUnmanaged bool
}

func Unlink(cfg config.Config, req UnlinkRequest) (MutationResult, error) {
	paths, err := mutationPaths(cfg, req.Name, req.Scope, req.Target)
	if err != nil {
		return MutationResult{}, err
	}
	if !req.Confirmed {
		return MutationResult{}, fmt.Errorf("unlink requires confirmation; rerun with -y")
	}

	info, err := os.Lstat(paths.active)
	if err != nil {
		if os.IsNotExist(err) {
			return MutationResult{}, fmt.Errorf("active skill not found: %s", paths.active)
		}
		return MutationResult{}, fmt.Errorf("inspect active skill %q: %w", paths.active, err)
	}

	if info.Mode()&os.ModeSymlink != 0 {
		if err := os.Remove(paths.active); err != nil {
			return MutationResult{}, fmt.Errorf("remove active link %q: %w", paths.active, err)
		}
		return MutationResult{Name: req.Name, Path: paths.active, Status: ResultRemovedActiveLink}, nil
	}

	if req.DeleteUnmanaged {
		if err := ensureUnmanagedSkillDirectory(paths.active, paths.archived); err != nil {
			return MutationResult{}, err
		}
		if err := os.RemoveAll(paths.active); err != nil {
			return MutationResult{}, fmt.Errorf("remove unmanaged active skill %q: %w", paths.active, err)
		}
		return MutationResult{Name: req.Name, Path: paths.active, Status: ResultRemovedUnmanaged}, nil
	}

	if err := migrateActiveDirectory(paths.active, paths.archived, false); err != nil {
		return MutationResult{}, err
	}
	return MutationResult{Name: req.Name, Path: paths.archived, Status: ResultMigratedUnlinked}, nil
}
