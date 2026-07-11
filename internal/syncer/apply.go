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
	"strings"

	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/fingerprint"
	"github.com/InkyQuill/x-skills/internal/manifest"
	"github.com/InkyQuill/x-skills/internal/repo"
)

type Progress struct {
	Completed int
	Total     int
	Skill     string
	Action    string
}

type ApplyOptions struct {
	Progress   func(Progress)
	filesystem applyFilesystem
}

type SkillResult struct {
	Name                    string
	Err                     error
	ArchiveChanged          bool
	SourceRemoved           bool
	LinksRolledBack         bool
	LinksRollbackIncomplete bool
}

type Result struct {
	Succeeded     []SkillResult
	Failed        []SkillResult
	Cancelled     bool
	PlanError     error
	ManifestError error
}

// Apply executes a preflighted plan synchronously. Each skill is an independent
// transaction for active links; successful earlier skills are retained.
func Apply(ctx context.Context, cfg config.Config, plan Plan, options ...ApplyOptions) Result {
	if plan.Cancelled {
		if len(plan.Migrations)+len(plan.Links)+len(plan.Conflicts) != 0 {
			return Result{PlanError: errors.New("cancelled sync plan contains mutations")}
		}
		return Result{Cancelled: true}
	}
	if err := validateApplyPlan(cfg, plan); err != nil {
		return Result{PlanError: err}
	}
	var option ApplyOptions
	if len(options) > 0 {
		option = options[0]
	}
	filesystem := option.filesystem
	defaults := defaultApplyFilesystem()
	if filesystem.removeAll == nil {
		filesystem.removeAll = defaults.removeAll
	}
	if filesystem.rename == nil {
		filesystem.rename = defaults.rename
	}
	if filesystem.afterStage == nil {
		filesystem.afterStage = defaults.afterStage
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
		if err := validateApplyPlan(cfg, skill.plan()); err != nil {
			result.Failed = append(result.Failed, SkillResult{Name: skill.name, Err: fmt.Errorf("revalidate skill before apply: %w", err)})
			continue
		}
		emit := func(action string) {
			completed++
			if option.Progress != nil {
				option.Progress(Progress{Completed: completed, Total: total, Skill: skill.name, Action: action})
			}
		}
		mutation, err := applySkill(cfg, filesystem, skill, emit)
		if err != nil {
			result.Failed = append(result.Failed, SkillResult{Name: skill.name, Err: err,
				ArchiveChanged: mutation.archiveChanged, SourceRemoved: mutation.sourceRemoved, LinksRolledBack: mutation.linksRolledBack,
				LinksRollbackIncomplete: mutation.linksRollbackIncomplete})
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

func validateApplyPlan(cfg config.Config, plan Plan) error {
	archiveRoot, err := canonicalPath(cfg.ArchiveSkillsRoot())
	if err != nil {
		return fmt.Errorf("canonicalize archive root: %w", err)
	}
	projectRoots := make(map[string]struct{})
	for _, root := range cfg.ManagedRoots() {
		if root.Scope == config.ScopeProject && root.Enabled {
			path, err := canonicalPath(root.Path)
			if err != nil {
				return fmt.Errorf("canonicalize configured Skills Folder: %w", err)
			}
			projectRoots[path] = struct{}{}
		}
	}
	identityByName := make(map[string]string)
	validateIdentity := func(id, name, fp string) error {
		if err := repo.ValidateName(name); err != nil {
			return err
		}
		if fp == "" || id != name+":"+fp {
			return fmt.Errorf("skill %q has invalid candidate identity", name)
		}
		if prior, exists := identityByName[name]; exists && prior != id {
			return fmt.Errorf("skill %q has inconsistent candidate identities", name)
		}
		identityByName[name] = id
		return nil
	}
	archivePath := func(path, name string) (string, error) {
		got, err := canonicalEntryPath(path)
		if err != nil {
			return "", err
		}
		if got != filepath.Join(archiveRoot, name) {
			return "", fmt.Errorf("archive path %q is outside configured archive", path)
		}
		return got, nil
	}
	activePath := func(path, name string) (string, error) {
		got, err := canonicalEntryPath(path)
		if err != nil {
			return "", err
		}
		if filepath.Base(got) != name {
			return "", fmt.Errorf("active path %q does not match skill %q", path, name)
		}
		if _, ok := projectRoots[filepath.Dir(got)]; !ok {
			return "", fmt.Errorf("active path %q is outside configured project Skills Folders", path)
		}
		return got, nil
	}
	migrations := make(map[string]Change)
	sources := make(map[string]struct{})
	links := make(map[string]Change)
	for _, change := range plan.Migrations {
		if err := validateIdentity(change.CandidateID, change.Name, change.Fingerprint); err != nil {
			return err
		}
		if change.Action != "migrate" || change.DestinationPath != "" {
			return fmt.Errorf("invalid migration for %q", change.Name)
		}
		archive, err := archivePath(change.ArchivePath, change.Name)
		if err != nil {
			return err
		}
		source, err := activePath(change.SourcePath, change.Name)
		if err != nil {
			return err
		}
		if _, exists := migrations[archive]; exists {
			return fmt.Errorf("duplicate archive migration %q", archive)
		}
		if _, exists := sources[source]; exists {
			return fmt.Errorf("duplicate migration source %q", source)
		}
		sources[source] = struct{}{}
		fingerprintSource, err := filepath.EvalSymlinks(source)
		if err != nil {
			return fmt.Errorf("resolve migration source %q: %w", source, err)
		}
		got, err := fingerprint.Directory(fingerprintSource)
		if err != nil || got != change.Fingerprint {
			return fmt.Errorf("migration source %q drifted from preflight", source)
		}
		migrations[archive] = change
	}
	for _, change := range plan.Links {
		if err := validateIdentity(change.CandidateID, change.Name, change.Fingerprint); err != nil {
			return err
		}
		if change.Action != LinkCreate && change.Action != LinkNormalize {
			return fmt.Errorf("invalid link action %q", change.Action)
		}
		if _, err := archivePath(change.ArchivePath, change.Name); err != nil {
			return err
		}
		destination, err := activePath(change.DestinationPath, change.Name)
		if err != nil {
			return err
		}
		if _, exists := links[destination]; exists {
			return fmt.Errorf("duplicate link destination %q", destination)
		}
		if _, source := sources[destination]; source {
			return fmt.Errorf("link destination %q is also a migration source", destination)
		}
		links[destination] = change
	}
	conflicts := make(map[string]Conflict)
	preserveNames := make(map[string]struct{})
	for _, conflict := range plan.Conflicts {
		if err := validateIdentity(conflict.CandidateID, conflict.Name, conflict.Fingerprint); err != nil {
			return err
		}
		if conflict.Resolution.Action != ConflictReplace {
			return fmt.Errorf("skill %q has unresolved or invalid conflict action %q", conflict.Name, conflict.Resolution.Action)
		}
		if err := repo.ValidateName(conflict.Resolution.PreserveAs); err != nil {
			return fmt.Errorf("validate preserve name: %w", err)
		}
		path, err := archivePath(conflict.DestinationPath, conflict.Name)
		if err != nil {
			path, err = activePath(conflict.DestinationPath, conflict.Name)
		}
		if err != nil {
			return fmt.Errorf("invalid conflict path %q", conflict.DestinationPath)
		}
		resolutionPath, err := canonicalEntryPath(conflict.Resolution.DestinationPath)
		if err != nil || resolutionPath != path {
			return fmt.Errorf("conflict resolution path does not match %q", path)
		}
		preserved := filepath.Join(archiveRoot, conflict.Resolution.PreserveAs)
		if _, duplicate := preserveNames[preserved]; duplicate {
			return fmt.Errorf("duplicate preserve archive path %q", preserved)
		}
		preserveNames[preserved] = struct{}{}
		if _, plannedArchive := migrations[preserved]; plannedArchive {
			return fmt.Errorf("preserve path %q collides with planned archive", preserved)
		}
		if _, err := os.Lstat(preserved); err == nil || !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("preserve archive path is occupied: %s", preserved)
		}
		if _, exists := conflicts[path]; exists {
			return fmt.Errorf("duplicate conflict %q", path)
		}
		conflicts[path] = conflict
	}
	for archive, migration := range migrations {
		_, statErr := os.Lstat(archive)
		_, replacing := conflicts[archive]
		if replacing {
			if statErr != nil {
				return fmt.Errorf("archive replacement %q drifted", archive)
			}
			if conflicts[archive].DestinationStatus != "managed" {
				return fmt.Errorf("archive replacement %q has invalid destination status", archive)
			}
			matches, err := pathMatchesFingerprint(archive, migration.Fingerprint)
			if err != nil || matches {
				return fmt.Errorf("archive replacement %q no longer diverges", archive)
			}
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return fmt.Errorf("migration archive %q drifted", archive)
		}
	}
	for destination, link := range links {
		_, replacing := conflicts[destination]
		archiveMatches, archiveErr := pathMatchesFingerprint(link.ArchivePath, link.Fingerprint)
		_, migration := migrations[link.ArchivePath]
		if !migration && (archiveErr != nil || !archiveMatches) {
			return fmt.Errorf("link archive %q drifted from preflight", link.ArchivePath)
		}
		classification, err := classifyDestination(cfg, destination, link.ArchivePath, link.Fingerprint, archiveMatches, migration)
		if err != nil {
			return err
		}
		if replacing && classification.kind != destinationDivergent {
			return fmt.Errorf("replacement destination %q no longer diverges", destination)
		}
		if replacing && conflicts[destination].DestinationStatus != classification.status {
			return fmt.Errorf("replacement destination %q status drifted from %q to %q", destination, conflicts[destination].DestinationStatus, classification.status)
		}
		if !replacing && link.Action == LinkCreate && classification.kind != destinationMissing {
			return fmt.Errorf("link destination %q drifted from missing state", destination)
		}
		if !replacing && link.Action == LinkNormalize && classification.kind != destinationMatching {
			return fmt.Errorf("link destination %q drifted from matching state", destination)
		}
	}
	for path := range conflicts {
		if _, ok := migrations[path]; !ok {
			if _, ok := links[path]; !ok {
				return fmt.Errorf("conflict %q has no matching mutation", path)
			}
		}
	}
	for _, skip := range plan.Skipped {
		if err := repo.ValidateName(skip.Name); err != nil || !strings.HasPrefix(skip.CandidateID, skip.Name+":") {
			return fmt.Errorf("invalid skipped skill %q", skip.Name)
		}
	}
	return nil
}

type applyWork struct {
	name         string
	migrations   []Change
	links        []Change
	replacements []Conflict
}

func (work applyWork) plan() Plan {
	return Plan{Migrations: work.migrations, Links: work.links, Conflicts: work.replacements}
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

type skillMutation struct {
	archiveChanged          bool
	sourceRemoved           bool
	linksRolledBack         bool
	linksRollbackIncomplete bool
}

type applyFilesystem struct {
	removeAll  func(string) error
	rename     func(string, string) error
	afterStage func(string) error
}

func defaultApplyFilesystem() applyFilesystem {
	return applyFilesystem{removeAll: os.RemoveAll, rename: os.Rename, afterStage: func(string) error { return nil }}
}

func applySkill(cfg config.Config, filesystem applyFilesystem, work applyWork, emit func(string)) (skillMutation, error) {
	var mutation skillMutation
	plannedArchives := make(map[string]struct{}, len(work.migrations))
	for _, migration := range work.migrations {
		plannedArchives[migration.ArchivePath] = struct{}{}
	}
	for _, link := range work.links {
		if _, planned := plannedArchives[link.ArchivePath]; !planned {
			info, err := os.Stat(link.ArchivePath)
			if err != nil || !info.IsDir() {
				if err == nil {
					err = fmt.Errorf("not a directory")
				}
				return mutation, fmt.Errorf("preflight link source %q: %w", link.ArchivePath, err)
			}
		}
		if err := validateWritableDirectoryShape(filepath.Dir(link.DestinationPath)); err != nil {
			return mutation, fmt.Errorf("preflight link destination %q: %w", link.DestinationPath, err)
		}
	}

	archiveConflicts := make(map[string]Conflict)
	for _, conflict := range work.replacements {
		for _, migration := range work.migrations {
			if sameCanonicalPath(conflict.DestinationPath, migration.ArchivePath) {
				archiveConflicts[migration.ArchivePath] = conflict
			}
		}
	}
	for _, migration := range work.migrations {
		staged, err := stageTree(migration.SourcePath, filepath.Dir(migration.ArchivePath))
		if err != nil {
			return mutation, fmt.Errorf("stage migration %q: %w", migration.Name, err)
		}
		if err := filesystem.afterStage(staged); err != nil {
			_ = os.RemoveAll(staged)
			return mutation, fmt.Errorf("inspect staged migration %q: %w", migration.Name, err)
		}
		stagedFingerprint, err := fingerprint.Directory(staged)
		if err != nil || stagedFingerprint != migration.Fingerprint {
			_ = os.RemoveAll(staged)
			return mutation, fmt.Errorf("staged migration %q drifted from approved fingerprint", migration.Name)
		}
		conflict, replacing := archiveConflicts[migration.ArchivePath]
		preserved := ""
		if replacing {
			preserved = filepath.Join(filepath.Dir(migration.ArchivePath), conflict.Resolution.PreserveAs)
			if err := os.Rename(migration.ArchivePath, preserved); err != nil {
				_ = os.RemoveAll(staged)
				return mutation, fmt.Errorf("preserve archive %q as %q: %w", migration.ArchivePath, preserved, err)
			}
			emit(ConflictReplace)
		}
		if err := os.Rename(staged, migration.ArchivePath); err != nil {
			restoreErr := error(nil)
			if preserved != "" {
				restoreErr = copyTreeAtomic(preserved, migration.ArchivePath)
			}
			return mutation, errors.Join(fmt.Errorf("publish migration %q: %w", migration.Name, err), restoreErr)
		}
		mutation.archiveChanged = true
		currentSource, err := filepath.EvalSymlinks(migration.SourcePath)
		if err != nil {
			return mutation, fmt.Errorf("revalidate migrated source %q before removal: %w", migration.SourcePath, err)
		}
		currentFingerprint, err := fingerprint.Directory(currentSource)
		if err != nil || currentFingerprint != migration.Fingerprint {
			return mutation, fmt.Errorf("migration source %q drifted before removal", migration.SourcePath)
		}
		if err := filesystem.removeAll(migration.SourcePath); err != nil {
			return mutation, fmt.Errorf("remove migrated source %q: %w", migration.SourcePath, err)
		}
		mutation.sourceRemoved = true
		emit(migration.Action)
	}

	for _, conflict := range work.replacements {
		if _, archiveConflict := archiveConflicts[conflict.DestinationPath]; archiveConflict {
			continue
		}
		archiveRoot := ""
		if len(work.links) > 0 {
			archiveRoot = filepath.Dir(work.links[0].ArchivePath)
		} else if len(work.migrations) > 0 {
			archiveRoot = filepath.Dir(work.migrations[0].ArchivePath)
		}
		preserved := filepath.Join(archiveRoot, conflict.Resolution.PreserveAs)
		if err := copyTreeAtomic(conflict.DestinationPath, preserved); err != nil {
			return mutation, fmt.Errorf("preserve destination %q as %q: %w", conflict.DestinationPath, preserved, err)
		}
		emit(ConflictReplace)
	}

	backups := make([]destinationBackup, 0, len(work.links))
	rollback := func(cause error) error {
		var rollbackErrs []error
		for i := len(backups) - 1; i >= 0; i-- {
			backup := backups[i]
			if err := filesystem.removeAll(backup.path); err != nil {
				rollbackErrs = append(rollbackErrs, err)
				continue
			}
			if !backup.created {
				if err := filesystem.rename(backup.backup, backup.path); err != nil {
					rollbackErrs = append(rollbackErrs, err)
				}
			}
		}
		mutation.linksRolledBack = len(rollbackErrs) == 0
		mutation.linksRollbackIncomplete = len(rollbackErrs) != 0
		return errors.Join(append([]error{cause}, rollbackErrs...)...)
	}
	for _, link := range work.links {
		if err := validateLinkImmediately(cfg, work, link); err != nil {
			return mutation, rollback(err)
		}
		backup := destinationBackup{path: link.DestinationPath, created: true}
		if _, err := os.Lstat(link.DestinationPath); err == nil {
			dir, err := os.MkdirTemp(filepath.Dir(link.DestinationPath), ".x-skills-backup-")
			if err != nil {
				return mutation, rollback(fmt.Errorf("prepare destination backup: %w", err))
			}
			if err := os.Remove(dir); err != nil {
				return mutation, rollback(fmt.Errorf("prepare destination backup: %w", err))
			}
			if err := filesystem.rename(link.DestinationPath, dir); err != nil {
				return mutation, rollback(fmt.Errorf("back up destination %q: %w", link.DestinationPath, err))
			}
			backup.created = false
			backup.backup = dir
		} else if !errors.Is(err, os.ErrNotExist) {
			return mutation, rollback(fmt.Errorf("inspect destination %q: %w", link.DestinationPath, err))
		}
		backups = append(backups, backup)
		if err := os.MkdirAll(filepath.Dir(link.DestinationPath), 0o755); err != nil {
			return mutation, rollback(fmt.Errorf("create destination parent: %w", err))
		}
		if err := os.Symlink(link.ArchivePath, link.DestinationPath); err != nil {
			return mutation, rollback(fmt.Errorf("link %q: %w", link.DestinationPath, err))
		}
		emit(link.Action)
	}
	for _, backup := range backups {
		if !backup.created {
			if err := os.RemoveAll(backup.backup); err != nil {
				return mutation, fmt.Errorf("remove destination backup %q: %w", backup.backup, err)
			}
		}
	}
	return mutation, nil
}

func validateLinkImmediately(cfg config.Config, work applyWork, link Change) error {
	replacement := Conflict{}
	archiveReplacement := false
	for _, conflict := range work.replacements {
		if sameCanonicalPath(conflict.DestinationPath, link.DestinationPath) {
			replacement = conflict
		}
		if sameCanonicalPath(conflict.DestinationPath, link.ArchivePath) {
			archiveReplacement = true
		}
	}
	archiveMatches, err := pathMatchesFingerprint(link.ArchivePath, link.Fingerprint)
	if err != nil || !archiveMatches {
		return fmt.Errorf("link archive %q drifted before mutation", link.ArchivePath)
	}
	classification, err := classifyDestination(cfg, link.DestinationPath, link.ArchivePath, link.Fingerprint, true, false)
	if err != nil {
		return err
	}
	if replacement.Resolution.Action != "" {
		if classification.kind != destinationDivergent || classification.status != replacement.DestinationStatus {
			return fmt.Errorf("replacement destination %q drifted before mutation", link.DestinationPath)
		}
		return nil
	}
	if link.Action == LinkCreate && classification.kind != destinationMissing {
		return fmt.Errorf("link destination %q appeared after validation", link.DestinationPath)
	}
	if link.Action == LinkNormalize && classification.kind != destinationMatching && !(archiveReplacement && classification.kind == destinationManaged) {
		return fmt.Errorf("link destination %q drifted before normalization", link.DestinationPath)
	}
	return nil
}

func stageTree(source, parent string) (string, error) {
	resolved, err := filepath.EvalSymlinks(source)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", err
	}
	temp, err := os.MkdirTemp(parent, ".x-skills-stage-")
	if err != nil {
		return "", err
	}
	if err := copyTree(resolved, temp); err != nil {
		_ = os.RemoveAll(temp)
		return "", err
	}
	return temp, nil
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
