package fingerprint

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDirectoryFingerprintIgnoresWalkOrder(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}

	first, err := Directory(dir)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Directory(dir)
	if err != nil {
		t.Fatal(err)
	}
	if first == "" || first != second {
		t.Fatalf("fingerprints differ: %q %q", first, second)
	}
}

func TestDirectoryFingerprintIncludesFileContents(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "skill.md")
	if err := os.WriteFile(path, []byte("first"), 0o644); err != nil {
		t.Fatal(err)
	}

	first, err := Directory(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("second"), 0o644); err != nil {
		t.Fatal(err)
	}
	second, err := Directory(dir)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatalf("fingerprint did not change: %q", first)
	}
}

func TestDirectoryFingerprintHashesSymlinkTarget(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(dir, "link")
	if err := os.Symlink("target-a", link); err != nil {
		t.Fatal(err)
	}

	first, err := Directory(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("target-b", link); err != nil {
		t.Fatal(err)
	}
	second, err := Directory(dir)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatalf("fingerprint did not change: %q", first)
	}
}
