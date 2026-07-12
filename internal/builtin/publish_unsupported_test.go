//go:build !linux && !darwin && !windows

package builtin

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/InkyQuill/x-skills/internal/config"
)

func TestArchiveRefusesUnsafePublishAndCleansStaging(t *testing.T) {
	cfg := config.Default(t.TempDir(), t.TempDir())

	_, err := Archive(cfg, []string{"x-find-skills"})
	if !errors.Is(err, ErrAtomicPublishUnsupported) {
		t.Fatalf("Archive() error = %v, want ErrAtomicPublishUnsupported", err)
	}
	entries, err := os.ReadDir(cfg.ArchiveSkillsRoot())
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() == "x-find-skills" || strings.HasPrefix(entry.Name(), ".x-find-skills-") {
			t.Fatalf("unsupported publish left archive content at %q", filepath.Join(cfg.ArchiveSkillsRoot(), entry.Name()))
		}
	}
}
