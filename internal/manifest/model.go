package manifest

import "github.com/InkyQuill/x-skills/internal/remote"

const (
	RecommendedFilename = ".x-skills.yaml"
	LocalFilename       = ".x-skills.local.yaml"

	SourceGitHub  = "github"
	SourceGit     = "git"
	SourceArchive = "archive"
)

type Manifest struct {
	Version int     `yaml:"version"`
	Skills  []Skill `yaml:"skills"`
}

type Skill struct {
	Name          string                       `yaml:"name"`
	Source        Source                       `yaml:"source"`
	Compatibility *remote.CompatibilityProfile `yaml:"compatibility,omitempty"`
	Fingerprint   string                       `yaml:"fingerprint,omitempty"`
}

type Source struct {
	Type       string `yaml:"type"`
	Repository string `yaml:"repository,omitempty"`
	Path       string `yaml:"path,omitempty"`
	Ref        string `yaml:"ref,omitempty"`
}
