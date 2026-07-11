package builtin

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	xskills "github.com/InkyQuill/x-skills"
	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/fingerprint"
	"github.com/InkyQuill/x-skills/internal/repo"
)

var ErrArchiveConflict = errors.New("built-in archive conflict")

type Skill struct {
	Name string
}

var builtInSkills fs.FS = xskills.BuiltInSkills
var copyBuiltIn = copyEmbeddedDir
var publishArchive = publishArchiveNoReplace

func List() ([]Skill, error) {
	entries, err := fs.ReadDir(builtInSkills, "skills")
	if err != nil {
		return nil, fmt.Errorf("list built-in skills: %w", err)
	}

	found := make([]Skill, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if err := validateName(entry.Name()); err != nil {
			return nil, err
		}
		found = append(found, Skill{Name: entry.Name()})
	}
	sort.Slice(found, func(i, j int) bool { return found[i].Name < found[j].Name })
	return found, nil
}

func Archive(cfg config.Config, names []string) ([]string, error) {
	catalog, err := List()
	if err != nil {
		return nil, err
	}
	available := make(map[string]struct{}, len(catalog))
	for _, skill := range catalog {
		available[skill.Name] = struct{}{}
	}

	archived := make([]string, 0, len(names))
	for _, name := range names {
		if err := validateName(name); err != nil {
			return archived, err
		}
		if _, ok := available[name]; !ok {
			return archived, fmt.Errorf("unknown built-in skill %q", name)
		}
		changed, err := archiveOne(cfg, name)
		if err != nil {
			return archived, err
		}
		if changed {
			archived = append(archived, name)
		}
	}
	return archived, nil
}

func validateName(name string) error {
	if !strings.HasPrefix(name, "x-") {
		return fmt.Errorf("invalid built-in skill name %q: expected x- prefix", name)
	}
	if err := repo.ValidateName(name); err != nil {
		return fmt.Errorf("invalid built-in skill name %q: %w", name, err)
	}
	return nil
}

func archiveOne(cfg config.Config, name string) (bool, error) {
	root := cfg.ArchiveSkillsRoot()
	if err := os.MkdirAll(root, 0o755); err != nil {
		return false, fmt.Errorf("create archive root: %w", err)
	}
	tempDir, err := os.MkdirTemp(root, "."+name+"-")
	if err != nil {
		return false, fmt.Errorf("create built-in archive temp: %w", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	if err := copyBuiltIn(path.Join("skills", name), tempDir); err != nil {
		return false, err
	}

	destination := filepath.Join(root, name)
	_, err = os.Lstat(destination)
	switch {
	case errors.Is(err, os.ErrNotExist):
		if err := publishArchive(tempDir, destination); err != nil {
			if errors.Is(err, os.ErrExist) {
				return false, fmt.Errorf("%w: %s", ErrArchiveConflict, name)
			}
			return false, fmt.Errorf("install built-in archive %q: %w", name, err)
		}
		return true, nil
	case err != nil:
		return false, fmt.Errorf("inspect built-in archive %q: %w", name, err)
	}

	same, err := directoriesMatch(tempDir, destination)
	if err != nil {
		return false, fmt.Errorf("compare built-in archive %q: %w", name, err)
	}
	if same {
		return false, nil
	}
	return false, fmt.Errorf("%w: %s", ErrArchiveConflict, name)
}

func directoriesMatch(first, second string) (bool, error) {
	firstFingerprint, err := fingerprint.Directory(first)
	if err != nil {
		return false, err
	}
	secondFingerprint, err := fingerprint.Directory(second)
	if err != nil {
		return false, err
	}
	return firstFingerprint == secondFingerprint, nil
}

func copyEmbeddedDir(source, destination string) error {
	return fs.WalkDir(builtInSkills, source, func(sourcePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if sourcePath == source {
			return nil
		}
		relative := strings.TrimPrefix(sourcePath, source+"/")
		target := filepath.Join(destination, filepath.FromSlash(relative))
		if entry.IsDir() {
			if err := os.Mkdir(target, 0o755); err != nil {
				return fmt.Errorf("create built-in directory %q: %w", relative, err)
			}
			return nil
		}
		if entry.Type()&fs.ModeType != 0 {
			return fmt.Errorf("unsupported embedded file type %q", sourcePath)
		}
		contents, err := fs.ReadFile(builtInSkills, sourcePath)
		if err != nil {
			return fmt.Errorf("read embedded file %q: %w", sourcePath, err)
		}
		if err := os.WriteFile(target, contents, 0o644); err != nil {
			return fmt.Errorf("write built-in file %q: %w", relative, err)
		}
		return nil
	})
}
