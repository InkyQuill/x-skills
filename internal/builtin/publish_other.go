//go:build !linux

package builtin

import (
	"errors"
	"os"
)

func publishArchiveNoReplace(staged, destination string) error {
	lock, err := os.OpenFile(destination+".x-skills-publish-lock", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if err := lock.Close(); err != nil {
		_ = os.Remove(lock.Name())
		return err
	}
	defer func() { _ = os.Remove(lock.Name()) }()

	if _, err := os.Lstat(destination); err == nil {
		return os.ErrExist
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Rename(staged, destination)
}
