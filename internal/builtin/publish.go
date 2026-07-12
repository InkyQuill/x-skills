package builtin

import (
	"fmt"
	"runtime"
)

func publishArchiveUnsupported(_, _ string) error {
	return fmt.Errorf("%w on %s", ErrAtomicPublishUnsupported, runtime.GOOS)
}
