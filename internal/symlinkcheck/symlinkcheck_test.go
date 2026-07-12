package symlinkcheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/InkyQuill/x-skills/internal/pathidentity"
)

// assertSamePath verifies two paths identify the same filesystem location.
func assertSamePath(t *testing.T, got, want string) {
	t.Helper()
	ok, err := pathidentity.EquivalentE(got, want)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatalf("path = %q, want same location as %q", got, want)
	}
}

func TestValidateSkillTargetAcceptsSkillDirectorySymlink(t *testing.T) {
	root := t.TempDir()
	target := makeSkillDir(t, root, "golang-testing")
	link := filepath.Join(root, "active")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	result := ValidateSkillTarget(link)
	if result.Broken {
		t.Fatalf("ValidateSkillTarget() marked valid target broken: %#v", result)
	}
	assertSamePath(t, result.ResolvedPath, target)
}

func TestValidateSkillTargetReportsMissingTarget(t *testing.T) {
	root := t.TempDir()
	link := filepath.Join(root, "missing")
	if err := os.Symlink(filepath.Join(root, "does-not-exist"), link); err != nil {
		t.Fatal(err)
	}

	result := ValidateSkillTarget(link)
	if !result.Broken {
		t.Fatalf("ValidateSkillTarget() = %#v, want broken", result)
	}
	if !strings.Contains(result.Reason, "resolve symlink:") {
		t.Fatalf("Reason = %q, want resolve symlink error", result.Reason)
	}
}

func TestValidateSkillTargetReportsFileTarget(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "not-a-dir")
	if err := os.WriteFile(target, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "active")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	result := ValidateSkillTarget(link)
	if !result.Broken {
		t.Fatalf("ValidateSkillTarget() = %#v, want broken", result)
	}
	if result.Reason != "target is not a directory" {
		t.Fatalf("Reason = %q", result.Reason)
	}
}

func TestValidateSkillTargetReportsNonSkillDirectory(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "plain-dir")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "active")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	result := ValidateSkillTarget(link)
	if !result.Broken {
		t.Fatalf("ValidateSkillTarget() = %#v, want broken", result)
	}
	if result.Reason != "target is not a skill directory" {
		t.Fatalf("Reason = %q", result.Reason)
	}
}

func makeSkillDir(t *testing.T, root, name string) string {
	t.Helper()

	path := filepath.Join(root, name)
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "SKILL.md"), []byte("---\nname: "+name+"\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
