package actions

import "sync"

// mutationMu serializes active-root and archive mutations whose validation and
// writes must form one transaction.
var mutationMu sync.Mutex
