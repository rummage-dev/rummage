package finder_test

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	finder "github.com/rummage-dev/rummage"
)

// renderDir builds a model over dir, runs the initial directory read, and
// returns the rendered view — the exact bytes that would be written to the
// terminal.
func renderDir(t *testing.T, opts finder.Options) string {
	t.Helper()
	m := finder.NewModel(opts)
	msg := m.Init()()
	updated, _ := m.Update(msg)
	updated, _ = updated.(finder.Model).Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return updated.(finder.Model).View()
}

// TestViewSanitizesMaliciousFilename verifies that a filename carrying terminal
// escape sequences cannot reach the rendered output. Without sanitization the
// raw ESC bytes would be written straight to the user's terminal, allowing
// window-title rewrites, cursor manipulation, or OSC 52 clipboard hijacking
// when browsing an attacker-controlled directory.
func TestViewSanitizesMaliciousFilename(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows forbids control characters in filenames")
	}
	dir := t.TempDir()
	// A name with an SGR sequence and an OSC set-window-title sequence.
	createFile(t, dir, "evil\x1b[31m\x1b]0;pwned\x07.txt", "x")

	opts := finder.DefaultOptions()
	opts.StartDir = dir
	out := renderDir(t, opts)

	// Legitimate lipgloss styling emits SGR (ESC [ … m) sequences, so the test
	// targets the markers an attacker needs that styling never produces: OSC
	// (ESC ]) and BEL (the OSC/title terminator).
	if strings.Contains(out, "\x1b]") || strings.Contains(out, "\x07") {
		t.Fatalf("view leaked a raw escape/control sequence from a filename:\n%q", out)
	}
}

// TestViewSanitizesPreviewContent verifies that escape sequences inside a
// previewed file's *contents* are neutralized before display. This is
// cross-platform because file contents (unlike filenames) may contain control
// bytes on every OS.
func TestViewSanitizesPreviewContent(t *testing.T) {
	dir := t.TempDir()
	createFile(t, dir, "notes.txt", "hello\x1b]0;pwned\x07\x1b[2Jworld")

	opts := finder.DefaultOptions()
	opts.StartDir = dir
	opts.Preview = true
	out := renderDir(t, opts)

	if strings.Contains(out, "\x1b]") || strings.Contains(out, "\x07") {
		t.Fatalf("view leaked a raw OSC/BEL sequence from preview content:\n%q", out)
	}
	// The benign text around the escapes should still be previewed.
	if !strings.Contains(out, "hello") || !strings.Contains(out, "world") {
		t.Errorf("expected preview to keep the readable text, got:\n%q", out)
	}
}

// TestViewSanitizesErrorBanner verifies that escape sequences reaching the
// error banner are neutralized. Filesystem errors embed the offending path —
// os.ReadDir returns a *PathError whose Error() contains it — so browsing to
// a directory whose *name* carries escapes would put them on the terminal
// through the error line even though the entry list itself is sanitized.
func TestViewSanitizesErrorBanner(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows forbids control characters in path names")
	}
	// A path that cannot be read, carrying OSC set-window-title and BEL.
	missing := filepath.Join(t.TempDir(), "gone\x1b]0;pwned\x07")

	opts := finder.DefaultOptions()
	opts.StartDir = missing
	out := renderDir(t, opts)

	if !strings.Contains(out, "Error:") {
		t.Fatalf("expected the error banner to render, got:\n%q", out)
	}
	if strings.Contains(out, "\x1b]") || strings.Contains(out, "\x07") {
		t.Fatalf("view leaked a raw OSC/BEL sequence through the error banner:\n%q", out)
	}
	// Only the control bytes need to go. The surrounding text stays readable —
	// "gone?]0;pwned?" is inert once ESC and BEL are gone, and keeping it means
	// the user can still see which path failed.
	if !strings.Contains(out, "no such file or directory") {
		t.Errorf("expected the banner to still explain the failure, got:\n%q", out)
	}
}
