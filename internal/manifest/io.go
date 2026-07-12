package manifest

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/InkyQuill/x-skills/internal/repo"
	"gopkg.in/yaml.v3"
)

const manifestVersion = 1

func LoadRecommended(projectRoot string) (Manifest, error) {
	return load(filepath.Join(projectRoot, RecommendedFilename), false)
}

func LoadLocal(projectRoot string) (Manifest, error) {
	return load(filepath.Join(projectRoot, LocalFilename), true)
}

func WriteRecommended(projectRoot string, manifest Manifest) error {
	return write(filepath.Join(projectRoot, RecommendedFilename), manifest, false)
}

func WriteLocal(projectRoot string, manifest Manifest) error {
	return write(filepath.Join(projectRoot, LocalFilename), manifest, true)
}

func load(filename string, allowArchive bool) (Manifest, error) {
	file, err := os.Open(filename)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Manifest{Version: manifestVersion, Skills: []Skill{}}, nil
		}
		return Manifest{}, fmt.Errorf("read manifest %q: %w", filename, err)
	}
	defer file.Close()

	manifest := Manifest{Skills: []Skill{}}
	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("parse manifest %q: %w", filename, err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("multiple YAML documents are not supported")
		}
		return Manifest{}, fmt.Errorf("parse manifest %q: %w", filename, err)
	}
	if err := normalizeAndValidate(&manifest, allowArchive); err != nil {
		return Manifest{}, fmt.Errorf("parse manifest %q: %w", filename, err)
	}
	return manifest, nil
}

func write(filename string, manifest Manifest, allowArchive bool) error {
	manifest = cloneManifest(manifest)
	if err := normalizeAndValidate(&manifest, allowArchive); err != nil {
		return fmt.Errorf("write manifest %q: %w", filename, err)
	}
	data, err := yaml.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("encode manifest %q: %w", filename, err)
	}

	temp, err := os.CreateTemp(filepath.Dir(filename), ".x-skills-manifest-*")
	if err != nil {
		return fmt.Errorf("write manifest %q: %w", filename, err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(0o644); err != nil {
		_ = temp.Close()
		return fmt.Errorf("write manifest %q: %w", filename, err)
	}
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return fmt.Errorf("write manifest %q: %w", filename, err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("write manifest %q: %w", filename, err)
	}
	if err := os.Rename(tempPath, filename); err != nil {
		return fmt.Errorf("write manifest %q: %w", filename, err)
	}
	return nil
}

func normalizeAndValidate(manifest *Manifest, allowArchive bool) error {
	if manifest.Version != manifestVersion {
		return fmt.Errorf("unsupported manifest version %d", manifest.Version)
	}
	if manifest.Skills == nil {
		manifest.Skills = []Skill{}
	}

	seen := make(map[string]struct{}, len(manifest.Skills))
	for i := range manifest.Skills {
		skill := &manifest.Skills[i]
		if err := repo.ValidateName(skill.Name); err != nil {
			return err
		}
		if _, exists := seen[skill.Name]; exists {
			return fmt.Errorf("duplicate skill name %q", skill.Name)
		}
		seen[skill.Name] = struct{}{}
		if err := normalizeAndValidateSkill(skill, allowArchive); err != nil {
			return fmt.Errorf("skill %q: %w", skill.Name, err)
		}
	}
	slices.SortStableFunc(manifest.Skills, compareSkillNames)
	return nil
}

func normalizeAndValidateSkill(skill *Skill, allowArchive bool) error {
	skill.Source.Path = normalizeGitPath(skill.Source.Path)
	switch skill.Source.Type {
	case SourceGitHub, SourceGit:
		if skill.Source.Repository == "" || skill.Source.Path == "" {
			return errors.New("git source requires repository and path")
		}
	case SourceArchive:
		if !allowArchive {
			return errors.New("archive source is not allowed in the Recommended Skill Manifest")
		}
		if skill.Source.Repository != "" || skill.Source.Path != "" || skill.Source.Ref != "" {
			return errors.New("archive source cannot contain repository, path, or ref")
		}
		fingerprint, err := normalizeContentFingerprint(skill.Fingerprint)
		if err != nil {
			return err
		}
		skill.Fingerprint = fingerprint
	default:
		return fmt.Errorf("unsupported source type %q", skill.Source.Type)
	}
	if skill.Compatibility != nil {
		profile := skill.Compatibility
		if profile.Agnostic == (len(profile.Agents) > 0) {
			return errors.New("compatibility must be agnostic or name at least one agent")
		}
		slices.Sort(profile.Agents)
	}
	return nil
}

func normalizeContentFingerprint(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.New("archive source requires a content fingerprint")
	}
	if len(value) >= len("sha256:") && strings.EqualFold(value[:len("sha256:")], "sha256:") {
		value = value[len("sha256:"):]
	}
	value = strings.ToLower(value)
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != 32 {
		return "", errors.New("invalid content fingerprint: expected a SHA-256 digest")
	}
	return value, nil
}

func normalizeGitPath(value string) string {
	if value == "" {
		return ""
	}
	value = strings.ReplaceAll(value, `\`, "/")
	return strings.Trim(path.Clean(value), "/")
}

func cloneManifest(manifest Manifest) Manifest {
	clone := Manifest{Version: manifest.Version, Skills: slices.Clone(manifest.Skills)}
	if clone.Skills == nil {
		clone.Skills = []Skill{}
	}
	for i := range clone.Skills {
		if clone.Skills[i].Compatibility == nil {
			continue
		}
		profile := *clone.Skills[i].Compatibility
		profile.Agents = slices.Clone(profile.Agents)
		clone.Skills[i].Compatibility = &profile
	}
	return clone
}
