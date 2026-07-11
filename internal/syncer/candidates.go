package syncer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/InkyQuill/x-skills/internal/actions"
	"github.com/InkyQuill/x-skills/internal/compatibility"
	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/fingerprint"
	"github.com/InkyQuill/x-skills/internal/remote"
	"github.com/InkyQuill/x-skills/internal/roots"
)

type Candidate struct {
	ID            string
	Name          string
	Fingerprint   string
	Occurrences   []actions.ActiveSkill
	Compatibility compatibility.Assessment
}

type NameGroup struct {
	Name     string
	Variants []Candidate
}

func Discover(cfg config.Config, destinations []roots.ActiveRoot) ([]NameGroup, error) {
	return DiscoverContext(context.Background(), cfg, destinations)
}

func DiscoverContext(ctx context.Context, cfg config.Config, destinations []roots.ActiveRoot) ([]NameGroup, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	destinationPaths, err := canonicalPaths(destinations)
	if err != nil {
		return nil, err
	}
	consumers, err := destinationConsumers(destinations)
	if err != nil {
		return nil, err
	}
	active, err := actions.ScanActive(cfg, actions.ScanFilter{Scope: config.ScopeProject})
	if err != nil {
		return nil, err
	}

	grouped := make(map[string]map[string]*Candidate)
	assessmentPaths := make(map[string]string)
	for _, occurrence := range active {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		rootPath, err := canonicalPath(occurrence.Root.Path)
		if err != nil {
			return nil, fmt.Errorf("resolve Skills Folder %q: %w", occurrence.Root.Path, err)
		}
		if _, excluded := destinationPaths[rootPath]; excluded {
			continue
		}

		skillPath, err := filepath.EvalSymlinks(occurrence.Path)
		if err != nil {
			return nil, fmt.Errorf("resolve active skill %q: %w", occurrence.Path, err)
		}
		fp, err := fingerprint.Directory(skillPath)
		if err != nil {
			return nil, fmt.Errorf("fingerprint active skill %q: %w", occurrence.Path, err)
		}
		name := filepath.Base(occurrence.Path)
		variants := grouped[name]
		if variants == nil {
			variants = make(map[string]*Candidate)
			grouped[name] = variants
		}
		candidate := variants[fp]
		if candidate == nil {
			candidate = &Candidate{ID: name + ":" + fp, Name: name, Fingerprint: fp}
			variants[fp] = candidate
			assessmentPaths[candidate.ID] = skillPath
		}
		candidate.Occurrences = append(candidate.Occurrences, occurrence)
	}

	groups := make([]NameGroup, 0, len(grouped))
	for name, variants := range grouped {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		group := NameGroup{Name: name, Variants: make([]Candidate, 0, len(variants))}
		for _, candidate := range variants {
			profile, err := archiveCompatibility(cfg, name, candidate.Fingerprint)
			if err != nil {
				return nil, err
			}
			candidate.Compatibility, err = compatibility.Assess(assessmentPaths[candidate.ID], profile, consumers)
			if err != nil {
				return nil, fmt.Errorf("assess skill %q compatibility: %w", name, err)
			}
			group.Variants = append(group.Variants, *candidate)
		}
		slices.SortFunc(group.Variants, func(a, b Candidate) int {
			return strings.Compare(a.Fingerprint, b.Fingerprint)
		})
		groups = append(groups, group)
	}
	slices.SortFunc(groups, func(a, b NameGroup) int {
		return strings.Compare(a.Name, b.Name)
	})
	return groups, nil
}

func canonicalPaths(activeRoots []roots.ActiveRoot) (map[string]struct{}, error) {
	paths := make(map[string]struct{}, len(activeRoots))
	for _, root := range activeRoots {
		path, err := canonicalPath(root.Path)
		if err != nil {
			return nil, fmt.Errorf("resolve destination Skills Folder %q: %w", root.Path, err)
		}
		paths[path] = struct{}{}
	}
	return paths, nil
}

func canonicalPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err == nil {
		info, err := os.Stat(resolved)
		if err != nil {
			return "", err
		}
		if !info.IsDir() {
			return "", fmt.Errorf("resolved path %q is not a directory", resolved)
		}
		return filepath.Clean(resolved), nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	current := abs
	var missing []string
	for {
		info, statErr := os.Lstat(current)
		switch {
		case statErr == nil:
			if !info.IsDir() && len(missing) > 0 {
				return "", fmt.Errorf("existing ancestor %q is not a directory", current)
			}
			resolved, err := filepath.EvalSymlinks(current)
			if err != nil {
				return "", err
			}
			for i := len(missing) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, missing[i])
			}
			return filepath.Clean(resolved), nil
		case !errors.Is(statErr, os.ErrNotExist):
			return "", statErr
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", err
		}
		missing = append(missing, filepath.Base(current))
		current = parent
	}
}

func destinationConsumers(destinations []roots.ActiveRoot) ([]string, error) {
	var consumers []string
	for _, destination := range destinations {
		consumers = append(consumers, destination.Consumers...)
	}
	consumers, err := config.NormalizeConsumers(consumers)
	if err != nil {
		return nil, fmt.Errorf("normalize destination consumers: %w", err)
	}
	return consumers, nil
}

func archiveCompatibility(cfg config.Config, name, candidateFingerprint string) (*remote.CompatibilityProfile, error) {
	archive := filepath.Join(cfg.ArchiveSkillsRoot(), name)
	metadata, found, err := remote.ReadSourceMetadata(archive)
	if err != nil {
		return nil, fmt.Errorf("read archived skill %q metadata: %w", name, err)
	}
	if !found {
		return nil, nil
	}
	archiveFingerprint, err := fingerprint.Directory(archive)
	if err != nil {
		return nil, fmt.Errorf("fingerprint archived skill %q: %w", name, err)
	}
	if archiveFingerprint != candidateFingerprint {
		return nil, nil
	}
	return metadata.Compatibility, nil
}
