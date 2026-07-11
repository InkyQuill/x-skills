package actions

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/InkyQuill/x-skills/internal/config"
)

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
	if !slices.Equal(result.RelinkedPaths, []string{filepath.Join(projectRoot, "new"), filepath.Join(globalRoot, "new")}) {
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
