package actions

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"

	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/repo"
	"gopkg.in/yaml.v3"
)

type RenameResult struct {
	ArchivePath                string
	RelinkedPaths              []string
	ManifestUpdates            []string
	OtherProjectsMayUseArchive bool
}

type renameFilesystem struct {
	rename  func(string, string) error
	remove  func(string) error
	symlink func(string, string) error
}

var renameArchiveFilesystem = renameFilesystem{
	rename:  os.Rename,
	remove:  os.Remove,
	symlink: os.Symlink,
}

type renameManifest struct {
	path    string
	label   string
	data    []byte
	mode    os.FileMode
	next    []byte
	changed bool
}

type renameUsage struct {
	oldPath string
	newPath string
}

func RenameArchive(cfg config.Config, oldName, newName string) (RenameResult, error) {
	result := RenameResult{OtherProjectsMayUseArchive: true, RelinkedPaths: []string{}, ManifestUpdates: []string{}}
	oldPath, newPath, err := renameArchivePaths(cfg, oldName, newName)
	if err != nil {
		return result, err
	}
	result.ArchivePath = newPath
	usages, err := managedRenameUsages(cfg, oldName, newName)
	if err != nil {
		return result, err
	}
	manifests, err := prepareRenameManifests(cfg.ProjectRoot, oldName, newName)
	if err != nil {
		return result, err
	}

	if err := renameArchiveFilesystem.rename(oldPath, newPath); err != nil {
		return result, fmt.Errorf("rename archive %q to %q: %w", oldPath, newPath, err)
	}
	relinked := []renameUsage{}
	rollback := func(cause error) (RenameResult, error) {
		rollbackErr := rollbackArchiveRename(oldPath, newPath, relinked, manifests)
		if rollbackErr != nil {
			return result, errors.Join(cause, rollbackErr)
		}
		return result, cause
	}
	for _, usage := range usages {
		if err := replaceRenameLink(usage, newPath); err != nil {
			return rollback(fmt.Errorf("relink visible usage %q: %w", usage.oldPath, err))
		}
		relinked = append(relinked, usage)
	}
	for i := range manifests {
		if !manifests[i].changed {
			continue
		}
		if err := writeRenameFile(manifests[i].path, manifests[i].next, manifests[i].mode); err != nil {
			return rollback(fmt.Errorf("update %s: %w", manifests[i].label, err))
		}
		result.ManifestUpdates = append(result.ManifestUpdates, manifests[i].label)
	}
	for _, usage := range relinked {
		result.RelinkedPaths = append(result.RelinkedPaths, usage.newPath)
	}
	return result, nil
}

func renameArchivePaths(cfg config.Config, oldName, newName string) (string, string, error) {
	oldPath, err := repo.SkillPath(cfg, oldName)
	if err != nil {
		return "", "", err
	}
	newPath, err := repo.SkillPath(cfg, newName)
	if err != nil {
		return "", "", err
	}
	if oldName == newName {
		return "", "", fmt.Errorf("archive name is unchanged: %q", oldName)
	}
	if !repo.HasSkill(cfg, oldName) {
		return "", "", fmt.Errorf("archive skill %q not found", oldName)
	}
	if _, err := os.Lstat(newPath); err == nil {
		return "", "", fmt.Errorf("archive name already exists: %s", newName)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", "", fmt.Errorf("inspect archive destination %q: %w", newPath, err)
	}
	return oldPath, newPath, nil
}

func managedRenameUsages(cfg config.Config, oldName, newName string) ([]renameUsage, error) {
	active, err := ScanActive(cfg, ScanFilter{})
	if err != nil {
		return nil, fmt.Errorf("scan visible Skills Folders: %w", err)
	}
	paths := []renameUsage{}
	for _, occurrence := range active {
		if occurrence.Status == StatusManaged && filepath.Base(occurrence.Path) == oldName {
			newPath := filepath.Join(filepath.Dir(occurrence.Path), newName)
			if _, err := os.Lstat(newPath); err == nil {
				return nil, fmt.Errorf("visible usage destination exists: %s", newPath)
			} else if !errors.Is(err, os.ErrNotExist) {
				return nil, fmt.Errorf("inspect visible usage destination %q: %w", newPath, err)
			}
			paths = append(paths, renameUsage{oldPath: occurrence.Path, newPath: newPath})
		}
	}
	sort.SliceStable(paths, func(i, j int) bool {
		return renameUsageOrder(cfg, paths[i].oldPath) < renameUsageOrder(cfg, paths[j].oldPath)
	})
	return paths, nil
}

func renameUsageOrder(cfg config.Config, path string) int {
	for i, root := range cfg.ManagedRoots() {
		if filepath.Dir(path) == filepath.Clean(root.Path) {
			return i
		}
	}
	return len(cfg.ManagedRoots())
}

func replaceRenameLink(usage renameUsage, target string) error {
	temp, err := temporaryRenameSibling(usage.newPath, "link")
	if err != nil {
		return err
	}
	defer os.Remove(temp)
	if err := renameArchiveFilesystem.symlink(target, temp); err != nil {
		return err
	}
	if err := renameArchiveFilesystem.rename(temp, usage.newPath); err != nil {
		return err
	}
	if err := renameArchiveFilesystem.remove(usage.oldPath); err != nil {
		_ = renameArchiveFilesystem.remove(usage.newPath)
		return err
	}
	return nil
}

func temporaryRenameSibling(path, kind string) (string, error) {
	file, err := os.CreateTemp(filepath.Dir(path), ".x-skills-rename-"+kind+"-")
	if err != nil {
		return "", err
	}
	temp := file.Name()
	if err := file.Close(); err != nil {
		_ = os.Remove(temp)
		return "", err
	}
	if err := os.Remove(temp); err != nil {
		return "", err
	}
	return temp, nil
}

func prepareRenameManifests(projectRoot, oldName, newName string) ([]renameManifest, error) {
	files := []renameManifest{
		{path: filepath.Join(projectRoot, ".x-skills.yaml"), label: ".x-skills.yaml"},
		{path: filepath.Join(projectRoot, ".x-skills.local.yaml"), label: ".x-skills.local.yaml"},
	}
	for i := range files {
		data, err := os.ReadFile(files[i].path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", files[i].label, err)
		}
		info, err := os.Stat(files[i].path)
		if err != nil {
			return nil, fmt.Errorf("inspect %s: %w", files[i].label, err)
		}
		next, changed, err := renameManifestIdentity(data, oldName, newName)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", files[i].label, err)
		}
		files[i].data, files[i].next, files[i].changed, files[i].mode = data, next, changed, info.Mode().Perm()
	}
	return files, nil
}

func renameManifestIdentity(data []byte, oldName, newName string) ([]byte, bool, error) {
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil, false, err
	}
	if len(document.Content) == 0 {
		return data, false, nil
	}
	root := document.Content[0]
	var skillsNode *yaml.Node
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == "skills" {
			skillsNode = root.Content[i+1]
			break
		}
	}
	if skillsNode == nil || skillsNode.Kind != yaml.SequenceNode {
		return data, false, nil
	}
	newExists := false
	oldNodes := []*yaml.Node{}
	for _, item := range skillsNode.Content {
		for i := 0; i+1 < len(item.Content); i += 2 {
			if item.Content[i].Value != "name" {
				continue
			}
			switch item.Content[i+1].Value {
			case oldName:
				oldNodes = append(oldNodes, item.Content[i+1])
			case newName:
				newExists = true
			}
		}
	}
	if len(oldNodes) == 0 {
		return data, false, nil
	}
	if newExists {
		return nil, false, fmt.Errorf("skill %q already exists", newName)
	}
	for _, node := range oldNodes {
		node.Value = newName
	}
	next, err := yaml.Marshal(root)
	return next, true, err
}

func writeRenameFile(path string, data []byte, mode os.FileMode) error {
	temp, err := os.CreateTemp(filepath.Dir(path), ".x-skills-rename-manifest-")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(mode); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return renameArchiveFilesystem.rename(tempPath, path)
}

func rollbackArchiveRename(oldPath, newPath string, relinked []renameUsage, manifests []renameManifest) error {
	errs := []error{}
	for _, manifest := range slices.Backward(manifests) {
		if manifest.changed {
			if err := writeRenameFile(manifest.path, manifest.data, manifest.mode); err != nil {
				errs = append(errs, fmt.Errorf("restore %s: %w", manifest.label, err))
			}
		}
	}
	for _, usage := range slices.Backward(relinked) {
		newUsagePath := filepath.Join(filepath.Dir(usage.oldPath), filepath.Base(newPath))
		if err := renameArchiveFilesystem.remove(newUsagePath); err != nil && !errors.Is(err, os.ErrNotExist) {
			errs = append(errs, fmt.Errorf("remove renamed link %q: %w", newUsagePath, err))
			continue
		}
		if err := renameArchiveFilesystem.symlink(oldPath, usage.oldPath); err != nil {
			errs = append(errs, fmt.Errorf("restore link %q: %w", usage.oldPath, err))
		}
	}
	if err := renameArchiveFilesystem.rename(newPath, oldPath); err != nil {
		errs = append(errs, fmt.Errorf("restore archive name: %w", err))
	}
	return errors.Join(errs...)
}
