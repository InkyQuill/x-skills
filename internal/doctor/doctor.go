package doctor

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/InkyQuill/x-skills/internal/actions"
	"github.com/InkyQuill/x-skills/internal/builtin"
	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/repo"
	"github.com/InkyQuill/x-skills/internal/roots"
	"github.com/InkyQuill/x-skills/internal/skills"
	"github.com/InkyQuill/x-skills/internal/symlinkcheck"
)

const (
	KindBrokenSymlink   = "broken-symlink"
	KindMissingBuiltIn  = "missing-builtin"
	KindInactiveBuiltIn = "inactive-builtin"

	KindRecommendedManifestUntracked = "recommended-manifest-untracked"
	KindLocalManifestTracked         = "local-manifest-tracked"
	KindSkillsFolderTracked          = "skills-folder-tracked"
)

type Filter struct {
	Scope  string
	Target string
}

type Issue struct {
	Kind        string
	Name        string
	Location    string
	Path        string
	Reason      string
	SafeFix     string
	RepoTarget  string
	ProjectRoot string
}

type FixOptions struct {
	Yes                 bool
	Filter              Filter
	BuiltInDestinations []roots.ActiveRoot
	ArchiveOnlyBuiltIns bool
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
	builtInIssues, err := diagnoseBuiltIns(cfg, filter)
	if err != nil {
		return nil, err
	}
	issues = append(issues, builtInIssues...)
	if filter.Scope == "" || filter.Scope == config.ScopeProject {
		gitIssues, err := diagnoseGitHygiene(cfg)
		if err != nil {
			return nil, err
		}
		issues = append(issues, gitIssues...)
	}

	return issues, nil
}

func diagnoseBuiltIns(cfg config.Config, filter Filter) ([]Issue, error) {
	if filter.Scope == config.ScopeProject {
		return nil, nil
	}
	catalog, err := builtin.List()
	if err != nil {
		return nil, err
	}
	globalRoots := roots.ActiveRoots(cfg, roots.Filter{Scope: config.ScopeGlobal, Target: filter.Target})
	issues := make([]Issue, 0, len(catalog))
	for _, skill := range catalog {
		archivePath := filepath.Join(cfg.ArchiveSkillsRoot(), skill.Name)
		if !skills.IsDir(archivePath) {
			issues = append(issues, Issue{Kind: KindMissingBuiltIn, Name: skill.Name, Location: "repo", Path: archivePath, Reason: "built-in skill is missing from the archive", SafeFix: "archive"})
			continue
		}
		if !builtInActiveGlobally(archivePath, skill.Name, globalRoots) {
			issues = append(issues, Issue{Kind: KindInactiveBuiltIn, Name: skill.Name, Location: "global", Path: archivePath, Reason: "built-in skill is archived but inactive in global Skills Folders", SafeFix: "link"})
		}
	}
	return issues, nil
}

func builtInActiveGlobally(archivePath, name string, globalRoots []roots.ActiveRoot) bool {
	resolvedArchive, err := filepath.EvalSymlinks(archivePath)
	if err != nil {
		return false
	}
	for _, root := range globalRoots {
		resolved, err := filepath.EvalSymlinks(filepath.Join(root.Path, name))
		if err == nil && resolved == resolvedArchive {
			return true
		}
	}
	return false
}

func Fix(cfg config.Config, opts FixOptions) ([]FixResult, error) {
	if !opts.Yes {
		return nil, fmt.Errorf("doctor fix requires confirmation; rerun with -y")
	}
	if err := ValidateBuiltInDestinations(opts.BuiltInDestinations); err != nil {
		return nil, err
	}

	issues, err := Diagnose(cfg, opts.Filter)
	if err != nil {
		return nil, err
	}

	results, err := FixIssues(issues)
	if err != nil {
		return results, err
	}
	builtInResults, err := FixBuiltIns(cfg, issues, opts)
	return append(results, builtInResults...), err
}

func FixBuiltIns(cfg config.Config, issues []Issue, opts FixOptions) ([]FixResult, error) {
	if err := ValidateBuiltInDestinations(opts.BuiltInDestinations); err != nil {
		return nil, err
	}
	if !opts.ArchiveOnlyBuiltIns && len(opts.BuiltInDestinations) == 0 {
		return nil, nil
	}
	var results []FixResult
	for _, issue := range issues {
		if issue.Kind != KindMissingBuiltIn && issue.Kind != KindInactiveBuiltIn {
			continue
		}
		if issue.Kind == KindMissingBuiltIn {
			if _, archiveErr := builtin.Archive(cfg, []string{issue.Name}); archiveErr != nil {
				return results, archiveErr
			}
			if opts.ArchiveOnlyBuiltIns {
				results = append(results, FixResult{Name: issue.Name, Action: "archived but inactive", Path: issue.Path})
			} else {
				results = append(results, FixResult{Name: issue.Name, Action: "archived", Path: issue.Path})
			}
		}
		if issue.Kind == KindInactiveBuiltIn && opts.ArchiveOnlyBuiltIns {
			results = append(results, FixResult{Name: issue.Name, Action: "archived but inactive", Path: issue.Path})
		}
		for _, destination := range opts.BuiltInDestinations {
			linked, linkErr := actions.Link(cfg, actions.LinkRequest{Name: issue.Name, Scope: destination.Scope, Target: destination.Target})
			if linkErr != nil {
				return results, linkErr
			}
			results = append(results, FixResult{Name: issue.Name, Action: "archived and linked", Path: linked.Path})
		}
	}
	return results, nil
}

func ValidateBuiltInDestinations(destinations []roots.ActiveRoot) error {
	for _, destination := range destinations {
		if destination.Scope != config.ScopeGlobal {
			return fmt.Errorf("built-in fixes require global Skills Folder destinations; got %s:%s", destination.Scope, destination.Target)
		}
	}
	return nil
}

func FixIssues(issues []Issue) ([]FixResult, error) {
	var results []FixResult
	for _, issue := range issues {
		switch issue.Kind {
		case KindBrokenSymlink:
			result, err := fixBrokenSymlink(issue)
			if err != nil {
				return results, err
			}
			results = append(results, result)
		case KindRecommendedManifestUntracked:
			results = append(results, FixResult{Name: issue.Name, Action: "run: " + issue.SafeFix, Path: issue.Path})
		case KindLocalManifestTracked, KindSkillsFolderTracked:
			entry, err := gitignoreEntry(issue)
			if err != nil {
				return results, err
			}
			if err := appendGitignoreEntry(issue.ProjectRoot, entry); err != nil {
				return results, err
			}
			results = append(results, FixResult{Name: issue.Name, Action: "ignored; run: " + issue.SafeFix, Path: issue.Path})
		}
	}

	return results, nil
}

func diagnoseGitHygiene(cfg config.Config) ([]Issue, error) {
	inside, err := gitInsideWorkTree(cfg.ProjectRoot)
	if err != nil || !inside {
		return nil, err
	}

	var issues []Issue
	recommended := filepath.Join(cfg.ProjectRoot, ".x-skills.yaml")
	if _, err := os.Stat(recommended); err == nil {
		tracked, trackErr := gitPathTracked(cfg.ProjectRoot, ".x-skills.yaml")
		if trackErr != nil {
			return nil, trackErr
		}
		if !tracked {
			ignored, ignoreErr := gitPathIgnored(cfg.ProjectRoot, ".x-skills.yaml")
			if ignoreErr != nil {
				return nil, ignoreErr
			}
			addFlag := ""
			reason := "recommended manifest exists but is not tracked by Git"
			if ignored {
				addFlag = " -f"
				reason = "recommended manifest exists but is ignored and not tracked by Git"
			}
			issues = append(issues, Issue{Kind: KindRecommendedManifestUntracked, Name: ".x-skills.yaml", Location: "project", Path: recommended, Reason: reason, SafeFix: "git add" + addFlag + " -- " + shellQuote(".x-skills.yaml"), ProjectRoot: cfg.ProjectRoot})
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("inspect recommended manifest: %w", err)
	}

	local := filepath.Join(cfg.ProjectRoot, ".x-skills.local.yaml")
	tracked, err := gitPathTracked(cfg.ProjectRoot, ".x-skills.local.yaml")
	if err != nil {
		return nil, err
	}
	if tracked {
		issues = append(issues, Issue{Kind: KindLocalManifestTracked, Name: ".x-skills.local.yaml", Location: "project", Path: local, Reason: "local manifest is tracked by Git", SafeFix: "git rm --cached -- " + shellQuote(".x-skills.local.yaml"), ProjectRoot: cfg.ProjectRoot})
	}

	for _, root := range roots.ActiveRoots(cfg, roots.Filter{Scope: config.ScopeProject}) {
		rel, err := filepath.Rel(cfg.ProjectRoot, root.Path)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		rel = filepath.ToSlash(rel)
		tracked, err := gitFolderTracked(cfg.ProjectRoot, rel)
		if err != nil {
			return nil, err
		}
		if tracked {
			issues = append(issues, Issue{Kind: KindSkillsFolderTracked, Name: root.Label, Location: root.Label, Path: root.Path, Reason: "configured project Skills Folder contains files tracked by Git", SafeFix: "git rm -r --cached -- " + shellQuote(rel), ProjectRoot: cfg.ProjectRoot})
		}
	}
	return issues, nil
}

func gitInsideWorkTree(projectRoot string) (bool, error) {
	cmd := exec.CommandContext(context.Background(), "git", "-C", projectRoot, "rev-parse", "--is-inside-work-tree")
	out, err := cmd.CombinedOutput()
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok && !hasGitMarker(projectRoot) {
			return false, nil
		}
		return false, fmt.Errorf("inspect Git work tree: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)) == "true", nil
}

func hasGitMarker(path string) bool {
	for {
		if _, err := os.Lstat(filepath.Join(path, ".git")); err == nil {
			return true
		}
		parent := filepath.Dir(path)
		if parent == path {
			return false
		}
		path = parent
	}
}

func gitPathIgnored(projectRoot, path string) (bool, error) {
	cmd := exec.CommandContext(context.Background(), "git", "-C", projectRoot, "check-ignore", "--quiet", "--", path)
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return false, nil
		}
		return false, fmt.Errorf("inspect ignored path %q: %w", path, err)
	}
	return true, nil
}

func gitPathTracked(projectRoot, path string) (bool, error) {
	cmd := exec.CommandContext(context.Background(), "git", "-C", projectRoot, "ls-files", "--error-unmatch", "--", path)
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return false, nil
		}
		return false, fmt.Errorf("inspect tracked path %q: %w", path, err)
	}
	return true, nil
}

func gitFolderTracked(projectRoot, path string) (bool, error) {
	cmd := exec.CommandContext(context.Background(), "git", "-C", projectRoot, "ls-files", "--", ":(literal)"+path)
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("inspect tracked Skills Folder %q: %w", path, err)
	}
	return len(strings.TrimSpace(string(out))) > 0, nil
}

func gitignoreEntry(issue Issue) (string, error) {
	if issue.Kind == KindLocalManifestTracked {
		return ".x-skills.local.yaml", nil
	}
	rel, err := filepath.Rel(issue.ProjectRoot, issue.Path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("skills folder is outside project root: %s", issue.Path)
	}
	return literalGitignorePattern(strings.TrimSuffix(filepath.ToSlash(rel), "/"))
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func literalGitignorePattern(path string) (string, error) {
	if strings.ContainsAny(path, "\r\n") {
		return "", fmt.Errorf("cannot represent path containing a newline in .gitignore: %q", path)
	}
	var pattern strings.Builder
	pattern.WriteByte('/')
	for _, char := range path {
		switch char {
		case '\\', '*', '?', '[', ']', '#', '!', ' ':
			pattern.WriteByte('\\')
		}
		pattern.WriteRune(char)
	}
	pattern.WriteByte('/')
	return pattern.String(), nil
}

func appendGitignoreEntry(projectRoot, entry string) (err error) {
	path := filepath.Join(projectRoot, ".gitignore")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open .gitignore: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close .gitignore: %w", closeErr))
		}
	}()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) == entry {
			return nil
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read .gitignore: %w", err)
	}
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("stat .gitignore: %w", err)
	}
	prefix := ""
	if info.Size() > 0 {
		last := []byte{0}
		if _, err := file.ReadAt(last, info.Size()-1); err != nil {
			return fmt.Errorf("inspect .gitignore ending: %w", err)
		}
		if last[0] != '\n' {
			prefix = "\n"
		}
	}
	if _, err := file.WriteString(prefix + entry + "\n"); err != nil {
		return fmt.Errorf("append .gitignore: %w", err)
	}
	return nil
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
	result := symlinkcheck.ValidateSkillTarget(path)
	return result.Reason, result.Broken
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
		issue.RepoTarget, _ = repo.SkillPath(cfg, name)
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
