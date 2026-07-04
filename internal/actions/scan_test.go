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
