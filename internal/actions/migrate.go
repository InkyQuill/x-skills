package actions

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/repo"
	"github.com/InkyQuill/x-skills/internal/skills"
)

const (
	ResultMigrated             = "migrated"
	ResultRelinked             = "relinked existing archive"
	ResultMigratedUnlinked     = "migrated unmanaged"
	ResultRemovedUnmanaged     = "removed unmanaged"
	ResultRemovedUnmanagedLink = "removed unmanaged symlink"
	ResultRemovedActiveLink    = "removed link"

	ConflictResolutionAsk         = ""
	ConflictResolutionKeepArchive = "keep-archive"
	ConflictResolutionUseActive   = "use-active"
)

type MigrateRequest struct {
	Name               string
	Scope              string
	Target             string
	Confirmed          bool
	ConflictResolution string
}

type ArchiveConflictError struct {
	Name         string
	ActivePath   string
	ArchivedPath string
	Summary      string
}

func (e *ArchiveConflictError) Error() string {
	return fmt.Sprintf("archive destination differs: %s\n%s", e.ArchivedPath, e.Summary)
}

func Migrate(cfg config.Config, req MigrateRequest) (MutationResult, error) {
	paths, err := mutationPaths(cfg, req.Name, req.Scope, req.Target)
	if err != nil {
		return MutationResult{}, err
	}
	if !req.Confirmed {
		return MutationResult{}, fmt.Errorf("migrate requires confirmation; rerun with -y")
	}

	status, err := migrateActiveDirectory(paths.active, paths.archived, true, req.ConflictResolution)
	if err != nil {
		var conflict *ArchiveConflictError
		if errors.As(err, &conflict) {
			conflict.Name = req.Name
		}
		return MutationResult{}, err
	}
	return MutationResult{Name: req.Name, Path: paths.archived, Status: status}, nil
}

type mutationPathSet struct {
	activeRoot string
	active     string
	archived   string
}

func mutationPaths(cfg config.Config, name, scope, target string) (mutationPathSet, error) {
	if err := repo.ValidateName(name); err != nil {
		return mutationPathSet{}, err
	}
	if !slices.Contains(config.Scopes, scope) {
		return mutationPathSet{}, fmt.Errorf("unknown scope %q", scope)
	}

	root, err := cfg.ActiveRoot(scope, target)
	if err != nil {
		return mutationPathSet{}, err
	}
	archived, err := repo.SkillPath(cfg, name)
	if err != nil {
		return mutationPathSet{}, err
	}
	return mutationPathSet{
		activeRoot: root,
		active:     filepath.Join(root, name),
		archived:   archived,
	}, nil
}

func migrateActiveDirectory(active, archived string, linkBack bool, conflictResolution string) (string, error) {
	if err := ensureUnmanagedSkillDirectory(active, archived); err != nil {
		return "", err
	}
	outcome, err := copySkillToArchive(active, archived, conflictResolution)
	if err != nil {
		return "", err
	}
	if err := os.RemoveAll(active); err != nil {
		return "", fmt.Errorf("remove active skill %q: %w", active, err)
	}
	if !linkBack {
		return ResultMigratedUnlinked, nil
	}
	if err := os.Symlink(archived, active); err != nil {
		linkErr := fmt.Errorf("link %q to %q: %w", archived, active, err)
		if rollbackErr := os.Rename(archived, active); rollbackErr != nil {
			return "", errors.Join(linkErr, fmt.Errorf("restore active skill: %w", rollbackErr))
		}
		return "", linkErr
	}
	if outcome == archiveCopyMatched || outcome == archiveCopyKept {
		return ResultRelinked, nil
	}
	return ResultMigrated, nil
}

func directoryDiffSummary(active, archived string) string {
	activeEntries := describeDirectory(active)
	archiveEntries := describeDirectory(archived)
	keys := make([]string, 0, len(activeEntries)+len(archiveEntries))
	seen := map[string]bool{}
	for key := range activeEntries {
		keys = append(keys, key)
		seen[key] = true
	}
	for key := range archiveEntries {
		if !seen[key] {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)

	lines := []string{"archive                              active"}
	for _, key := range keys {
		archiveValue := archiveEntries[key]
		activeValue := activeEntries[key]
		if archiveValue == activeValue {
			continue
		}
		lines = append(lines, fmt.Sprintf("%-36s %s", diffCell(key, archiveValue), diffCell(key, activeValue)))
		if len(lines) >= 13 {
			lines = append(lines, "...")
			break
		}
	}
	if len(lines) == 1 {
		return "contents differ, but no individual file summary was available"
	}
	return strings.Join(lines, "\n")
}

func diffCell(path, value string) string {
	if value == "" {
		return "-"
	}
	return truncateDiff(path+" "+value, 34)
}

func truncateDiff(value string, width int) string {
	if len(value) <= width {
		return value
	}
	if width <= 3 {
		return value[:width]
	}
	return value[:width-3] + "..."
}

func describeDirectory(root string) map[string]string {
	entries := map[string]string{}
	_ = filepath.WalkDir(root, func(path string, dirEntry fs.DirEntry, err error) error {
		if err != nil || path == root {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		switch {
		case dirEntry.Type()&os.ModeSymlink != 0:
			target, err := os.Readlink(path)
			if err != nil {
				entries[rel] = "symlink unreadable"
				return nil
			}
			entries[rel] = "symlink -> " + target
		case dirEntry.IsDir():
			entries[rel] = "dir"
		default:
			entries[rel] = fileDigest(path)
		}
		return nil
	})
	return entries
}

func fileDigest(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return "file unreadable"
	}
	defer func() {
		_ = file.Close()
	}()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "file unreadable"
	}
	return "file " + hex.EncodeToString(hash.Sum(nil))[:12]
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
