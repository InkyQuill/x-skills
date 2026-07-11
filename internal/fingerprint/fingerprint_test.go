package fingerprint

import (
	"crypto/sha256"
	"encoding/hex"
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

func TestDirectoryFingerprintFramesFileContentWithSize(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "skill.md"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Directory(dir)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.New()
	_, _ = hash.Write([]byte("file\x00skill.md\x005\x00hello"))
	want := hex.EncodeToString(hash.Sum(nil))
	if got != want {
		t.Fatalf("Directory() = %q, want size-framed file hash %q", got, want)
	}
}

func TestDirectoryFingerprintIncludesRelativePaths(t *testing.T) {
	firstDir := t.TempDir()
	secondDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(firstDir, "a.txt"), []byte("same"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(secondDir, "b.txt"), []byte("same"), 0o644); err != nil {
		t.Fatal(err)
	}

	first, err := Directory(firstDir)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Directory(secondDir)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatalf("fingerprint ignored relative paths: %q", first)
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
