package actions

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/fingerprint"
	"github.com/InkyQuill/x-skills/internal/remote"
)

func TestRenameArchivePreservesSkillAndSourceMetadataIdentity(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	archive := makeRenameSkill(t, cfg.ArchiveSkillsRoot(), "old")
	if err := remote.WriteSourceMetadata(archive, remote.SourceMetadata{SourceType: remote.SourceTypeGitHub, Owner: "owner", Repo: "repo", SkillPath: "skills/original"}); err != nil {
		t.Fatal(err)
	}
	skillBefore, _ := os.ReadFile(filepath.Join(archive, "SKILL.md"))
	metaBefore, _ := os.ReadFile(filepath.Join(archive, remote.MetadataFile))
	fingerprintBefore, err := fingerprint.Directory(archive)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RenameArchive(cfg, "old", "new"); err != nil {
		t.Fatal(err)
	}
	newArchive := filepath.Join(cfg.ArchiveSkillsRoot(), "new")
	skillAfter, _ := os.ReadFile(filepath.Join(newArchive, "SKILL.md"))
	metaAfter, _ := os.ReadFile(filepath.Join(newArchive, remote.MetadataFile))
	fingerprintAfter, err := fingerprint.Directory(newArchive)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(skillBefore, skillAfter) || !bytes.Equal(metaBefore, metaAfter) {
		t.Fatal("archive content identity changed during rename")
	}
	if fingerprintAfter != fingerprintBefore {
		t.Fatalf("fingerprint = %q, want %q", fingerprintAfter, fingerprintBefore)
	}
}

func TestRenameArchiveReportsManifestWriteAndRollbackFailure(t *testing.T) {
	project := t.TempDir()
	cfg := config.Default(project, t.TempDir())
	archive := makeRenameSkill(t, cfg.ArchiveSkillsRoot(), "old")
	makeRenameLink(t, filepath.Join(project, ".agents", "skills"), "alias", archive)
	writeRenameManifest(t, filepath.Join(project, ".x-skills.local.yaml"), "old", "archive")
	original := renameArchiveFilesystem
	renameArchiveFilesystem.rename = func(oldPath, newPath string) error {
		if strings.Contains(oldPath, ".x-skills-rename-manifest-") {
			return errors.New("injected manifest rename failure")
		}
		return os.Rename(oldPath, newPath)
	}
	renameArchiveFilesystem.symlink = func(target, path string) error {
		if strings.Contains(target, "old") {
			return errors.New("injected symlink rollback failure")
		}
		return os.Symlink(target, path)
	}
	t.Cleanup(func() { renameArchiveFilesystem = original })
	_, err := RenameArchive(cfg, "old", "new")
	if err == nil || !strings.Contains(err.Error(), "update .x-skills.local.yaml") || !strings.Contains(err.Error(), "restore .x-skills.local.yaml") || !strings.Contains(err.Error(), "injected symlink rollback failure") {
		t.Fatalf("error = %v", err)
	}
}

func TestRenameArchiveDiscoversAliasesByTargetAndPreservesLinkStyle(t *testing.T) {
	project := t.TempDir()
	cfg := config.Default(project, t.TempDir())
	archive := makeRenameSkill(t, cfg.ArchiveSkillsRoot(), "old")
	root := filepath.Join(project, ".agents", "skills")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "friendly-name")
	relative, err := filepath.Rel(root, archive)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(relative, alias); err != nil {
		t.Fatal(err)
	}

	result, err := RenameArchive(cfg, "old", "new")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(result.RelinkedPaths, []string{alias}) {
		t.Fatalf("relinked paths = %#v", result.RelinkedPaths)
	}
	text, err := os.Readlink(alias)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := filepath.Rel(root, filepath.Join(cfg.ArchiveSkillsRoot(), "new"))
	if text != want || filepath.IsAbs(text) {
		t.Fatalf("link text = %q, want relative %q", text, want)
	}
}

func TestRenameArchiveLeavesSameNameUnmanagedSymlinkUntouched(t *testing.T) {
	project := t.TempDir()
	cfg := config.Default(project, t.TempDir())
	makeRenameSkill(t, cfg.ArchiveSkillsRoot(), "old")
	other := makeRenameSkill(t, t.TempDir(), "other")
	path := makeRenameLink(t, filepath.Join(project, ".agents", "skills"), "old", other)
	before, _ := os.Readlink(path)
	result, err := RenameArchive(cfg, "old", "new")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.RelinkedPaths) != 0 {
		t.Fatalf("relinked = %#v", result.RelinkedPaths)
	}
	after, err := os.Readlink(path)
	if err != nil || after != before {
		t.Fatalf("unmanaged link = %q, %v", after, err)
	}
}

func TestRenameArchivePreservesManifestFormattingCommentsAndSourcePath(t *testing.T) {
	project := t.TempDir()
	cfg := config.Default(project, t.TempDir())
	makeRenameSkill(t, cfg.ArchiveSkillsRoot(), "old")
	manifestPath := filepath.Join(project, ".x-skills.yaml")
	before := "# header\nversion: 1\nskills:\n  - name: old # identity\n    source:\n      type: github\n      repository: github.com/example/skills\n      path: skills/old # source path is not identity\n"
	if err := os.WriteFile(manifestPath, []byte(before), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := RenameArchive(cfg, "old", "new"); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(before, "name: old", "name: new", 1)
	if string(after) != want {
		t.Fatalf("manifest formatting changed:\n%s\nwant:\n%s", after, want)
	}
}

func TestRenameArchiveContextCancelsBeforeMutation(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	old := makeRenameSkill(t, cfg.ArchiveSkillsRoot(), "old")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := RenameArchiveContext(ctx, cfg, "old", "new")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
	if _, err := os.Stat(old); err != nil {
		t.Fatalf("archive mutated: %v", err)
	}
}

func TestRenameArchiveRejectsArchiveAndUsageDriftBeforeMutation(t *testing.T) {
	for _, drift := range []string{"archive", "usage"} {
		t.Run(drift, func(t *testing.T) {
			project := t.TempDir()
			cfg := config.Default(project, t.TempDir())
			archive := makeRenameSkill(t, cfg.ArchiveSkillsRoot(), "old")
			link := makeRenameLink(t, filepath.Join(project, ".agents", "skills"), "alias", archive)
			original := renameArchiveFilesystem
			renameArchiveFilesystem.beforeMutation = func(boundary string) error {
				if boundary == drift {
					if drift == "archive" {
						return os.WriteFile(filepath.Join(archive, "drift"), []byte("x"), 0o644)
					}
					if err := os.Remove(link); err != nil {
						return err
					}
					return os.Symlink(t.TempDir(), link)
				}
				return nil
			}
			t.Cleanup(func() { renameArchiveFilesystem = original })
			_, err := RenameArchive(cfg, "old", "new")
			if err == nil || !strings.Contains(err.Error(), "drift") {
				t.Fatalf("error = %v", err)
			}
			if _, statErr := os.Stat(archive); statErr != nil {
				t.Fatalf("archive moved: %v", statErr)
			}
			if drift == "usage" {
				text, readErr := os.Readlink(link)
				if readErr != nil || text == archive {
					t.Fatalf("tampered usage was lost: %q, %v", text, readErr)
				}
			}
		})
	}
}

func TestRenameArchiveRelinksVisibleManagedUsagesAndUpdatesManifests(t *testing.T) {
	project := t.TempDir()
	home := t.TempDir()
	cfg := config.Default(project, home)
	archive := makeRenameSkill(t, cfg.ArchiveSkillsRoot(), "old")
	projectRoot := filepath.Join(project, ".agents", "skills")
	globalRoot := filepath.Join(home, ".codex", "skills")
	makeRenameLink(t, projectRoot, "old", archive)
	makeRenameLink(t, globalRoot, "old", archive)
	unmanaged := makeRenameSkill(t, filepath.Join(project, ".claude", "skills"), "old")
	writeRenameManifest(t, filepath.Join(project, ".x-skills.yaml"), "old", "github")
	writeRenameManifest(t, filepath.Join(project, ".x-skills.local.yaml"), "old", "archive")

	result, err := RenameArchive(cfg, "old", "new")
	if err != nil {
		t.Fatal(err)
	}
	if result.ArchivePath != filepath.Join(cfg.ArchiveSkillsRoot(), "new") {
		t.Fatalf("archive path = %q", result.ArchivePath)
	}
	if !slices.Equal(result.RelinkedPaths, []string{filepath.Join(projectRoot, "old"), filepath.Join(globalRoot, "old")}) {
		t.Fatalf("relinked paths = %#v", result.RelinkedPaths)
	}
	if !result.OtherProjectsMayUseArchive || !slices.Equal(result.ManifestUpdates, []string{".x-skills.yaml", ".x-skills.local.yaml"}) {
		t.Fatalf("result = %#v", result)
	}
	for _, link := range result.RelinkedPaths {
		resolved, resolveErr := filepath.EvalSymlinks(link)
		if resolveErr != nil || resolved != result.ArchivePath {
			t.Fatalf("link %q resolves to %q, %v", link, resolved, resolveErr)
		}
	}
	if _, err := os.Stat(unmanaged); err != nil {
		t.Fatalf("unmanaged same-name entry changed: %v", err)
	}
	for _, filename := range []string{".x-skills.yaml", ".x-skills.local.yaml"} {
		data, readErr := os.ReadFile(filepath.Join(project, filename))
		if readErr != nil || !strings.Contains(string(data), "name: new") || strings.Contains(string(data), "name: old") {
			t.Fatalf("manifest %s = %q, %v", filename, data, readErr)
		}
	}
}

func TestRenameArchiveValidatesNamesAndDestination(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	makeRenameSkill(t, cfg.ArchiveSkillsRoot(), "old")
	makeRenameSkill(t, cfg.ArchiveSkillsRoot(), "taken")
	for _, tc := range []struct {
		oldName string
		newName string
		want    string
	}{
		{oldName: "../old", newName: "new", want: "invalid"},
		{oldName: "old", newName: "../new", want: "invalid"},
		{oldName: "missing", newName: "new", want: "not found"},
		{oldName: "old", newName: "taken", want: "already exists"},
	} {
		t.Run(tc.oldName+"_to_"+tc.newName, func(t *testing.T) {
			_, err := RenameArchive(cfg, tc.oldName, tc.newName)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestRenameArchiveRollsBackAfterRelinkFailure(t *testing.T) {
	project := t.TempDir()
	cfg := config.Default(project, t.TempDir())
	archive := makeRenameSkill(t, cfg.ArchiveSkillsRoot(), "old")
	first := makeRenameLink(t, filepath.Join(project, ".agents", "skills"), "old", archive)
	makeRenameLink(t, filepath.Join(project, ".codex", "skills"), "old", archive)
	writeRenameManifest(t, filepath.Join(project, ".x-skills.local.yaml"), "old", "archive")

	original := renameArchiveFilesystem
	calls := 0
	renameArchiveFilesystem.rename = func(oldPath, newPath string) error {
		if strings.Contains(oldPath, ".x-skills-rename-link-") {
			calls++
			if calls == 2 {
				return errors.New("injected relink failure")
			}
		}
		return os.Rename(oldPath, newPath)
	}
	t.Cleanup(func() { renameArchiveFilesystem = original })

	_, err := RenameArchive(cfg, "old", "new")
	if err == nil || !strings.Contains(err.Error(), "injected relink failure") {
		t.Fatalf("error = %v", err)
	}
	if _, err := os.Stat(archive); err != nil {
		t.Fatalf("old archive was not restored: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cfg.ArchiveSkillsRoot(), "new")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("new archive remains after rollback: %v", err)
	}
	resolved, err := filepath.EvalSymlinks(first)
	if err != nil || resolved != archive {
		t.Fatalf("first link rollback = %q, %v", resolved, err)
	}
	data, _ := os.ReadFile(filepath.Join(project, ".x-skills.local.yaml"))
	if !strings.Contains(string(data), "name: old") {
		t.Fatalf("manifest changed after rollback: %s", data)
	}
}

func makeRenameSkill(t *testing.T, root, name string) string {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "SKILL.md"), []byte("---\nname: "+name+"\ndescription: test\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func makeRenameLink(t *testing.T, root, name, target string) string {
	t.Helper()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, name)
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeRenameManifest(t *testing.T, path, name, sourceType string) {
	t.Helper()
	source := "type: " + sourceType
	if sourceType == "github" {
		source += "\n      repository: github.com/example/skills\n      path: skills/" + name
	}
	if err := os.WriteFile(path, []byte("version: 1\nskills:\n  - name: "+name+"\n    source:\n      "+source+"\n    fingerprint: "+strings.Repeat("a", 64)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}
