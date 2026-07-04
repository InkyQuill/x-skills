package doctor

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/repo"
	"github.com/InkyQuill/x-skills/internal/roots"
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

	var results []FixResult
	for _, issue := range issues {
		if issue.Kind != KindBrokenSymlink {
			continue
		}

		result, err := fixBrokenSymlink(issue)
		if err != nil {
			return nil, err
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

		if _, err := filepath.EvalSymlinks(activePath); err != nil {
			issues = append(issues, brokenSymlinkIssue(cfg, root, entry.Name(), activePath, err))
		}
	}

	return issues, nil
}

func brokenSymlinkIssue(
	cfg config.Config,
	root roots.ActiveRoot,
	name string,
	path string,
	cause error,
) Issue {
	issue := Issue{
		Kind:     KindBrokenSymlink,
		Name:     name,
		Location: root.Label,
		Path:     path,
		Reason:   fmt.Sprintf("resolve symlink: %v", cause),
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

	if issue.RepoTarget != "" {
		if err := os.Remove(issue.Path); err != nil {
			return FixResult{}, fmt.Errorf("remove broken symlink %q: %w", issue.Path, err)
		}
		if err := os.Symlink(issue.RepoTarget, issue.Path); err != nil {
			return FixResult{}, fmt.Errorf("relink %q to %q: %w", issue.Name, issue.Path, err)
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
