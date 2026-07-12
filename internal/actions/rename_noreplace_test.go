//go:build linux || darwin || windows

package actions

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRenameNoReplaceMovesWhenDestinationMissing(t *testing.T) {
	root := t.TempDir()
	oldPath := filepath.Join(root, "old")
	newPath := filepath.Join(root, "new")
	if err := os.WriteFile(oldPath, []byte("old content"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := renameNoReplace(oldPath, newPath); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(oldPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("old path stat error = %v, want not exist", err)
	}
	data, err := os.ReadFile(newPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "old content" {
		t.Fatalf("new path content = %q, want old content", data)
	}
}

func TestRenameNoReplaceRefusesExistingDestination(t *testing.T) {
	root := t.TempDir()
	oldPath := filepath.Join(root, "old")
	newPath := filepath.Join(root, "new")
	if err := os.WriteFile(oldPath, []byte("old content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newPath, []byte("new content"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := renameNoReplace(oldPath, newPath); err == nil {
		t.Fatal("expected existing destination error")
	}

	oldData, err := os.ReadFile(oldPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(oldData) != "old content" {
		t.Fatalf("old path content = %q, want old content", oldData)
	}
	newData, err := os.ReadFile(newPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(newData) != "new content" {
		t.Fatalf("new path content = %q, want new content", newData)
	}
}
