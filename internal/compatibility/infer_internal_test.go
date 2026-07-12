package compatibility

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExecutableLinesIgnoresFrontMatterAndClosingHashExampleHeading(t *testing.T) {
	content := "---\ndescription: Read from $CLAUDE_PROJECT_DIR.\n---\n## Examples ##   \nRead from $CLAUDE_PROJECT_DIR.\n## Instructions\nPortable workflow.\n"
	lines := executableLines(content)
	if len(lines) != 1 || lines[0] != "Portable workflow." {
		t.Fatalf("executableLines() = %#v, want only portable instruction", lines)
	}
	level, title, ok := markdownHeading("## Examples ##   ")
	if !ok || level != 2 || title != "Examples" {
		t.Fatalf("markdownHeading() = (%d, %q, %v), want (2, Examples, true)", level, title, ok)
	}
}

func TestInferDoesNotTreatFrontMatterDescriptionAsExecutable(t *testing.T) {
	dir := t.TempDir()
	data := []byte("---\nname: portable\ndescription: Read from $CLAUDE_PROJECT_DIR.\n---\n\nPortable workflow.\n")
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := infer(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Confidence == ConfidenceHigh || len(got.Agents) != 0 {
		t.Fatalf("inference = %#v, want no executable compatibility signal", got)
	}
}
