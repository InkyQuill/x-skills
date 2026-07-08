package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrateWithYesFlag(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	active := makeSkill(t, filepath.Join(project, ".codex", "skills"), "local-only", "Local.")

	var out bytes.Buffer
	err := Execute([]string{"--home", home, "--project-root", project, "-y", "migrate", "local-only", "--project", "--target", "codex"}, strings.NewReader(""), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	archived := filepath.Join(home, ".x-skills", "skills", "local-only")
	resolved, err := filepath.EvalSymlinks(active)
	if err != nil {
		t.Fatal(err)
	}
	if resolved != archived {
		t.Fatalf("resolved = %q, want %q", resolved, archived)
	}
}

func TestMigrateWithoutYesPromptsAndCancelsOnEmptyAnswer(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	active := makeSkill(t, filepath.Join(project, ".codex", "skills"), "local-only", "Local.")

	var out bytes.Buffer
	err := Execute([]string{"--home", home, "--project-root", project, "migrate", "local-only", "--project", "--target", "codex"}, strings.NewReader(""), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "cancelled") {
		t.Fatalf("output = %q, want cancelled", out.String())
	}
	if _, err := os.Stat(active); err != nil {
		t.Fatalf("active skill changed: %v", err)
	}
}

func TestMigrateFailsNoInputWhenActiveSkillNameIsAmbiguous(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	makeSkill(t, filepath.Join(project, ".codex", "skills"), "svelte-coder", "Project.")
	makeSkill(t, filepath.Join(home, ".agents", "skills"), "svelte-coder", "Global.")

	err := Execute([]string{"--home", home, "--project-root", project, "--no-input", "migrate", "svelte-coder"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected ambiguous active skill error")
	}
	if !strings.Contains(err.Error(), `multiple active skills named "svelte-coder"`) {
		t.Fatalf("error = %q, want multiple active skills", err)
	}
	if !strings.Contains(err.Error(), "x-skills migrate svelte-coder --target codex --project") ||
		!strings.Contains(err.Error(), "x-skills migrate svelte-coder --target agents --global") {
		t.Fatalf("error missing one-shot hints: %v", err)
	}
}

func TestMigratePromptsForAmbiguousActiveSkillAndConfirmation(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	active := makeSkill(t, filepath.Join(project, ".codex", "skills"), "svelte-coder", "Project.")
	makeSkill(t, filepath.Join(home, ".agents", "skills"), "svelte-coder", "Global.")

	var out bytes.Buffer
	err := Execute([]string{"--home", home, "--project-root", project, "migrate", "svelte-coder"}, strings.NewReader("1\ny\n"), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Select skill to migrate [1-2]:") {
		t.Fatalf("output missing selection prompt:\n%s", out.String())
	}
	if !strings.Contains(out.String(), `Migrate project codex skill "svelte-coder" into repo? [y/N]:`) {
		t.Fatalf("output missing confirmation prompt:\n%s", out.String())
	}
	archived := filepath.Join(home, ".x-skills", "skills", "svelte-coder")
	resolved, err := filepath.EvalSymlinks(active)
	if err != nil {
		t.Fatal(err)
	}
	if resolved != archived {
		t.Fatalf("resolved = %q, want %q", resolved, archived)
	}
}

func TestBatchMigrateSummaryIncludesSkippedConfirmations(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	first := makeSkill(t, filepath.Join(project, ".codex", "skills"), "first-skill", "First.")
	second := makeSkill(t, filepath.Join(project, ".agents", "skills"), "second-skill", "Second.")

	var out bytes.Buffer
	err := Execute([]string{"--home", home, "--project-root", project, "migrate", "first-skill", "second-skill"}, strings.NewReader("y\nn\n"), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	output := out.String()
	if !strings.Contains(output, "migrated: first-skill") || !strings.Contains(output, "skipped: second-skill") {
		t.Fatalf("summary missing migrated or skipped item:\n%s", output)
	}
	if _, err := filepath.EvalSymlinks(first); err != nil {
		t.Fatalf("first skill should be migrated and relinked: %v", err)
	}
	if _, err := os.Stat(second); err != nil {
		t.Fatalf("second skill should remain active: %v", err)
	}
}

func TestUnlinkUnmanagedDeleteWithYes(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	active := makeSkill(t, filepath.Join(project, ".codex", "skills"), "local-only", "Local.")

	var out bytes.Buffer
	err := Execute([]string{"--home", home, "--project-root", project, "-y", "unlink", "local-only", "--project", "--target", "codex", "--delete-unmanaged"}, strings.NewReader(""), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(active); !os.IsNotExist(err) {
		t.Fatalf("active still exists or unexpected err: %v", err)
	}
	if !strings.Contains(out.String(), "removed unmanaged") {
		t.Fatalf("unlink output:\n%s", out.String())
	}
}

func TestUnlinkUnmanagedPromptsForArchiveOrDelete(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	active := makeSkill(t, filepath.Join(project, ".codex", "skills"), "local-only", "Local.")

	var out bytes.Buffer
	err := Execute([]string{"--home", home, "--project-root", project, "unlink", "local-only", "--project", "--target", "codex"}, strings.NewReader("1\n"), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Select unlink action [1-3]:") {
		t.Fatalf("output missing unmanaged choice prompt:\n%s", out.String())
	}
	if _, err := os.Lstat(active); !os.IsNotExist(err) {
		t.Fatalf("active skill still exists or unexpected err: %v", err)
	}
	archived := filepath.Join(home, ".x-skills", "skills", "local-only")
	if _, err := os.Stat(archived); err != nil {
		t.Fatalf("archived skill missing: %v", err)
	}
}

func TestBatchUnlinkSummaryIncludesSkippedConfirmations(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	first := makeSkill(t, filepath.Join(project, ".codex", "skills"), "first-skill", "First.")
	second := makeSkill(t, filepath.Join(project, ".agents", "skills"), "second-skill", "Second.")

	var out bytes.Buffer
	err := Execute([]string{"--home", home, "--project-root", project, "unlink", "first-skill", "second-skill", "--delete-unmanaged"}, strings.NewReader("y\nn\n"), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	output := out.String()
	if !strings.Contains(output, "removed unmanaged: first-skill") || !strings.Contains(output, "skipped: second-skill") {
		t.Fatalf("summary missing removed or skipped item:\n%s", output)
	}
	if _, err := os.Lstat(first); !os.IsNotExist(err) {
		t.Fatalf("first skill still exists or unexpected err: %v", err)
	}
	if _, err := os.Stat(second); err != nil {
		t.Fatalf("second skill should remain active: %v", err)
	}
}

func TestUnlinkUnmanagedPromptCanDeleteWithoutArchive(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	active := makeSkill(t, filepath.Join(project, ".codex", "skills"), "local-only", "Local.")

	var out bytes.Buffer
	err := Execute([]string{"--home", home, "--project-root", project, "unlink", "local-only", "--project", "--target", "codex"}, strings.NewReader("2\n"), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "removed unmanaged") {
		t.Fatalf("unlink output:\n%s", out.String())
	}
	if _, err := os.Lstat(active); !os.IsNotExist(err) {
		t.Fatalf("active skill still exists or unexpected err: %v", err)
	}
	archived := filepath.Join(home, ".x-skills", "skills", "local-only")
	if _, err := os.Stat(archived); !os.IsNotExist(err) {
		t.Fatalf("archived skill should not exist or unexpected err: %v", err)
	}
}

func TestUnlinkUnmanagedNoInputRequiresChoice(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	active := makeSkill(t, filepath.Join(project, ".codex", "skills"), "local-only", "Local.")

	err := Execute([]string{"--home", home, "--project-root", project, "--no-input", "unlink", "local-only", "--project", "--target", "codex"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected unmanaged choice error")
	}
	if !strings.Contains(err.Error(), "archive then unlink with -y") ||
		!strings.Contains(err.Error(), "remove without archiving with --delete-unmanaged -y") {
		t.Fatalf("error missing one-shot hints: %v", err)
	}
	if _, err := os.Stat(active); err != nil {
		t.Fatalf("active skill changed: %v", err)
	}
}

func TestUnlinkWithYesStillRequiresAmbiguousLocationSelection(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	agents := makeSkill(t, filepath.Join(home, ".agents", "skills"), "code-review", "Review.")
	claudeRoot := filepath.Join(home, ".claude", "skills")
	if err := os.MkdirAll(claudeRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	claude := filepath.Join(claudeRoot, "code-review")
	if err := os.Symlink(agents, claude); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	err := Execute([]string{"--home", home, "--project-root", project, "-y", "unlink", "code-review", "--delete-unmanaged"}, strings.NewReader(""), &out, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected selection error")
	}
	if !strings.Contains(err.Error(), "invalid selection") {
		t.Fatalf("error = %q, want invalid selection", err)
	}
	if _, err := os.Stat(agents); err != nil {
		t.Fatalf("agents skill should remain: %v", err)
	}
	if _, err := os.Lstat(claude); err != nil {
		t.Fatalf("claude link should remain: %v", err)
	}
}

func TestUnlinkFailsNoInputWhenActiveSkillNameIsAmbiguous(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	makeSkill(t, filepath.Join(project, ".codex", "skills"), "code-review", "Project.")
	makeSkill(t, filepath.Join(home, ".agents", "skills"), "code-review", "Global.")

	err := Execute([]string{"--home", home, "--project-root", project, "--no-input", "unlink", "code-review"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected ambiguous active skill error")
	}
	if !strings.Contains(err.Error(), `multiple active skills named "code-review"`) {
		t.Fatalf("error = %q, want multiple active skills", err)
	}
	if !strings.Contains(err.Error(), "x-skills unlink code-review --target codex --project") ||
		!strings.Contains(err.Error(), "x-skills unlink code-review --target agents --global") {
		t.Fatalf("error missing one-shot hints: %v", err)
	}
}

func TestUnlinkPromptsForAmbiguousActiveSkill(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	active := makeSkill(t, filepath.Join(project, ".codex", "skills"), "code-review", "Project.")
	makeSkill(t, filepath.Join(home, ".agents", "skills"), "code-review", "Global.")

	var out bytes.Buffer
	err := Execute([]string{"--home", home, "--project-root", project, "-y", "unlink", "code-review", "--delete-unmanaged"}, strings.NewReader("1\n"), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Select skill to unlink [1-2]:") {
		t.Fatalf("output missing selection prompt:\n%s", out.String())
	}
	if _, err := os.Lstat(active); !os.IsNotExist(err) {
		t.Fatalf("selected active skill still exists or unexpected err: %v", err)
	}
}

func TestUnlinkGlobalWithYesDefaultsToGlobalMatchingRoots(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	global := makeSkill(t, filepath.Join(home, ".agents", "skills"), "commit-context", "Context.")
	projectSkill := makeSkill(t, filepath.Join(project, ".agents", "skills"), "commit-context", "Project.")

	var out bytes.Buffer
	err := Execute([]string{"--home", home, "--project-root", project, "-y", "unlink", "--global", "commit-context", "--delete-unmanaged"}, strings.NewReader(""), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(global); !os.IsNotExist(err) {
		t.Fatalf("global skill still exists or unexpected err: %v", err)
	}
	if _, err := os.Stat(projectSkill); err != nil {
		t.Fatalf("project skill should remain: %v", err)
	}
}

func TestUnlinkDeleteUnmanagedWithoutYesPromptsAndCancelsOnEmptyAnswer(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	active := makeSkill(t, filepath.Join(project, ".codex", "skills"), "local-only", "Local.")

	var out bytes.Buffer
	err := Execute([]string{"--home", home, "--project-root", project, "unlink", "local-only", "--project", "--target", "codex", "--delete-unmanaged"}, strings.NewReader(""), &out, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "cancelled") {
		t.Fatalf("output = %q, want cancelled", out.String())
	}
	if _, err := os.Stat(active); err != nil {
		t.Fatalf("active skill changed: %v", err)
	}
}
