package remote

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const MetadataFile = ".x-skills.json"

const (
	SourceTypeGitHub = "github"
	SourceTypeGit    = "git"
)

type SourceMetadata struct {
	SourceType   string `json:"source_type"`
	Owner        string `json:"owner,omitempty"`
	Repo         string `json:"repo,omitempty"`
	CloneURL     string `json:"clone_url"`
	Ref          string `json:"ref,omitempty"`
	Commit       string `json:"commit"`
	SkillPath    string `json:"skill_path"`
	UpstreamName string `json:"upstream_name,omitempty"`
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
	return meta, true, nil
}

func WriteSourceMetadata(skillDir string, meta SourceMetadata) error {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("encode source metadata: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(filepath.Join(skillDir, MetadataFile), data, 0o644); err != nil {
		return fmt.Errorf("write source metadata: %w", err)
	}
	return nil
}

func (m SourceMetadata) SameIdentity(other SourceMetadata) bool {
	if m.SourceType == "" || other.SourceType == "" || m.SourceType != other.SourceType {
		return false
	}
	if m.SourceType == SourceTypeGitHub {
		return strings.EqualFold(m.Owner, other.Owner) &&
			strings.EqualFold(m.Repo, other.Repo) &&
			cleanSkillPath(m.SkillPath) == cleanSkillPath(other.SkillPath)
	}
	return m.CloneURL == other.CloneURL && cleanSkillPath(m.SkillPath) == cleanSkillPath(other.SkillPath)
}

func cleanSkillPath(path string) string {
	return strings.Trim(strings.ReplaceAll(path, `\`, `/`), `/`)
}
