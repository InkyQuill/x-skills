package symlinkcheck

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/InkyQuill/x-skills/internal/pathidentity"
	"github.com/InkyQuill/x-skills/internal/skills"
)

type Result struct {
	ResolvedPath string
	Reason       string
	Broken       bool
}

func ValidateSkillTarget(path string) Result {
	resolvedPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return Result{Broken: true, Reason: fmt.Sprintf("resolve symlink: %v", err)}
	}
	canonical, err := pathidentity.Canonical(resolvedPath)
	if err != nil {
		return Result{Broken: true, Reason: fmt.Sprintf("canonicalize target: %v", err)}
	}
	resolvedPath = canonical

	info, err := os.Stat(resolvedPath)
	if err != nil {
		return Result{Broken: true, Reason: fmt.Sprintf("stat target: %v", err)}
	}
	if !info.IsDir() {
		return Result{Broken: true, Reason: "target is not a directory"}
	}
	if !skills.IsDir(resolvedPath) {
		return Result{Broken: true, Reason: "target is not a skill directory"}
	}

	return Result{ResolvedPath: resolvedPath}
}
