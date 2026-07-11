package manifest

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/remote"
	"github.com/InkyQuill/x-skills/internal/repo"
)

func Recommend(cfg config.Config, names []string) error {
	reconcileMu.Lock()
	defer reconcileMu.Unlock()

	recommended, err := LoadRecommended(cfg.ProjectRoot)
	if err != nil {
		return err
	}
	local, err := LoadLocal(cfg.ProjectRoot)
	if err != nil {
		return err
	}

	planned := make([]Skill, 0, len(names))
	for _, name := range names {
		skill, err := sourcedArchiveSkill(cfg, name)
		if err != nil {
			return err
		}
		planned = append(planned, skill)
	}
	for _, skill := range planned {
		recommended.Skills = upsertSkill(recommended.Skills, skill)
		local.Skills = removeSkill(local.Skills, skill.Name)
	}
	return writeManifestPair(cfg.ProjectRoot, recommended, local)
}

func Unrecommend(cfg config.Config, names []string) error {
	reconcileMu.Lock()
	defer reconcileMu.Unlock()

	recommended, err := LoadRecommended(cfg.ProjectRoot)
	if err != nil {
		return err
	}
	local, err := LoadLocal(cfg.ProjectRoot)
	if err != nil {
		return err
	}
	remove := make(map[string]struct{}, len(names))
	for _, name := range names {
		if _, duplicate := remove[name]; duplicate {
			return fmt.Errorf("duplicate skill name %q", name)
		}
		if !slices.ContainsFunc(recommended.Skills, func(skill Skill) bool { return skill.Name == name }) {
			return fmt.Errorf("skill %q is not in the Recommended Skill Manifest", name)
		}
		remove[name] = struct{}{}
	}
	recommended.Skills = slices.DeleteFunc(recommended.Skills, func(skill Skill) bool {
		_, removed := remove[skill.Name]
		return removed
	})
	local, err = planLocalReconciliation(cfg, local, recommended)
	if err != nil {
		return err
	}
	return writeManifestPair(cfg.ProjectRoot, recommended, local)
}

func sourcedArchiveSkill(cfg config.Config, name string) (Skill, error) {
	if err := repo.ValidateName(name); err != nil {
		return Skill{}, err
	}
	archivePath := filepath.Join(cfg.ArchiveSkillsRoot(), name)
	if _, err := os.Stat(archivePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Skill{}, fmt.Errorf("archived skill %q does not exist", name)
		}
		return Skill{}, fmt.Errorf("inspect archived skill %q: %w", name, err)
	}
	meta, ok, err := remote.ReadSourceMetadata(archivePath)
	if err != nil {
		return Skill{}, fmt.Errorf("read source metadata for %q: %w", name, err)
	}
	if !ok || meta.SkillPath == "" {
		return Skill{}, fmt.Errorf("skill %q requires reproducible Git or GitHub source metadata", name)
	}
	skill := Skill{Name: name, Compatibility: meta.Compatibility}
	skill.Source.Path = meta.SkillPath
	skill.Source.Ref = meta.Ref
	if skill.Source.Ref == "" {
		skill.Source.Ref = meta.Commit
	}
	switch meta.SourceType {
	case remote.SourceTypeGitHub:
		if meta.Owner == "" || meta.Repo == "" {
			return Skill{}, fmt.Errorf("skill %q requires reproducible Git or GitHub source metadata", name)
		}
		skill.Source.Type = SourceGitHub
		skill.Source.Repository = meta.Owner + "/" + meta.Repo
	case remote.SourceTypeGit:
		if meta.CloneURL == "" {
			return Skill{}, fmt.Errorf("skill %q requires reproducible Git or GitHub source metadata", name)
		}
		skill.Source.Type = SourceGit
		skill.Source.Repository = meta.CloneURL
	default:
		return Skill{}, fmt.Errorf("skill %q requires reproducible Git or GitHub source metadata", name)
	}
	return skill, nil
}

func upsertSkill(skills []Skill, replacement Skill) []Skill {
	index := slices.IndexFunc(skills, func(skill Skill) bool { return skill.Name == replacement.Name })
	if index < 0 {
		return append(skills, replacement)
	}
	skills[index] = replacement
	return skills
}

func removeSkill(skills []Skill, name string) []Skill {
	return slices.DeleteFunc(skills, func(skill Skill) bool { return skill.Name == name })
}

func writeManifestPair(projectRoot string, recommended, local Manifest) error {
	recommendedPath := filepath.Join(projectRoot, RecommendedFilename)
	backup, readErr := os.ReadFile(recommendedPath)
	hadRecommended := readErr == nil
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		return fmt.Errorf("back up Recommended Skill Manifest: %w", readErr)
	}
	if err := WriteRecommended(projectRoot, recommended); err != nil {
		return err
	}
	if err := WriteLocal(projectRoot, local); err != nil {
		restoreErr := restoreRecommended(recommendedPath, backup, hadRecommended)
		if restoreErr != nil {
			return errors.Join(
				fmt.Errorf("write Local Skill Manifest: %w", err),
				fmt.Errorf("restore Recommended Skill Manifest: %w", restoreErr),
			)
		}
		return fmt.Errorf("write Local Skill Manifest: %w", err)
	}
	return nil
}

func restoreRecommended(filename string, backup []byte, existed bool) error {
	if !existed {
		if err := os.Remove(filename); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	temp, err := os.CreateTemp(filepath.Dir(filename), ".x-skills-recommended-backup-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer func() { _ = os.Remove(tempPath) }()
	if err := temp.Chmod(0o644); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(backup); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, filename)
}
