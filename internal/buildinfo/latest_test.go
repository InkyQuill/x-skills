package buildinfo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGitHubReleaseCheckerLatestRelease(t *testing.T) {
	t.Run("redirect resolves release tag", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("/releases/latest", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/releases/tag/v1.4.0", http.StatusFound)
		})
		mux.HandleFunc("/releases/tag/v1.4.0", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		checker := githubReleaseChecker{
			endpoint: server.URL + "/releases/latest",
			client:   server.Client(),
		}
		got, err := checker.LatestRelease(t.Context())
		if err != nil {
			t.Fatalf("LatestRelease() error = %v", err)
		}
		if got != "v1.4.0" {
			t.Fatalf("LatestRelease() = %q, want %q", got, "v1.4.0")
		}
	})

	t.Run("final url has no release tag", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		checker := githubReleaseChecker{endpoint: server.URL, client: server.Client()}
		if _, err := checker.LatestRelease(t.Context()); err == nil {
			t.Fatal("LatestRelease() error = nil, want error")
		}
	})

	t.Run("non-ok response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		checker := githubReleaseChecker{endpoint: server.URL, client: server.Client()}
		if _, err := checker.LatestRelease(t.Context()); err == nil {
			t.Fatal("LatestRelease() error = nil, want error")
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		started := make(chan struct{})
		server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			close(started)
			<-r.Context().Done()
		}))
		defer server.Close()

		checker := githubReleaseChecker{endpoint: server.URL, client: server.Client()}
		ctx, cancel := context.WithCancel(t.Context())
		result := make(chan error, 1)
		go func() {
			_, err := checker.LatestRelease(ctx)
			result <- err
		}()

		<-started
		cancel()
		if err := <-result; err == nil {
			t.Fatal("LatestRelease() error = nil, want error")
		}
	})
}
