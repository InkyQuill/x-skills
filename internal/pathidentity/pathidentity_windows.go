//go:build windows

package pathidentity

import (
	"strings"
	"unicode/utf16"

	"golang.org/x/sys/windows"
)

// fileNameNormalized requests the normalized final path spelling.
const fileNameNormalized = 0x0

// platformCanonical resolves the final path name through the Windows API,
// stripping \\?\ and \\?\UNC\ prefixes returned by GetFinalPathNameByHandle.
// It falls back to the input path on any filesystem error.
func platformCanonical(path string) (string, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return "", err
	}
	handle, err := windows.CreateFile(
		p,
		0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
	if err != nil {
		return path, nil
	}
	defer windows.CloseHandle(handle)

	buf := make([]uint16, windows.MAX_LONG_PATH)
	n, err := windows.GetFinalPathNameByHandle(
		handle,
		&buf[0],
		uint32(len(buf)),
		fileNameNormalized,
	)
	if err != nil || n == 0 {
		return path, nil
	}
	if n > uint32(len(buf)) {
		buf = make([]uint16, n)
		n, err = windows.GetFinalPathNameByHandle(
			handle,
			&buf[0],
			uint32(len(buf)),
			fileNameNormalized,
		)
		if err != nil || n == 0 {
			return path, nil
		}
	}
	result := string(utf16.Decode(buf[:n]))
	if strings.HasPrefix(result, `\\?\UNC\`) {
		return `\\` + result[len(`\\?\UNC\`):], nil
	}
	return strings.TrimPrefix(result, `\\?\`), nil
}
