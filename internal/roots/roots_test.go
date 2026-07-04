package roots

import (
	"testing"

	"github.com/InkyQuill/x-skills/internal/config"
)

func TestActiveRootsCanBeFiltered(t *testing.T) {
	cfg := config.Default("/project", "/home/inky")

	all := ActiveRoots(cfg, Filter{})
	if len(all) != 6 {
		t.Fatalf("len(all) = %d, want 6", len(all))
	}

	filtered := ActiveRoots(cfg, Filter{Scope: "project", Target: "codex"})
	if len(filtered) != 1 {
		t.Fatalf("len(filtered) = %d, want 1", len(filtered))
	}
	if filtered[0].Label != "./.codex" {
		t.Fatalf("Label = %q", filtered[0].Label)
	}
}

func TestActiveRootsRejectInvalidFilter(t *testing.T) {
	cfg := config.Default("/project", "/home/inky")

	if roots := ActiveRoots(cfg, Filter{Scope: "workspace"}); len(roots) != 0 {
		t.Fatalf("roots = %#v, want none", roots)
	}
	if roots := ActiveRoots(cfg, Filter{Target: "cursor"}); len(roots) != 0 {
		t.Fatalf("roots = %#v, want none", roots)
	}
}
