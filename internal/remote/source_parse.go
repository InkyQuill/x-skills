package remote

import (
	"fmt"
	"net/url"
	"path"
	"strings"
)

type AddSource struct {
	Source        GitSource
	Names         []string
	PreferredPath string
}

func ParseAddSource(sourceArg, gitURL, ref string) (AddSource, error) {
	if gitURL != "" {
		return AddSource{Source: GitSource{CloneURL: gitURL, Ref: ref}}, nil
	}
	if sourceArg == "" {
		return AddSource{}, fmt.Errorf("source is required")
	}
	if isURLLike(sourceArg) {
		return parseAddSourceURL(sourceArg)
	}
	return parseGitHubShorthand(sourceArg)
}

func parseGitHubShorthand(sourceArg string) (AddSource, error) {
	repoPart, skillName, hasSkill := strings.Cut(sourceArg, "@")
	parts := strings.Split(repoPart, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" || strings.Contains(repoPart, "://") {
		return AddSource{}, fmt.Errorf("invalid GitHub source %q", sourceArg)
	}
	result := AddSource{
		Source: GitSource{
			CloneURL: "https://github.com/" + parts[0] + "/" + parts[1] + ".git",
			Owner:    parts[0],
			Repo:     parts[1],
		},
	}
	if hasSkill {
		if skillName == "" || strings.Contains(skillName, "/") {
			return AddSource{}, fmt.Errorf("invalid skill name in source %q", sourceArg)
		}
		result.Names = []string{skillName}
	}
	return result, nil
}

func parseAddSourceURL(sourceArg string) (AddSource, error) {
	parsed, err := url.Parse(sourceArg)
	if err != nil {
		return AddSource{}, fmt.Errorf("parse source URL: %w", err)
	}
	if parsed.Scheme == "file" {
		return AddSource{}, unsupportedURLInstallError(sourceArg)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return AddSource{}, unsupportedURLInstallError(sourceArg)
	}
	if !strings.EqualFold(parsed.Hostname(), "github.com") {
		return AddSource{}, unsupportedURLInstallError(sourceArg)
	}

	parts := strings.Split(strings.Trim(parsed.EscapedPath(), "/"), "/")
	if len(parts) < 5 || parts[0] == "" || parts[1] == "" || parts[2] != "tree" || parts[3] == "" {
		return AddSource{}, unsupportedURLInstallError(sourceArg)
	}
	ref, preferred, err := splitGitHubTreeRefAndPath(parts[3:])
	if err != nil {
		return AddSource{}, fmt.Errorf("parse GitHub tree path: %w", err)
	}
	if preferred == "." || preferred == "" {
		return AddSource{}, unsupportedURLInstallError(sourceArg)
	}
	return AddSource{
		Source: GitSource{
			CloneURL: "https://github.com/" + parts[0] + "/" + parts[1] + ".git",
			Owner:    parts[0],
			Repo:     parts[1],
			Ref:      ref,
		},
		PreferredPath: preferred,
	}, nil
}

func splitGitHubTreeRefAndPath(parts []string) (string, string, error) {
	refParts, pathParts := parts[:1], parts[1:]
	for i := 1; i < len(parts); i++ {
		if parts[i] == "skills" {
			refParts, pathParts = parts[:i], parts[i:]
			break
		}
	}

	ref, err := url.PathUnescape(path.Join(refParts...))
	if err != nil {
		return "", "", err
	}
	preferred, err := url.PathUnescape(path.Join(pathParts...))
	if err != nil {
		return "", "", err
	}
	return ref, preferred, nil
}

func isURLLike(sourceArg string) bool {
	if strings.Contains(sourceArg, "://") {
		return true
	}
	parsed, err := url.Parse(sourceArg)
	return err == nil && parsed.Scheme != ""
}

func unsupportedURLInstallError(sourceArg string) error {
	return fmt.Errorf("URL installs are not supported yet: %s", sourceArg)
}
