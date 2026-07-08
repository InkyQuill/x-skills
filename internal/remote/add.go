package remote

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/fingerprint"
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

	incomingFP, err := fingerprint.Directory(incomingDir)
	if err != nil {
		return ArchivePlan{}, fmt.Errorf("fingerprint incoming: %w", err)
	}
	archiveFP, err := fingerprint.Directory(archivePath)
	if err != nil {
		return ArchivePlan{}, fmt.Errorf("fingerprint archive: %w", err)
	}
	if incomingFP == archiveFP {
		return ArchivePlan{State: ArchiveStateArchived, ArchivePath: archivePath, Existing: &existing}, nil
	}
	return ArchivePlan{State: ArchiveStateUpdateAvailable, ArchivePath: archivePath, Existing: &existing}, nil
}

func ApplyArchive(req AddRequest) (AddResult, error) {
	if req.Conflict == ConflictCancel {
		return AddResult{Name: req.ArchiveName, Status: AddStatusSkipped}, nil
	}
	if req.Conflict == "" {
		req.Conflict = ConflictReplaceArchive
	}

	archivePath, err := archiveSkillPath(req.Config, req.ArchiveName)
	if err != nil {
		return AddResult{}, err
	}
	if req.Conflict == ConflictReplaceArchive {
		if err := os.RemoveAll(archivePath); err != nil {
			return AddResult{}, fmt.Errorf("replace archive: %w", err)
		}
	}
	if err := copyDir(req.IncomingDir, archivePath); err != nil {
		return AddResult{}, err
	}
	if err := WriteSourceMetadata(archivePath, req.Metadata); err != nil {
		return AddResult{}, err
	}
	return AddResult{Name: req.ArchiveName, Path: archivePath, Status: AddStatusArchived}, nil
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

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, dirEntry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
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
