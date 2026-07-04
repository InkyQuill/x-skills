package roots

import "github.com/InkyQuill/x-skills/internal/config"

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
	if filter.Scope != "" && !contains(config.Scopes, filter.Scope) {
		return nil
	}
	if filter.Target != "" && !contains(config.Targets, filter.Target) {
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
			roots = append(roots, ActiveRoot{
				Scope:  scope,
				Target: target,
				Path:   cfg.ActiveRoot(scope, target),
				Label:  config.LocationLabel(scope, target),
			})
		}
	}

	return roots
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
