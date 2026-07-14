package manifest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/InkyQuill/x-skills/internal/actions"
	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/fingerprint"
	"github.com/InkyQuill/x-skills/internal/remote"
	"github.com/InkyQuill/x-skills/internal/repo"
	"github.com/InkyQuill/x-skills/internal/roots"
)

const (
	ChangeLink    = "link"
	ChangeRemove  = "remove"
	ChangeMigrate = "migrate"
)

type RestoreRequest struct {
	Destinations []roots.ActiveRoot
	Full         bool
}

type PlannedSkill struct {
	Name         string
	ArchivePath  string
	IncomingDir  string
	Metadata     remote.SourceMetadata
	NeedsArchive bool
}

type UnavailableSkill struct {
	Name   string
	Reason string
}

type Change struct {
	Kind        string
	Name        string
	Path        string
	Destination roots.ActiveRoot
	ArchiveName string
}

type RestorePlan struct {
	Available       []PlannedSkill
	Unavailable     []UnavailableSkill
	Additions       []Change
	Normalizations  []Change
	Removals        []Change
	RemovalsBlocked bool
	Notices         []Notice
	Conflicts       []MigrationConflict
	checkoutRoot    string
}

type MigrationConflict struct {
	Name            string
	Path            string
	ExistingArchive string
	SuggestedName   string
}

// StagingRootForTest returns the staging path recorded by this plan value for
// filesystem assertions in tests. It does not report shared lifecycle state:
// copies do not observe another copy's Close clearing the path.
func (plan RestorePlan) StagingRootForTest() string {
	return plan.checkoutRoot
}

func (plan *RestorePlan) Close() error {
	if plan == nil || plan.checkoutRoot == "" {
		return nil
	}
	err := os.RemoveAll(plan.checkoutRoot)
	if err == nil {
		plan.checkoutRoot = ""
	}
	return err
}

func (plan *RestorePlan) Discard() error { return plan.Close() }

type RestoreResult struct {
	Additions       []Change
	Normalizations  []Change
	Removals        []Change
	Unavailable     []UnavailableSkill
	RemovalsBlocked bool
}

func PlanRestore(ctx context.Context, cfg config.Config, request RestoreRequest) (RestorePlan, error) {
	if err := ctx.Err(); err != nil {
		return RestorePlan{}, err
	}
	destinations, err := validateRestoreDestinations(cfg, request.Destinations)
	if err != nil {
		return RestorePlan{}, err
	}
	recommended, err := LoadRecommended(cfg.ProjectRoot)
	if err != nil {
		return RestorePlan{}, err
	}
	local, err := LoadLocal(cfg.ProjectRoot)
	if err != nil {
		return RestorePlan{}, err
	}
	effective, notices := Effective(recommended, local)
	checkoutRoot, err := os.MkdirTemp("", "x-skills-restore-*")
	if err != nil {
		return RestorePlan{}, fmt.Errorf("create restore staging root: %w", err)
	}
	plan := RestorePlan{Notices: notices, checkoutRoot: checkoutRoot}
	cache := remote.NewCheckoutCache(checkoutRoot)
	reservedArchives := map[string]struct{}{}
	for _, skill := range effective.Skills {
		resolved, resolveErr := resolveRestoreSkill(ctx, cfg, cache, skill)
		if resolveErr != nil {
			plan.Unavailable = append(plan.Unavailable, UnavailableSkill{Name: skill.Name, Reason: resolveErr.Error()})
			continue
		}
		plan.Available = append(plan.Available, resolved)
	}

	desired := make(map[string]struct{}, len(effective.Skills))
	for _, skill := range effective.Skills {
		desired[skill.Name] = struct{}{}
	}
	for _, skill := range plan.Available {
		for _, destination := range destinations {
			path := filepath.Join(destination.Path, skill.Name)
			if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
				plan.Additions = append(plan.Additions, Change{Kind: ChangeLink, Name: skill.Name, Path: path, Destination: destination})
			} else if err != nil {
				return cleanupRestorePlan(plan, fmt.Errorf("inspect restore destination %q: %w", path, err))
			} else {
				change, conflict, normalizeErr := planDestinationNormalization(cfg, destination, skill.Name, path, reservedArchives)
				if normalizeErr != nil {
					return cleanupRestorePlan(plan, normalizeErr)
				}
				if change != nil {
					plan.Normalizations = append(plan.Normalizations, *change)
					plan.Additions = append(plan.Additions, Change{Kind: ChangeLink, Name: skill.Name, Path: path, Destination: destination})
				}
				if conflict != nil {
					plan.Conflicts = append(plan.Conflicts, *conflict)
				}
			}
		}
	}
	if request.Full {
		removals, conflicts, err := planRestoreRemovals(cfg, destinations, desired, reservedArchives)
		if err != nil {
			return cleanupRestorePlan(plan, err)
		}
		plan.Conflicts = append(plan.Conflicts, conflicts...)
		if len(plan.Unavailable) > 0 {
			plan.RemovalsBlocked = len(removals) > 0 || len(plan.Normalizations) > 0
		} else {
			plan.Removals = removals
		}
	}
	if len(plan.Unavailable) > 0 && len(plan.Normalizations) > 0 {
		plan.RemovalsBlocked = true
	}
	sortRestorePlan(&plan)
	return plan, nil
}

func ApplyRestore(ctx context.Context, cfg config.Config, plan RestorePlan) (result RestoreResult, returnErr error) {
	defer func() { _ = plan.Close() }()
	mutated := false
	defer func() {
		if !mutated {
			return
		}
		if _, err := ReconcileLocal(cfg); err != nil {
			reconcileErr := fmt.Errorf("restore filesystem changes succeeded but Local Skill Manifest reconciliation failed: %w", err)
			returnErr = errors.Join(returnErr, reconcileErr)
		}
	}()
	result = RestoreResult{Unavailable: slices.Clone(plan.Unavailable), RemovalsBlocked: plan.RemovalsBlocked}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if err := validateRestorePlanForApply(cfg, plan); err != nil {
		return result, err
	}
	for _, skill := range plan.Available {
		if !skill.NeedsArchive {
			continue
		}
		if _, err := remote.ApplyArchive(remote.AddRequest{Config: cfg, IncomingDir: skill.IncomingDir, ArchiveName: skill.Name, Metadata: skill.Metadata, Conflict: remote.ConflictArchiveOnly}); err != nil {
			return result, fmt.Errorf("archive restored skill %q: %w", skill.Name, err)
		}
		mutated = true
	}
	if len(plan.Unavailable) == 0 {
		for _, change := range plan.Normalizations {
			if err := applyRestoreRemoval(ctx, cfg, change); err != nil {
				return result, err
			}
			mutated = true
			result.Normalizations = append(result.Normalizations, change)
		}
	}
	for _, change := range plan.Additions {
		if len(plan.Unavailable) > 0 && hasNormalizationAtPath(plan.Normalizations, change.Path) {
			continue
		}
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if err := validatePlannedChange(cfg, change); err != nil {
			return result, err
		}
		if _, err := actions.Link(cfg, actions.LinkRequest{Name: change.Name, Scope: change.Destination.Scope, Target: change.Destination.Target}); err != nil {
			return result, fmt.Errorf("restore link %q: %w", change.Name, err)
		}
		mutated = true
		result.Additions = append(result.Additions, change)
	}
	if len(plan.Unavailable) > 0 {
		return result, nil
	}
	for _, change := range plan.Removals {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if err := applyRestoreRemoval(ctx, cfg, change); err != nil {
			return result, fmt.Errorf("restore %s %q: %w", change.Kind, change.Name, err)
		}
		mutated = true
		result.Removals = append(result.Removals, change)
	}
	return result, nil
}

func hasNormalizationAtPath(changes []Change, path string) bool {
	return slices.ContainsFunc(changes, func(change Change) bool {
		return filepath.Clean(change.Path) == filepath.Clean(path)
	})
}

func validateRestorePlanForApply(cfg config.Config, plan RestorePlan) error {
	for _, skill := range plan.Available {
		if !skill.NeedsArchive {
			continue
		}
		if info, err := os.Stat(skill.IncomingDir); err != nil || !info.IsDir() {
			return fmt.Errorf("staged skill %q is unavailable; discard and re-plan", skill.Name)
		}
	}
	changeSets := [][]Change{plan.Normalizations, plan.Additions, plan.Removals}
	if len(plan.Unavailable) > 0 {
		eligibleAdditions := make([]Change, 0, len(plan.Additions))
		for _, change := range plan.Additions {
			if !hasNormalizationAtPath(plan.Normalizations, change.Path) {
				eligibleAdditions = append(eligibleAdditions, change)
			}
		}
		changeSets = [][]Change{eligibleAdditions}
	}
	plannedArchives := make(map[string]string)
	for _, changes := range changeSets {
		for _, change := range changes {
			if err := validatePlannedChange(cfg, change); err != nil {
				return err
			}
			if change.Kind != ChangeMigrate {
				continue
			}
			if change.ArchiveName == "" {
				return fmt.Errorf("migration conflict for %q requires an archive name", change.Name)
			}
			if err := repo.ValidateName(change.ArchiveName); err != nil {
				return fmt.Errorf("invalid migration archive name %q: %w", change.ArchiveName, err)
			}
			if change.ArchiveName != change.Name {
				if _, err := os.Lstat(filepath.Join(cfg.ArchiveSkillsRoot(), change.ArchiveName)); err == nil {
					return fmt.Errorf("migration archive destination exists: %s", change.ArchiveName)
				} else if !errors.Is(err, os.ErrNotExist) {
					return err
				}
			}
			archivePath := filepath.Join(cfg.ArchiveSkillsRoot(), change.ArchiveName)
			if prior, exists := plannedArchives[archivePath]; exists {
				return fmt.Errorf("migration archive destination %q is already planned for %q", change.ArchiveName, prior)
			}
			plannedArchives[archivePath] = change.Name
		}
	}
	return nil
}

func planDestinationNormalization(cfg config.Config, destination roots.ActiveRoot, name, path string, reserved map[string]struct{}) (*Change, *MigrationConflict, error) {
	active, err := actions.ScanActive(cfg, actions.ScanFilter{Scope: destination.Scope, Target: destination.Target})
	if err != nil {
		return nil, nil, err
	}
	index := slices.IndexFunc(active, func(skill actions.ActiveSkill) bool { return filepath.Clean(skill.Path) == filepath.Clean(path) })
	if index < 0 {
		return nil, nil, fmt.Errorf("destination entry %q is not a readable skill", path)
	}
	entry := active[index]
	if entry.Status == actions.StatusManaged {
		return nil, nil, nil
	}
	change := &Change{Kind: ChangeRemove, Name: name, Path: path, Destination: destination}
	if entry.Status == actions.StatusBroken {
		return change, nil, nil
	}
	change.Kind, change.ArchiveName = ChangeMigrate, name
	archive := filepath.Join(cfg.ArchiveSkillsRoot(), name)
	activeFP, activeErr := fingerprint.Directory(path)
	archiveFP, archiveErr := fingerprint.Directory(archive)
	if activeErr == nil && archiveErr == nil && activeFP == archiveFP {
		change.Kind, change.ArchiveName = ChangeRemove, ""
		return change, nil, nil
	}
	change.ArchiveName = ""
	suggested, err := availableRestoreArchiveName(cfg, name+"-preserved", reserved)
	if err != nil {
		return nil, nil, err
	}
	conflict := &MigrationConflict{Name: name, Path: path, ExistingArchive: archive, SuggestedName: suggested}
	return change, conflict, nil
}

func applyRestoreRemoval(ctx context.Context, cfg config.Config, change Change) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validatePlannedChange(cfg, change); err != nil {
		return err
	}
	if change.Kind == ChangeMigrate && change.ArchiveName != change.Name {
		if change.ArchiveName == "" {
			return fmt.Errorf("migration conflict for %q requires an archive name", change.Name)
		}
		return migrateRestoreExtra(change.Path, filepath.Join(cfg.ArchiveSkillsRoot(), change.ArchiveName))
	}
	_, err := actions.Unlink(cfg, actions.UnlinkRequest{Name: change.Name, Scope: change.Destination.Scope, Target: change.Destination.Target, Confirmed: true})
	return err
}

func resolveRestoreSkill(ctx context.Context, cfg config.Config, cache *remote.CheckoutCache, skill Skill) (PlannedSkill, error) {
	archivePath := filepath.Join(cfg.ArchiveSkillsRoot(), skill.Name)
	if info, err := os.Stat(archivePath); err == nil && info.IsDir() {
		if err := verifyExistingRestoreArchive(archivePath, skill); err != nil {
			return PlannedSkill{}, err
		}
		return PlannedSkill{Name: skill.Name, ArchivePath: archivePath}, nil
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return PlannedSkill{}, fmt.Errorf("inspect archive: %w", err)
	}
	if skill.Source.Type == SourceArchive {
		return PlannedSkill{}, errors.New("required archived copy is unavailable")
	}
	source := remote.GitSource{CloneURL: skill.Source.Repository, Ref: skill.Source.Ref}
	if skill.Source.Type == SourceGitHub {
		parts := strings.Split(skill.Source.Repository, "/")
		if len(parts) != 2 {
			return PlannedSkill{}, fmt.Errorf("invalid GitHub repository %q", skill.Source.Repository)
		}
		source.Owner, source.Repo = parts[0], parts[1]
		source.CloneURL = "https://github.com/" + skill.Source.Repository + ".git"
	}
	checkout, err := cache.Checkout(ctx, source)
	if err != nil {
		return PlannedSkill{}, err
	}
	found, err := checkout.FindSkillContext(ctx, skill.Name, skill.Source.Path)
	if err != nil {
		return PlannedSkill{}, err
	}
	found.Metadata.Compatibility = skill.Compatibility
	return PlannedSkill{Name: skill.Name, ArchivePath: archivePath, IncomingDir: found.SkillDir, Metadata: found.Metadata, NeedsArchive: true}, nil
}

func validateRestoreDestinations(cfg config.Config, requested []roots.ActiveRoot) ([]roots.ActiveRoot, error) {
	if len(requested) == 0 {
		return nil, errors.New("restore requires at least one explicit project Skills Folder")
	}
	configured := roots.ActiveRoots(cfg, roots.Filter{Scope: config.ScopeProject})
	seen := make(map[string]struct{}, len(requested))
	result := make([]roots.ActiveRoot, 0, len(requested))
	for _, candidate := range requested {
		if candidate.Scope != config.ScopeProject {
			return nil, fmt.Errorf("restore destination %q is not a project Skills Folder", candidate.Path)
		}
		index := slices.IndexFunc(configured, func(root roots.ActiveRoot) bool {
			return root.Target == candidate.Target && filepath.Clean(root.Path) == filepath.Clean(candidate.Path)
		})
		if index < 0 {
			return nil, fmt.Errorf("restore destination %q is not an enabled configured project Skills Folder", candidate.Path)
		}
		key := filepath.Clean(configured[index].Path)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, configured[index])
	}
	return result, nil
}

func planRestoreRemovals(cfg config.Config, destinations []roots.ActiveRoot, desired map[string]struct{}, reserved map[string]struct{}) ([]Change, []MigrationConflict, error) {
	var changes []Change
	var conflicts []MigrationConflict
	for _, destination := range destinations {
		active, err := actions.ScanActive(cfg, actions.ScanFilter{Scope: destination.Scope, Target: destination.Target})
		if err != nil {
			return nil, nil, err
		}
		for _, skill := range active {
			if filepath.Clean(skill.Root.Path) != filepath.Clean(destination.Path) {
				continue
			}
			occurrenceName := skill.Identity
			if _, keep := desired[occurrenceName]; keep {
				continue
			}
			kind := ChangeRemove
			archiveName := ""
			if skill.Status == actions.StatusUnmanaged {
				kind, archiveName = ChangeMigrate, occurrenceName
				archive := filepath.Join(cfg.ArchiveSkillsRoot(), occurrenceName)
				_, alreadyPlanned := reserved[occurrenceName]
				if _, err := os.Lstat(archive); err == nil || alreadyPlanned {
					if !alreadyPlanned && err == nil {
						activeFP, activeErr := fingerprint.Directory(skill.Path)
						archiveFP, archiveErr := fingerprint.Directory(archive)
						if activeErr == nil && archiveErr == nil && activeFP == archiveFP {
							kind, archiveName = ChangeRemove, ""
						}
					}
					if kind == ChangeMigrate {
						archiveName = ""
						suggested, err := availableRestoreArchiveName(cfg, occurrenceName+"-preserved", reserved)
						if err != nil {
							return nil, nil, err
						}
						conflicts = append(conflicts, MigrationConflict{Name: occurrenceName, Path: skill.Path, ExistingArchive: archive, SuggestedName: suggested})
					}
				} else if !errors.Is(err, os.ErrNotExist) {
					return nil, nil, err
				} else {
					reserved[occurrenceName] = struct{}{}
				}
			}
			changes = append(changes, Change{Kind: kind, Name: occurrenceName, Path: skill.Path, Destination: destination, ArchiveName: archiveName})
		}
	}
	return changes, conflicts, nil
}

func cleanupRestorePlan(plan RestorePlan, err error) (RestorePlan, error) {
	_ = plan.Close()
	return RestorePlan{}, err
}

func verifyExistingRestoreArchive(archivePath string, skill Skill) error {
	if skill.Fingerprint != "" {
		got, err := fingerprint.Directory(archivePath)
		if err != nil {
			return fmt.Errorf("fingerprint existing archive: %w", err)
		}
		if got != skill.Fingerprint {
			return fmt.Errorf("existing archive fingerprint %q does not match manifest fingerprint %q", got, skill.Fingerprint)
		}
	}
	if skill.Source.Type == SourceArchive {
		return nil
	}
	metadata, ok, err := remote.ReadSourceMetadata(archivePath)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("existing archive has no source identity")
	}
	want := remote.SourceMetadata{Ref: skill.Source.Ref, SkillPath: skill.Source.Path}
	switch skill.Source.Type {
	case SourceGit:
		want.SourceType, want.CloneURL = remote.SourceTypeGit, skill.Source.Repository
	case SourceGitHub:
		parts := strings.Split(skill.Source.Repository, "/")
		if len(parts) != 2 {
			return fmt.Errorf("invalid GitHub repository %q", skill.Source.Repository)
		}
		want.SourceType, want.Owner, want.Repo = remote.SourceTypeGitHub, parts[0], parts[1]
		want.CloneURL = "https://github.com/" + skill.Source.Repository + ".git"
	}
	if !metadata.SameIdentity(want) || metadata.Ref != want.Ref {
		return errors.New("existing archive source identity, ref, or path does not match manifest")
	}
	return nil
}

func validatePlannedChange(cfg config.Config, change Change) error {
	destinations, err := validateRestoreDestinations(cfg, []roots.ActiveRoot{change.Destination})
	if err != nil {
		return err
	}
	want := filepath.Join(destinations[0].Path, change.Name)
	if filepath.Clean(change.Path) != filepath.Clean(want) {
		return fmt.Errorf("planned path %q does not match explicit destination path %q", change.Path, want)
	}
	return nil
}

func availableRestoreArchiveName(cfg config.Config, base string, reserved map[string]struct{}) (string, error) {
	for index := 0; ; index++ {
		name := base
		if index > 0 {
			name = fmt.Sprintf("%s-%d", base, index+1)
		}
		if _, taken := reserved[name]; taken {
			continue
		}
		if _, err := os.Lstat(filepath.Join(cfg.ArchiveSkillsRoot(), name)); errors.Is(err, os.ErrNotExist) {
			reserved[name] = struct{}{}
			return name, nil
		} else if err != nil {
			return "", fmt.Errorf("inspect restore archive name %q: %w", name, err)
		}
	}
}

func migrateRestoreExtra(activePath, archivePath string) error {
	if _, err := os.Lstat(archivePath); err == nil {
		return fmt.Errorf("archive destination exists: %s", archivePath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	source := activePath
	info, err := os.Lstat(activePath)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		source, err = filepath.EvalSymlinks(activePath)
		if err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(archivePath), 0o755); err != nil {
		return err
	}
	temp, err := os.MkdirTemp(filepath.Dir(archivePath), ".restore-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(temp) }()
	if err := copyRestoreTree(source, temp); err != nil {
		return err
	}
	if err := os.Rename(temp, archivePath); err != nil {
		return err
	}
	if err := os.RemoveAll(activePath); err != nil {
		return fmt.Errorf("archive preserved at %q but active removal failed: %w", archivePath, err)
	}
	return nil
}

func copyRestoreTree(source, destination string) error {
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
		switch {
		case rel == ".":
			return os.Chmod(destination, info.Mode().Perm())
		case entry.Type()&os.ModeSymlink != 0:
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		case entry.IsDir():
			return os.Mkdir(target, info.Mode().Perm())
		default:
			in, err := os.Open(path)
			if err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
			if err != nil {
				_ = in.Close()
				return err
			}
			_, copyErr := io.Copy(out, in)
			inCloseErr := in.Close()
			closeErr := out.Close()
			return errors.Join(copyErr, inCloseErr, closeErr)
		}
	})
}

func sortRestorePlan(plan *RestorePlan) {
	slices.SortFunc(plan.Available, func(a, b PlannedSkill) int { return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)) })
	slices.SortFunc(plan.Unavailable, func(a, b UnavailableSkill) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	compareChanges := func(a, b Change) int { return strings.Compare(strings.ToLower(a.Path), strings.ToLower(b.Path)) }
	slices.SortFunc(plan.Additions, compareChanges)
	slices.SortFunc(plan.Normalizations, compareChanges)
	slices.SortFunc(plan.Removals, compareChanges)
}
