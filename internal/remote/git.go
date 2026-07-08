package remote

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/InkyQuill/x-skills/internal/skills"
)

type GitSource struct {
	CloneURL string
	Ref      string
	Owner    string
	Repo     string
}

type CheckoutCache struct {
	root      string
	checkouts map[string]Checkout
}

type Checkout struct {
	Path   string
	Source GitSource
	Commit string
}

type FoundSkill struct {
	SkillDir string
	Info     skills.Info
	Metadata SourceMetadata
}

func NewCheckoutCache(root string) *CheckoutCache {
	return &CheckoutCache{root: root, checkouts: map[string]Checkout{}}
}

func (c *CheckoutCache) Checkout(ctx context.Context, source GitSource) (Checkout, error) {
	key := source.CloneURL + "@" + source.Ref
	if checkout, ok := c.checkouts[key]; ok {
		return checkout, nil
	}
	if err := os.MkdirAll(c.root, 0o755); err != nil {
		return Checkout{}, fmt.Errorf("create checkout cache: %w", err)
	}
	dir, err := os.MkdirTemp(c.root, "repo-*")
	if err != nil {
		return Checkout{}, fmt.Errorf("create checkout dir: %w", err)
	}
	args := []string{"clone", "--depth", "1"}
	if source.Ref != "" {
		args = append(args, "--branch", source.Ref)
	}
	args = append(args, source.CloneURL, dir)
	if err := runGitCommand(ctx, "", args...); err != nil {
		return Checkout{}, err
	}
	commit, err := gitCommandOutput(ctx, dir, "rev-parse", "HEAD")
	if err != nil {
		return Checkout{}, err
	}
	checkout := Checkout{Path: dir, Source: source, Commit: strings.TrimSpace(commit)}
	c.checkouts[key] = checkout
	return checkout, nil
}

func (c Checkout) FindSkill(name, preferredPath string) (FoundSkill, error) {
	if preferredPath != "" {
		return c.foundAt(filepath.Join(c.Path, filepath.FromSlash(preferredPath)), preferredPath)
	}
	var matches []string
	err := filepath.WalkDir(c.Path, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}
		if skills.IsDir(path) {
			info, err := skills.Read(path)
			if err == nil && (info.Name == name || filepath.Base(path) == name) {
				rel, _ := filepath.Rel(c.Path, path)
				matches = append(matches, filepath.ToSlash(rel))
			}
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		return FoundSkill{}, fmt.Errorf("find skill: %w", err)
	}
	if len(matches) == 0 {
		return FoundSkill{}, fmt.Errorf("skill %q not found in checkout", name)
	}
	if len(matches) > 1 {
		return FoundSkill{}, fmt.Errorf("ambiguous skill %q: %s", name, strings.Join(matches, ", "))
	}
	return c.foundAt(filepath.Join(c.Path, filepath.FromSlash(matches[0])), matches[0])
}

func (c Checkout) foundAt(path, rel string) (FoundSkill, error) {
	info, err := skills.Read(path)
	if err != nil {
		return FoundSkill{}, err
	}
	meta := SourceMetadata{
		SourceType:   SourceTypeGit,
		Owner:        c.Source.Owner,
		Repo:         c.Source.Repo,
		CloneURL:     c.Source.CloneURL,
		Ref:          c.Source.Ref,
		Commit:       c.Commit,
		SkillPath:    filepath.ToSlash(rel),
		UpstreamName: info.Name,
	}
	if c.Source.Owner != "" && c.Source.Repo != "" {
		meta.SourceType = SourceTypeGitHub
	}
	return FoundSkill{SkillDir: path, Info: info, Metadata: meta}, nil
}

func runGitCommand(ctx context.Context, dir string, args ...string) error {
	_, err := gitCommandOutput(ctx, dir, args...)
	return err
}

func gitCommandOutput(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %v failed: %w\n%s", args, err, out)
	}
	return string(out), nil
}
