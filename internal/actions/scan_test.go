package actions

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/InkyQuill/x-skills/internal/config"
)

func makeSkill(t *testing.T, root, name, desc string) string {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: "+name+"\ndescription: "+desc+"\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestScanActiveStatusesAndBrokenReasons(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	managed := makeSkill(t, cfg.ArchiveSkillsRoot(), "managed-codex", "Managed.")
	codexRoot := cfg.ActiveRoot("project", "codex")
	if err := os.MkdirAll(codexRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(managed, filepath.Join(codexRoot, "managed-codex")); err != nil {
		t.Fatal(err)
	}
	makeSkill(t, cfg.ActiveRoot("project", "claude"), "local-claude", "Local.")
	globalAgents := cfg.ActiveRoot("global", "agents")
	if err := os.MkdirAll(globalAgents, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(home, "missing"), filepath.Join(globalAgents, "broken-agents")); err != nil {
		t.Fatal(err)
	}

	skills, err := ScanActive(cfg, ScanFilter{})
	if err != nil {
		t.Fatal(err)
	}

	status := map[string]string{}
	reason := map[string]string{}
	for _, skill := range skills {
		status[skill.Name] = skill.Status
		reason[skill.Name] = skill.Reason
	}
	if status["managed-codex"] != StatusManaged {
		t.Fatalf("managed-codex status = %q", status["managed-codex"])
	}
	if status["local-claude"] != StatusUnmanaged {
		t.Fatalf("local-claude status = %q", status["local-claude"])
	}
	if status["broken-agents"] != StatusBroken {
		t.Fatalf("broken-agents status = %q", status["broken-agents"])
	}
	if reason["broken-agents"] == "" {
		t.Fatal("missing broken reason")
	}
}

func TestScanActiveReportsBrokenSymlinkTargetNotDirectory(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	target := filepath.Join(home, "not-a-dir")
	if err := os.WriteFile(target, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	root := cfg.ActiveRoot("project", "agents")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, "file-target")); err != nil {
		t.Fatal(err)
	}

	skills, err := ScanActive(cfg, ScanFilter{Scope: "project", Target: "agents"})
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatalf("len(skills) = %d, want 1", len(skills))
	}
	if skills[0].Status != StatusBroken {
		t.Fatalf("Status = %q, want broken", skills[0].Status)
	}
	if skills[0].Reason != "target is not a directory" {
		t.Fatalf("Reason = %q", skills[0].Reason)
	}
}

func TestScanActiveReportsBrokenSymlinkTargetMissingSkillMD(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)
	target := filepath.Join(home, "missing-skill-md")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	root := cfg.ActiveRoot("project", "agents")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, "missing-skill-md")); err != nil {
		t.Fatal(err)
	}

	skills, err := ScanActive(cfg, ScanFilter{Scope: "project", Target: "agents"})
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatalf("len(skills) = %d, want 1", len(skills))
	}
	if skills[0].Status != StatusBroken {
		t.Fatalf("Status = %q, want broken", skills[0].Status)
	}
	if skills[0].Reason != "target is not a skill directory" {
		t.Fatalf("Reason = %q", skills[0].Reason)
	}
}

func TestScanActiveManagedThroughSymlinkedArchiveRoot(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cfg := config.Default(project, home)

	realArchive := filepath.Join(home, "archive-real")
	cfg.ArchiveRoot = filepath.Join(home, "archive-link")
	if err := os.MkdirAll(realArchive, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(realArchive, cfg.ArchiveRoot); err != nil {
		t.Fatal(err)
	}

	source := makeSkill(t, cfg.ArchiveSkillsRoot(), "managed-agents", "Managed.")
	resolvedSource, err := filepath.EvalSymlinks(source)
	if err != nil {
		t.Fatal(err)
	}
	root := cfg.ActiveRoot("project", "agents")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(resolvedSource, filepath.Join(root, "managed-agents")); err != nil {
		t.Fatal(err)
	}

	skills, err := ScanActive(cfg, ScanFilter{Scope: "project", Target: "agents"})
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatalf("len(skills) = %d, want 1", len(skills))
	}
	if skills[0].Status != StatusManaged {
		t.Fatalf("Status = %q, want managed", skills[0].Status)
	}
}
