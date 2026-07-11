package syncer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/manifest"
)

type Progress struct {
	Completed int
	Total     int
	Skill     string
	Action    string
}

type ApplyOptions struct {
	Progress func(Progress)
}

type SkillResult struct {
	Name string
	Err  error
}

type Result struct {
	Succeeded     []SkillResult
	Failed        []SkillResult
	Cancelled     bool
	ManifestError error
}

// Apply executes a preflighted plan synchronously. Each skill is an independent
// transaction for active links; successful earlier skills are retained.
func Apply(ctx context.Context, cfg config.Config, plan Plan, options ...ApplyOptions) Result {
	if plan.Cancelled {
		return Result{Cancelled: true}
	}
	var option ApplyOptions
	if len(options) > 0 {
		option = options[0]
	}
	work := indexApplyWork(plan)
	result := Result{}
	completed := 0
	total := 0
	for _, skill := range work {
		total += len(skill.migrations) + len(skill.links) + len(skill.replacements)
	}
	mutated := false
	for _, skill := range work {
		if err := ctx.Err(); err != nil {
			result.Cancelled = true
			break
		}
		emit := func(action string) {
			completed++
			if option.Progress != nil {
				option.Progress(Progress{Completed: completed, Total: total, Skill: skill.name, Action: action})
			}
		}
		if err := applySkill(skill, emit); err != nil {
			result.Failed = append(result.Failed, SkillResult{Name: skill.name, Err: err})
			mutated = true
			continue
		}
		result.Succeeded = append(result.Succeeded, SkillResult{Name: skill.name})
		mutated = mutated || len(skill.migrations)+len(skill.links)+len(skill.replacements) > 0
	}
	if mutated {
		_, result.ManifestError = manifest.ReconcileLocal(cfg)
	}
	return result
}

type applyWork struct {
	name         string
	migrations   []Change
	links        []Change
	replacements []Conflict
}

func indexApplyWork(plan Plan) []applyWork {
	byName := make(map[string]*applyWork)
	order := make([]string, 0)
	get := func(name string) *applyWork {
		if work := byName[name]; work != nil {
			return work
		}
		work := &applyWork{name: name}
		byName[name] = work
		order = append(order, name)
		return work
	}
	for _, change := range plan.Migrations {
		get(change.Name).migrations = append(get(change.Name).migrations, change)
	}
	for _, change := range plan.Links {
		get(change.Name).links = append(get(change.Name).links, change)
	}
	for _, conflict := range plan.Conflicts {
		if conflict.Resolution.Action == ConflictReplace {
			get(conflict.Name).replacements = append(get(conflict.Name).replacements, conflict)
		}
	}
	sort.Strings(order)
	result := make([]applyWork, 0, len(order))
	for _, name := range order {
		result = append(result, *byName[name])
	}
	return result
}

type destinationBackup struct {
	path    string
	backup  string
	created bool
}

func applySkill(work applyWork, emit func(string)) error {
	plannedArchives := make(map[string]struct{}, len(work.migrations))
	for _, migration := range work.migrations {
		plannedArchives[migration.ArchivePath] = struct{}{}
	}
	for _, link := range work.links {
		if _, planned := plannedArchives[link.ArchivePath]; planned {
			continue
		}
		info, err := os.Stat(link.ArchivePath)
		if err != nil || !info.IsDir() {
			if err == nil {
				err = fmt.Errorf("not a directory")
			}
			return fmt.Errorf("preflight link source %q: %w", link.ArchivePath, err)
		}
		if err := validateWritableDirectoryShape(filepath.Dir(link.DestinationPath)); err != nil {
			return fmt.Errorf("preflight link destination %q: %w", link.DestinationPath, err)
		}
	}

	for _, conflict := range work.replacements {
		archiveRoot := ""
		if len(work.links) > 0 {
			archiveRoot = filepath.Dir(work.links[0].ArchivePath)
		} else if len(work.migrations) > 0 {
			archiveRoot = filepath.Dir(work.migrations[0].ArchivePath)
		}
		preserved := filepath.Join(archiveRoot, conflict.Resolution.PreserveAs)
		for _, migration := range work.migrations {
			if sameCanonicalPath(conflict.DestinationPath, migration.ArchivePath) {
				preserved = filepath.Join(filepath.Dir(migration.ArchivePath), conflict.Resolution.PreserveAs)
				if err := os.Rename(conflict.DestinationPath, preserved); err != nil {
					return fmt.Errorf("preserve archive %q as %q: %w", conflict.DestinationPath, preserved, err)
				}
				emit(ConflictReplace)
				goto preserved
			}
		}
		if err := copyTreeAtomic(conflict.DestinationPath, preserved); err != nil {
			return fmt.Errorf("preserve destination %q as %q: %w", conflict.DestinationPath, preserved, err)
		}
		emit(ConflictReplace)
	preserved:
	}

	for _, migration := range work.migrations {
		if err := copyTreeAtomic(migration.SourcePath, migration.ArchivePath); err != nil {
			return fmt.Errorf("migrate %q to archive: %w", migration.Name, err)
		}
		if err := os.RemoveAll(migration.SourcePath); err != nil {
			return fmt.Errorf("remove migrated source %q: %w", migration.SourcePath, err)
		}
		emit(migration.Action)
	}

	backups := make([]destinationBackup, 0, len(work.links))
	rollback := func(cause error) error {
		var rollbackErrs []error
		for i := len(backups) - 1; i >= 0; i-- {
			backup := backups[i]
			if err := os.RemoveAll(backup.path); err != nil {
				rollbackErrs = append(rollbackErrs, err)
				continue
			}
			if !backup.created {
				if err := os.Rename(backup.backup, backup.path); err != nil {
					rollbackErrs = append(rollbackErrs, err)
				}
			}
		}
		return errors.Join(append([]error{cause}, rollbackErrs...)...)
	}
	for _, link := range work.links {
		backup := destinationBackup{path: link.DestinationPath, created: true}
		if _, err := os.Lstat(link.DestinationPath); err == nil {
			dir, err := os.MkdirTemp(filepath.Dir(link.DestinationPath), ".x-skills-backup-")
			if err != nil {
				return rollback(fmt.Errorf("prepare destination backup: %w", err))
			}
			if err := os.Remove(dir); err != nil {
				return rollback(fmt.Errorf("prepare destination backup: %w", err))
			}
			if err := os.Rename(link.DestinationPath, dir); err != nil {
				return rollback(fmt.Errorf("back up destination %q: %w", link.DestinationPath, err))
			}
			backup.created = false
			backup.backup = dir
		} else if !errors.Is(err, os.ErrNotExist) {
			return rollback(fmt.Errorf("inspect destination %q: %w", link.DestinationPath, err))
		}
		backups = append(backups, backup)
		if err := os.MkdirAll(filepath.Dir(link.DestinationPath), 0o755); err != nil {
			return rollback(fmt.Errorf("create destination parent: %w", err))
		}
		if err := os.Symlink(link.ArchivePath, link.DestinationPath); err != nil {
			return rollback(fmt.Errorf("link %q: %w", link.DestinationPath, err))
		}
		emit(link.Action)
	}
	for _, backup := range backups {
		if !backup.created {
			_ = os.RemoveAll(backup.backup)
		}
	}
	return nil
}

func copyTreeAtomic(source, destination string) error {
	resolvedSource, err := filepath.EvalSymlinks(source)
	if err != nil {
		return fmt.Errorf("resolve source %q: %w", source, err)
	}
	source = resolvedSource
	if _, err := os.Lstat(destination); err == nil {
		return fmt.Errorf("destination exists: %s", destination)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	temp, err := os.MkdirTemp(filepath.Dir(destination), ".x-skills-copy-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	if err := copyTree(source, temp); err != nil {
		return err
	}
	return os.Rename(temp, destination)
}

func copyTree(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, rel)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		if entry.Type()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		}
		return copyApplyFile(path, target, info.Mode().Perm())
	})
}

func copyApplyFile(source, destination string, mode fs.FileMode) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return errors.Join(err, in.Close())
	}
	_, copyErr := io.Copy(out, in)
	return errors.Join(copyErr, out.Close(), in.Close())
}
