package validation

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/InkyQuill/x-skills/internal/remote"
)

func validateSource(skillDir string, consumers []string, checkConsumers bool) []Diagnostic {
	metadata, exists, err := remote.ReadSourceMetadata(skillDir)
	if err != nil {
		code := CodeMetadataInvalid
		field := ""
		var metadataErr *remote.MetadataError
		if errors.As(err, &metadataErr) {
			code = metadataErr.Code
			field = metadataErr.Field
		}
		return []Diagnostic{{
			Path:        skillDir,
			Level:       LevelError,
			Code:        code,
			Message:     err.Error(),
			Field:       field,
			RelatedPath: filepath.Join(skillDir, remote.MetadataFile),
		}}
	}
	if !exists || !checkConsumers || metadata.Compatibility == nil || metadata.Compatibility.Agnostic {
		return nil
	}

	unknown := []string{}
	for _, agent := range metadata.Compatibility.Agents {
		if !slices.Contains(consumers, agent) {
			unknown = append(unknown, agent)
		}
	}
	if len(unknown) == 0 {
		return nil
	}
	return []Diagnostic{{
		Path:        skillDir,
		Level:       LevelError,
		Code:        CodeCompatibilityConsumer,
		Message:     fmt.Sprintf("declared agents are not consumers of the selected roots: %q", unknown),
		Field:       "compatibility.agents",
		RelatedPath: filepath.Join(skillDir, remote.MetadataFile),
	}}
}
