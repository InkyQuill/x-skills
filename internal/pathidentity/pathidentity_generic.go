//go:build !windows

package pathidentity

// platformCanonical applies OS-level final-form normalization after
// filepath.Abs and filepath.EvalSymlinks have already run. On non-Windows
// platforms it is a no-op.
func platformCanonical(path string) (string, error) {
	return path, nil
}
