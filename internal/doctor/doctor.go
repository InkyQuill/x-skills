package doctor

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/repo"
	"github.com/InkyQuill/x-skills/internal/roots"
	"github.com/InkyQuill/x-skills/internal/skills"
)

const KindBrokenSymlink = "broken-symlink"

type Filter struct {
	Scope  string
	Target string
}

type Issue struct {
	Kind       string
	Name       string
	Location   string
	Path       string
	Reason     string
	SafeFix    string
	RepoTarget string
}

type FixOptions struct {
	Yes    bool
	Filter Filter
}

type FixResult struct {
	Name   string
	Action string
	Path   string
}

func Diagnose(cfg config.Config, filter Filter) ([]Issue, error) {
	activeRoots := roots.ActiveRoots(cfg, roots.Filter{
		Scope:  filter.Scope,
		Target: filter.Target,
	})

	var issues []Issue
	for _, root := range activeRoots {
		rootIssues, err := diagnoseRoot(cfg, root)
		if err != nil {
			return nil, err
		}
		issues = append(issues, rootIssues...)
	}

	return issues, nil
}

func Fix(cfg config.Config, opts FixOptions) ([]FixResult, error) {
	if !opts.Yes {
		return nil, fmt.Errorf("doctor fix requires confirmation; rerun with -y")
	}

	issues, err := Diagnose(cfg, opts.Filter)
	if err != nil {
		return nil, err
	}

	return FixIssues(issues)
}

func FixIssues(issues []Issue) ([]FixResult, error) {
	var results []FixResult
	for _, issue := range issues {
		if issue.Kind != KindBrokenSymlink {
			continue
		}

		result, err := fixBrokenSymlink(issue)
		if err != nil {
			return results, err
		}
		results = append(results, result)
	}

	return results, nil
}

func diagnoseRoot(cfg config.Config, root roots.ActiveRoot) ([]Issue, error) {
	entries, err := os.ReadDir(root.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("diagnose active root %q: %w", root.Path, err)
	}

	var issues []Issue
	for _, entry := range entries {
		activePath := filepath.Join(root.Path, entry.Name())
		info, err := os.Lstat(activePath)
		if err != nil {
			return nil, fmt.Errorf("inspect active skill %q: %w", activePath, err)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			continue
		}

		if reason, broken := classifyBrokenSymlink(activePath); broken {
			issues = append(issues, brokenSymlinkIssue(cfg, root, entry.Name(), activePath, reason))
		}
	}

	return issues, nil
}

func classifyBrokenSymlink(path string) (string, bool) {
	resolvedPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return fmt.Sprintf("resolve symlink: %v", err), true
	}

	info, err := os.Stat(resolvedPath)
	if err != nil {
		return fmt.Sprintf("stat target: %v", err), true
	}
	if !info.IsDir() {
		return "target is not a directory", true
	}
	if !skills.IsDir(resolvedPath) {
		return "target is not a skill directory", true
	}

	return "", false
}

func brokenSymlinkIssue(
	cfg config.Config,
	root roots.ActiveRoot,
	name string,
	path string,
	reason string,
) Issue {
	issue := Issue{
		Kind:     KindBrokenSymlink,
		Name:     name,
		Location: root.Label,
		Path:     path,
		Reason:   reason,
		SafeFix:  "remove",
	}
	if repo.HasSkill(cfg, name) {
		issue.SafeFix = "relink"
		issue.RepoTarget = repo.SkillPath(cfg, name)
	}
	return issue
}

func fixBrokenSymlink(issue Issue) (FixResult, error) {
	if err := ensureSymlink(issue.Path); err != nil {
		return FixResult{}, err
	}
	if _, broken := classifyBrokenSymlink(issue.Path); !broken {
		return FixResult{}, fmt.Errorf("refusing to fix %q because it is no longer broken", issue.Path)
	}

	if issue.RepoTarget != "" {
		if !skills.IsDir(issue.RepoTarget) {
			return FixResult{}, fmt.Errorf("repo target is no longer a skill directory: %s", issue.RepoTarget)
		}
		if err := replaceSymlink(issue.Path, issue.RepoTarget); err != nil {
			return FixResult{}, fmt.Errorf("relink %q to %q: %w", issue.Name, issue.RepoTarget, err)
		}
		return FixResult{Name: issue.Name, Action: "relinked", Path: issue.Path}, nil
	}

	if err := os.Remove(issue.Path); err != nil {
		return FixResult{}, fmt.Errorf("remove broken symlink %q: %w", issue.Path, err)
	}
	return FixResult{Name: issue.Name, Action: "removed", Path: issue.Path}, nil
}

func ensureSymlink(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspect symlink %q: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return fmt.Errorf("refusing to mutate non-symlink %q", path)
	}
	return nil
}

func replaceSymlink(path, target string) error {
	tempPath, err := createTempSymlink(path, target)
	if err != nil {
		return err
	}
	defer func() {
		_ = os.Remove(tempPath)
	}()

	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace symlink: %w", err)
	}
	return nil
}

func createTempSymlink(path, target string) (string, error) {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	for i := 0; i < 100; i++ {
		tempPath := filepath.Join(dir, fmt.Sprintf(".%s.tmp.%d.%d", base, os.Getpid(), i))
		if err := os.Symlink(target, tempPath); err != nil {
			if os.IsExist(err) {
				continue
			}
			return "", fmt.Errorf("create replacement symlink %q: %w", tempPath, err)
		}
		return tempPath, nil
	}
	return "", fmt.Errorf("create replacement symlink for %q: too many temporary path collisions", path)
}
