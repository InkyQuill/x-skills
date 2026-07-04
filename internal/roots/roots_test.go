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

func TestActiveRootsCanBeFilteredByScopeOnly(t *testing.T) {
	cfg := config.Default("/project", "/home/inky")

	filtered := ActiveRoots(cfg, Filter{Scope: "project"})
	if len(filtered) != 3 {
		t.Fatalf("len(filtered) = %d, want 3", len(filtered))
	}
	for _, root := range filtered {
		if root.Scope != "project" {
			t.Fatalf("Scope = %q, want project", root.Scope)
		}
	}
}

func TestActiveRootsCanBeFilteredByTargetOnly(t *testing.T) {
	cfg := config.Default("/project", "/home/inky")

	filtered := ActiveRoots(cfg, Filter{Target: "codex"})
	if len(filtered) != 2 {
		t.Fatalf("len(filtered) = %d, want 2", len(filtered))
	}
	for _, root := range filtered {
		if root.Target != "codex" {
			t.Fatalf("Target = %q, want codex", root.Target)
		}
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
