package tui

import (
	"net/http"

	"github.com/InkyQuill/x-skills/internal/remote"
)

type installState struct {
	Query        string
	Owner        string
	Searching    bool
	Results      []installResultView
	Message      string
	searchClient remote.SearchClient
	checkouts    *remote.CheckoutCache
}

type installResultView struct {
	Result       remote.SearchResult
	ArchiveState string
	AuditPill    string
}

func newInstallState() installState {
	return installState{
		Message:      "type at least 2 characters",
		searchClient: remote.NewSearchClient(remote.DefaultSearchEndpoint, http.DefaultClient),
	}
}
