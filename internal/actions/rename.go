package actions

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"

	"github.com/InkyQuill/x-skills/internal/config"
	"github.com/InkyQuill/x-skills/internal/fingerprint"
	"github.com/InkyQuill/x-skills/internal/pathidentity"
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
	rename          func(string, string) error
	renameNoReplace func(string, string) error
	remove          func(string) error
	symlink         func(string, string) error
	beforeMutation  func(string) error
}

var renameArchiveFilesystem = renameFilesystem{
	rename:          os.Rename,
	renameNoReplace: renameNoReplace,
	remove:          os.Remove,
	symlink:         os.Symlink,
	beforeMutation:  func(string) error { return nil },
}

type renameManifest struct {
	path    string
	label   string
	data    []byte
	mode    os.FileMode
	next    []byte
	changed bool
	present bool
}

type renameUsage struct {
	path    string
	oldText string
	newText string
}

func RenameArchive(cfg config.Config, oldName, newName string) (RenameResult, error) {
	return RenameArchiveContext(context.Background(), cfg, oldName, newName)
}

func RenameArchiveContext(ctx context.Context, cfg config.Config, oldName, newName string) (RenameResult, error) {
	mutationMu.Lock()
	defer mutationMu.Unlock()

	result := RenameResult{OtherProjectsMayUseArchive: true, RelinkedPaths: []string{}, ManifestUpdates: []string{}}
	oldPath, newPath, err := renameArchivePaths(cfg, oldName, newName)
	if err != nil {
		return result, err
	}
	result.ArchivePath = newPath
	archiveFingerprint, err := fingerprint.Directory(oldPath)
	if err != nil {
		return result, fmt.Errorf("snapshot archive identity: %w", err)
	}
	usages, err := managedRenameUsages(cfg, oldPath, newPath)
	if err != nil {
		return result, err
	}
	manifests, err := prepareRenameManifests(cfg.ProjectRoot, oldName, newName)
	if err != nil {
		return result, err
	}

	if err := checkRenameContext(ctx); err != nil {
		return result, err
	}
	if err := renameArchiveFilesystem.beforeMutation("archive"); err != nil {
		return result, err
	}
	currentFingerprint, err := fingerprint.Directory(oldPath)
	if err != nil || currentFingerprint != archiveFingerprint {
		return result, fmt.Errorf("archive identity drifted before mutation")
	}
	for _, usage := range usages {
		if err := revalidateRenameUsageTarget(usage, oldPath); err != nil {
			return result, err
		}
	}
	if err := requireRenamePathAbsent(newPath); err != nil {
		return result, err
	}
	if err := renameArchiveFilesystem.renameNoReplace(oldPath, newPath); err != nil {
		return result, fmt.Errorf("rename archive %q to %q: %w", oldPath, newPath, err)
	}
	relinked := []renameUsage{}
	writtenManifests := []renameManifest{}
	pendingManifestUpdates := []string{}
	rollback := func(cause error) (RenameResult, error) {
		rollbackErr := rollbackArchiveRename(oldPath, newPath, relinked, writtenManifests)
		if rollbackErr != nil {
			return result, errors.Join(cause, rollbackErr)
		}
		return result, cause
	}
	for _, usage := range usages {
		if err := checkRenameContext(ctx); err != nil {
			return rollback(err)
		}
		if err := renameArchiveFilesystem.beforeMutation("usage"); err != nil {
			return rollback(err)
		}
		if err := revalidateRenameUsage(usage); err != nil {
			return rollback(err)
		}
		if err := replaceRenameLink(usage); err != nil {
			return rollback(fmt.Errorf("relink visible usage %q: %w", usage.path, err))
		}
		relinked = append(relinked, usage)
	}
	for i := range manifests {
		if !manifests[i].changed {
			continue
		}
		if err := checkRenameContext(ctx); err != nil {
			return rollback(err)
		}
		if err := renameArchiveFilesystem.beforeMutation("manifest:" + manifests[i].label); err != nil {
			return rollback(err)
		}
		if err := revalidateRenameManifest(manifests[i], manifests[i].data); err != nil {
			return rollback(err)
		}
		if err := writeRenameFile(manifests[i].path, manifests[i].next, manifests[i].mode); err != nil {
			return rollback(fmt.Errorf("update %s: %w", manifests[i].label, err))
		}
		writtenManifests = append(writtenManifests, manifests[i])
		pendingManifestUpdates = append(pendingManifestUpdates, manifests[i].label)
	}
	for _, usage := range relinked {
		result.RelinkedPaths = append(result.RelinkedPaths, usage.path)
	}
	result.ManifestUpdates = pendingManifestUpdates
	return result, nil
}

func requireRenamePathAbsent(path string) error {
	if _, err := os.Lstat(path); err == nil {
		return fmt.Errorf("archive destination %q appeared before mutation", path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect archive destination %q: %w", path, err)
	}
	return nil
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

func managedRenameUsages(cfg config.Config, oldPath, newPath string) ([]renameUsage, error) {
	paths := []renameUsage{}
	canonicalOld, err := pathidentity.Canonical(oldPath)
	if err != nil {
		return nil, err
	}
	for _, root := range cfg.ManagedRoots() {
		if !root.Enabled {
			continue
		}
		entries, err := os.ReadDir(root.Path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("scan Skills Folder %q: %w", root.Path, err)
		}
		for _, entry := range entries {
			path := filepath.Join(root.Path, entry.Name())
			info, err := os.Lstat(path)
			if err != nil || info.Mode()&os.ModeSymlink == 0 {
				continue
			}
			resolved, err := filepath.EvalSymlinks(path)
			if err != nil {
				continue
			}
			same, sameErr := pathidentity.EquivalentE(resolved, canonicalOld)
			if sameErr != nil {
				return nil, fmt.Errorf("compare visible usage %q target %q with archive %q: %w", path, resolved, canonicalOld, sameErr)
			}
			if !same {
				continue
			}
			text, err := os.Readlink(path)
			if err != nil {
				return nil, err
			}
			newText := newPath
			if !filepath.IsAbs(text) {
				newText, err = filepath.Rel(filepath.Dir(path), newPath)
				if err != nil {
					return nil, err
				}
			}
			paths = append(paths, renameUsage{path: path, oldText: text, newText: newText})
		}
	}
	sort.SliceStable(paths, func(i, j int) bool {
		return renameUsageOrder(cfg, paths[i].path) < renameUsageOrder(cfg, paths[j].path)
	})
	return paths, nil
}

func VisibleArchiveUsagePaths(cfg config.Config, name string) ([]string, error) {
	oldPath, err := repo.SkillPath(cfg, name)
	if err != nil {
		return nil, err
	}
	usages, err := managedRenameUsages(cfg, oldPath, oldPath)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(usages))
	for _, usage := range usages {
		paths = append(paths, usage.path)
	}
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

// replaceRenameLink swaps a visible usage symlink to point at the renamed archive.
// It uses rename-first replacement when the platform supports it, then falls
// back to remove-and-rename for platforms that reject replacing an existing link.
func replaceRenameLink(usage renameUsage) (err error) {
	temp, err := temporaryRenameSibling(usage.path, "link")
	if err != nil {
		return err
	}
	defer func() { err = joinRenameCleanupError(err, renameArchiveFilesystem.remove(temp)) }()
	if err := renameArchiveFilesystem.symlink(usage.newText, temp); err != nil {
		return err
	}
	if err := renameArchiveFilesystem.rename(temp, usage.path); err != nil {
		if !renameLinkReplacePermissionDenied(err) {
			return err
		}
		return replaceRenameLinkAfterRenameFailure(usage, temp, err)
	}
	return nil
}

// renameLinkReplacePermissionDenied reports whether a failed symlink replacement
// looks like a platform refusal to overwrite the existing link path.
func renameLinkReplacePermissionDenied(err error) bool {
	return errors.Is(err, os.ErrPermission)
}

// replaceRenameLinkAfterRenameFailure handles filesystems that cannot rename a
// staged symlink over an existing symlink, preserving drift checks and restoring
// the old link if the fallback publish fails after removal.
func replaceRenameLinkAfterRenameFailure(usage renameUsage, temp string, renameErr error) error {
	if err := revalidateRenameUsage(usage); err != nil {
		return errors.Join(renameErr, err)
	}
	if err := renameArchiveFilesystem.remove(usage.path); err != nil {
		return errors.Join(renameErr, fmt.Errorf("remove existing usage before relink: %w", err))
	}
	if err := renameArchiveFilesystem.rename(temp, usage.path); err != nil {
		restoreErr := renameArchiveFilesystem.symlink(usage.oldText, usage.path)
		if restoreErr != nil {
			restoreErr = fmt.Errorf("restore previous usage after failed relink: %w", restoreErr)
		}
		return errors.Join(renameErr, fmt.Errorf("publish replacement usage: %w", err), restoreErr)
	}
	return nil
}

func revalidateRenameUsage(usage renameUsage) error {
	text, err := os.Readlink(usage.path)
	if err != nil || text != usage.oldText {
		return fmt.Errorf("visible usage %q drifted before mutation", usage.path)
	}
	return nil
}

func revalidateRenameUsageTarget(usage renameUsage, oldPath string) error {
	if err := revalidateRenameUsage(usage); err != nil {
		return err
	}
	resolved, err := filepath.EvalSymlinks(usage.path)
	if err != nil {
		return fmt.Errorf("visible usage %q target drifted before mutation", usage.path)
	}
	same, sameErr := pathidentity.EquivalentE(resolved, oldPath)
	if sameErr != nil || !same {
		return fmt.Errorf("visible usage %q target drifted before mutation", usage.path)
	}
	return nil
}

func checkRenameContext(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
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
		files[i].data, files[i].next, files[i].changed, files[i].mode, files[i].present = data, next, changed, info.Mode().Perm(), true
	}
	return files, nil
}

func revalidateRenameManifest(manifest renameManifest, want []byte) error {
	data, err := os.ReadFile(manifest.path)
	if errors.Is(err, os.ErrNotExist) && !manifest.present {
		return nil
	}
	if err != nil || !manifest.present || !bytes.Equal(data, want) {
		return fmt.Errorf("manifest %s drifted before mutation", manifest.label)
	}
	return nil
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
	next := slices.Clone(data)
	for _, node := range slices.Backward(oldNodes) {
		var err error
		next, err = replaceYAMLScalar(next, node, oldName, newName)
		if err != nil {
			return nil, false, err
		}
	}
	return next, true, nil
}

func replaceYAMLScalar(data []byte, node *yaml.Node, oldName, newName string) ([]byte, error) {
	lines := bytes.SplitAfter(data, []byte("\n"))
	if node.Line < 1 || node.Line > len(lines) {
		return nil, errors.New("invalid YAML scalar position")
	}
	start := 0
	for i := 0; i < node.Line-1; i++ {
		start += len(lines[i])
	}
	line := lines[node.Line-1]
	column := node.Column - 1
	if column < 0 || column >= len(line) {
		return nil, errors.New("invalid YAML scalar column")
	}
	rel := bytes.Index(line[column:], []byte(oldName))
	if rel < 0 {
		return nil, fmt.Errorf("cannot locate manifest identity %q", oldName)
	}
	from := start + column + rel
	to := from + len(oldName)
	result := make([]byte, 0, len(data)-len(oldName)+len(newName))
	result = append(result, data[:from]...)
	result = append(result, newName...)
	result = append(result, data[to:]...)
	return result, nil
}

func writeRenameFile(path string, data []byte, mode os.FileMode) (err error) {
	temp, err := os.CreateTemp(filepath.Dir(path), ".x-skills-rename-manifest-")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer func() { err = joinRenameCleanupError(err, renameArchiveFilesystem.remove(tempPath)) }()
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

func joinRenameCleanupError(primary, cleanup error) error {
	if cleanup == nil || errors.Is(cleanup, os.ErrNotExist) {
		return primary
	}
	return errors.Join(primary, fmt.Errorf("remove rename temporary artifact: %w", cleanup))
}

func rollbackArchiveRename(oldPath, newPath string, relinked []renameUsage, manifests []renameManifest) error {
	errs := []error{}
	for _, manifest := range slices.Backward(manifests) {
		if err := revalidateRenameManifest(renameManifest{path: manifest.path, label: manifest.label, present: true}, manifest.next); err != nil {
			errs = append(errs, fmt.Errorf("restore %s: rollback drift: %w", manifest.label, err))
			continue
		}
		if err := writeRenameFile(manifest.path, manifest.data, manifest.mode); err != nil {
			errs = append(errs, fmt.Errorf("restore %s: %w", manifest.label, err))
		}
	}
	for _, usage := range slices.Backward(relinked) {
		text, err := os.Readlink(usage.path)
		if err != nil || text != usage.newText {
			errs = append(errs, fmt.Errorf("restore link %q: rollback drift: current target no longer matches renamed target", usage.path))
			continue
		}
		if err := renameArchiveFilesystem.remove(usage.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			errs = append(errs, fmt.Errorf("remove renamed link %q: %w", usage.path, err))
			continue
		}
		if err := renameArchiveFilesystem.symlink(usage.oldText, usage.path); err != nil {
			errs = append(errs, fmt.Errorf("restore link %q: %w", usage.path, err))
		}
	}
	if err := renameArchiveFilesystem.rename(newPath, oldPath); err != nil {
		errs = append(errs, fmt.Errorf("restore archive name: %w", err))
	}
	return errors.Join(errs...)
}
