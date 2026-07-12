//go:build !linux && !darwin && !windows

package actions

import (
	"fmt"
	"runtime"
)

// renameNoReplace reports unsupported platforms outside Linux, Darwin, and Windows.
func renameNoReplace(oldPath, newPath string) error {
	return fmt.Errorf("atomic no-replace rename is unsupported on %s", runtime.GOOS)
}
