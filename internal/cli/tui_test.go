package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestTUIRejectsNoInput(t *testing.T) {
	err := Execute([]string{"tui", "--no-input"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected no-input error")
	}
	if !strings.Contains(err.Error(), "interactive terminal") {
		t.Fatalf("error = %q, want interactive terminal", err)
	}
}

func TestTUIAcceptsASCIIFlagWithNoInput(t *testing.T) {
	err := Execute([]string{"tui", "--ascii", "--no-input"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected no-input error")
	}
	if !strings.Contains(err.Error(), "interactive terminal") {
		t.Fatalf("error = %v, want interactive terminal message", err)
	}
}
