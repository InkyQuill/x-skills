package syncer

import (
	"fmt"
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
	destinationPaths, err := canonicalPaths(destinations)
	if err != nil {
		return nil, err
	}
	consumers := destinationConsumers(destinations)
	active, err := actions.ScanActive(cfg, actions.ScanFilter{Scope: config.ScopeProject})
	if err != nil {
		return nil, err
	}

	grouped := make(map[string]map[string]*Candidate)
	assessmentPaths := make(map[string]string)
	for _, occurrence := range active {
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
		group := NameGroup{Name: name, Variants: make([]Candidate, 0, len(variants))}
		for _, candidate := range variants {
			profile, err := archiveCompatibility(cfg, name)
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
		return filepath.Clean(resolved), nil
	}
	return filepath.Clean(abs), nil
}

func destinationConsumers(destinations []roots.ActiveRoot) []string {
	var consumers []string
	for _, destination := range destinations {
		consumers = append(consumers, destination.Consumers...)
	}
	consumers, _ = config.NormalizeConsumers(consumers)
	return consumers
}

func archiveCompatibility(cfg config.Config, name string) (*remote.CompatibilityProfile, error) {
	archive := filepath.Join(cfg.ArchiveSkillsRoot(), name)
	metadata, found, err := remote.ReadSourceMetadata(archive)
	if err != nil {
		return nil, fmt.Errorf("read archived skill %q metadata: %w", name, err)
	}
	if !found {
		return nil, nil
	}
	return metadata.Compatibility, nil
}
