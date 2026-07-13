package remote

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
)

type PreviewRequest struct {
	Source        GitSource
	Name          string
	PreferredPath string
}

type PreviewResult struct {
	Repository    string
	RequestedName string
	SkillDir      string
	SkillPath     string
	SkillMD       []byte
	Commit        string
}

type PreviewCheckoutError struct {
	Repository string
	Err        error
}

func (e *PreviewCheckoutError) Error() string {
	return fmt.Sprintf("checkout preview repository %q: %v", e.Repository, e.Err)
}

func (e *PreviewCheckoutError) Unwrap() error {
	return e.Err
}

type PreviewReadError struct {
	SkillPath string
	Err       error
}

func (e *PreviewReadError) Error() string {
	return fmt.Sprintf("read preview document %q: %v", e.SkillPath, e.Err)
}

func (e *PreviewReadError) Unwrap() error {
	return e.Err
}

func ResolvePreview(
	ctx context.Context,
	cache *CheckoutCache,
	request PreviewRequest,
) (PreviewResult, error) {
	return resolvePreview(ctx, cache, request, readPreviewDocument)
}

func resolvePreview(
	ctx context.Context,
	cache *CheckoutCache,
	request PreviewRequest,
	readDocument func(context.Context, string) ([]byte, error),
) (PreviewResult, error) {
	if err := ctx.Err(); err != nil {
		return PreviewResult{}, err
	}
	repository := previewRepository(request.Source)
	if cache == nil {
		return PreviewResult{}, &PreviewCheckoutError{
			Repository: repository,
			Err:        fmt.Errorf("checkout cache is nil"),
		}
	}

	checkout, err := cache.Checkout(ctx, request.Source)
	if err != nil {
		return PreviewResult{}, &PreviewCheckoutError{Repository: repository, Err: err}
	}
	if err := ctx.Err(); err != nil {
		return PreviewResult{}, err
	}

	found, err := checkout.FindSkillContext(ctx, request.Name, request.PreferredPath)
	if err != nil {
		return PreviewResult{}, fmt.Errorf("find preview skill: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return PreviewResult{}, err
	}

	skillPath := path.Join(found.Metadata.SkillPath, "SKILL.md")
	skillMD, err := readDocument(ctx, filepath.Join(found.SkillDir, "SKILL.md"))
	if err != nil {
		return PreviewResult{}, &PreviewReadError{SkillPath: skillPath, Err: err}
	}
	if err := ctx.Err(); err != nil {
		return PreviewResult{}, err
	}

	return PreviewResult{
		Repository:    repository,
		RequestedName: request.Name,
		SkillDir:      found.SkillDir,
		SkillPath:     skillPath,
		SkillMD:       skillMD,
		Commit:        checkout.Commit,
	}, nil
}

func readPreviewDocument(ctx context.Context, skillPath string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(skillPath)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return data, nil
}

func previewRepository(source GitSource) string {
	if source.Owner != "" && source.Repo != "" {
		return source.Owner + "/" + source.Repo
	}
	return source.CloneURL
}
