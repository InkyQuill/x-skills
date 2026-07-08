package remote

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/skills"
)

const (
	ArchiveStateNotArchived     = "not archived"
	ArchiveStateArchived        = "archived"
	ArchiveStateUpdateAvailable = "update available"
	ArchiveStateNameConflict    = "name conflict"

	ConflictReplaceArchive = "replace-archive"
	ConflictRenameIncoming = "rename-incoming"
	ConflictCancel         = "cancel"

	AddStatusArchived = "archived"
	AddStatusUpdated  = "updated"
	AddStatusSkipped  = "skipped"
)

type ArchivePlan struct {
	State       string
	ArchivePath string
	Existing    *SourceMetadata
}

type AddRequest struct {
	Config      config.Config
	IncomingDir string
	ArchiveName string
	Metadata    SourceMetadata
	Conflict    string
}

type AddResult struct {
	Name   string
	Path   string
	Status string
}

var renamePath = os.Rename

func PlanArchive(
	cfg config.Config,
	incomingDir string,
	archiveName string,
	meta SourceMetadata,
) (ArchivePlan, error) {
	archivePath, err := archiveSkillPath(cfg, archiveName)
	if err != nil {
		return ArchivePlan{}, err
	}
	if !hasArchivedSkill(cfg, archiveName) {
		return ArchivePlan{State: ArchiveStateNotArchived, ArchivePath: archivePath}, nil
	}

	existing, ok, err := ReadSourceMetadata(archivePath)
	if err != nil {
		return ArchivePlan{}, err
	}
	if !ok || !existing.SameIdentity(meta) {
		return ArchivePlan{State: ArchiveStateNameConflict, ArchivePath: archivePath}, nil
	}

	incomingFP, err := archiveContentFingerprint(incomingDir)
	if err != nil {
		return ArchivePlan{}, fmt.Errorf("fingerprint incoming: %w", err)
	}
	archiveFP, err := archiveContentFingerprint(archivePath)
	if err != nil {
		return ArchivePlan{}, fmt.Errorf("fingerprint archive: %w", err)
	}
	if incomingFP == archiveFP {
		return ArchivePlan{State: ArchiveStateArchived, ArchivePath: archivePath, Existing: &existing}, nil
	}
	return ArchivePlan{State: ArchiveStateUpdateAvailable, ArchivePath: archivePath, Existing: &existing}, nil
}

func ApplyArchive(req AddRequest) (AddResult, error) {
	switch req.Conflict {
	case ConflictCancel:
		return AddResult{Name: req.ArchiveName, Status: AddStatusSkipped}, nil
	case "", ConflictReplaceArchive, ConflictRenameIncoming:
	default:
		return AddResult{}, fmt.Errorf("unknown archive conflict %q", req.Conflict)
	}

	archivePath, err := archiveSkillPath(req.Config, req.ArchiveName)
	if err != nil {
		return AddResult{}, err
	}
	existed, err := pathExists(archivePath)
	if err != nil {
		return AddResult{}, err
	}
	if req.Conflict == ConflictRenameIncoming && existed {
		return AddResult{}, fmt.Errorf("archive destination already exists: %s", req.ArchiveName)
	}

	tempPath, err := prepareArchiveTemp(req)
	if err != nil {
		return AddResult{}, err
	}
	cleanupTemp := true
	defer func() {
		if cleanupTemp {
			_ = os.RemoveAll(tempPath)
		}
	}()

	if err := installArchiveTemp(req, tempPath, archivePath, existed); err != nil {
		return AddResult{}, err
	}
	cleanupTemp = false

	status := AddStatusArchived
	if existed && (req.Conflict == "" || req.Conflict == ConflictReplaceArchive) {
		status = AddStatusUpdated
	}
	return AddResult{Name: req.ArchiveName, Path: archivePath, Status: status}, nil
}

func archiveSkillPath(cfg config.Config, name string) (string, error) {
	if err := validateArchiveName(name); err != nil {
		return "", err
	}
	return filepath.Join(cfg.ArchiveSkillsRoot(), name), nil
}

func validateArchiveName(name string) error {
	if name == "" {
		return fmt.Errorf("invalid skill name: %q", name)
	}
	if filepath.IsAbs(name) || name == "." || name == ".." || filepath.Clean(name) != name {
		return fmt.Errorf("invalid skill name: %q", name)
	}
	if strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("invalid skill name: %q", name)
	}
	return nil
}

func hasArchivedSkill(cfg config.Config, name string) bool {
	path, err := archiveSkillPath(cfg, name)
	if err != nil {
		return false
	}
	return skills.IsDir(path)
}

func pathExists(path string) (bool, error) {
	if _, err := os.Lstat(path); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("stat archive destination: %w", err)
	}
	return true, nil
}

func prepareArchiveTemp(req AddRequest) (string, error) {
	root := req.Config.ArchiveSkillsRoot()
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", fmt.Errorf("create archive root: %w", err)
	}
	tempPath, err := os.MkdirTemp(root, "."+req.ArchiveName+"-")
	if err != nil {
		return "", fmt.Errorf("create archive temp: %w", err)
	}
	cleanupTemp := true
	defer func() {
		if cleanupTemp {
			_ = os.RemoveAll(tempPath)
		}
	}()

	if err := copyDir(req.IncomingDir, tempPath); err != nil {
		return "", err
	}
	if err := WriteSourceMetadata(tempPath, req.Metadata); err != nil {
		return "", err
	}

	cleanupTemp = false
	return tempPath, nil
}

func installArchiveTemp(req AddRequest, tempPath, archivePath string, existed bool) error {
	replace := req.Conflict == "" || req.Conflict == ConflictReplaceArchive
	if !replace || !existed {
		if err := renamePath(tempPath, archivePath); err != nil {
			return fmt.Errorf("install archive: %w", err)
		}
		return nil
	}

	backupPath, err := reserveArchiveBackup(req.Config, req.ArchiveName)
	if err != nil {
		return err
	}
	backupActive := false
	defer func() {
		if backupActive {
			_ = os.RemoveAll(backupPath)
		}
	}()

	if err := renamePath(archivePath, backupPath); err != nil {
		return fmt.Errorf("backup archive: %w", err)
	}
	backupActive = true

	if err := renamePath(tempPath, archivePath); err != nil {
		if restoreErr := renamePath(backupPath, archivePath); restoreErr != nil {
			return fmt.Errorf("install archive: %w; restore backup: %v", err, restoreErr)
		}
		backupActive = false
		return fmt.Errorf("install archive: %w", err)
	}

	if err := os.RemoveAll(backupPath); err != nil {
		return fmt.Errorf("remove archive backup: %w", err)
	}
	backupActive = false
	return nil
}

func reserveArchiveBackup(cfg config.Config, archiveName string) (string, error) {
	root := cfg.ArchiveSkillsRoot()
	backupPath, err := os.MkdirTemp(root, "."+archiveName+"-backup-")
	if err != nil {
		return "", fmt.Errorf("create archive backup: %w", err)
	}
	if err := os.Remove(backupPath); err != nil {
		return "", fmt.Errorf("reserve archive backup: %w", err)
	}
	return backupPath, nil
}

func archiveContentFingerprint(root string) (string, error) {
	var entries []contentFingerprintEntry
	if err := filepath.WalkDir(root, func(path string, dirEntry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == MetadataFile {
			return nil
		}
		entries = append(entries, contentFingerprintEntry{
			path: rel,
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
		if err := hashContentFingerprintEntry(hash, root, entry); err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

type contentFingerprintEntry struct {
	path string
	info fs.DirEntry
}

func hashContentFingerprintEntry(hash io.Writer, root string, entry contentFingerprintEntry) error {
	mode := entry.info.Type()
	switch {
	case mode&os.ModeSymlink != 0:
		target, err := os.Readlink(filepath.Join(root, filepath.FromSlash(entry.path)))
		if err != nil {
			return fmt.Errorf("read symlink %q: %w", entry.path, err)
		}
		writeContentFingerprint(hash, "symlink", entry.path, target)
	case entry.info.IsDir():
		writeContentFingerprint(hash, "dir", entry.path, "")
	default:
		info, err := entry.info.Info()
		if err != nil {
			return fmt.Errorf("stat file %q: %w", entry.path, err)
		}
		file, err := os.Open(filepath.Join(root, filepath.FromSlash(entry.path)))
		if err != nil {
			return fmt.Errorf("read file %q: %w", entry.path, err)
		}
		defer func() {
			_ = file.Close()
		}()
		writeContentFingerprintFilePrefix(hash, "file", entry.path, info.Size())
		if _, err := io.Copy(hash, file); err != nil {
			return fmt.Errorf("hash file %q: %w", entry.path, err)
		}
		_, _ = hash.Write([]byte{0})
	}
	return nil
}

func writeContentFingerprint(hash io.Writer, kind, path, value string) {
	_, _ = fmt.Fprintf(hash, "%s\x00%s\x00%d\x00%s\x00", kind, path, len(value), value)
}

func writeContentFingerprintFilePrefix(hash io.Writer, kind, path string, size int64) {
	_, _ = fmt.Fprintf(hash, "%s\x00%s\x00%d\x00", kind, path, size)
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, dirEntry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		target := filepath.Join(dst, rel)

		info, err := dirEntry.Info()
		if err != nil {
			return fmt.Errorf("stat %q: %w", path, err)
		}
		if dirEntry.IsDir() {
			if err := os.MkdirAll(target, info.Mode().Perm()); err != nil {
				return fmt.Errorf("create directory %q: %w", target, err)
			}
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported file type in incoming skill: %s", rel)
		}

		if err := copyFile(path, target, info.Mode().Perm()); err != nil {
			return err
		}
		return nil
	})
}

func copyFile(src, dst string, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("create parent directory %q: %w", filepath.Dir(dst), err)
	}

	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source file %q: %w", src, err)
	}
	defer func() {
		_ = in.Close()
	}()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("open destination file %q: %w", dst, err)
	}
	defer func() {
		_ = out.Close()
	}()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy file %q: %w", src, err)
	}
	if err := out.Chmod(mode); err != nil {
		return fmt.Errorf("set file mode %q: %w", dst, err)
	}
	return nil
}
