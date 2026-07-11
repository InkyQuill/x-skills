package manifest

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"github.com/InkyQuill/x-skills/internal/actions"
	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/fingerprint"
	"github.com/InkyQuill/x-skills/internal/remote"
)

type Result struct {
	Changed bool
	Skills  []Skill
}

func ReconcileLocal(cfg config.Config) (Result, error) {
	old, err := LoadLocal(cfg.ProjectRoot)
	if err != nil {
		return Result{}, err
	}
	recommended, err := LoadRecommended(cfg.ProjectRoot)
	if err != nil {
		return Result{}, err
	}
	recommendedNames := make(map[string]struct{}, len(recommended.Skills))
	for _, skill := range recommended.Skills {
		recommendedNames[skill.Name] = struct{}{}
	}

	retained := make(map[string]Skill)
	for _, skill := range old.Skills {
		if skill.Source.Type != SourceArchive {
			continue
		}
		if _, err := os.Stat(filepath.Join(cfg.ArchiveSkillsRoot(), skill.Name)); errors.Is(err, os.ErrNotExist) {
			retained[skill.Name] = skill
		} else if err != nil {
			return Result{}, fmt.Errorf("inspect archived skill %q: %w", skill.Name, err)
		}
	}

	active, err := actions.ScanActive(cfg, actions.ScanFilter{Scope: config.ScopeProject})
	if err != nil {
		return Result{}, fmt.Errorf("scan project skills: %w", err)
	}
	observed := make(map[string]Skill)
	for _, occurrence := range active {
		if occurrence.Status == actions.StatusBroken {
			continue
		}
		name := occurrence.Name
		if _, excluded := recommendedNames[name]; excluded {
			continue
		}
		skill, err := reconciledSkill(cfg, name, occurrence.Path, occurrence.Status)
		if err != nil {
			return Result{}, err
		}
		observed[name] = skill
	}
	for _, skill := range observed {
		retained[skill.Name] = skill
	}
	next := Manifest{Version: manifestVersion, Skills: make([]Skill, 0, len(retained))}
	for _, skill := range retained {
		next.Skills = append(next.Skills, skill)
	}
	if err := normalizeAndValidate(&next, true); err != nil {
		return Result{}, fmt.Errorf("reconcile local manifest: %w", err)
	}
	if reflect.DeepEqual(old, next) {
		return Result{Skills: next.Skills}, nil
	}
	if err := WriteLocal(cfg.ProjectRoot, next); err != nil {
		return Result{}, err
	}
	return Result{Changed: true, Skills: next.Skills}, nil
}

func reconciledSkill(cfg config.Config, name, activePath, status string) (Skill, error) {
	sourcePath := activePath
	if status == actions.StatusManaged {
		sourcePath = filepath.Join(cfg.ArchiveSkillsRoot(), name)
	}
	fp, err := fingerprint.Directory(sourcePath)
	if err != nil {
		return Skill{}, fmt.Errorf("fingerprint skill %q: %w", name, err)
	}
	skill := Skill{Name: name, Source: Source{Type: SourceArchive}, Fingerprint: fp}
	if status != actions.StatusManaged {
		return skill, nil
	}
	meta, ok, err := remote.ReadSourceMetadata(sourcePath)
	if err != nil {
		return Skill{}, fmt.Errorf("read source metadata for %q: %w", name, err)
	}
	if !ok {
		return skill, nil
	}
	skill.Compatibility = meta.Compatibility
	skill.Source.Ref = meta.Ref
	skill.Source.Path = meta.SkillPath
	switch meta.SourceType {
	case remote.SourceTypeGitHub:
		if meta.Owner == "" || meta.Repo == "" || meta.SkillPath == "" {
			return Skill{Name: name, Source: Source{Type: SourceArchive}, Fingerprint: fp}, nil
		}
		skill.Source.Type = SourceGitHub
		skill.Source.Repository = meta.Owner + "/" + meta.Repo
	case remote.SourceTypeGit:
		if meta.CloneURL == "" || meta.SkillPath == "" {
			return Skill{Name: name, Source: Source{Type: SourceArchive}, Fingerprint: fp}, nil
		}
		skill.Source.Type = SourceGit
		skill.Source.Repository = meta.CloneURL
	default:
		return Skill{}, fmt.Errorf("skill %q has unsupported source type %q", name, meta.SourceType)
	}
	return skill, nil
}
