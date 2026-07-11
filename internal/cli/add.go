package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/InkyQuill/x-skills/internal/actions"
	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/remote"
	"github.com/InkyQuill/x-skills/internal/roots"
	"github.com/spf13/cobra"
)

type addOptions struct {
	gitURL    string
	ref       string
	all       bool
	noLink    bool
	at        []string
	replace   bool
	archiveAs string
}

type addSelection struct {
	source        remote.AddSource
	names         []string
	sourceDisplay string
}

type addSkillResult struct {
	name   string
	status string
	linked []roots.ActiveRoot
}

func newAddCommand(rootOptions *options) *cobra.Command {
	var opts addOptions
	cmd := &cobra.Command{
		Use:   "add SOURCE [SKILL_NAME...]",
		Short: "Add remote skills to the archive and optionally link them",
		Args: func(cmd *cobra.Command, args []string) error {
			if opts.gitURL == "" && len(args) == 0 {
				return fmt.Errorf("source is required")
			}
			if opts.gitURL != "" && !opts.all && len(args) == 0 {
				return fmt.Errorf("--git requires at least one skill name or --all")
			}
			if opts.gitURL != "" && opts.all && len(args) > 0 {
				return fmt.Errorf("--all cannot be used with skill names")
			}
			if opts.gitURL == "" && opts.all && len(args) > 1 {
				return fmt.Errorf("--all cannot be used with skill names")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAdd(cmd, rootOptions, opts, args)
		},
	}
	cmd.Flags().StringVar(&opts.gitURL, "git", "", "git clone URL")
	cmd.Flags().StringVar(&opts.ref, "ref", "", "git ref to checkout")
	cmd.Flags().BoolVar(&opts.all, "all", false, "add every discovered skill")
	cmd.Flags().BoolVar(&opts.noLink, "no-link", false, "archive only; do not link")
	cmd.Flags().StringArrayVar(&opts.at, "at", nil, "managed root location; repeat for multiple locations")
	cmd.Flags().BoolVar(&opts.replace, "replace", false, "replace an existing archive")
	cmd.Flags().StringVar(&opts.archiveAs, "archive-as", "", "archive name for a single selected skill")
	return cmd
}

func runAdd(cmd *cobra.Command, rootOptions *options, opts addOptions, args []string) error {
	selection, err := resolveAddSelection(opts, args)
	if err != nil {
		return err
	}
	if opts.all && len(selection.names) > 0 {
		return fmt.Errorf("--all cannot be used with skill names")
	}
	if opts.archiveAs != "" && !opts.all && addSelectionCount(selection) != 1 {
		return fmt.Errorf("--archive-as is only valid for exactly one selected skill")
	}
	cfg := rootOptions.config()
	destinations, err := defaultAddLocations(cfg, opts.noLink, opts.at)
	if err != nil {
		return err
	}

	cacheRoot, err := os.MkdirTemp("", "x-skills-add-cache-*")
	if err != nil {
		return fmt.Errorf("create checkout cache: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(cacheRoot)
	}()

	cache := remote.NewCheckoutCache(cacheRoot)
	checkout, err := cache.Checkout(cmd.Context(), selection.source.Source)
	if err != nil {
		return fmt.Errorf("checkout source: %w", err)
	}

	found, err := resolveAddSkills(cmd, rootOptions, opts, selection, checkout)
	if err != nil {
		return err
	}
	if opts.archiveAs != "" && len(found) != 1 {
		return fmt.Errorf("--archive-as is only valid for exactly one selected skill")
	}
	if len(found) == 0 {
		return fmt.Errorf("no skills found in source")
	}

	results, failures := applyAddSkills(cfg, opts, destinations, found)
	if len(found) == 1 && len(failures) == 0 {
		writeAddSingleSuccess(cmd.OutOrStdout(), results[0])
		return nil
	}
	if len(found) == 1 && len(failures) == 1 && len(results) == 0 {
		return failures[0].err
	}

	writeAddSummary(cmd.OutOrStdout(), results, failures)
	if len(failures) > 0 {
		return fmt.Errorf("add failed for %d skill(s)", len(failures))
	}
	return nil
}

func resolveAddSelection(opts addOptions, args []string) (addSelection, error) {
	var sourceArg string
	var explicitNames []string
	if opts.gitURL == "" {
		if len(args) == 0 {
			return addSelection{}, fmt.Errorf("source is required")
		}
		sourceArg = args[0]
		explicitNames = args[1:]
	} else {
		explicitNames = args
	}
	parsed, err := remote.ParseAddSource(sourceArg, opts.gitURL, opts.ref)
	if err != nil {
		return addSelection{}, err
	}
	names := make([]string, 0, len(parsed.Names)+len(explicitNames))
	names = append(names, parsed.Names...)
	names = append(names, explicitNames...)
	sourceDisplay := sourceArg
	if opts.gitURL != "" {
		sourceDisplay = "--git " + opts.gitURL
	}
	if sourceDisplay == "" {
		sourceDisplay = parsed.Source.CloneURL
	}
	return addSelection{source: parsed, names: names, sourceDisplay: sourceDisplay}, nil
}

func addSelectionCount(selection addSelection) int {
	if len(selection.names) > 0 {
		return len(selection.names)
	}
	if selection.source.PreferredPath != "" {
		return 1
	}
	return 0
}

func resolveAddSkills(
	cmd *cobra.Command,
	rootOptions *options,
	opts addOptions,
	selection addSelection,
	checkout remote.Checkout,
) ([]remote.FoundSkill, error) {
	if opts.all {
		found, err := checkout.ListSkillsContext(cmd.Context())
		if err != nil {
			return nil, fmt.Errorf("list skills: %w", err)
		}
		ok, err := confirm(
			cmd,
			rootOptions,
			fmt.Sprintf("Add all %d discovered skill(s)? [y/N]: ", len(found)),
			"add --all requires confirmation; rerun with -y to confirm",
		)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("add canceled")
		}
		return found, nil
	}
	if len(selection.names) == 0 {
		if selection.source.PreferredPath != "" {
			skill, err := checkout.FindSkillContext(
				cmd.Context(),
				"",
				selection.source.PreferredPath,
			)
			if err != nil {
				return nil, fmt.Errorf("find skill at %q: %w", selection.source.PreferredPath, err)
			}
			return []remote.FoundSkill{skill}, nil
		}
		found, err := checkout.ListSkillsContext(cmd.Context())
		if err != nil {
			return nil, fmt.Errorf("list skills: %w", err)
		}
		return nil, addNeedsSkillNameError(selection.sourceDisplay, found)
	}

	found := make([]remote.FoundSkill, 0, len(selection.names))
	for _, name := range selection.names {
		skill, err := checkout.FindSkillContext(cmd.Context(), name, selection.source.PreferredPath)
		if err != nil {
			return nil, fmt.Errorf("find skill %q: %w", name, err)
		}
		found = append(found, skill)
	}
	return found, nil
}

func applyAddSkills(
	cfg config.Config,
	opts addOptions,
	destinations []roots.ActiveRoot,
	found []remote.FoundSkill,
) ([]addSkillResult, []mutationFailure) {
	results := make([]addSkillResult, 0, len(found))
	var failures []mutationFailure
	conflict := remote.ConflictArchiveOnly
	if opts.replace {
		conflict = remote.ConflictReplaceArchive
	}

	for _, skill := range found {
		archiveName := skill.Info.Name
		if opts.archiveAs != "" {
			archiveName = opts.archiveAs
		}
		result, err := remote.ApplyArchive(remote.AddRequest{
			Config:      cfg,
			IncomingDir: skill.SkillDir,
			ArchiveName: archiveName,
			Metadata:    skill.Metadata,
			Conflict:    conflict,
		})
		if err != nil {
			failures = append(failures, mutationFailure{name: archiveName, err: addArchiveErrorWithHint(err)})
			continue
		}

		added := addSkillResult{name: archiveName, status: result.Status}
		for _, destination := range destinations {
			_, err := actions.Link(cfg, actions.LinkRequest{
				Name:   archiveName,
				Scope:  destination.Scope,
				Target: destination.Target,
			})
			if err != nil {
				failures = append(failures, mutationFailure{name: archiveName, err: err})
				continue
			}
			added.linked = append(added.linked, destination)
		}
		results = append(results, added)
	}
	return results, failures
}

func addNeedsSkillNameError(source string, found []remote.FoundSkill) error {
	if len(found) == 0 {
		return fmt.Errorf("no skills found in source")
	}
	names := make([]string, 0, len(found))
	for _, skill := range found {
		names = append(names, skill.Info.Name)
	}
	first := names[0]
	return fmt.Errorf(
		"multiple skills found; specify a name or use --all:\n  found: %s\n  x-skills add %s %s\n  x-skills add %s --all",
		strings.Join(names, ", "),
		source,
		first,
		source,
	)
}

func addArchiveErrorWithHint(err error) error {
	text := err.Error()
	if strings.Contains(text, "archive conflict") ||
		strings.Contains(text, "update available") ||
		strings.Contains(text, "archive destination already exists") {
		return fmt.Errorf("%w; rerun with --replace or inspect in tui", err)
	}
	return err
}

func writeAddSingleSuccess(out io.Writer, result addSkillResult) {
	if result.status == remote.AddStatusSkipped {
		_, _ = fmt.Fprintf(out, "skipped: %s (already archived)\n", result.name)
	} else {
		_, _ = fmt.Fprintf(out, "added: %s\n", result.name)
	}
	if len(result.linked) > 0 {
		_, _ = fmt.Fprintf(out, "linked: %s\n", addDestinationLabels(result.linked))
	}
}

func writeAddSummary(out io.Writer, results []addSkillResult, failures []mutationFailure) {
	_, _ = fmt.Fprintln(out, "Summary:")
	added, skipped := addResultNamesByStatus(results)
	_, _ = fmt.Fprintf(out, "added: %s\n", addSummaryValue(added))
	_, _ = fmt.Fprintf(out, "skipped: %s\n", addSummaryValue(skipped))
	linked := addLinkedSummaries(results)
	_, _ = fmt.Fprintf(out, "linked: %s\n", addSummaryValue(linked))
	if len(failures) == 0 {
		_, _ = fmt.Fprintln(out, "failed: none")
		return
	}
	for _, failure := range failures {
		_, _ = fmt.Fprintf(out, "failed: %s (%v)\n", failure.name, failure.err)
	}
}

func addSummaryValue(values []string) string {
	if len(values) == 0 {
		return "none"
	}
	return strings.Join(values, ", ")
}

func addResultNamesByStatus(results []addSkillResult) ([]string, []string) {
	var added []string
	var skipped []string
	for _, result := range results {
		if result.status == remote.AddStatusSkipped {
			skipped = append(skipped, result.name+" (already archived)")
			continue
		}
		added = append(added, result.name)
	}
	return added, skipped
}

func addLinkedSummaries(results []addSkillResult) []string {
	var linked []string
	for _, result := range results {
		for _, destination := range result.linked {
			linked = append(linked, result.name+" -> "+addDestinationLabel(destination))
		}
	}
	return linked
}

func defaultAddLocations(cfg config.Config, noLink bool, selectors []string) ([]roots.ActiveRoot, error) {
	if noLink && len(selectors) > 0 {
		return nil, fmt.Errorf("--no-link cannot be used with --at locations")
	}
	if noLink {
		return nil, nil
	}
	if len(selectors) == 0 {
		location, err := resolveLocation(cfg, config.ScopeProject+":"+config.TargetAgents)
		if err != nil {
			return nil, err
		}
		return []roots.ActiveRoot{location}, nil
	}
	return resolveLocations(cfg, selectors)
}

func addDestinationLabels(destinations []roots.ActiveRoot) string {
	labels := make([]string, 0, len(destinations))
	for _, destination := range destinations {
		labels = append(labels, addDestinationLabel(destination))
	}
	return strings.Join(labels, ", ")
}

func addDestinationLabel(destination roots.ActiveRoot) string {
	if destination.Label != "" {
		return destination.Label
	}
	scope := "."
	if destination.Scope == config.ScopeGlobal {
		scope = "~"
	}
	switch destination.Target {
	case config.TargetAgents:
		return scope + "Ag"
	case config.TargetClaude:
		return scope + "Cl"
	case config.TargetCodex:
		return scope + "Cd"
	default:
		return scope + filepath.Base(destination.Target)
	}
}
