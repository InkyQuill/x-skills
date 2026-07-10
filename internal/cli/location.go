package cli

import (
	"fmt"
	"strings"

	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/roots"
)

func resolveLocations(cfg config.Config, selectors []string) ([]roots.ActiveRoot, error) {
	if len(selectors) == 0 {
		return nil, fmt.Errorf("at least one --at location is required")
	}

	locations := make([]roots.ActiveRoot, 0, len(selectors))
	for _, selector := range selectors {
		location, err := resolveLocation(cfg, selector)
		if err != nil {
			return nil, err
		}
		locations = append(locations, location)
	}
	return locations, nil
}

func resolveOptionalLocations(cfg config.Config, selectors []string) ([]roots.ActiveRoot, error) {
	if len(selectors) == 0 {
		return roots.ActiveRoots(cfg, roots.Filter{}), nil
	}
	return resolveLocations(cfg, selectors)
}

func resolveOneLocation(cfg config.Config, selectors []string) (roots.ActiveRoot, error) {
	locations, err := resolveLocations(cfg, selectors)
	if err != nil {
		return roots.ActiveRoot{}, err
	}
	if len(locations) != 1 {
		return roots.ActiveRoot{}, fmt.Errorf("choose exactly one --at location")
	}
	return locations[0], nil
}

func optionalOneLocation(cfg config.Config, selectors []string) (*roots.ActiveRoot, error) {
	if len(selectors) == 0 {
		return nil, nil
	}
	location, err := resolveOneLocation(cfg, selectors)
	if err != nil {
		return nil, err
	}
	return &location, nil
}

func resolveLocation(cfg config.Config, selector string) (roots.ActiveRoot, error) {
	trimmed := strings.TrimSpace(selector)
	if trimmed == "" {
		return roots.ActiveRoot{}, fmt.Errorf("empty --at location")
	}

	var matches []roots.ActiveRoot
	for _, root := range roots.ActiveRoots(cfg, roots.Filter{}) {
		if locationMatches(root, trimmed) {
			matches = append(matches, root)
		}
	}

	if len(matches) == 0 {
		return roots.ActiveRoot{}, fmt.Errorf("unknown --at location %q; run x-skills list-roots", selector)
	}
	if len(matches) > 1 {
		return roots.ActiveRoot{}, fmt.Errorf("ambiguous --at location %q; use project:target or global:target", selector)
	}
	return matches[0], nil
}

func locationMatches(root roots.ActiveRoot, selector string) bool {
	return selector == root.Scope+":"+root.Target ||
		strings.EqualFold(selector, root.Label) ||
		selector == scopePrefix(root.Scope)+root.Target
}

func scopePrefix(scope string) string {
	if scope == config.ScopeGlobal {
		return "~"
	}
	return "."
}

func locationKey(root roots.ActiveRoot) string {
	return root.Scope + "\x00" + root.Target
}

func locationSet(locations []roots.ActiveRoot) map[string]bool {
	allowed := make(map[string]bool, len(locations))
	for _, location := range locations {
		allowed[locationKey(location)] = true
	}
	return allowed
}

func pathPrefixSet(locations []roots.ActiveRoot) map[string]bool {
	allowed := make(map[string]bool, len(locations))
	for _, location := range locations {
		allowed[location.Path] = true
	}
	return allowed
}
