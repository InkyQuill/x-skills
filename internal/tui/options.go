package tui

import (
	"context"

	"github.com/InkyQuill/x-skills/internal/buildinfo"
	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/doctor"
	"github.com/InkyQuill/x-skills/internal/repo"
)

type dataLoader func(context.Context, config.Config) ([]ActiveGroup, []repo.Skill, []doctor.Issue, map[string][]string, error)

type Options struct {
	ASCII                bool
	BuildInfo            buildinfo.Info
	LatestReleaseChecker buildinfo.LatestReleaseChecker
	loadData             dataLoader
}

func defaultOptions() Options {
	return Options{BuildInfo: buildinfo.Current(), loadData: loadTUIData}
}
