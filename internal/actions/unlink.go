package actions

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

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
		classification := classifySymlink(cfg, paths.active, req.Name)
		if classification.status == StatusUnmanaged && !req.DeleteUnmanaged {
			if err := archiveResolvedSymlinkTarget(classification.resolvedPath, paths.archived, ConflictResolutionAsk); err != nil {
				return MutationResult{}, err
			}
			if err := os.Remove(paths.active); err != nil {
				return MutationResult{}, fmt.Errorf("remove active link %q: %w", paths.active, err)
			}
			return MutationResult{Name: req.Name, Path: paths.archived, Status: ResultMigratedUnlinked}, nil
		}
		if err := os.Remove(paths.active); err != nil {
			return MutationResult{}, fmt.Errorf("remove active link %q: %w", paths.active, err)
		}
		if classification.status == StatusUnmanaged {
			return MutationResult{Name: req.Name, Path: paths.active, Status: ResultRemovedUnmanagedLink}, nil
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

	if _, err := migrateActiveDirectory(paths.active, paths.archived, false, ConflictResolutionAsk); err != nil {
		return MutationResult{}, err
	}
	return MutationResult{Name: req.Name, Path: paths.archived, Status: ResultMigratedUnlinked}, nil
}

func archiveResolvedSymlinkTarget(active, archived, conflictResolution string) error {
	if _, err := os.Lstat(archived); err == nil {
		_, err := handleExistingArchive(active, archived, false, conflictResolution)
		return err
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect archive destination %q: %w", archived, err)
	}
	if err := os.MkdirAll(filepath.Dir(archived), 0o755); err != nil {
		return fmt.Errorf("create archive root %q: %w", filepath.Dir(archived), err)
	}
	if err := copySkillDirectory(active, archived); err != nil {
		return err
	}
	return nil
}

func copySkillDirectory(src, dst string) error {
	return filepath.WalkDir(src, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		switch {
		case rel == ".":
			return os.MkdirAll(dst, info.Mode().Perm())
		case entry.Type()&os.ModeSymlink != 0:
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(linkTarget, target)
		case entry.IsDir():
			return os.MkdirAll(target, info.Mode().Perm())
		default:
			return copyFile(path, target, info.Mode().Perm())
		}
	})
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		_ = in.Close()
	}()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	defer func() {
		_ = out.Close()
	}()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}
