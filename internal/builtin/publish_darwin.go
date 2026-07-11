//go:build darwin

package builtin

import "golang.org/x/sys/unix"

func publishArchiveNoReplace(staged, destination string) error {
	return unix.RenamexNp(staged, destination, unix.RENAME_EXCL)
}
