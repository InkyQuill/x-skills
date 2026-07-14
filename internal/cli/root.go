package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/InkyQuill/x-skills/internal/buildinfo"
	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const skipConfigAnnotation = "x-skills.io/skip-config"

type options struct {
	projectRoot          string
	homeDir              string
	archiveRoot          string
	yes                  bool
	no                   bool
	noInput              bool
	json                 bool
	buildInfo            buildinfo.Info
	latestReleaseChecker buildinfo.LatestReleaseChecker

	flags  *pflag.FlagSet
	loaded *config.Config
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
		projectRoot:          cfg.ProjectRoot,
		homeDir:              cfg.HomeDir,
		archiveRoot:          cfg.ArchiveRoot,
		buildInfo:            buildinfo.Current(),
		latestReleaseChecker: buildinfo.NewGitHubReleaseChecker(nil),
	}

	root := &cobra.Command{
		Use:           "x-skills",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if opts.yes && opts.no {
				return fmt.Errorf("--yes and --no are mutually exclusive")
			}
			if commandSkipsConfig(cmd) {
				return nil
			}
			_, err := opts.configE()
			return err
		},
	}
	root.SetIn(stdin)
	root.SetOut(stdout)
	root.SetErr(stderr)

	flags := root.PersistentFlags()
	opts.flags = flags
	flags.StringVar(&opts.projectRoot, "project-root", opts.projectRoot, "project root")
	flags.StringVar(&opts.archiveRoot, "archive-root", opts.archiveRoot, "archive root")
	flags.BoolVarP(&opts.yes, "yes", "y", false, "confirm mutating commands")
	flags.BoolVarP(&opts.no, "no", "n", false, "decline confirmations")
	flags.BoolVar(&opts.noInput, "no-input", false, "fail instead of prompting")
	flags.BoolVarP(&opts.json, "json", "j", false, "print JSON output for supported read-only commands")
	flags.StringVar(&opts.homeDir, "home", opts.homeDir, "home directory")
	if err := flags.MarkHidden("home"); err != nil {
		return nil, err
	}

	root.AddCommand(
		newAddCommand(&opts),
		newPreviewCommand(&opts),
		newSearchCommand(&opts),
		newListCommand(&opts),
		newListRootsCommand(&opts),
		newValidateCommand(&opts),
		newRepoCommand(&opts),
		newLinkCommand(&opts),
		newMigrateCommand(&opts),
		newUnlinkCommand(&opts),
		newRecommendCommand(&opts),
		newUnrecommendCommand(&opts),
		newRestoreCommand(&opts),
		newSyncCommand(&opts),
		newDoctorCommand(&opts),
		newTUICommand(&opts),
		newVersionCommand(opts.buildInfo),
	)

	return root, nil
}

func commandSkipsConfig(cmd *cobra.Command) bool {
	_, ok := cmd.Annotations[skipConfigAnnotation]
	return ok
}

func (o *options) config() config.Config {
	cfg, err := o.configE()
	if err != nil {
		panic(err)
	}
	return cfg
}

func (o *options) configE() (config.Config, error) {
	if o.loaded != nil {
		return *o.loaded, nil
	}
	cfg := config.Default(o.projectRoot, o.homeDir)
	if o.flags != nil {
		if o.flags.Changed("archive-root") {
			cfg.ArchiveRoot = o.archiveRoot
		}
	}
	loaded, err := config.LoadGlobal(cfg)
	if err != nil {
		return config.Config{}, err
	}
	o.loaded = &loaded
	return loaded, nil
}
