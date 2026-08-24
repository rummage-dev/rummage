package finder

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/charmbracelet/lipgloss"
)

// View implements tea.Model. It renders the full picker UI.
func (m Model) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder

	b.WriteString(m.styles.Title.Render(truncate(m.options.Title, m.width)))
	b.WriteString("\n")

	displayDir := sanitizeControl(m.fsys.Display(m.dir))
	maxPath := m.width - 4
	if maxPath > 0 && lipgloss.Width(displayDir) > maxPath {
		displayDir = "…" + truncateTail(displayDir, maxPath-1)
	}
	b.WriteString(m.styles.Path.Render(displayDir))
	b.WriteString("\n")

	if m.options.Mode == ModeFolder || m.options.Mode == ModeAny {
		b.WriteString(m.styles.Help.Render("  press 's' to select this directory"))
		b.WriteString("\n")
	}

	if m.err != nil {
		// Sanitized like every other untrusted string: filesystem errors embed
		// the path (os.ReadDir returns *PathError), so a directory whose name
		// carries escape sequences would otherwise reach the terminal raw
		// through this banner even though the entry list itself is safe.
		b.WriteString(m.styles.Cursor.Render(sanitizeControl(fmt.Sprintf("Error: %v", m.err))))
		b.WriteString("\n")
		return m.clampView(b.String())
	}

	listW, prevW, prevH := m.layout()
	visible := m.visibleRows()

	var listLines []string
	if len(m.entries) == 0 {
		listLines = append(listLines, m.styles.Help.Render("  (empty directory)"))
	}

	end := m.offset + visible
	if end > len(m.entries) {
		end = len(m.entries)
	}
	for i := m.offset; i < end; i++ {
		entry := m.entries[i]
		isCursor := i == m.cursor
		_, isSelected := m.selected[entry.Path]
		var matched []int
		if m.searching {
			matched = m.matchIdx[entry.Path]
		}
		listLines = append(listLines, m.renderEntry(entry, isCursor, isSelected, listW, matched))
	}

	if prevW > 0 && prevH > 0 {
		b.WriteString(m.renderSplit(listLines, listW, prevW, prevH))
	} else {
		b.WriteString(strings.Join(listLines, "\n"))
	}
	b.WriteString("\n")

	var statusParts []string
	if len(m.entries) > visible {
		statusParts = append(statusParts, fmt.Sprintf("%d/%d", m.cursor+1, len(m.entries)))
	}
	if m.options.Mode == ModeMultiple && len(m.selected) > 0 {
		statusParts = append(statusParts, fmt.Sprintf("%d selected", len(m.selected)))
	}
	if len(statusParts) > 0 {
		b.WriteString(m.styles.StatusBar.Render("  " + strings.Join(statusParts, "  •  ")))
		b.WriteString("\n")
	}

	if m.searching {
		b.WriteString(m.styles.SearchPrompt.Render("/"))
		b.WriteString(m.styles.SearchText.Render(sanitizeControl(m.searchTerm)))
		b.WriteString(m.styles.SearchPrompt.Render("_"))
		b.WriteString("\n")
	}

	switch m.inputMode {
	case inputNewFile:
		b.WriteString(m.styles.SearchPrompt.Render("  New file: "))
		b.WriteString(m.styles.SearchText.Render(sanitizeControl(m.inputText)))
		b.WriteString(m.styles.SearchPrompt.Render("_"))
		b.WriteString("\n")
	case inputNewFolder:
		b.WriteString(m.styles.SearchPrompt.Render("  New folder: "))
		b.WriteString(m.styles.SearchText.Render(sanitizeControl(m.inputText)))
		b.WriteString(m.styles.SearchPrompt.Render("_"))
		b.WriteString("\n")
	case inputConfirmDelete:
		if m.cursor >= 0 && m.cursor < len(m.entries) {
			name := sanitizeControl(m.entries[m.cursor].Name)
			if m.entries[m.cursor].IsDir {
				name += "/"
			}
			b.WriteString(m.styles.Cursor.Render(
				fmt.Sprintf("  Delete %s? (y/n)", name),
			))
			b.WriteString("\n")
		}
	}

	if m.statusMsg != "" {
		b.WriteString(m.styles.Cursor.Render("  " + sanitizeControl(m.statusMsg)))
		b.WriteString("\n")
	}

	b.WriteString(m.renderHelp())

	return m.clampView(b.String())
}

// clampView hard-limits the rendered view to the terminal dimensions: each line
// is truncated to the width and the whole frame to the height. This guarantees
// no line wraps and the frame never exceeds the screen — which would otherwise
// make the terminal scroll and corrupt the display on every repaint.
func (m Model) clampView(s string) string {
	if m.width <= 0 || m.height <= 0 {
		return s
	}
	return lipgloss.NewStyle().MaxWidth(m.width).MaxHeight(m.height).Render(s)
}

// truncate shortens s to fit width display cells, appending an ellipsis when
// shortened. It is width-aware (counts display cells, not bytes) so it handles
// multibyte runes such as arrows in titles.
func truncate(s string, width int) string {
	if width <= 0 {
		return s
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	if width == 1 {
		return "…"
	}
	var b strings.Builder
	used := 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if used+rw > width-1 {
			break
		}
		b.WriteRune(r)
		used += rw
	}
	b.WriteString("…")
	return b.String()
}

// truncateTail returns the trailing portion of s that fits within width display
// cells, appending nothing. It is rune-aware so multibyte runes are never split
// (the byte-offset variant could emit invalid UTF-8). Used for the breadcrumb,
// which is truncated from the left so the deepest directory stays visible.
func truncateTail(s string, width int) string {
	if width <= 0 {
		return ""
	}
	r := []rune(s)
	used := 0
	i := len(r)
	for i > 0 {
		rw := lipgloss.Width(string(r[i-1]))
		if used+rw > width {
			break
		}
		used += rw
		i--
	}
	return string(r[i:])
}

// sanitizeControl defuses terminal escape/control-sequence injection from
// untrusted input (filenames and file contents are attacker-controlled when the
// picker browses directories the user did not author). It replaces every
// control character — C0 (0x00–0x1F), DEL (0x7F), and C1 (0x80–0x9F), which
// unicode.IsControl covers — with '?'. The replacement is single-byte, so for
// legitimate names (which contain no control characters) this is a no-op and
// byte offsets used elsewhere, e.g. fuzzy-match highlighting, stay aligned.
//
// Runes listed in keep are passed through unchanged; callers preserve '\n' and
// '\t' for multi-line preview text, which clipLines handles separately.
func sanitizeControl(s string, keep ...rune) string {
	if !strings.ContainsFunc(s, func(r rune) bool { return isUnsafeRune(r, keep) }) {
		return s // fast path: the common case has nothing to sanitize
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if isUnsafeRune(r, keep) {
			b.WriteByte('?')
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func isUnsafeRune(r rune, keep []rune) bool {
	for _, k := range keep {
		if r == k {
			return false
		}
	}
	return unicode.IsControl(r)
}

// renderSplit lays the file list and the preview pane out side by side. Both
// columns are pinned to prevH rows so the body's height stays constant as the
// selection changes — otherwise the footer would jump as previews vary in
// length.
func (m Model) renderSplit(listLines []string, listW, prevW, prevH int) string {
	left := lipgloss.NewStyle().Width(listW).Height(prevH).Render(strings.Join(listLines, "\n"))
	content := m.styles.Preview.Render(strings.Join(m.previewLines, "\n"))
	right := m.styles.PreviewBorder.Width(prevW).Height(prevH).Render(content)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func (m Model) renderEntry(e FileEntry, isCursor, isSelected bool, width int, matched []int) string {
	var prefix string
	if isCursor {
		prefix = m.styles.Cursor.Render("> ")
	} else {
		prefix = "  "
	}

	var marker string
	markerWidth := 0
	if m.options.Mode == ModeMultiple {
		markerWidth = 4
		if isSelected {
			marker = m.styles.Selected.Render("[x] ")
		} else {
			marker = "[ ] "
		}
	}

	sizeWidth := 10
	overhead := 2 + markerWidth + 2 + sizeWidth
	maxName := width - overhead
	if maxName < 10 {
		maxName = 10
	}

	displayName := sanitizeControl(e.Name)
	if e.IsDir {
		displayName += "/"
	}
	displayName = truncate(displayName, maxName)

	var baseStyle lipgloss.Style
	switch {
	case isSelected:
		baseStyle = m.styles.Selected
	case e.IsDir && e.IsHidden:
		baseStyle = m.styles.HiddenDir
	case e.IsDir:
		baseStyle = m.styles.Directory
	case e.IsHidden:
		baseStyle = m.styles.HiddenFile
	default:
		baseStyle = m.styles.File
	}

	var name string
	if len(matched) > 0 && !isSelected {
		name = highlightName(displayName, baseStyle, m.styles.Match, matched)
	} else {
		name = baseStyle.Render(displayName)
	}

	var size string
	if !e.IsDir {
		size = m.styles.FileSize.Render(FormatSize(e.Size))
	} else {
		size = m.styles.FileSize.Render("")
	}

	return prefix + marker + name + "  " + size
}

// highlightName renders s with matched characters in the match style and the
// rest in the base style. matched holds byte offsets into the original name (as
// returned by the fuzzy matcher); offsets beyond the displayed (possibly
// truncated) name are simply never reached.
func highlightName(s string, base, match lipgloss.Style, matched []int) string {
	set := make(map[int]bool, len(matched))
	for _, i := range matched {
		set[i] = true
	}
	var b strings.Builder
	for bytePos, r := range s {
		if set[bytePos] {
			b.WriteString(match.Render(string(r)))
		} else {
			b.WriteString(base.Render(string(r)))
		}
	}
	return b.String()
}

// helpBinding is a key-description pair for the help bar.
type helpBinding struct {
	key  string
	desc string
}

func (m Model) renderHelp() string {
	return strings.Join(m.helpLines(), "\n")
}

// helpLines builds the help bar, wrapped to the terminal width, returning one
// string per line. visibleRows uses its length to size the file list so the
// view never overflows the terminal when the help bar wraps.
func (m Model) helpLines() []string {
	var bindings []helpBinding

	switch m.options.Mode {
	case ModeFile:
		bindings = append(bindings,
			helpBinding{m.keys.Select.Help().Key, "open/select"},
		)
	case ModeFolder:
		bindings = append(bindings,
			helpBinding{m.keys.Select.Help().Key, "select dir"},
			helpBinding{m.keys.Navigate.Help().Key, "open dir"},
			helpBinding{m.keys.SelectDir.Help().Key, "select here"},
		)
	case ModeAny:
		bindings = append(bindings,
			helpBinding{m.keys.Select.Help().Key, "open/select"},
			helpBinding{m.keys.Toggle.Help().Key, "select item"},
			helpBinding{m.keys.SelectDir.Help().Key, "select here"},
		)
	case ModeMultiple:
		bindings = append(bindings,
			helpBinding{m.keys.Toggle.Help().Key, "toggle"},
			helpBinding{m.keys.ToggleAll.Help().Key, "toggle all"},
			helpBinding{m.keys.Navigate.Help().Key, "open dir"},
			helpBinding{m.keys.Select.Help().Key, "confirm"},
		)
	}

	bindings = append(bindings,
		helpBinding{m.keys.Back.Help().Key, m.keys.Back.Help().Desc},
		helpBinding{m.keys.Search.Help().Key, m.keys.Search.Help().Desc},
	)
	if !m.hiddenForced {
		bindings = append(bindings,
			helpBinding{m.keys.Hidden.Help().Key, m.keys.Hidden.Help().Desc},
		)
	}

	if m.options.Interactive {
		if m.options.Mode != ModeFolder {
			bindings = append(bindings,
				helpBinding{m.keys.NewFile.Help().Key, m.keys.NewFile.Help().Desc},
			)
			bindings = append(bindings,
				helpBinding{m.keys.NewFolder.Help().Key, m.keys.NewFolder.Help().Desc},
			)
		} else {
			bindings = append(bindings,
				helpBinding{m.keys.NewFile.Help().Key + "/" + m.keys.NewFolder.Help().Key, m.keys.NewFolder.Help().Desc},
			)
		}
		bindings = append(bindings,
			helpBinding{m.keys.Delete.Help().Key, m.keys.Delete.Help().Desc},
		)
	}

	bindings = append(bindings,
		helpBinding{m.keys.Cancel.Help().Key, "quit"},
	)

	sep := m.styles.HelpSep.Render(" | ")
	var rendered []string
	for _, b := range bindings {
		item := m.styles.HelpKey.Render(b.key) + " " + m.styles.HelpDesc.Render(b.desc)
		rendered = append(rendered, item)
	}

	maxWidth := m.width
	if maxWidth <= 0 {
		maxWidth = 80
	}

	sepWidth := 3
	var lines []string
	var currentLine string
	currentWidth := 0

	for i, item := range rendered {
		visibleWidth := len(bindings[i].key) + 1 + len(bindings[i].desc)

		needed := visibleWidth
		if currentWidth > 0 {
			needed += sepWidth
		}

		if currentWidth > 0 && currentWidth+needed > maxWidth {
			lines = append(lines, currentLine)
			currentLine = item
			currentWidth = visibleWidth
		} else {
			if currentWidth > 0 {
				currentLine += sep + item
				currentWidth += sepWidth + visibleWidth
			} else {
				currentLine = item
				currentWidth = visibleWidth
			}
		}
	}
	if currentLine != "" {
		lines = append(lines, currentLine)
	}

	return lines
}
