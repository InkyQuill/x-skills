package roots

import (
	"slices"

	"github.com/InkyQuill/x-skills/internal/config"
)

type ActiveRoot struct {
	Scope     string
	Target    string
	Path      string
	Label     string
	Consumers []string
}

type Filter struct {
	Scope  string
	Target string
}

func ActiveRoots(cfg config.Config, filter Filter) []ActiveRoot {
	if filter.Scope != "" && !config.ValidScope(filter.Scope) {
		return nil
	}

	var roots []ActiveRoot
	for _, managed := range cfg.ManagedRoots() {
		if !managed.Enabled {
			continue
		}
		if filter.Scope != "" && managed.Scope != filter.Scope {
			continue
		}
		if filter.Target != "" && managed.Target != filter.Target {
			continue
		}
		roots = append(roots, ActiveRoot{
			Scope:     managed.Scope,
			Target:    managed.Target,
			Path:      managed.Path,
			Label:     managed.Label,
			Consumers: slices.Clone(managed.Consumers),
		})
	}

	return roots
}
