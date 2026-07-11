package manifest

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/InkyQuill/x-skills/internal/actions"
	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/remote"
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
	Removals        []Change
	RemovalsBlocked bool
	Notices         []Notice
	checkoutRoot    string
}

type RestoreResult struct {
	Additions       []Change
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
			}
		}
	}
	if request.Full {
		removals, err := planRestoreRemovals(cfg, destinations, desired)
		if err != nil {
			return cleanupRestorePlan(plan, err)
		}
		if len(plan.Unavailable) > 0 {
			plan.RemovalsBlocked = len(removals) > 0
		} else {
			plan.Removals = removals
		}
	}
	sortRestorePlan(&plan)
	return plan, nil
}

func ApplyRestore(ctx context.Context, cfg config.Config, plan RestorePlan) (RestoreResult, error) {
	defer os.RemoveAll(plan.checkoutRoot)
	result := RestoreResult{Unavailable: slices.Clone(plan.Unavailable), RemovalsBlocked: plan.RemovalsBlocked}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	for _, skill := range plan.Available {
		if !skill.NeedsArchive {
			continue
		}
		if _, err := remote.ApplyArchive(remote.AddRequest{Config: cfg, IncomingDir: skill.IncomingDir, ArchiveName: skill.Name, Metadata: skill.Metadata, Conflict: remote.ConflictArchiveOnly}); err != nil {
			return result, fmt.Errorf("archive restored skill %q: %w", skill.Name, err)
		}
	}
	for _, change := range plan.Additions {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if _, err := actions.Link(cfg, actions.LinkRequest{Name: change.Name, Scope: change.Destination.Scope, Target: change.Destination.Target}); err != nil {
			return result, fmt.Errorf("restore link %q: %w", change.Name, err)
		}
		result.Additions = append(result.Additions, change)
	}
	if len(plan.Unavailable) > 0 {
		return result, nil
	}
	for _, change := range plan.Removals {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if change.ArchiveName != "" && change.ArchiveName != change.Name {
			return result, fmt.Errorf("migration archive name %q for %q must be applied by an interactive caller", change.ArchiveName, change.Name)
		}
		_, err := actions.Unlink(cfg, actions.UnlinkRequest{Name: change.Name, Scope: change.Destination.Scope, Target: change.Destination.Target, Confirmed: true})
		if err != nil {
			return result, fmt.Errorf("restore %s %q: %w", change.Kind, change.Name, err)
		}
		result.Removals = append(result.Removals, change)
	}
	if len(result.Additions) > 0 || len(result.Removals) > 0 {
		if _, err := ReconcileLocal(cfg); err != nil {
			return result, fmt.Errorf("restore filesystem changes succeeded but Local Skill Manifest reconciliation failed: %w", err)
		}
	}
	return result, nil
}

func resolveRestoreSkill(ctx context.Context, cfg config.Config, cache *remote.CheckoutCache, skill Skill) (PlannedSkill, error) {
	archivePath := filepath.Join(cfg.ArchiveSkillsRoot(), skill.Name)
	if info, err := os.Stat(archivePath); err == nil && info.IsDir() {
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

func planRestoreRemovals(cfg config.Config, destinations []roots.ActiveRoot, desired map[string]struct{}) ([]Change, error) {
	var changes []Change
	for _, destination := range destinations {
		active, err := actions.ScanActive(cfg, actions.ScanFilter{Scope: destination.Scope, Target: destination.Target})
		if err != nil {
			return nil, err
		}
		for _, skill := range active {
			if filepath.Clean(skill.Root.Path) != filepath.Clean(destination.Path) {
				continue
			}
			if _, keep := desired[skill.Name]; keep {
				continue
			}
			kind := ChangeRemove
			archiveName := ""
			if skill.Status != actions.StatusManaged {
				kind, archiveName = ChangeMigrate, skill.Name
			}
			changes = append(changes, Change{Kind: kind, Name: skill.Name, Path: skill.Path, Destination: destination, ArchiveName: archiveName})
		}
	}
	return changes, nil
}

func cleanupRestorePlan(plan RestorePlan, err error) (RestorePlan, error) {
	_ = os.RemoveAll(plan.checkoutRoot)
	return RestorePlan{}, err
}

func sortRestorePlan(plan *RestorePlan) {
	slices.SortFunc(plan.Available, func(a, b PlannedSkill) int { return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)) })
	slices.SortFunc(plan.Unavailable, func(a, b UnavailableSkill) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	compareChanges := func(a, b Change) int { return strings.Compare(strings.ToLower(a.Path), strings.ToLower(b.Path)) }
	slices.SortFunc(plan.Additions, compareChanges)
	slices.SortFunc(plan.Removals, compareChanges)
}
