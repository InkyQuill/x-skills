package tui

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/InkyQuill/x-skills/internal/remote"
)

type directoryDiff struct {
	ActivePath  string
	ArchivePath string
	Files       []diffFile
}

type diffFile struct {
	Path string
	Kind string
	Text string
}

func buildDirectoryDiff(active, archive string) (directoryDiff, error) {
	activeFiles, err := readDiffFiles(active)
	if err != nil {
		return directoryDiff{}, err
	}
	archiveFiles, err := readDiffFiles(archive)
	if err != nil {
		return directoryDiff{}, err
	}
	keys := mergedKeys(activeFiles, archiveFiles)
	result := directoryDiff{ActivePath: active, ArchivePath: archive}
	for _, key := range keys {
		activeData, activeOK := activeFiles[key]
		archiveData, archiveOK := archiveFiles[key]
		if activeOK && archiveOK && string(activeData) == string(archiveData) {
			continue
		}
		file := diffFile{Path: key, Kind: "changed"}
		switch {
		case activeOK && !archiveOK:
			file.Kind = "added"
			file.Text = fullFileDiff("", string(activeData))
		case !activeOK && archiveOK:
			file.Kind = "removed"
			file.Text = fullFileDiff(string(archiveData), "")
		case isBinary(activeData) || isBinary(archiveData):
			file.Kind = "binary"
			file.Text = binaryMetadata(activeData, archiveData)
		default:
			file.Text = fullFileDiff(string(archiveData), string(activeData))
		}
		result.Files = append(result.Files, file)
	}
	return result, nil
}

func readDiffFiles(root string) (map[string][]byte, error) {
	files := map[string][]byte{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if filepath.ToSlash(rel) == remote.MetadataFile {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			files[filepath.ToSlash(rel)] = []byte("unreadable: " + err.Error())
			return nil
		}
		files[filepath.ToSlash(rel)] = data
		return nil
	})
	return files, err
}

func mergedKeys(left, right map[string][]byte) []string {
	seen := map[string]bool{}
	var keys []string
	for key := range left {
		keys = append(keys, key)
		seen[key] = true
	}
	for key := range right {
		if !seen[key] {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func isBinary(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	if !utf8.Valid(data) {
		return true
	}
	for _, b := range data {
		if b == 0 {
			return true
		}
	}
	return false
}

func fullFileDiff(archiveText, activeText string) string {
	var lines []string
	lines = append(lines, "--- archive", "+++ active")
	archiveLines := strings.Split(strings.TrimSuffix(archiveText, "\n"), "\n")
	activeLines := strings.Split(strings.TrimSuffix(activeText, "\n"), "\n")
	max := len(archiveLines)
	if len(activeLines) > max {
		max = len(activeLines)
	}
	for i := range max {
		var archiveLine, activeLine string
		if i < len(archiveLines) {
			archiveLine = archiveLines[i]
		}
		if i < len(activeLines) {
			activeLine = activeLines[i]
		}
		switch {
		case archiveLine == activeLine:
			lines = append(lines, " "+archiveLine)
		case archiveLine == "":
			lines = append(lines, "+"+activeLine)
		case activeLine == "":
			lines = append(lines, "-"+archiveLine)
		default:
			lines = append(lines, "-"+archiveLine, "+"+activeLine)
		}
	}
	return strings.Join(lines, "\n")
}

func binaryMetadata(active, archive []byte) string {
	return fmt.Sprintf(
		"Binary file\narchive: %d B  sha256: %s\nactive:  %d B  sha256: %s\n\nNo text diff is available.",
		len(archive), shortSHA(archive), len(active), shortSHA(active),
	)
}

func shortSHA(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])[:12]
}
