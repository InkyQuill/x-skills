//go:build !linux

package actions

import (
	"fmt"
	"runtime"
)

func renameNoReplace(oldPath, newPath string) error {
	return fmt.Errorf("atomic no-replace rename is unsupported on %s", runtime.GOOS)
}
