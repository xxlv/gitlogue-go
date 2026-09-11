package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/xxlv/gitlogue-go/internal/animator"
	"github.com/xxlv/gitlogue-go/internal/gitengine"
	"github.com/xxlv/gitlogue-go/internal/highlighter"
	"github.com/xxlv/gitlogue-go/internal/scheduler"
)

// Model is the Elm-architecture player. The scheduler owns time; the stage
// owns the document. Update never blocks on I/O or timers.
type Model struct {
	tracks []Track
	idx    int
	sched  *scheduler.Scheduler
	stage  *animator.Stage
	hl     *highlighter.Pool
	files  []gitengine.FileChange
	hash   string
	commit gitengine.CommitInfo
	fps    int
	opts   scheduler.Options

	// InterGap is the wait after a finished track before the next commit.
	// Zero means same-frame advance (used by tests).
	InterGap time.Duration

	browsing bool
	selected string
	between  bool
	gapLeft  time.Duration

	// fileReplay is true after Enter/Space on a focused file: Done must
	// not auto-advance the playlist, and Space-when-done restarts that file.
	fileReplay bool

	width  int
	height int
	last   time.Time
	frames uint64

	// editOff is the first visible 0-based line of the editor body. Playback
	// keeps it locked to the caret; once paused or finished the user can
	// scroll (mouse wheel, PgUp/PgDn, Home/End) so a long file is not stuck
	// on its last lines.
	editOff int

	// played are files in the current commit whose replay has finished
	// (playhead moved on, or the scheduler is Done). The tree marks them
	// with ✓ so you can see what has already typed itself.
	played map[string]struct{}

	// finalView hides the post-playback review overlay and shows the
	// committed file instead. Toggle with v; playback itself is unchanged.
	finalView bool

	// inspecting is set when the user steps to another commit with j/k/[ /].
	// install() clears browsing, so this flag must survive that reset or the
	// next frame would treat the parked commit as Done and auto-play the next.
	inspecting bool
}

// NewPlayer constructs a dual-pane player for a single compiled script.
func NewPlayer(script animator.Script, diff *gitengine.CommitDiff, opts scheduler.Options, theme string) Model {
	return NewPlaylist([]Track{{Diff: diff, Script: script}}, opts, theme)
}

func stageFromDiff(diff *gitengine.CommitDiff) *animator.Stage {
	old := make(map[string]string)
	if diff != nil {
		for _, f := range diff.Files {
			old[f.DisplayPath()] = f.OldContent
		}
	}
	return animator.NewStage(old)
}

// Init starts the 60 FPS heartbeat.
func (m Model) Init() tea.Cmd {
	return scheduler.Tick(m.fps)
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case scheduler.FrameMsg:
		dt := scheduler.Interval(m.fps)
		if !m.last.IsZero() {
			dt = msg.Time.Sub(m.last)
			if dt < 0 {
				dt = scheduler.Interval(m.fps)
			}
		}
		m.last = msg.Time
		wasPlaying := m.sched != nil && m.sched.Playing()
		m.tickPlayback(dt)
		if wasPlaying {
			m.syncEditOff()
		}
		m.frames++
		return m, scheduler.Tick(m.fps)

	case tea.MouseMsg:
		m.onMouse(msg)

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case " ":
			m.onSpace()
		case "enter":
			m.onEnter()
		case "j", "n", "tab", "down":
			m.moveTree(1)
		case "k", "p", "shift+tab", "up":
			m.moveTree(-1)
		case "left", "-", "_":
			m.nudgeSpeed(-1)
		case "right", "+", "=":
			m.nudgeSpeed(1)
		case "v":
			m.toggleFinalView()
		case "r":
			m.restart()
		case "pgup":
			m.scrollEditor(-m.editorPage())
		case "pgdown":
			m.scrollEditor(m.editorPage())
		case "home":
			m.scrollEditorTo(0)
		case "end":
			m.scrollEditorTo(m.editorMaxOff())
		case "]":
			m.nextTrack()
		case "[":
			m.prevTrack()
		}
	}
	return m, nil
}

// View implements tea.Model.
func (m Model) View() string {
	snap := scheduler.Snapshot{}
	if m.sched != nil {
		snap = m.sched.Snapshot()
	}
	treeW, editW, bodyH := m.layout()

	tree := m.renderTree(treeW, bodyH)
	edit := m.renderEditor(editW, bodyH)
	body := lipgloss.JoinHorizontal(lipgloss.Top, tree, vbar(bodyH), edit)

	var b strings.Builder
	b.WriteString(m.renderStatus(snap))
	b.WriteByte('\n')
	b.WriteString(m.renderCredits())
	b.WriteByte('\n')
	b.WriteString(m.renderProgress())
	b.WriteByte('\n')
	b.WriteString(body)
	b.WriteByte('\n')
	b.WriteString(helpStyle.Render("[Space] pause/resume | [Enter] replay file | [j/k/↑↓] files | [+/-] speed | [PgUp/PgDn] scroll | [v] overlay | [r] restart | [q] quit"))
	return b.String()
}

func (m Model) renderStatus(snap scheduler.Snapshot) string {
	width := m.width
	if width <= 0 {
		width = 80
	}

	icon := "▶"
	if snap.Done && m.idx+1 >= len(m.tracks) {
		icon = "■"
	} else if m.between || !snap.Playing {
		icon = "⏸"
	}
	file := "(idle)"
	if m.selected != "" {
		file = m.selected
	} else if buf := m.editorBuffer(); buf != nil && buf.Path() != "" {
		file = buf.Path()
	}
	hash := m.hash
	if hash == "" {
		hash = "replay"
	}
	n := len(m.tracks)
	if n < 1 {
		n = 1
	}

	// Clock is second-resolution and fixed-width so the bar does not
	// reflow every frame (Duration.String jumps 990ms → 1s → 1.01s).
	right := dimStyle.Render(fmt.Sprintf("%s   %.2f×", clockPair(snap.Elapsed, snap.Duration), snap.Speed))
	rightW := lipgloss.Width(right)

	prefix := fmt.Sprintf("gitlogue-go  %s  %d/%d  %s  ", icon, m.idx+1, n, hash)
	fileW := width - lipgloss.Width(prefix) - rightW - 1
	if fileW < 8 {
		fileW = 8
	}
	left := statusStyle.Render(prefix + truncate(file, fileW))

	gap := width - lipgloss.Width(left) - rightW
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

// clockPair is a stable mm:ss / mm:ss field. Width stays 13 until a
// replay exceeds 100 minutes.
func clockPair(elapsed, total time.Duration) string {
	return fmt.Sprintf("%5s / %5s", clock(elapsed), clock(total))
}

func clock(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	sec := int(d / time.Second)
	m, s := sec/60, sec%60
	if m < 100 {
		return fmt.Sprintf("%02d:%02d", m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

func (m Model) renderCredits() string {
	width := m.width
	if width <= 0 {
		width = 80
	}
	var parts []string
	if sub := m.commit.Subject(); sub != "" {
		parts = append(parts, creditName.Render(sub))
	}
	for _, cr := range m.commit.Credits() {
		who := cr.Who.Name
		if who == "" {
			who = cr.Who.Email
		}
		bit := creditRole.Render(cr.Role.Label()) + " " + creditName.Render(who)
		if cr.Agent {
			bit += " " + agentTag.Render("[agent]")
		}
		parts = append(parts, bit)
	}
	if len(parts) == 0 {
		return dimStyle.MaxWidth(width).Render("—")
	}
	line := strings.Join(parts, dimStyle.Render("  ·  "))
	return lipgloss.NewStyle().MaxWidth(width).Render(line)
}

func (m Model) renderProgress() string {
	width := m.width
	if width <= 0 {
		width = 80
	}
	filled := int(m.playlistProgress() * float64(width))
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}
	return barFill.Render(strings.Repeat("█", filled)) +
		barEmpty.Render(strings.Repeat("░", width-filled))
}

const reviewGutterW = 2

func (m Model) renderEditor(width, height int) string {
	if width < 1 || height < 1 {
		return ""
	}
	buf := m.editorBuffer()
	review, inReview := m.reviewDoc()
	header := "(no file open)"
	if buf != nil && buf.Path() != "" {
		header = buf.Path()
	}
	if inReview && m.finalView {
		header = finalHeader(header, width)
	} else if inReview {
		header = reviewHeader(header, review, width)
	}

	var lines []string
	lines = append(lines, headerStyle.Render(truncate(header, width)))
	lines = append(lines, dimStyle.Render(strings.Repeat("─", max(1, width))))

	bodyH := height - 2
	if bodyH < 1 {
		bodyH = 1
	}

	n := 0
	if inReview {
		n = len(review)
	} else if buf != nil {
		n = buf.LineCount()
	}
	if n == 0 {
		msg := "  waiting for first keystroke"
		if inReview || m.showingFinal() {
			msg = "  (empty file)"
		}
		lines = append(lines, dimStyle.Render(msg))
		return clip(strings.Join(lines, "\n"), width, height)
	}

	caretRow, caretCol := -1, -1
	playhead := m.playheadPath()
	onPlayhead := buf != nil && buf.Path() == playhead
	if buf != nil && onPlayhead {
		caretRow, caretCol = buf.Caret()
	}
	row := caretRow
	if inReview {
		row = reviewIndex(review, caretRow) + 1
	}
	from, to := m.editorRange(n, bodyH, row, onPlayhead && !inReview)

	numW := len(strconv.Itoa(n))
	if inReview {
		if w := reviewNumWidth(review); w > numW {
			numW = w
		}
	}
	if numW < 3 {
		numW = 3
	}
	nums := numStyle.Width(numW)
	blink := onPlayhead && !inReview && blinkOn(m.frames, m.fps)
	src := ""
	if buf != nil {
		src = buf.String()
	}
	if inReview {
		if ns := reviewNewSource(m.lookupSelected()); ns != "" {
			src = ns
		}
	}
	pool := m.hl
	if pool == nil {
		pool = highlighter.NewPool("")
	}
	path := ""
	if buf != nil {
		path = buf.Path()
	}
	styled := pool.For(path).Lines(src)
	codeW := width - numW - 3 - reviewGutterW
	if codeW < 8 {
		codeW = 8
	}
	origin := 0
	if onPlayhead && caretCol >= 0 {
		origin = hOrigin(caretCol, codeW)
	}

	for i := from + 1; i <= to; i++ {
		if inReview {
			rr := review[i-1]
			active := onPlayhead && i == row
			lineOrigin := 0
			if active {
				lineOrigin = origin
			}
			line := m.renderReviewLine(styled, rr, codeW, lineOrigin)
			lines = append(lines, formatEditorRow(rr.Number, numW, codeW, width, nums, line, active, rr.Mark))
			continue
		}
		plain := buf.Line(i)
		active := onPlayhead && i == row
		line := m.renderLine(styled, i, plain, row, caretCol, origin, codeW, blink, active, reviewContext)
		lines = append(lines, formatEditorRow(i, numW, codeW, width, nums, line, active, reviewContext))
	}
	return clip(strings.Join(lines, "\n"), width, height)
}

func reviewHeader(path string, rows []reviewRow, width int) string {
	add, mod, del := reviewStats(rows)
	var bits []string
	if add > 0 {
		bits = append(bits, kindAdd.Render("+"+strconv.Itoa(add)))
	}
	if mod > 0 {
		bits = append(bits, kindMod.Render("~"+strconv.Itoa(mod)))
	}
	if del > 0 {
		bits = append(bits, kindDel.Render("-"+strconv.Itoa(del)))
	}
	if len(bits) == 0 {
		return path
	}
	legend := strings.Join(bits, " ")
	room := width - lipgloss.Width(legend) - 2
	if room < 8 {
		room = 8
	}
	return truncate(path, room) + "  " + legend
}

func finalHeader(path string, width int) string {
	tag := dimStyle.Render("final")
	room := width - lipgloss.Width(tag) - 2
	if room < 8 {
		room = 8
	}
	return truncate(path, room) + "  " + tag
}

func reviewNumWidth(rows []reviewRow) int {
	maxN := 0
	for _, r := range rows {
		if r.Number > maxN {
			maxN = r.Number
		}
	}
	return len(strconv.Itoa(maxN))
}

func reviewNewSource(fc *gitengine.FileChange) string {
	if fc == nil {
		return ""
	}
	return fc.NewContent
}

func formatEditorRow(i, numW, codeW, width int, nums lipgloss.Style, code string, active bool, mark reviewMark) string {
	n := ""
	if i > 0 {
		n = strconv.Itoa(i)
	}
	gutter, gut := reviewGutter(mark)
	sep, sepSty := " │ ", dimStyle
	numSty := nums.Width(numW)
	cell := reviewCell(mark)
	if active {
		sep, sepSty = " ▌ ", playBar
		numSty = playNum.Width(numW)
		if mark == reviewContext {
			cell = playRow
		}
	}
	code = cell.Width(codeW).MaxWidth(codeW).MaxHeight(1).Render(code)
	row := gut.Width(reviewGutterW).Render(gutter) + numSty.Render(n) + sepSty.Render(sep) + code
	return lipgloss.NewStyle().MaxWidth(width).MaxHeight(1).Render(row)
}

func reviewGutter(mark reviewMark) (string, lipgloss.Style) {
	g := mark.glyph() + " "
	switch mark {
	case reviewAdd:
		return g, gutAdd
	case reviewDel:
		return g, gutDel
	case reviewMod:
		return g, gutMod
	default:
		return "  ", dimStyle
	}
}

func reviewCell(mark reviewMark) lipgloss.Style {
	switch mark {
	case reviewAdd:
		return addRow
	case reviewDel:
		return delRow
	case reviewMod:
		return modRow
	default:
		return lipgloss.NewStyle()
	}
}

func (m Model) renderReviewLine(styled [][]highlighter.Span, rr reviewRow, codeW, origin int) string {
	if rr.Mark == reviewDel {
		return cropRunes(rr.Text, origin, codeW)
	}
	return m.renderLine(styled, rr.Number, rr.Text, -1, -1, origin, codeW, false, false, rr.Mark)
}

func (m Model) renderLine(styled [][]highlighter.Span, row int, plain string, caretRow, caretCol, origin, codeW int, blink, active bool, mark reviewMark) string {
	var spans []highlighter.Span
	if row >= 1 && row <= len(styled) {
		spans = styled[row-1]
	}
	useHL := highlighter.Visible(spans) == plain
	onCaret := row == caretRow
	rel := caretCol - origin
	if rel < 0 {
		rel = 0
	}
	bg, tinted := reviewBG(mark)
	switch {
	case useHL:
		spans = highlighter.Slice(spans, origin, codeW)
		switch {
		case tinted:
			spans = highlighter.Tint(spans, bg)
		case active:
			spans = highlighter.Emphasize(spans, playLineBg)
		}
		if onCaret {
			return highlighter.RenderWithCaret(spans, rel, blink)
		}
		return highlighter.Render(spans)
	case onCaret:
		return withCaret(cropRunes(plain, origin, codeW), rel, blink)
	default:
		return cropRunes(plain, origin, codeW)
	}
}

func reviewBG(mark reviewMark) (lipgloss.Color, bool) {
	switch mark {
	case reviewAdd:
		return addRowBg, true
	case reviewMod:
		return modRowBg, true
	default:
		return "", false
	}
}

func withCaret(line string, col int, on bool) string {
	if !on {
		return line
	}
	rs := []rune(line)
	if col < 0 {
		col = 0
	}
	if col > len(rs) {
		col = len(rs)
	}
	if col == len(rs) {
		return string(rs) + caretStyle.Render(" ")
	}
	ch := string(rs[col])
	if ch == "\t" {
		ch = " "
	}
	return string(rs[:col]) + caretStyle.Render(ch) + string(rs[col+1:])
}

func blinkOn(frames uint64, fps int) bool {
	period := uint64(fps) / 2
	if period == 0 {
		period = 1
	}
	return (frames/period)%2 == 0
}

const editorWheelDelta = 3

func (m Model) followCaret(onPlayhead bool) bool {
	return onPlayhead && !m.browsing && m.sched != nil && m.sched.Playing()
}

func (m Model) editorRange(n, bodyH, row int, onPlayhead bool) (from, to int) {
	if m.followCaret(onPlayhead) {
		return window(max(row-1, 0), n, bodyH)
	}
	return scrollWindow(m.editOff, n, bodyH)
}

func (m *Model) syncEditOff() {
	n := m.editorDisplayCount()
	if n == 0 {
		m.editOff = 0
		return
	}
	_, _, paneH := m.layout()
	bodyH := editorBodyH(paneH)
	from, _ := window(m.editorCaretIndex(), n, bodyH)
	m.editOff = from
}

func (m Model) editorDisplayCount() int {
	if rows, ok := m.reviewDoc(); ok {
		return len(rows)
	}
	buf := m.editorBuffer()
	if buf == nil {
		return 0
	}
	return buf.LineCount()
}

func (m Model) editorCaretIndex() int {
	if rows, ok := m.reviewDoc(); ok {
		row := 0
		if buf := m.editorBuffer(); buf != nil {
			row, _ = buf.Caret()
		}
		return reviewIndex(rows, row)
	}
	buf := m.editorBuffer()
	if buf == nil {
		return 0
	}
	row, _ := buf.Caret()
	return max(row-1, 0)
}

func (m Model) editorPage() int {
	_, _, paneH := m.layout()
	h := editorBodyH(paneH)
	if h > 1 {
		return h - 1
	}
	return 1
}

func (m Model) editorMaxOff() int {
	n := m.editorDisplayCount()
	if n == 0 {
		return 0
	}
	_, _, paneH := m.layout()
	off := n - editorBodyH(paneH)
	if off < 0 {
		return 0
	}
	return off
}

func (m *Model) pauseForScroll() {
	if m.sched != nil && m.sched.Playing() {
		m.sched.Pause()
		m.syncEditOff()
	}
}

func (m *Model) scrollEditor(delta int) {
	m.pauseForScroll()
	m.scrollEditorTo(m.editOff + delta)
}

func (m *Model) scrollEditorTo(off int) {
	m.pauseForScroll()
	if off < 0 {
		off = 0
	}
	if maxOff := m.editorMaxOff(); off > maxOff {
		off = maxOff
	}
	m.editOff = off
}

func (m *Model) onMouse(msg tea.MouseMsg) {
	treeW, _, _ := m.layout()
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		if msg.X < treeW {
			m.moveTree(-1)
			return
		}
		m.scrollEditor(-editorWheelDelta)
	case tea.MouseButtonWheelDown:
		if msg.X < treeW {
			m.moveTree(1)
			return
		}
		m.scrollEditor(editorWheelDelta)
	}
}
