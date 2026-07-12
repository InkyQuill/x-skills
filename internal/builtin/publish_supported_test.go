//go:build linux || darwin || windows

package builtin

import "testing"

func TestArchivePublishDoesNotReplaceConcurrentDestination(t *testing.T) {
	testArchivePublishDoesNotReplaceConcurrentDestination(t)
}
