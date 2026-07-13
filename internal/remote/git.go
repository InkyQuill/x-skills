package remote

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"

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
	mu        sync.Mutex
	checkouts map[checkoutKey]Checkout
	inflight  map[checkoutKey]*checkoutCall
}

type checkoutKey struct {
	cloneURL string
	ref      string
}

type checkoutCall struct {
	done     chan struct{}
	checkout Checkout
	err      error
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

type MissingSkillError struct {
	Name          string
	PreferredPath string
	RepoURL       string
	Err           error
}

type AmbiguousSkillError struct {
	Name  string
	Paths []string
}

func (e *AmbiguousSkillError) Error() string {
	return fmt.Sprintf("ambiguous skill %q: %s", e.Name, strings.Join(e.Paths, ", "))
}

func (e *MissingSkillError) Error() string {
	if e.PreferredPath == "" {
		if e.Name == "" {
			return "skill not found in repo"
		}
		return fmt.Sprintf("skill %q not found in repo", e.Name)
	}
	if e.Name == "" {
		return fmt.Sprintf("skill not found at %q in repo", e.PreferredPath)
	}
	return fmt.Sprintf("skill %q not found at %q in repo", e.Name, e.PreferredPath)
}

func (e *MissingSkillError) Unwrap() error {
	return e.Err
}

func NewCheckoutCache(root string) *CheckoutCache {
	return &CheckoutCache{
		root:      root,
		checkouts: map[checkoutKey]Checkout{},
		inflight:  map[checkoutKey]*checkoutCall{},
	}
}

func (c *CheckoutCache) Checkout(ctx context.Context, source GitSource) (Checkout, error) {
	key := checkoutKey{cloneURL: source.CloneURL, ref: source.Ref}
	c.mu.Lock()
	if checkout, ok := c.checkouts[key]; ok {
		c.mu.Unlock()
		checkout.Source = source
		return checkout, nil
	}
	if call, ok := c.inflight[key]; ok {
		c.mu.Unlock()
		select {
		case <-call.done:
			if call.err != nil {
				return Checkout{}, call.err
			}
			checkout := call.checkout
			checkout.Source = source
			return checkout, nil
		case <-ctx.Done():
			return Checkout{}, ctx.Err()
		}
	}
	call := &checkoutCall{done: make(chan struct{})}
	c.inflight[key] = call
	c.mu.Unlock()

	checkout, err := c.checkout(ctx, source)
	c.mu.Lock()
	if err == nil {
		c.checkouts[key] = checkout
	}
	call.checkout = checkout
	call.err = err
	delete(c.inflight, key)
	close(call.done)
	c.mu.Unlock()
	return checkout, err
}

func (c *CheckoutCache) checkout(ctx context.Context, source GitSource) (Checkout, error) {
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
	args = append(args, "--", source.CloneURL, dir)
	if err := runGitCommand(ctx, "", args...); err != nil {
		return Checkout{}, cleanupCheckoutDir(dir, err)
	}
	commit, err := gitCommandOutput(ctx, dir, "rev-parse", "HEAD")
	if err != nil {
		return Checkout{}, cleanupCheckoutDir(dir, err)
	}
	checkout := Checkout{Path: dir, Source: source, Commit: strings.TrimSpace(commit)}
	return checkout, nil
}

func (c Checkout) FindSkill(name, preferredPath string) (FoundSkill, error) {
	return c.FindSkillContext(context.Background(), name, preferredPath)
}

func (c Checkout) ListSkillsContext(ctx context.Context) ([]FoundSkill, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var found []FoundSkill
	err := filepath.WalkDir(c.Path, func(path string, entry os.DirEntry, err error) error {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}
		if !skills.IsDir(path) {
			return nil
		}
		rel, err := filepath.Rel(c.Path, path)
		if err != nil {
			return err
		}
		skill, err := c.foundAt(path, filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		found = append(found, skill)
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return filepath.SkipDir
	})
	if err != nil {
		return nil, fmt.Errorf("list skills: %w", err)
	}
	sort.Slice(found, func(i, j int) bool {
		if found[i].Info.Name != found[j].Info.Name {
			return found[i].Info.Name < found[j].Info.Name
		}
		return found[i].Metadata.SkillPath < found[j].Metadata.SkillPath
	})
	return found, nil
}

func (c Checkout) FindSkillContext(ctx context.Context, name, preferredPath string) (FoundSkill, error) {
	if err := ctx.Err(); err != nil {
		return FoundSkill{}, err
	}
	if preferredPath != "" {
		skillDir, rel, err := c.resolvePreferredSkillPath(preferredPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				missing := &MissingSkillError{
					Name:          name,
					PreferredPath: preferredPath,
					RepoURL:       c.Source.CloneURL,
					Err:           err,
				}
				return c.findSkillByRepoSearchOrErr(ctx, name, missing)
			}
			return FoundSkill{}, err
		}
		if err := ctx.Err(); err != nil {
			return FoundSkill{}, err
		}
		found, err := c.foundAt(skillDir, rel)
		if err != nil {
			return FoundSkill{}, err
		}
		if name != "" && found.Info.Name != name && filepath.Base(filepath.FromSlash(rel)) != name {
			missing := &MissingSkillError{
				Name:          name,
				PreferredPath: preferredPath,
				RepoURL:       c.Source.CloneURL,
				Err: fmt.Errorf(
					"found skill %q at %q",
					found.Info.Name,
					preferredPath,
				),
			}
			return c.findSkillByRepoSearchOrErr(ctx, name, missing)
		}
		return found, nil
	}
	return c.findSkillByRepoSearch(ctx, name)
}

func (c Checkout) findSkillByRepoSearchOrErr(ctx context.Context, name string, fallbackErr error) (FoundSkill, error) {
	if name == "" {
		return FoundSkill{}, fallbackErr
	}
	found, err := c.findSkillByRepoSearch(ctx, name)
	if err != nil {
		var missing *MissingSkillError
		if errors.As(err, &missing) {
			return FoundSkill{}, fallbackErr
		}
		return FoundSkill{}, err
	}
	return found, nil
}

func (c Checkout) findSkillByRepoSearch(ctx context.Context, name string) (FoundSkill, error) {
	var matches []string
	err := filepath.WalkDir(c.Path, func(path string, entry os.DirEntry, err error) error {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
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
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		return FoundSkill{}, fmt.Errorf("find skill: %w", err)
	}
	if len(matches) == 0 {
		return FoundSkill{}, &MissingSkillError{Name: name, RepoURL: c.Source.CloneURL}
	}
	if len(matches) > 1 {
		return FoundSkill{}, &AmbiguousSkillError{
			Name:  name,
			Paths: append([]string{}, matches...),
		}
	}
	return c.foundAt(filepath.Join(c.Path, filepath.FromSlash(matches[0])), matches[0])
}

func (c Checkout) resolvePreferredSkillPath(preferredPath string) (string, string, error) {
	normalized := strings.ReplaceAll(preferredPath, `\`, `/`)
	cleanRel := path.Clean(normalized)
	if path.IsAbs(normalized) || filepath.IsAbs(preferredPath) || cleanRel == "." || hasParentTraversal(normalized) {
		return "", "", fmt.Errorf("invalid skill path %q", preferredPath)
	}

	root, err := filepath.Abs(c.Path)
	if err != nil {
		return "", "", fmt.Errorf("resolve checkout root: %w", err)
	}
	target, err := filepath.Abs(filepath.Join(root, filepath.FromSlash(cleanRel)))
	if err != nil {
		return "", "", fmt.Errorf("resolve skill path: %w", err)
	}
	relToRoot, err := filepath.Rel(root, target)
	if err != nil {
		return "", "", fmt.Errorf("check skill path: %w", err)
	}
	if relToRoot == ".." ||
		strings.HasPrefix(relToRoot, ".."+string(os.PathSeparator)) ||
		filepath.IsAbs(relToRoot) {
		return "", "", fmt.Errorf("invalid skill path %q", preferredPath)
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", "", fmt.Errorf("resolve checkout root: %w", err)
	}
	resolvedTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		return "", "", err
	}
	relToResolvedRoot, err := filepath.Rel(resolvedRoot, resolvedTarget)
	if err != nil {
		return "", "", fmt.Errorf("check skill path: %w", err)
	}
	if relToResolvedRoot == ".." ||
		strings.HasPrefix(relToResolvedRoot, ".."+string(os.PathSeparator)) ||
		filepath.IsAbs(relToResolvedRoot) {
		return "", "", fmt.Errorf("invalid skill path %q", preferredPath)
	}
	return target, cleanRel, nil
}

func hasParentTraversal(path string) bool {
	for _, part := range strings.Split(path, "/") {
		if part == ".." {
			return true
		}
	}
	return false
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

func cleanupCheckoutDir(dir string, err error) error {
	if cleanupErr := os.RemoveAll(dir); cleanupErr != nil {
		return errors.Join(err, fmt.Errorf("cleanup checkout dir: %w", cleanupErr))
	}
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
