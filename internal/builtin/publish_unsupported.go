//go:build !linux && !darwin && !windows

package builtin

func publishArchiveNoReplace(staged, destination string) error {
	return publishArchiveUnsupported(staged, destination)
}
