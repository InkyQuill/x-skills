package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/InkyQuill/x-skills/internal/pathidentity"
)

func TestAddArchivesAndLinksDefaultProjectAgents(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	repo := makeAddGitRepo(t)
	writeAddRemoteSkill(t, repo, "skills/alpha-skill", "alpha-skill", "Alpha.")
	addGitCommit(t, repo, "initial")

	var out bytes.Buffer
	err := Execute([]string{
		"--home", home,
		"--project-root", project,
		"add", "--git", repo, "alpha-skill", "-y",
	}, strings.NewReader(""), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "added: alpha-skill") {
		t.Fatalf("output missing added line:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "linked: .Ag") {
		t.Fatalf("output missing linked destination:\n%s", out.String())
	}
	assertAddArchived(t, home, "alpha-skill")
	assertAddLink(t, project, home, ".agents", "alpha-skill")
}

func TestAddDefaultDestinationUsesConfiguredLabel(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	configPath := filepath.Join(home, ".x-skills", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("active_roots:\n  - scope: project\n    target: agents\n    path: .agents/skills\n    label: .Aa\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repo := makeAddGitRepo(t)
	writeAddRemoteSkill(t, repo, "skills/alpha-skill", "alpha-skill", "Alpha.")
	addGitCommit(t, repo, "initial")

	var out bytes.Buffer
	err := Execute([]string{
		"--home", home,
		"--project-root", project,
		"add", "--git", repo, "alpha-skill", "-y",
	}, strings.NewReader(""), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "linked: .Aa") {
		t.Fatalf("output missing configured default label:\n%s", out.String())
	}
	assertAddLink(t, project, home, ".agents", "alpha-skill")
}

func TestAddNoLinkArchivesOnly(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	repo := makeAddGitRepo(t)
	writeAddRemoteSkill(t, repo, "skills/archive-only", "archive-only", "Archive only.")
	addGitCommit(t, repo, "initial")

	var out bytes.Buffer
	err := Execute([]string{
		"--home", home,
		"--project-root", project,
		"add", "--git", repo, "archive-only", "--no-link", "-y",
	}, strings.NewReader(""), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	assertAddArchived(t, home, "archive-only")
	assertAddNoPath(t, filepath.Join(project, ".agents", "skills", "archive-only"))
	if strings.Contains(out.String(), "linked:") {
		t.Fatalf("output should not include linked line:\n%s", out.String())
	}
}

func TestAddRejectsNoLinkWithAt(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	repo := makeAddGitRepo(t)
	writeAddRemoteSkill(t, repo, "skills/archive-only", "archive-only", "Archive only.")
	addGitCommit(t, repo, "initial")

	err := Execute([]string{
		"--home", home,
		"--project-root", project,
		"add", "--git", repo, "archive-only", "--no-link", "--at", ".Ag", "-y",
	}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected --no-link and --at conflict")
	}
	if !strings.Contains(err.Error(), "--no-link") || !strings.Contains(err.Error(), "--at") {
		t.Fatalf("error = %q, want --no-link and --at", err)
	}
}

func TestAddToMultipleDestinations(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	repo := makeAddGitRepo(t)
	writeAddRemoteSkill(t, repo, "skills/multi-target", "multi-target", "Multi.")
	addGitCommit(t, repo, "initial")

	var out bytes.Buffer
	err := Execute([]string{
		"--home", home,
		"--project-root", project,
		"add", "--git", repo, "multi-target", "--at", ".Cl", "--at", "~Cd", "-y",
	}, strings.NewReader(""), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	assertAddArchived(t, home, "multi-target")
	assertAddLink(t, project, home, ".claude", "multi-target")
	assertAddLinkAt(t, filepath.Join(home, ".codex", "skills", "multi-target"), home, "multi-target")
	if !strings.Contains(out.String(), "linked: .Cl, ~Cd") {
		t.Fatalf("output missing destinations:\n%s", out.String())
	}
}

func TestAddUsesConfiguredDestinationLabel(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	configPath := filepath.Join(home, ".x-skills", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("active_roots:\n  - scope: project\n    target: opencode\n    path: .opencode/skills\n    label: .Oc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repo := makeAddGitRepo(t)
	writeAddRemoteSkill(t, repo, "skills/custom-label", "custom-label", "Custom.")
	addGitCommit(t, repo, "initial")

	var out bytes.Buffer
	err := Execute([]string{
		"--home", home,
		"--project-root", project,
		"add", "--git", repo, "custom-label", "--at", ".Oc", "-y",
	}, strings.NewReader(""), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	assertAddArchived(t, home, "custom-label")
	assertAddLink(t, project, home, ".opencode", "custom-label")
	if !strings.Contains(out.String(), "linked: .Oc") {
		t.Fatalf("output missing configured destination label:\n%s", out.String())
	}
	if strings.Contains(out.String(), ".opencode") {
		t.Fatalf("output used path-derived label instead of configured label:\n%s", out.String())
	}
}

func TestAddWithLocalGitSourceSelectsSkillName(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	repo := makeAddGitRepo(t)
	writeAddRemoteSkill(t, repo, "skills/local-source", "local-source", "Local.")
	addGitCommit(t, repo, "initial")

	err := Execute([]string{
		"--home", home,
		"--project-root", project,
		"add", "--git", repo, "local-source", "--no-link", "-y",
	}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	assertAddArchived(t, home, "local-source")
}

func TestAddGitHubShorthandArchivesFromRewrittenLocalRepo(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	gitHome := t.TempDir()
	gitConfig := filepath.Join(gitHome, ".gitconfig")
	t.Setenv("HOME", gitHome)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(gitHome, ".config"))
	t.Setenv("GIT_CONFIG_GLOBAL", gitConfig)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_ALLOW_PROTOCOL", "file")

	repo := makeAddGitRepo(t)
	writeAddRemoteSkill(t, repo, "skills/shorthand-skill", "shorthand-skill", "Shorthand.")
	addGitCommit(t, repo, "initial")
	localRepoURL := "file://" + filepath.ToSlash(repo)
	runAddGit(t, "", "config", "--global", "url."+localRepoURL+".insteadOf", "https://github.com/inky/test-skills.git")

	var out bytes.Buffer
	err := Execute([]string{
		"--home", home,
		"--project-root", project,
		"add", "inky/test-skills@shorthand-skill", "--no-link", "-y",
	}, strings.NewReader(""), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	assertAddArchived(t, home, "shorthand-skill")
	assertAddNoPath(t, filepath.Join(project, ".agents", "skills", "shorthand-skill"))
	if !strings.Contains(out.String(), "added: shorthand-skill") {
		t.Fatalf("output missing added line:\n%s", out.String())
	}
	if strings.Contains(out.String(), "linked:") {
		t.Fatalf("output should not include linked line:\n%s", out.String())
	}
}

func TestAddGitHubTreeURLArchivesPathOnlyFromRewrittenLocalRepo(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	gitHome := t.TempDir()
	gitConfig := filepath.Join(gitHome, ".gitconfig")
	t.Setenv("HOME", gitHome)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(gitHome, ".config"))
	t.Setenv("GIT_CONFIG_GLOBAL", gitConfig)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_ALLOW_PROTOCOL", "file")

	repo := makeAddGitRepo(t)
	runAddGit(t, repo, "checkout", "-b", "main")
	writeAddRemoteSkill(t, repo, "skills/tree-skill", "tree-skill", "Tree.")
	writeAddRemoteSkill(t, repo, "skills/other-skill", "other-skill", "Other.")
	addGitCommit(t, repo, "initial")
	localRepoURL := "file://" + filepath.ToSlash(repo)
	runAddGit(t, "", "config", "--global", "url."+localRepoURL+".insteadOf", "https://github.com/inky/tree-skills.git")

	var out bytes.Buffer
	err := Execute([]string{
		"--home", home,
		"--project-root", project,
		"add", "https://github.com/inky/tree-skills/tree/main/skills/tree-skill", "--no-link", "-y",
	}, strings.NewReader(""), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	assertAddArchived(t, home, "tree-skill")
	assertAddNoPath(t, filepath.Join(home, ".x-skills", "skills", "other-skill"))
	assertAddNoPath(t, filepath.Join(project, ".agents", "skills", "tree-skill"))
	if !strings.Contains(out.String(), "added: tree-skill") {
		t.Fatalf("output missing added line:\n%s", out.String())
	}
}

func TestAddArchiveAsArchivesSelectedSkillUnderCustomName(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	repo := makeAddGitRepo(t)
	writeAddRemoteSkill(t, repo, "skills/source-name", "source-name", "Original.")
	addGitCommit(t, repo, "initial")

	var out bytes.Buffer
	err := Execute([]string{
		"--home", home,
		"--project-root", project,
		"add", "--git", repo, "source-name", "--archive-as", "custom-name", "--no-link", "-y",
	}, strings.NewReader(""), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	assertAddArchived(t, home, "custom-name")
	assertAddNoPath(t, filepath.Join(home, ".x-skills", "skills", "source-name"))
	if !strings.Contains(out.String(), "added: custom-name") {
		t.Fatalf("output missing custom archive name:\n%s", out.String())
	}
}

func TestAddArchiveAsRequiresSingleSelectedSkill(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	repo := makeAddGitRepo(t)
	writeAddRemoteSkill(t, repo, "skills/first-skill", "first-skill", "First.")
	writeAddRemoteSkill(t, repo, "skills/second-skill", "second-skill", "Second.")
	addGitCommit(t, repo, "initial")

	err := Execute([]string{
		"--home", home,
		"--project-root", project,
		"add", "--git", repo, "first-skill", "second-skill", "--archive-as", "custom-name", "--no-link", "-y",
	}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected --archive-as validation error")
	}
	if !strings.Contains(err.Error(), "--archive-as is only valid for exactly one selected skill") {
		t.Fatalf("error = %v, want --archive-as validation", err)
	}
}

func TestAddWithRefChecksOutLocalBranchAndTag(t *testing.T) {
	tests := []struct {
		name  string
		ref   string
		skill string
		setup func(t *testing.T, repo string)
	}{
		{
			name:  "branch",
			ref:   "feature/add-skill",
			skill: "branch-skill",
			setup: func(t *testing.T, repo string) {
				runAddGit(t, repo, "checkout", "-b", "feature/add-skill")
				writeAddRemoteSkill(t, repo, "skills/branch-skill", "branch-skill", "Branch.")
				addGitCommit(t, repo, "add branch skill")
			},
		},
		{
			name:  "tag",
			ref:   "v1.0.0",
			skill: "tagged-skill",
			setup: func(t *testing.T, repo string) {
				writeAddRemoteSkill(t, repo, "skills/tagged-skill", "tagged-skill", "Tagged.")
				addGitCommit(t, repo, "add tagged skill")
				runAddGit(t, repo, "tag", "v1.0.0")
				if err := os.RemoveAll(filepath.Join(repo, "skills", "tagged-skill")); err != nil {
					t.Fatal(err)
				}
				addGitCommit(t, repo, "remove tagged skill")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			project := t.TempDir()
			repo := makeAddGitRepo(t)
			writeAddRemoteSkill(t, repo, "skills/main-skill", "main-skill", "Main.")
			addGitCommit(t, repo, "initial")
			tt.setup(t, repo)

			err := Execute([]string{
				"--home", home,
				"--project-root", project,
				"add", "--git", repo, "--ref", tt.ref, tt.skill, "--no-link", "-y",
			}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
			if err != nil {
				t.Fatal(err)
			}
			assertAddArchived(t, home, tt.skill)
		})
	}
}

func TestAddAllRequiresConfirmationWithoutYes(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	repo := makeAddGitRepo(t)
	writeAddRemoteSkill(t, repo, "skills/alpha-skill", "alpha-skill", "Alpha.")
	addGitCommit(t, repo, "initial")

	err := Execute([]string{
		"--home", home,
		"--project-root", project,
		"--no-input",
		"add", "--git", repo, "--all", "--no-link",
	}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected confirmation error")
	}
	if !strings.Contains(err.Error(), "rerun with -y") {
		t.Fatalf("error = %v, want rerun hint", err)
	}
}

func TestAddAllArchivesEveryDiscoveredSkill(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	repo := makeAddGitRepo(t)
	writeAddRemoteSkill(t, repo, "skills/alpha-skill", "alpha-skill", "Alpha.")
	writeAddRemoteSkill(t, repo, "packs/beta-skill", "beta-skill", "Beta.")
	addGitCommit(t, repo, "initial")

	var out bytes.Buffer
	err := Execute([]string{
		"--home", home,
		"--project-root", project,
		"add", "--git", repo, "--all", "--no-link", "-y",
	}, strings.NewReader(""), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	assertAddArchived(t, home, "alpha-skill")
	assertAddArchived(t, home, "beta-skill")
	if !strings.Contains(out.String(), "Summary:") || !strings.Contains(out.String(), "added: alpha-skill, beta-skill") {
		t.Fatalf("output missing batch summary:\n%s", out.String())
	}
}

func TestAddWithGitWithoutNamesOrAllFailsValidation(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	repo := makeAddGitRepo(t)
	writeAddRemoteSkill(t, repo, "skills/alpha-skill", "alpha-skill", "Alpha.")
	writeAddRemoteSkill(t, repo, "packs/beta-skill", "beta-skill", "Beta.")
	addGitCommit(t, repo, "initial")

	err := Execute([]string{
		"--home", home,
		"--project-root", project,
		"add", "--git", repo, "--no-link", "-y",
	}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected missing skill name error")
	}
	if !strings.Contains(err.Error(), "--git requires at least one skill name or --all") {
		t.Fatalf("error = %q, want --git validation error", err)
	}
}

func TestAddConflictReturnsRerunHintWithoutReplace(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	first := makeAddGitRepo(t)
	writeAddRemoteSkill(t, first, "skills/conflict-skill", "conflict-skill", "First.")
	addGitCommit(t, first, "initial")
	second := makeAddGitRepo(t)
	writeAddRemoteSkill(t, second, "skills/conflict-skill", "conflict-skill", "Second.")
	addGitCommit(t, second, "initial")

	err := Execute([]string{
		"--home", home,
		"--project-root", project,
		"add", "--git", first, "conflict-skill", "--no-link", "-y",
	}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}

	err = Execute([]string{
		"--home", home,
		"--project-root", project,
		"add", "--git", second, "conflict-skill", "--no-link", "-y",
	}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected conflict")
	}
	if !strings.Contains(err.Error(), "--replace") || !strings.Contains(err.Error(), "tui") {
		t.Fatalf("error = %v, want replace/tui hint", err)
	}
}

func TestAddReplaceUpdatesSameNameArchive(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	repo := makeAddGitRepo(t)
	writeAddRemoteSkill(t, repo, "skills/update-skill", "update-skill", "Version 1.")
	writeAddRemoteFile(t, repo, "skills/update-skill/version.txt", "v1\n")
	addGitCommit(t, repo, "initial")

	err := Execute([]string{
		"--home", home,
		"--project-root", project,
		"add", "--git", repo, "update-skill", "--no-link", "-y",
	}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}

	writeAddRemoteSkill(t, repo, "skills/update-skill", "update-skill", "Version 2.")
	writeAddRemoteFile(t, repo, "skills/update-skill/version.txt", "v2\n")
	addGitCommit(t, repo, "update")

	err = Execute([]string{
		"--home", home,
		"--project-root", project,
		"add", "--git", repo, "update-skill", "--replace", "--no-link", "-y",
	}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(home, ".x-skills", "skills", "update-skill", "version.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimRight(string(data), "\r\n") != "v2" {
		t.Fatalf("version.txt = %q, want v2", data)
	}
}

func TestResolveAddSelectionGitHubShorthandWithEmbeddedAndExtraNames(t *testing.T) {
	got, err := resolveAddSelection(addOptions{}, []string{"owner/repo@skill", "extra"})
	if err != nil {
		t.Fatal(err)
	}
	if got.source.Source.CloneURL != "https://github.com/owner/repo.git" {
		t.Fatalf("CloneURL = %q, want GitHub clone URL", got.source.Source.CloneURL)
	}
	if got.source.Source.Owner != "owner" {
		t.Fatalf("Owner = %q, want owner", got.source.Source.Owner)
	}
	if got.source.Source.Repo != "repo" {
		t.Fatalf("Repo = %q, want repo", got.source.Source.Repo)
	}
	if !slices.Equal(got.names, []string{"skill", "extra"}) {
		t.Fatalf("names = %#v, want [skill extra]", got.names)
	}
}

func makeAddGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runAddGit(t, dir, "init")
	runAddGit(t, dir, "config", "user.email", "test@example.com")
	runAddGit(t, dir, "config", "user.name", "Test")
	return dir
}

func writeAddRemoteSkill(t *testing.T, root, rel, name, desc string) {
	t.Helper()
	dir := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	data := "---\nname: " + name + "\ndescription: " + desc + "\n---\n# " + name + "\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeAddRemoteFile(t *testing.T, root, rel, data string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func addGitCommit(t *testing.T, repo, msg string) {
	t.Helper()
	runAddGit(t, repo, "add", ".")
	runAddGit(t, repo, "commit", "-m", msg)
}

func runAddGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

func assertAddArchived(t *testing.T, home, name string) {
	t.Helper()
	path := filepath.Join(home, ".x-skills", "skills", name, "SKILL.md")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("archive %q not found: %v", name, err)
	}
}

func assertAddLink(t *testing.T, project, home, dotDir, name string) {
	t.Helper()
	assertAddLinkAt(t, filepath.Join(project, dotDir, "skills", name), home, name)
}

func assertAddLinkAt(t *testing.T, linkPath, home, name string) {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(linkPath)
	if err != nil {
		t.Fatalf("resolve link %q: %v", linkPath, err)
	}
	want := filepath.Join(home, ".x-skills", "skills", name)
	ok, err := pathidentity.EquivalentE(resolved, want)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatalf("link %q resolved to %q, want same location as %q", linkPath, resolved, want)
	}
}

func assertAddNoPath(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("path %q stat error = %v, want not exist", path, err)
	}
}
