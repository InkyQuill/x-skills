package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunPrintsErrorsToStderr(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"list", "--target", "bogus"}, strings.NewReader(""), &stdout, &stderr)

	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "unknown target") {
		t.Fatalf("stderr = %q, want unknown target", stderr.String())
	}
}
