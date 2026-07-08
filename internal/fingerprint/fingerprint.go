package fingerprint

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

func Directory(root string) (string, error) {
	var entries []entry
	if err := filepath.WalkDir(root, func(path string, dirEntry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		entries = append(entries, entry{
			path: filepath.ToSlash(rel),
			info: dirEntry,
		})
		return nil
	}); err != nil {
		return "", fmt.Errorf("walk directory: %w", err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].path < entries[j].path
	})

	hash := sha256.New()
	for _, entry := range entries {
		if err := hashEntry(hash, root, entry); err != nil {
			return "", err
		}
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

type entry struct {
	path string
	info fs.DirEntry
}

type hashWriter interface {
	Write([]byte) (int, error)
}

func hashEntry(hash hashWriter, root string, entry entry) error {
	mode := entry.info.Type()
	switch {
	case mode&os.ModeSymlink != 0:
		target, err := os.Readlink(filepath.Join(root, filepath.FromSlash(entry.path)))
		if err != nil {
			return fmt.Errorf("read symlink %q: %w", entry.path, err)
		}
		writeHash(hash, "symlink", entry.path, target)
	case entry.info.IsDir():
		writeHash(hash, "dir", entry.path, "")
	default:
		file, err := os.Open(filepath.Join(root, filepath.FromSlash(entry.path)))
		if err != nil {
			return fmt.Errorf("read file %q: %w", entry.path, err)
		}
		defer func() {
			_ = file.Close()
		}()
		writeHashFilePrefix(hash, "file", entry.path)
		if _, err := io.Copy(hash, file); err != nil {
			return fmt.Errorf("hash file %q: %w", entry.path, err)
		}
		_, _ = hash.Write([]byte{0})
	}

	return nil
}

func writeHash(hash hashWriter, kind, path, value string) {
	_, _ = fmt.Fprintf(hash, "%s\x00%s\x00%d\x00%s\x00", kind, path, len(value), value)
}

func writeHashFilePrefix(hash hashWriter, kind, path string) {
	_, _ = fmt.Fprintf(hash, "%s\x00%s\x00", kind, path)
}
