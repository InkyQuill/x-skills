package actions

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/repo"
	"github.com/InkyQuill/x-skills/internal/skills"
)

const (
	ResultMigrated             = "migrated"
	ResultMigratedUnlinked     = "migrated unmanaged"
	ResultRemovedUnmanaged     = "removed unmanaged"
	ResultRemovedUnmanagedLink = "removed unmanaged symlink"
	ResultRemovedActiveLink    = "removed link"
)

type MigrateRequest struct {
	Name      string
	Scope     string
	Target    string
	Confirmed bool
}

func Migrate(cfg config.Config, req MigrateRequest) (MutationResult, error) {
	paths, err := mutationPaths(cfg, req.Name, req.Scope, req.Target)
	if err != nil {
		return MutationResult{}, err
	}
	if !req.Confirmed {
		return MutationResult{}, fmt.Errorf("migrate requires confirmation; rerun with -y")
	}

	if err := migrateActiveDirectory(paths.active, paths.archived, true); err != nil {
		return MutationResult{}, err
	}
	return MutationResult{Name: req.Name, Path: paths.archived, Status: ResultMigrated}, nil
}

type mutationPathSet struct {
	active   string
	archived string
}

func mutationPaths(cfg config.Config, name, scope, target string) (mutationPathSet, error) {
	if err := repo.ValidateName(name); err != nil {
		return mutationPathSet{}, err
	}
	if !slices.Contains(config.Scopes, scope) {
		return mutationPathSet{}, fmt.Errorf("unknown scope %q", scope)
	}
	if !slices.Contains(config.Targets, target) {
		return mutationPathSet{}, fmt.Errorf("unknown target %q", target)
	}

	root := cfg.ActiveRoot(scope, target)
	return mutationPathSet{
		active:   filepath.Join(root, name),
		archived: repo.SkillPath(cfg, name),
	}, nil
}

func migrateActiveDirectory(active, archived string, linkBack bool) error {
	if err := ensureUnmanagedSkillDirectory(active, archived); err != nil {
		return err
	}
	if _, err := os.Lstat(archived); err == nil {
		return fmt.Errorf("archive destination exists: %s", archived)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect archive destination %q: %w", archived, err)
	}
	if err := os.MkdirAll(filepath.Dir(archived), 0o755); err != nil {
		return fmt.Errorf("create archive root %q: %w", filepath.Dir(archived), err)
	}
	if err := os.Rename(active, archived); err != nil {
		return fmt.Errorf("move %q to %q: %w", active, archived, err)
	}
	if !linkBack {
		return nil
	}
	if err := os.Symlink(archived, active); err != nil {
		if restoreErr := os.Rename(archived, active); restoreErr != nil {
			return fmt.Errorf("link %q to %q: %w (restore failed: %v)", archived, active, err, restoreErr)
		}
		return fmt.Errorf("link %q to %q: %w", archived, active, err)
	}
	return nil
}

func ensureUnmanagedSkillDirectory(active, archived string) error {
	info, err := os.Lstat(active)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("active skill not found: %s", active)
		}
		return fmt.Errorf("inspect active skill %q: %w", active, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		resolved, err := filepath.EvalSymlinks(active)
		if err == nil && samePath(resolved, archived) {
			return fmt.Errorf("active skill already managed: %s", active)
		}
		return fmt.Errorf("active skill is not an unmanaged directory: %s", active)
	}
	if !info.IsDir() {
		return fmt.Errorf("active skill is not a directory: %s", active)
	}
	if !skills.IsDir(active) {
		return fmt.Errorf("active skill is not a skill directory: %s", active)
	}
	return nil
}
