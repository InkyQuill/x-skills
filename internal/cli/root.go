package cli

import (
	"io"
	"os"

	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type options struct {
	projectRoot      string
	homeDir          string
	archiveRoot      string
	globalAgentsRoot string
	globalClaudeRoot string
	globalCodexRoot  string
	yes              bool

	flags *pflag.FlagSet
}

func Execute(argv []string, stdin io.Reader, stdout, stderr io.Writer) error {
	cmd, err := newRootCommand(stdin, stdout, stderr)
	if err != nil {
		return err
	}
	cmd.SetArgs(argv)
	return cmd.Execute()
}

func newRootCommand(stdin io.Reader, stdout, stderr io.Writer) (*cobra.Command, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	projectRoot, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	cfg := config.Default(projectRoot, homeDir)
	opts := options{
		projectRoot:      cfg.ProjectRoot,
		homeDir:          cfg.HomeDir,
		archiveRoot:      cfg.ArchiveRoot,
		globalAgentsRoot: cfg.GlobalAgentsRoot,
		globalClaudeRoot: cfg.GlobalClaudeRoot,
		globalCodexRoot:  cfg.GlobalCodexRoot,
	}

	root := &cobra.Command{
		Use:           "x-skills",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetIn(stdin)
	root.SetOut(stdout)
	root.SetErr(stderr)

	flags := root.PersistentFlags()
	opts.flags = flags
	flags.StringVar(&opts.projectRoot, "project-root", opts.projectRoot, "project root")
	flags.StringVar(&opts.archiveRoot, "archive-root", opts.archiveRoot, "archive root")
	flags.StringVar(&opts.globalAgentsRoot, "global-root", opts.globalAgentsRoot, "global agents skills root")
	flags.StringVar(&opts.globalClaudeRoot, "claude-global-root", opts.globalClaudeRoot, "global Claude skills root")
	flags.StringVar(&opts.globalCodexRoot, "codex-global-root", opts.globalCodexRoot, "global Codex skills root")
	flags.BoolVarP(&opts.yes, "yes", "y", false, "confirm mutating commands")
	flags.StringVar(&opts.homeDir, "home", opts.homeDir, "home directory")
	if err := flags.MarkHidden("home"); err != nil {
		return nil, err
	}

	root.AddCommand(
		newListCommand(&opts),
		newRepoCommand(&opts),
		newLinkCommand(&opts),
		newMigrateCommand(&opts),
		newUnlinkCommand(&opts),
		newDoctorCommand(&opts),
		newTUICommand(&opts),
	)

	return root, nil
}

func (o options) config() config.Config {
	cfg := config.Default(o.projectRoot, o.homeDir)
	if o.flags == nil {
		return cfg
	}
	if o.flags.Changed("archive-root") {
		cfg.ArchiveRoot = o.archiveRoot
	}
	if o.flags.Changed("global-root") {
		cfg.GlobalAgentsRoot = o.globalAgentsRoot
	}
	if o.flags.Changed("claude-global-root") {
		cfg.GlobalClaudeRoot = o.globalClaudeRoot
	}
	if o.flags.Changed("codex-global-root") {
		cfg.GlobalCodexRoot = o.globalCodexRoot
	}
	return cfg
}
