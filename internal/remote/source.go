package remote

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
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

var agentIDPattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

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

type MetadataError struct {
	Code  string
	Field string
	Err   error
}

func (e *MetadataError) Error() string {
	return e.Err.Error()
}

func (e *MetadataError) Unwrap() error {
	return e.Err
}

func DecodeSourceMetadata(data []byte) (SourceMetadata, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	var metadata SourceMetadata
	if err := decoder.Decode(&metadata); err != nil {
		field, isUnknown := unknownJSONField(err)
		if isUnknown {
			return SourceMetadata{}, metadataError("metadata.unknown_field", field, err)
		}
		return SourceMetadata{}, fmt.Errorf("decode source metadata: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("decode source metadata: multiple JSON values")
		} else {
			err = fmt.Errorf("decode source metadata trailer: %w", err)
		}
		return SourceMetadata{}, metadataError("metadata.trailing_json", "", err)
	}

	if metadata.SchemaVersion == 0 {
		metadata.SchemaVersion = metadataSchemaV1
	}
	if err := ValidateSourceMetadata(metadata); err != nil {
		return SourceMetadata{}, err
	}
	if err := normalizeCompatibility(&metadata); err != nil {
		return SourceMetadata{}, err
	}
	return metadata, nil
}

func ValidateSourceMetadata(metadata SourceMetadata) error {
	schemaVersion := metadata.SchemaVersion
	if schemaVersion == 0 {
		schemaVersion = metadataSchemaV1
	}
	if schemaVersion != metadataSchemaV1 && schemaVersion != metadataSchemaV2 {
		return metadataError(
			"metadata.schema",
			"schema_version",
			fmt.Errorf("unsupported source metadata schema version %d", schemaVersion),
		)
	}
	if err := validateSourceIdentity(metadata); err != nil {
		return err
	}
	return validateCompatibility(metadata.Compatibility)
}

func ReadSourceMetadata(skillDir string) (SourceMetadata, bool, error) {
	metadataPath := filepath.Join(skillDir, MetadataFile)
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return SourceMetadata{}, false, nil
		}
		return SourceMetadata{}, false, fmt.Errorf("read source metadata %q: %w", metadataPath, err)
	}
	metadata, err := DecodeSourceMetadata(data)
	if err != nil {
		return SourceMetadata{}, false, fmt.Errorf("read source metadata %q: %w", metadataPath, err)
	}
	return metadata, true, nil
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
	if err := validateCompatibility(meta.Compatibility); err != nil {
		return err
	}
	profile := *meta.Compatibility
	profile.Agents = slices.Clone(profile.Agents)
	slices.Sort(profile.Agents)
	meta.Compatibility = &profile
	return nil
}

func validateSourceIdentity(metadata SourceMetadata) error {
	hasSourceIdentity := metadata.SourceType != "" || metadata.Owner != "" || metadata.Repo != "" ||
		metadata.CloneURL != "" || metadata.Ref != "" || metadata.Commit != "" ||
		metadata.SkillPath != "" || metadata.UpstreamName != ""
	if !hasSourceIdentity {
		return nil
	}
	if metadata.SourceType == "" {
		return sourceFieldRequired("source_type")
	}

	switch metadata.SourceType {
	case SourceTypeGitHub:
		for _, required := range []struct {
			field string
			value string
		}{
			{field: "owner", value: metadata.Owner},
			{field: "repo", value: metadata.Repo},
			{field: "skill_path", value: metadata.SkillPath},
		} {
			if required.value == "" {
				return sourceFieldRequired(required.field)
			}
		}
	case SourceTypeGit:
		if metadata.CloneURL == "" {
			return sourceFieldRequired("clone_url")
		}
		if metadata.SkillPath == "" {
			return sourceFieldRequired("skill_path")
		}
	default:
		return metadataError(
			"metadata.source",
			"source_type",
			fmt.Errorf("unknown source type %q", metadata.SourceType),
		)
	}
	return nil
}

func validateCompatibility(profile *CompatibilityProfile) error {
	if profile == nil {
		return nil
	}
	if profile.Agnostic && len(profile.Agents) > 0 {
		return metadataError(
			"metadata.compatibility",
			"compatibility",
			errors.New("compatibility must specify exactly one of agnostic or agents"),
		)
	}
	if !profile.Agnostic && len(profile.Agents) == 0 {
		return metadataError(
			"metadata.compatibility",
			"compatibility.agents",
			errors.New("compatibility agents must name at least one agent"),
		)
	}

	seen := make(map[string]struct{}, len(profile.Agents))
	for _, agentID := range profile.Agents {
		if !agentIDPattern.MatchString(agentID) {
			return metadataError(
				"metadata.compatibility",
				"compatibility.agents",
				fmt.Errorf("invalid agent id %q", agentID),
			)
		}
		if _, exists := seen[agentID]; exists {
			return metadataError(
				"metadata.compatibility",
				"compatibility.agents",
				fmt.Errorf("duplicate agent id %q", agentID),
			)
		}
		seen[agentID] = struct{}{}
	}
	return nil
}

func sourceFieldRequired(field string) error {
	return metadataError(
		"metadata.source",
		field,
		fmt.Errorf("source metadata field %q is required", field),
	)
}

func metadataError(code, field string, err error) error {
	return &MetadataError{Code: code, Field: field, Err: err}
}

func unknownJSONField(err error) (string, bool) {
	const prefix = "json: unknown field \""
	message := err.Error()
	if !strings.HasPrefix(message, prefix) || !strings.HasSuffix(message, "\"") {
		return "", false
	}
	return strings.TrimSuffix(strings.TrimPrefix(message, prefix), "\""), true
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
