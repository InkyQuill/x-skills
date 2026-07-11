package syncer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/InkyQuill/x-skills/internal/actions"
	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/fingerprint"
	"github.com/InkyQuill/x-skills/internal/repo"
	"github.com/InkyQuill/x-skills/internal/roots"
)

const (
	ConflictReplace = "replace"
	ConflictKeep    = "keep"
	ConflictCancel  = "cancel"

	LinkCreate    = "link"
	LinkNormalize = "normalize"

	SkipAlreadyManaged  = "already managed"
	SkipKeptDestination = "kept destination"
)

type Selection struct {
	CandidateIDs  []string
	VariantByName map[string]string
}

type ConflictResolution struct {
	DestinationPath string
	PreserveAs      string
	Action          string
}

type Change struct {
	CandidateID     string
	Name            string
	Action          string
	SourcePath      string
	ArchivePath     string
	DestinationPath string
}

type Conflict struct {
	CandidateID         string
	Name                string
	DestinationPath     string
	DestinationStatus   string
	SuggestedPreserveAs string
	Resolution          ConflictResolution
}

type Skip struct {
	CandidateID     string
	Name            string
	DestinationPath string
	Reason          string
}

type Plan struct {
	Migrations []Change
	Links      []Change
	Conflicts  []Conflict
	Skipped    []Skip
	Cancelled  bool
}

// Preflight describes the filesystem changes needed to reconcile selected
// candidates into destinations. It performs no filesystem mutation.
func Preflight(
	cfg config.Config,
	groups []NameGroup,
	destinations []roots.ActiveRoot,
	selection Selection,
	resolutions []ConflictResolution,
) (Plan, error) {
	selected, err := selectedCandidates(groups, selection)
	if err != nil {
		return Plan{}, err
	}
	resolutionByPath, err := indexResolutions(cfg, resolutions)
	if err != nil {
		return Plan{}, err
	}

	reservedNames, err := archivedNames(cfg)
	if err != nil {
		return Plan{}, err
	}
	for _, candidate := range selected {
		reservedNames[candidate.Name] = struct{}{}
	}
	for _, resolution := range resolutions {
		if resolution.Action != ConflictReplace {
			continue
		}
		if _, exists := reservedNames[resolution.PreserveAs]; exists {
			return Plan{}, fmt.Errorf("preserve name %q already exists or is reserved", resolution.PreserveAs)
		}
		reservedNames[resolution.PreserveAs] = struct{}{}
	}

	var plan Plan
	usedResolutions := make(map[string]struct{}, len(resolutions))
	for _, candidate := range selected {
		archivePath := filepath.Join(cfg.ArchiveSkillsRoot(), candidate.Name)
		archiveMatches, err := pathMatchesFingerprint(archivePath, candidate.Fingerprint)
		if err != nil {
			return Plan{}, fmt.Errorf("inspect archive for %q: %w", candidate.Name, err)
		}
		if !archiveMatches {
			source, ok := migrationSource(candidate)
			if !ok {
				return Plan{}, fmt.Errorf("selected candidate %q has no unmanaged source to migrate", candidate.ID)
			}
			plan.Migrations = append(plan.Migrations, Change{
				CandidateID: candidate.ID,
				Name:        candidate.Name,
				Action:      "migrate",
				SourcePath:  source.Path,
				ArchivePath: archivePath,
			})
		}

		for _, destination := range destinations {
			destinationPath := filepath.Join(destination.Path, candidate.Name)
			classification, err := classifyDestination(cfg, destinationPath, archivePath, candidate.Fingerprint)
			if err != nil {
				return Plan{}, err
			}
			switch classification.kind {
			case destinationMissing:
				plan.Links = append(plan.Links, linkChange(candidate, archivePath, destinationPath, LinkCreate))
			case destinationManaged:
				plan.Skipped = append(plan.Skipped, Skip{CandidateID: candidate.ID, Name: candidate.Name, DestinationPath: destinationPath, Reason: SkipAlreadyManaged})
			case destinationMatching:
				plan.Links = append(plan.Links, linkChange(candidate, archivePath, destinationPath, LinkNormalize))
			case destinationDivergent:
				resolution := resolutionByPath[destinationPath]
				if resolution.Action != "" {
					usedResolutions[destinationPath] = struct{}{}
				}
				switch resolution.Action {
				case ConflictCancel:
					return Plan{Cancelled: true}, nil
				case ConflictKeep:
					plan.Skipped = append(plan.Skipped, Skip{CandidateID: candidate.ID, Name: candidate.Name, DestinationPath: destinationPath, Reason: SkipKeptDestination})
				case "", ConflictReplace:
					suggestion, err := suggestPreserveName(candidate.Name, destination.Target, reservedNames)
					if err != nil {
						return Plan{}, err
					}
					conflict := Conflict{
						CandidateID: candidate.ID, Name: candidate.Name,
						DestinationPath: destinationPath, DestinationStatus: classification.status,
						SuggestedPreserveAs: suggestion, Resolution: resolution,
					}
					plan.Conflicts = append(plan.Conflicts, conflict)
				default:
					return Plan{}, fmt.Errorf("unknown conflict action %q", resolution.Action)
				}
			}
		}
	}
	for path := range resolutionByPath {
		if _, used := usedResolutions[path]; !used {
			return Plan{}, fmt.Errorf("conflict resolution does not match a divergent destination: %s", path)
		}
	}
	return plan, nil
}

func selectedCandidates(groups []NameGroup, selection Selection) ([]Candidate, error) {
	byID := make(map[string]Candidate)
	for _, group := range groups {
		for _, candidate := range group.Variants {
			byID[candidate.ID] = candidate
		}
	}
	ids := slices.Clone(selection.CandidateIDs)
	for name, id := range selection.VariantByName {
		candidate, ok := byID[id]
		if !ok || candidate.Name != name {
			return nil, fmt.Errorf("unknown variant %q for skill %q", id, name)
		}
		ids = append(ids, id)
	}
	slices.Sort(ids)
	ids = slices.Compact(ids)
	selected := make([]Candidate, 0, len(ids))
	selectedNames := make(map[string]string, len(ids))
	for _, id := range ids {
		candidate, ok := byID[id]
		if !ok {
			return nil, fmt.Errorf("unknown candidate %q", id)
		}
		if prior, exists := selectedNames[candidate.Name]; exists && prior != id {
			return nil, fmt.Errorf("multiple variants selected for skill %q", candidate.Name)
		}
		selectedNames[candidate.Name] = id
		selected = append(selected, candidate)
	}
	return selected, nil
}

func indexResolutions(cfg config.Config, resolutions []ConflictResolution) (map[string]ConflictResolution, error) {
	indexed := make(map[string]ConflictResolution, len(resolutions))
	for _, resolution := range resolutions {
		if resolution.DestinationPath == "" {
			return nil, fmt.Errorf("conflict resolution destination is required")
		}
		if _, exists := indexed[resolution.DestinationPath]; exists {
			return nil, fmt.Errorf("duplicate conflict resolution for %q", resolution.DestinationPath)
		}
		switch resolution.Action {
		case ConflictReplace:
			if err := repo.ValidateName(resolution.PreserveAs); err != nil {
				return nil, fmt.Errorf("validate preserve name: %w", err)
			}
		case ConflictKeep, ConflictCancel:
			if resolution.PreserveAs != "" {
				return nil, fmt.Errorf("preserve name is only valid with %q", ConflictReplace)
			}
		default:
			return nil, fmt.Errorf("unknown conflict action %q", resolution.Action)
		}
		indexed[resolution.DestinationPath] = resolution
	}
	return indexed, nil
}

func archivedNames(cfg config.Config) (map[string]struct{}, error) {
	names := make(map[string]struct{})
	entries, err := os.ReadDir(cfg.ArchiveSkillsRoot())
	if errors.Is(err, os.ErrNotExist) {
		return names, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read archive: %w", err)
	}
	for _, entry := range entries {
		names[entry.Name()] = struct{}{}
	}
	return names, nil
}

func migrationSource(candidate Candidate) (actions.ActiveSkill, bool) {
	for _, occurrence := range candidate.Occurrences {
		if occurrence.Status == actions.StatusUnmanaged {
			return occurrence, true
		}
	}
	return actions.ActiveSkill{}, false
}

type destinationKind int

const (
	destinationMissing destinationKind = iota
	destinationManaged
	destinationMatching
	destinationDivergent
)

type destinationClassification struct {
	kind   destinationKind
	status string
}

func classifyDestination(cfg config.Config, path, archivePath, candidateFingerprint string) (destinationClassification, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return destinationClassification{kind: destinationMissing}, nil
	}
	if err != nil {
		return destinationClassification{}, fmt.Errorf("inspect destination %q: %w", path, err)
	}
	resolved := path
	status := actions.StatusUnmanaged
	if info.Mode()&os.ModeSymlink != 0 {
		resolved, err = filepath.EvalSymlinks(path)
		if err != nil {
			return destinationClassification{kind: destinationDivergent, status: actions.StatusUnmanaged}, nil
		}
		if isArchivedPath(cfg, resolved) {
			status = actions.StatusManaged
		}
		if sameCanonicalPath(resolved, archivePath) {
			return destinationClassification{kind: destinationManaged, status: actions.StatusManaged}, nil
		}
	} else if !info.IsDir() {
		return destinationClassification{kind: destinationDivergent, status: status}, nil
	}
	matches, err := pathMatchesFingerprint(resolved, candidateFingerprint)
	if err != nil {
		return destinationClassification{}, fmt.Errorf("fingerprint destination %q: %w", path, err)
	}
	if matches {
		return destinationClassification{kind: destinationMatching, status: status}, nil
	}
	return destinationClassification{kind: destinationDivergent, status: status}, nil
}

func pathMatchesFingerprint(path, want string) (bool, error) {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.IsDir() {
		return false, nil
	}
	got, err := fingerprint.Directory(path)
	if err != nil {
		return false, err
	}
	return got == want, nil
}

func isArchivedPath(cfg config.Config, path string) bool {
	root, err := filepath.EvalSymlinks(cfg.ArchiveSkillsRoot())
	if err != nil {
		root = cfg.ArchiveSkillsRoot()
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func sameCanonicalPath(a, b string) bool {
	for _, value := range []*string{&a, &b} {
		if resolved, err := filepath.EvalSymlinks(*value); err == nil {
			*value = resolved
		}
		if abs, err := filepath.Abs(*value); err == nil {
			*value = abs
		}
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

func suggestPreserveName(name, target string, reserved map[string]struct{}) (string, error) {
	target = strings.TrimSpace(strings.ToLower(target))
	if target == "" {
		target = "destination"
	}
	base := name + "-from-" + target
	for suffix := 1; ; suffix++ {
		candidate := base
		if suffix > 1 {
			candidate = fmt.Sprintf("%s-%d", base, suffix)
		}
		if err := repo.ValidateName(candidate); err != nil {
			return "", err
		}
		if _, exists := reserved[candidate]; exists {
			continue
		}
		reserved[candidate] = struct{}{}
		return candidate, nil
	}
}

func linkChange(candidate Candidate, archivePath, destinationPath, action string) Change {
	return Change{
		CandidateID:     candidate.ID,
		Name:            candidate.Name,
		Action:          action,
		ArchivePath:     archivePath,
		DestinationPath: destinationPath,
	}
}
