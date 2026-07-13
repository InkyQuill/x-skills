package buildinfo

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const latestReleaseURL = "https://github.com/InkyQuill/x-skills/releases/latest"

type LatestReleaseChecker interface {
	LatestRelease(context.Context) (string, error)
}

type githubReleaseChecker struct {
	endpoint string
	client   *http.Client
}

func NewGitHubReleaseChecker(client *http.Client) LatestReleaseChecker {
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Second}
	}
	return githubReleaseChecker{endpoint: latestReleaseURL, client: client}
}

func (c githubReleaseChecker) LatestRelease(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("create latest release request: %w", err)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("resolve latest release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("resolve latest release: status %s", resp.Status)
	}
	return releaseTagFromURL(resp.Request.URL)
}

func releaseTagFromURL(value *url.URL) (string, error) {
	const marker = "/releases/tag/"
	index := strings.LastIndex(value.EscapedPath(), marker)
	if index < 0 {
		return "", fmt.Errorf("latest release response has no release tag: %s", value)
	}
	tag, err := url.PathUnescape(value.EscapedPath()[index+len(marker):])
	if err != nil || tag == "" || strings.Contains(tag, "/") {
		return "", fmt.Errorf("latest release response has invalid release tag: %s", value)
	}
	return tag, nil
}
