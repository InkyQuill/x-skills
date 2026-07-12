//go:build windows

package builtin

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

func publishArchiveNoReplace(staged, destination string) error {
	stagedPath, err := windows.UTF16PtrFromString(staged)
	if err != nil {
		return err
	}
	destinationPath, err := windows.UTF16PtrFromString(destination)
	if err != nil {
		return err
	}
	if err := windows.MoveFile(stagedPath, destinationPath); err != nil {
		if errors.Is(err, windows.ERROR_ALREADY_EXISTS) || errors.Is(err, windows.ERROR_FILE_EXISTS) {
			return os.ErrExist
		}
		return err
	}
	return nil
}
