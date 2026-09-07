package ui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

func (m Model) layout() (treeW, editW, bodyH int) {
	w, h := m.width, m.height
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}
	const chrome = 4 // status + credits + progress + help
	bodyH = h - chrome
	if bodyH < 8 {
		bodyH = 8
	}
	treeW = w / 5
	if treeW < 16 {
		treeW = 16
	}
	if treeW > 36 {
		treeW = 36
	}
	const sep = 1
	editW = w - treeW - sep
	if editW < 24 {
		editW = 24
		treeW = w - editW - sep
		if treeW < 12 {
			treeW = 12
			editW = w - treeW - sep
		}
	}
	return treeW, editW, bodyH
}

func (m Model) renderTree(width, height int) string {
	if width < 1 || height < 1 {
		return ""
	}
	rows := treeRows(m.files)
	playhead := m.playheadPath()
	selected := m.selected
	if selected == "" {
		selected = playhead
	}

	var lines []string
	lines = append(lines, headerStyle.Render("FILES"))
	lines = append(lines, dimStyle.Render(strings.Repeat("─", max(1, width))))

	if len(rows) == 0 {
		lines = append(lines, dimStyle.Render("  (none)"))
	}

	focusIdx := 2
	for _, r := range rows {
		line := formatTreeRow(r, playhead, selected, m.filePlayed(r.Path), width)
		if !r.IsDir && r.Path == selected {
			focusIdx = len(lines)
		}
		lines = append(lines, line)
	}

	start, end := window(focusIdx, len(lines), height)
	body := strings.Join(lines[start:end], "\n")
	return clip(body, width, height)
}

func formatTreeRow(r treeRow, playhead, selected string, played bool, width int) string {
	indent := strings.Repeat("  ", r.Depth)
	if r.IsDir {
		return dirStyle.MaxWidth(width).Render(truncate(indent+r.Display+"/", width))
	}
	cursor := treeCursor(r.Path == playhead, played)
	stats := ""
	if r.Ins != 0 || r.Del != 0 {
		stats = "  +" + strconv.Itoa(r.Ins) + "/-" + strconv.Itoa(r.Del)
	}
	raw := indent + cursor + " " + kindMark(r.Kind) + " " + r.Display + stats
	if r.Path == selected {
		return treeActive.MaxWidth(width).Render(truncate(raw, width))
	}
	mark := cursor
	if played {
		mark = treePlayed.Render(cursor)
	}
	prefix := indent + mark + " "
	name := r.Display
	if played {
		name = dimStyle.Render(r.Display)
	}
	colored := styleKind(r.Kind).Render(kindMark(r.Kind)) + " " + name
	if stats != "" {
		colored += kindAdd.Render("  +"+strconv.Itoa(r.Ins)) + kindDel.Render("/-"+strconv.Itoa(r.Del))
	}
	return truncate(prefix+colored, width)
}

func treeCursor(onPlayhead, played bool) string {
	switch {
	case onPlayhead && !played:
		return "▸"
	case played:
		return "✓"
	default:
		return " "
	}
}

func clip(content string, w, h int) string {
	vp := viewport.New(w, h)
	vp.SetContent(content)
	return vp.View()
}

func window(center, total, height int) (start, end int) {
	if total <= height {
		return 0, total
	}
	start = center - height/2
	if start < 0 {
		start = 0
	}
	end = start + height
	if end > total {
		end = total
		start = end - height
	}
	return start, end
}

func editorBodyH(paneH int) int {
	h := paneH - 2 // path header + rule
	if h < 1 {
		return 1
	}
	return h
}

func scrollWindow(off, total, height int) (start, end int) {
	if total <= height {
		return 0, total
	}
	if off < 0 {
		off = 0
	}
	maxOff := total - height
	if off > maxOff {
		off = maxOff
	}
	return off, off + height
}

// hOrigin is the first visible rune so the caret cell stays in the code pane.
// A 2-column right pad keeps newly typed runes off the clipped edge.
func hOrigin(col, viewW int) int {
	if col < 0 || viewW < 1 {
		return 0
	}
	pad := 2
	if viewW <= pad {
		pad = 0
	}
	inner := viewW - pad
	if col < inner {
		return 0
	}
	return col - inner + 1
}

func cropRunes(s string, origin, n int) string {
	if n <= 0 {
		return ""
	}
	rs := []rune(s)
	if origin < 0 {
		origin = 0
	}
	if origin >= len(rs) {
		return ""
	}
	end := origin + n
	if end > len(rs) {
		end = len(rs)
	}
	return string(rs[origin:end])
}

func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	return lipgloss.NewStyle().MaxWidth(width).Render(s)
}

func vbar(height int) string {
	if height < 1 {
		return ""
	}
	lines := make([]string, height)
	for i := range lines {
		lines[i] = sepStyle.Render("│")
	}
	return strings.Join(lines, "\n")
}
