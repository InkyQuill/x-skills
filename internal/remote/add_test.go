package remote

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/skills"
)

func TestApplyArchiveOnlyCopiesSkillAndWritesMetadata(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	incoming := writeIncomingSkill(t, "svelte-coder", "Svelte help.")
	meta := SourceMetadata{
		SourceType:   SourceTypeGitHub,
		Owner:        "vercel-labs",
		Repo:         "skills",
		CloneURL:     "https://github.com/vercel-labs/skills.git",
		Commit:       "abc",
		SkillPath:    "skills/svelte-coder",
		UpstreamName: "svelte-coder",
	}

	result, err := ApplyArchive(AddRequest{
		Config:      cfg,
		IncomingDir: incoming,
		ArchiveName: "svelte-coder",
		Metadata:    meta,
		Conflict:    ConflictReplaceArchive,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != AddStatusArchived {
		t.Fatalf("status = %q", result.Status)
	}
	info, err := skills.Read(filepath.Join(cfg.ArchiveSkillsRoot(), "svelte-coder"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Description != "Svelte help." {
		t.Fatalf("description = %q", info.Description)
	}
	if _, ok, err := ReadSourceMetadata(filepath.Join(cfg.ArchiveSkillsRoot(), "svelte-coder")); err != nil || !ok {
		t.Fatalf("source metadata missing: ok=%v err=%v", ok, err)
	}
}

func TestPlanArchiveDetectsNameConflictWithoutSourceIdentity(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	makeArchivedSkillForRemoteTest(t, cfg, "svelte-coder", "Local archived.")
	incoming := writeIncomingSkill(t, "svelte-coder", "Remote.")
	meta := SourceMetadata{
		SourceType: SourceTypeGitHub,
		Owner:      "vercel-labs",
		Repo:       "skills",
		SkillPath:  "skills/svelte-coder",
	}
	plan, err := PlanArchive(cfg, incoming, "svelte-coder", meta)
	if err != nil {
		t.Fatal(err)
	}
	if plan.State != ArchiveStateNameConflict {
		t.Fatalf("state = %q, want name conflict", plan.State)
	}
}

func TestPlanArchiveIgnoresSourceMetadataWhenContentMatches(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	incoming := writeIncomingSkill(t, "svelte-coder", "Svelte help.")
	meta := SourceMetadata{
		SourceType: SourceTypeGitHub,
		Owner:      "vercel-labs",
		Repo:       "skills",
		SkillPath:  "skills/svelte-coder",
	}
	if _, err := ApplyArchive(AddRequest{
		Config:      cfg,
		IncomingDir: incoming,
		ArchiveName: "svelte-coder",
		Metadata:    meta,
		Conflict:    ConflictReplaceArchive,
	}); err != nil {
		t.Fatal(err)
	}

	nextIncoming := writeIncomingSkill(t, "svelte-coder", "Svelte help.")
	plan, err := PlanArchive(cfg, nextIncoming, "svelte-coder", meta)
	if err != nil {
		t.Fatal(err)
	}
	if plan.State != ArchiveStateArchived {
		t.Fatalf("state = %q, want archived", plan.State)
	}
}

func TestApplyArchiveRejectsSymlinkWithoutIngestingTarget(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	incoming := writeIncomingSkill(t, "svelte-coder", "Svelte help.")
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("outside secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(incoming, "leak.txt")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	_, err := ApplyArchive(AddRequest{
		Config:      cfg,
		IncomingDir: incoming,
		ArchiveName: "svelte-coder",
		Metadata:    SourceMetadata{SourceType: SourceTypeGit, CloneURL: "https://example.com/repo.git", SkillPath: "svelte-coder"},
		Conflict:    ConflictReplaceArchive,
	})
	if err == nil {
		t.Fatal("expected symlink error")
	}
	if !strings.Contains(err.Error(), "unsupported file type in incoming skill: leak.txt") {
		t.Fatalf("error = %q", err)
	}
	if data, readErr := os.ReadFile(filepath.Join(cfg.ArchiveSkillsRoot(), "svelte-coder", "leak.txt")); readErr == nil {
		t.Fatalf("outside file content was ingested: %q", data)
	} else if !os.IsNotExist(readErr) {
		t.Fatal(readErr)
	}
	entries, readErr := os.ReadDir(cfg.ArchiveSkillsRoot())
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("temporary archive entries remain: %v", entries)
	}
}

func TestApplyArchiveCancelDoesNotMutateExistingArchive(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	makeArchivedSkillForRemoteTest(t, cfg, "svelte-coder", "Existing.")
	incoming := writeIncomingSkill(t, "svelte-coder", "Incoming.")

	result, err := ApplyArchive(AddRequest{
		Config:      cfg,
		IncomingDir: incoming,
		ArchiveName: "svelte-coder",
		Metadata:    SourceMetadata{SourceType: SourceTypeGit, CloneURL: "https://example.com/repo.git", SkillPath: "svelte-coder"},
		Conflict:    ConflictCancel,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != AddStatusSkipped {
		t.Fatalf("status = %q", result.Status)
	}
	info, err := skills.Read(filepath.Join(cfg.ArchiveSkillsRoot(), "svelte-coder"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Description != "Existing." {
		t.Fatalf("description = %q", info.Description)
	}
}

func TestApplyArchiveRejectsUnknownConflict(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	incoming := writeIncomingSkill(t, "svelte-coder", "Svelte help.")

	_, err := ApplyArchive(AddRequest{
		Config:      cfg,
		IncomingDir: incoming,
		ArchiveName: "svelte-coder",
		Metadata:    SourceMetadata{SourceType: SourceTypeGit, CloneURL: "https://example.com/repo.git", SkillPath: "svelte-coder"},
		Conflict:    "overwrite-ish",
	})
	if err == nil {
		t.Fatal("expected unknown conflict error")
	}
	if !strings.Contains(err.Error(), `unknown archive conflict "overwrite-ish"`) {
		t.Fatalf("error = %q", err)
	}
	if _, err := os.Stat(filepath.Join(cfg.ArchiveSkillsRoot(), "svelte-coder")); !os.IsNotExist(err) {
		t.Fatalf("archive was created: err=%v", err)
	}
}

func TestApplyArchiveReplaceRemovesStaleFilesAndReturnsUpdated(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	archive := makeArchivedSkillForRemoteTest(t, cfg, "svelte-coder", "Old.")
	if err := os.WriteFile(filepath.Join(archive, "stale.txt"), []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	incoming := writeIncomingSkill(t, "svelte-coder", "New.")

	result, err := ApplyArchive(AddRequest{
		Config:      cfg,
		IncomingDir: incoming,
		ArchiveName: "svelte-coder",
		Metadata:    SourceMetadata{SourceType: SourceTypeGit, CloneURL: "https://example.com/repo.git", SkillPath: "svelte-coder"},
		Conflict:    ConflictReplaceArchive,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != AddStatusUpdated {
		t.Fatalf("status = %q", result.Status)
	}
	if _, err := os.Stat(filepath.Join(archive, "stale.txt")); !os.IsNotExist(err) {
		t.Fatalf("stale file remains: err=%v", err)
	}
	info, err := skills.Read(archive)
	if err != nil {
		t.Fatal(err)
	}
	if info.Description != "New." {
		t.Fatalf("description = %q", info.Description)
	}
}

func TestApplyArchiveRenameIncomingDoesNotOverlayExistingArchive(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	makeArchivedSkillForRemoteTest(t, cfg, "svelte-coder", "Existing.")
	incoming := writeIncomingSkill(t, "svelte-coder", "Incoming.")

	_, err := ApplyArchive(AddRequest{
		Config:      cfg,
		IncomingDir: incoming,
		ArchiveName: "svelte-coder",
		Metadata:    SourceMetadata{SourceType: SourceTypeGit, CloneURL: "https://example.com/repo.git", SkillPath: "svelte-coder"},
		Conflict:    ConflictRenameIncoming,
	})
	if err == nil {
		t.Fatal("expected rename conflict error")
	}
	if !strings.Contains(err.Error(), "archive destination already exists: svelte-coder") {
		t.Fatalf("error = %q", err)
	}
	info, err := skills.Read(filepath.Join(cfg.ArchiveSkillsRoot(), "svelte-coder"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Description != "Existing." {
		t.Fatalf("description = %q", info.Description)
	}
}

func TestApplyArchiveReplaceRestoresBackupWhenFinalRenameFails(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())
	archive := makeArchivedSkillForRemoteTest(t, cfg, "svelte-coder", "Existing.")
	incoming := writeIncomingSkill(t, "svelte-coder", "Incoming.")
	renameErr := errors.New("rename failed")

	originalRenamePath := renamePath
	t.Cleanup(func() {
		renamePath = originalRenamePath
	})
	failedFinalRename := false
	renamePath = func(oldpath, newpath string) error {
		if newpath == archive && !failedFinalRename {
			failedFinalRename = true
			return renameErr
		}
		return originalRenamePath(oldpath, newpath)
	}

	_, err := ApplyArchive(AddRequest{
		Config:      cfg,
		IncomingDir: incoming,
		ArchiveName: "svelte-coder",
		Metadata:    SourceMetadata{SourceType: SourceTypeGit, CloneURL: "https://example.com/repo.git", SkillPath: "svelte-coder"},
		Conflict:    ConflictReplaceArchive,
	})
	if err == nil {
		t.Fatal("expected rename error")
	}
	if !strings.Contains(err.Error(), "install archive") {
		t.Fatalf("error = %q", err)
	}
	info, err := skills.Read(archive)
	if err != nil {
		t.Fatal(err)
	}
	if info.Description != "Existing." {
		t.Fatalf("description = %q", info.Description)
	}
}

func TestArchiveContentFingerprintFramesFileContents(t *testing.T) {
	first := t.TempDir()
	if err := os.WriteFile(filepath.Join(first, "a"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(first, "b"), []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}

	second := t.TempDir()
	if err := os.WriteFile(filepath.Join(second, "a"), []byte("x\x00file\x00b\x00y"), 0o644); err != nil {
		t.Fatal(err)
	}

	firstFP, err := archiveContentFingerprint(first)
	if err != nil {
		t.Fatal(err)
	}
	secondFP, err := archiveContentFingerprint(second)
	if err != nil {
		t.Fatal(err)
	}
	if firstFP == secondFP {
		t.Fatalf("fingerprint collision: %q", firstFP)
	}
}

func TestArchiveContentFingerprintRejectsSpecialFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fifo unavailable on windows")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "pipe")
	if err := exec.Command("mkfifo", path).Run(); err != nil {
		t.Skipf("mkfifo unavailable: %v", err)
	}

	_, err := archiveContentFingerprint(dir)
	if err == nil {
		t.Fatal("expected special file error")
	}
	if !strings.Contains(err.Error(), "unsupported file type in archive content: pipe") {
		t.Fatalf("error = %q", err)
	}
}

func writeIncomingSkill(t *testing.T, name, desc string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: "+name+"\ndescription: "+desc+"\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func makeArchivedSkillForRemoteTest(t *testing.T, cfg config.Config, name, desc string) string {
	t.Helper()
	dir := filepath.Join(cfg.ArchiveSkillsRoot(), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: "+name+"\ndescription: "+desc+"\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}
