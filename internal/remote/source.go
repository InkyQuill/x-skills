package remote

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const MetadataFile = ".x-skills.json"

const (
	metadataSchemaV1 = 1
	metadataSchemaV2 = 2
)

const (
	SourceTypeGitHub = "github"
	SourceTypeGit    = "git"
)

type SourceMetadata struct {
	SchemaVersion int                   `json:"schema_version"`
	SourceType    string                `json:"source_type"`
	Owner         string                `json:"owner,omitempty"`
	Repo          string                `json:"repo,omitempty"`
	CloneURL      string                `json:"clone_url"`
	Ref           string                `json:"ref,omitempty"`
	Commit        string                `json:"commit"`
	SkillPath     string                `json:"skill_path"`
	UpstreamName  string                `json:"upstream_name,omitempty"`
	Compatibility *CompatibilityProfile `json:"compatibility,omitempty"`
}

type CompatibilityProfile struct {
	Agnostic bool     `json:"agnostic,omitempty"`
	Agents   []string `json:"agents,omitempty"`
}

func ReadSourceMetadata(skillDir string) (SourceMetadata, bool, error) {
	data, err := os.ReadFile(filepath.Join(skillDir, MetadataFile))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return SourceMetadata{}, false, nil
		}
		return SourceMetadata{}, false, fmt.Errorf("read source metadata: %w", err)
	}
	var meta SourceMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return SourceMetadata{}, false, fmt.Errorf("parse source metadata: %w", err)
	}
	if meta.SchemaVersion == 0 {
		meta.SchemaVersion = metadataSchemaV1
	}
	if err := normalizeCompatibility(&meta); err != nil {
		return SourceMetadata{}, false, fmt.Errorf("parse source metadata: %w", err)
	}
	return meta, true, nil
}

func WriteSourceMetadata(skillDir string, meta SourceMetadata) error {
	meta.SchemaVersion = metadataSchemaV2
	if err := normalizeCompatibility(&meta); err != nil {
		return fmt.Errorf("encode source metadata: %w", err)
	}
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("encode source metadata: %w", err)
	}
	data = append(data, '\n')
	temp, err := os.CreateTemp(skillDir, ".x-skills-metadata-*")
	if err != nil {
		return fmt.Errorf("write source metadata: %w", err)
	}
	tempPath := temp.Name()
	defer func() { _ = os.Remove(tempPath) }()
	if err := temp.Chmod(0o644); err != nil {
		_ = temp.Close()
		return fmt.Errorf("write source metadata: %w", err)
	}
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return fmt.Errorf("write source metadata: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("write source metadata: %w", err)
	}
	if err := os.Rename(tempPath, filepath.Join(skillDir, MetadataFile)); err != nil {
		return fmt.Errorf("write source metadata: %w", err)
	}
	return nil
}

func normalizeCompatibility(meta *SourceMetadata) error {
	if meta.Compatibility == nil {
		return nil
	}
	profile := *meta.Compatibility
	profile.Agents = slices.Clone(profile.Agents)
	if profile.Agnostic == (len(profile.Agents) > 0) {
		return errors.New("compatibility must be agnostic or name at least one agent")
	}
	slices.Sort(profile.Agents)
	meta.Compatibility = &profile
	return nil
}

func (m SourceMetadata) SameIdentity(other SourceMetadata) bool {
	if m.SourceType == "" || other.SourceType == "" || m.SourceType != other.SourceType {
		return false
	}
	switch m.SourceType {
	case SourceTypeGitHub:
		return strings.EqualFold(m.Owner, other.Owner) &&
			strings.EqualFold(m.Repo, other.Repo) &&
			cleanSkillPath(m.SkillPath) == cleanSkillPath(other.SkillPath)
	case SourceTypeGit:
		return m.CloneURL == other.CloneURL && cleanSkillPath(m.SkillPath) == cleanSkillPath(other.SkillPath)
	default:
		return false
	}
}

func cleanSkillPath(path string) string {
	return strings.Trim(strings.ReplaceAll(path, `\`, `/`), `/`)
}
