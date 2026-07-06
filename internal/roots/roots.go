package roots

import (
	"slices"

	"github.com/InkyQuill/x-skills/internal/config"
)

type ActiveRoot struct {
	Scope  string
	Target string
	Path   string
	Label  string
}

type Filter struct {
	Scope  string
	Target string
}

func ActiveRoots(cfg config.Config, filter Filter) []ActiveRoot {
	if filter.Scope != "" && !slices.Contains(config.Scopes, filter.Scope) {
		return nil
	}
	if filter.Target != "" && !slices.Contains(config.Targets, filter.Target) {
		return nil
	}

	var roots []ActiveRoot
	for _, scope := range config.Scopes {
		if filter.Scope != "" && scope != filter.Scope {
			continue
		}
		for _, target := range config.Targets {
			if filter.Target != "" && target != filter.Target {
				continue
			}
			path, err := cfg.ActiveRoot(scope, target)
			if err != nil {
				continue
			}
			roots = append(roots, ActiveRoot{
				Scope:  scope,
				Target: target,
				Path:   path,
				Label:  config.LocationLabel(scope, target),
			})
		}
	}

	return roots
}
