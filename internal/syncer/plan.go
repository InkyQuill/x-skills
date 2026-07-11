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
	Fingerprint     string
	Action          string
	SourcePath      string
	ArchivePath     string
	DestinationPath string
}

type Conflict struct {
	CandidateID         string
	Name                string
	Fingerprint         string
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
	destinations, err := validateDestinations(cfg, destinations)
	if err != nil {
		return Plan{}, err
	}
	selected, err := selectedCandidates(groups, selection)
	if err != nil {
		return Plan{}, err
	}
	if err := validateSelectedCandidates(cfg, selected, destinations); err != nil {
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
	archiveStorageValidated := false
	ensureArchiveStorage := func() error {
		if archiveStorageValidated {
			return nil
		}
		if err := validateWritableDirectoryShape(cfg.ArchiveSkillsRoot()); err != nil {
			return fmt.Errorf("archive storage cannot publish skills: %w", err)
		}
		archiveStorageValidated = true
		return nil
	}
	for _, candidate := range selected {
		archivePath, err := canonicalEntryPath(filepath.Join(cfg.ArchiveSkillsRoot(), candidate.Name))
		if err != nil {
			return Plan{}, fmt.Errorf("canonicalize archive path for %q: %w", candidate.Name, err)
		}
		archiveExists, err := pathExists(archivePath)
		if err != nil {
			return Plan{}, fmt.Errorf("inspect archive for %q: %w", candidate.Name, err)
		}
		archiveMatches, err := pathMatchesFingerprint(archivePath, candidate.Fingerprint)
		if err != nil {
			return Plan{}, fmt.Errorf("inspect archive for %q: %w", candidate.Name, err)
		}
		archiveReplacement := false
		if archiveExists && !archiveMatches {
			resolution := resolutionByPath[archivePath]
			if resolution.Action != "" {
				usedResolutions[archivePath] = struct{}{}
			}
			suggestion, err := suggestPreserveName(candidate.Name, "archive", reservedNames)
			if err != nil {
				return Plan{}, err
			}
			conflict := Conflict{
				CandidateID: candidate.ID, Name: candidate.Name,
				Fingerprint:     candidate.Fingerprint,
				DestinationPath: archivePath, DestinationStatus: actions.StatusManaged,
				SuggestedPreserveAs: suggestion, Resolution: resolution,
			}
			switch resolution.Action {
			case ConflictCancel:
				return Plan{Cancelled: true}, nil
			case ConflictKeep:
				plan.Skipped = append(plan.Skipped, Skip{CandidateID: candidate.ID, Name: candidate.Name, DestinationPath: archivePath, Reason: SkipKeptDestination})
				continue
			case "":
				plan.Conflicts = append(plan.Conflicts, conflict)
				continue
			case ConflictReplace:
				if err := ensureArchiveStorage(); err != nil {
					return Plan{}, err
				}
				plan.Conflicts = append(plan.Conflicts, conflict)
				archiveReplacement = true
			default:
				return Plan{}, fmt.Errorf("unknown conflict action %q", resolution.Action)
			}
		}
		if !archiveMatches {
			if err := ensureArchiveStorage(); err != nil {
				return Plan{}, err
			}
			source, ok := migrationSource(candidate)
			if !ok {
				return Plan{}, fmt.Errorf("selected candidate %q has no unmanaged source to migrate", candidate.ID)
			}
			plan.Migrations = append(plan.Migrations, Change{
				CandidateID: candidate.ID,
				Name:        candidate.Name,
				Fingerprint: candidate.Fingerprint,
				Action:      "migrate",
				SourcePath:  source.Path,
				ArchivePath: archivePath,
			})
		}

		for _, destination := range destinations {
			destinationPath := filepath.Join(destination.Path, candidate.Name)
			classification, err := classifyDestination(cfg, destinationPath, archivePath, candidate.Fingerprint, archiveMatches, archiveReplacement)
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
					if resolution.Action == ConflictReplace {
						if err := ensureArchiveStorage(); err != nil {
							return Plan{}, err
						}
					}
					suggestion, err := suggestPreserveName(candidate.Name, destination.Target, reservedNames)
					if err != nil {
						return Plan{}, err
					}
					conflict := Conflict{
						CandidateID: candidate.ID, Name: candidate.Name,
						Fingerprint:     candidate.Fingerprint,
						DestinationPath: destinationPath, DestinationStatus: classification.status,
						SuggestedPreserveAs: suggestion, Resolution: resolution,
					}
					plan.Conflicts = append(plan.Conflicts, conflict)
					if resolution.Action == ConflictReplace {
						plan.Links = append(plan.Links, linkChange(candidate, archivePath, destinationPath, LinkNormalize))
					}
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
	groupNames := make(map[string]struct{}, len(groups))
	for _, group := range groups {
		if err := repo.ValidateName(group.Name); err != nil {
			return nil, fmt.Errorf("validate candidate group name: %w", err)
		}
		if _, exists := groupNames[group.Name]; exists {
			return nil, fmt.Errorf("duplicate candidate group %q", group.Name)
		}
		groupNames[group.Name] = struct{}{}
		for _, candidate := range group.Variants {
			if candidate.Name != group.Name {
				return nil, fmt.Errorf("candidate %q does not belong to group %q", candidate.Name, group.Name)
			}
			if err := repo.ValidateName(candidate.Name); err != nil {
				return nil, fmt.Errorf("validate candidate name: %w", err)
			}
			if candidate.ID != candidate.Name+":"+candidate.Fingerprint {
				return nil, fmt.Errorf("candidate %q has invalid ID %q", candidate.Name, candidate.ID)
			}
			if _, exists := byID[candidate.ID]; exists {
				return nil, fmt.Errorf("duplicate candidate ID %q", candidate.ID)
			}
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

func validateSelectedCandidates(cfg config.Config, candidates []Candidate, destinations []roots.ActiveRoot) error {
	configuredRoots := make(map[string]string)
	for _, root := range roots.ActiveRoots(cfg, roots.Filter{Scope: config.ScopeProject}) {
		canonical, err := canonicalPath(root.Path)
		if err != nil {
			return fmt.Errorf("validate configured source Skills Folder %q: %w", root.Path, err)
		}
		configuredRoots[root.Target] = canonical
	}
	destinationRoots := make(map[string]struct{}, len(destinations))
	for _, destination := range destinations {
		destinationRoots[destination.Path] = struct{}{}
	}
	for _, candidate := range candidates {
		if len(candidate.Occurrences) == 0 {
			return fmt.Errorf("selected candidate %q has no occurrences", candidate.ID)
		}
		archivePath := filepath.Join(cfg.ArchiveSkillsRoot(), candidate.Name)
		for _, occurrence := range candidate.Occurrences {
			if occurrence.Name != candidate.Name || filepath.Base(filepath.Clean(occurrence.Path)) != candidate.Name {
				return fmt.Errorf("occurrence %q does not match candidate %q", occurrence.Path, candidate.Name)
			}
			resolved, err := filepath.EvalSymlinks(occurrence.Path)
			if err != nil {
				return fmt.Errorf("resolve occurrence %q: %w", occurrence.Path, err)
			}
			matches, err := pathMatchesFingerprint(resolved, candidate.Fingerprint)
			if err != nil {
				return fmt.Errorf("fingerprint occurrence %q: %w", occurrence.Path, err)
			}
			if !matches {
				return fmt.Errorf("occurrence %q no longer matches candidate %q", occurrence.Path, candidate.ID)
			}
			if occurrence.Root.Scope != config.ScopeProject {
				return fmt.Errorf("occurrence %q is not project-scoped", occurrence.Path)
			}
			configuredRoot, ok := configuredRoots[occurrence.Root.Target]
			if !ok {
				return fmt.Errorf("occurrence %q has unknown Skills Folder target %q", occurrence.Path, occurrence.Root.Target)
			}
			occurrenceRoot, err := canonicalPath(occurrence.Root.Path)
			if err != nil {
				return fmt.Errorf("canonicalize occurrence Skills Folder %q: %w", occurrence.Root.Path, err)
			}
			occurrenceParent, err := canonicalPath(filepath.Dir(occurrence.Path))
			if err != nil {
				return fmt.Errorf("canonicalize occurrence parent %q: %w", occurrence.Path, err)
			}
			if occurrenceRoot != configuredRoot || occurrenceParent != configuredRoot {
				return fmt.Errorf("occurrence %q is outside configured Skills Folder %q", occurrence.Path, configuredRoot)
			}
			if _, selectedDestination := destinationRoots[occurrenceRoot]; selectedDestination {
				return fmt.Errorf("occurrence %q belongs to selected destination %q", occurrence.Path, occurrenceRoot)
			}
			switch occurrence.Status {
			case actions.StatusManaged:
				if !sameCanonicalPath(resolved, archivePath) {
					return fmt.Errorf("managed occurrence %q does not target archive %q", occurrence.Path, archivePath)
				}
			case actions.StatusUnmanaged:
				if isArchivedPath(cfg, resolved) {
					return fmt.Errorf("unmanaged occurrence %q points into archive", occurrence.Path)
				}
			default:
				return fmt.Errorf("occurrence %q has unsupported status %q", occurrence.Path, occurrence.Status)
			}
		}
	}
	return nil
}

func indexResolutions(cfg config.Config, resolutions []ConflictResolution) (map[string]ConflictResolution, error) {
	indexed := make(map[string]ConflictResolution, len(resolutions))
	for _, resolution := range resolutions {
		if resolution.DestinationPath == "" {
			return nil, fmt.Errorf("conflict resolution destination is required")
		}
		canonicalDestination, err := canonicalEntryPath(resolution.DestinationPath)
		if err != nil {
			return nil, fmt.Errorf("canonicalize conflict resolution destination: %w", err)
		}
		resolution.DestinationPath = canonicalDestination
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

func validateDestinations(cfg config.Config, destinations []roots.ActiveRoot) ([]roots.ActiveRoot, error) {
	allowed := make(map[string]roots.ActiveRoot)
	for _, root := range roots.ActiveRoots(cfg, roots.Filter{Scope: config.ScopeProject}) {
		canonical, err := canonicalPath(root.Path)
		if err != nil {
			return nil, fmt.Errorf("validate configured destination %q: %w", root.Path, err)
		}
		allowed[canonical] = root
	}

	validated := make([]roots.ActiveRoot, 0, len(destinations))
	seen := make(map[string]struct{}, len(destinations))
	for _, destination := range destinations {
		if destination.Scope != config.ScopeProject {
			return nil, fmt.Errorf("sync destination must be project-scoped: %s", destination.Path)
		}
		canonical, err := canonicalPath(destination.Path)
		if err != nil {
			return nil, fmt.Errorf("validate destination %q: %w", destination.Path, err)
		}
		configured, ok := allowed[canonical]
		if !ok || configured.Target != destination.Target {
			return nil, fmt.Errorf("destination is not a configured project Skills Folder: %s", destination.Path)
		}
		if _, duplicate := seen[canonical]; duplicate {
			return nil, fmt.Errorf("duplicate or aliased destination: %s", destination.Path)
		}
		for prior := range seen {
			if pathsOverlap(prior, canonical) {
				return nil, fmt.Errorf("overlapping destinations: %s and %s", prior, canonical)
			}
		}
		if err := validateWritableDirectoryShape(canonical); err != nil {
			return nil, fmt.Errorf("destination %q cannot host skills: %w", destination.Path, err)
		}
		seen[canonical] = struct{}{}
		destination.Path = canonical
		validated = append(validated, destination)
	}
	return validated, nil
}

func validateWritableDirectoryShape(path string) error {
	current := path
	for {
		info, err := os.Stat(current)
		switch {
		case err == nil:
			if !info.IsDir() {
				return fmt.Errorf("existing path %q is not a directory", current)
			}
			if info.Mode().Perm()&0o222 == 0 || info.Mode().Perm()&0o111 == 0 {
				return fmt.Errorf("existing directory %q is not writable and searchable", current)
			}
			return nil
		case !errors.Is(err, os.ErrNotExist):
			return err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return fmt.Errorf("no existing directory ancestor for %q", path)
		}
		current = parent
	}
}

func pathsOverlap(a, b string) bool {
	rel, err := filepath.Rel(a, b)
	if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return true
	}
	rel, err = filepath.Rel(b, a)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func canonicalEntryPath(path string) (string, error) {
	if path == "" || filepath.Base(filepath.Clean(path)) == "." {
		return "", fmt.Errorf("invalid entry path %q", path)
	}
	parent, err := canonicalPath(filepath.Dir(path))
	if err != nil {
		return "", err
	}
	return filepath.Join(parent, filepath.Base(filepath.Clean(path))), nil
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

func classifyDestination(cfg config.Config, path, archivePath, candidateFingerprint string, archiveMatches, archiveReplacement bool) (destinationClassification, error) {
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
			if archiveMatches {
				return destinationClassification{kind: destinationManaged, status: actions.StatusManaged}, nil
			}
			if archiveReplacement {
				return destinationClassification{kind: destinationMatching, status: actions.StatusManaged}, nil
			}
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

func pathExists(path string) (bool, error) {
	_, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
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
		Fingerprint:     candidate.Fingerprint,
		Action:          action,
		ArchivePath:     archivePath,
		DestinationPath: destinationPath,
	}
}
