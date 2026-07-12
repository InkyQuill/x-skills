package xskills

import "embed"

// BuiltInSkills contains the canonical skills shipped with x-skills.
//
//go:embed all:skills/*
var BuiltInSkills embed.FS
