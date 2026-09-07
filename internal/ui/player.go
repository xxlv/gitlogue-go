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
		m.tickPlayback(dt)
		m.frames++
		return m, scheduler.Tick(m.fps)

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case " ":
			m.onSpace()
		case "enter":
			m.onEnter()
		case "j", "left", "-", "_":
			m.nudgeSpeed(-1)
		case "k", "right", "+", "=":
			m.nudgeSpeed(1)
		case "r":
			m.restart()
		case "n", "tab", "down":
			m.moveTree(1)
		case "p", "shift+tab", "up":
			m.moveTree(-1)
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
	b.WriteString(helpStyle.Render("[Space] pause/resume | [Enter] replay file | [j/k] speed | [Tab/↑↓] files | [n/p] next file | [r] restart | [q] quit"))
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

func (m Model) renderEditor(width, height int) string {
	if width < 1 || height < 1 {
		return ""
	}
	buf := m.editorBuffer()
	header := "(no file open)"
	if buf != nil && buf.Path() != "" {
		header = buf.Path()
	}

	var lines []string
	lines = append(lines, headerStyle.Render(truncate(header, width)))
	lines = append(lines, dimStyle.Render(strings.Repeat("─", max(1, width))))

	bodyH := height - 2
	if bodyH < 1 {
		bodyH = 1
	}

	if buf == nil || buf.LineCount() == 0 {
		lines = append(lines, dimStyle.Render("  waiting for first keystroke"))
		return clip(strings.Join(lines, "\n"), width, height)
	}

	n := buf.LineCount()
	row, col := buf.Caret()
	playhead := m.playheadPath()
	onPlayhead := buf.Path() == playhead
	if !onPlayhead {
		row, col = -1, -1
	}
	from, to := window(max(row-1, 0), n, bodyH) // 0-based [from, to)

	numW := len(strconv.Itoa(n))
	if numW < 3 {
		numW = 3
	}
	nums := numStyle.Width(numW)
	blink := onPlayhead && blinkOn(m.frames, m.fps)
	src := buf.String()
	pool := m.hl
	if pool == nil {
		pool = highlighter.NewPool("")
	}
	styled := pool.For(buf.Path()).Lines(src)
	codeW := width - numW - 3
	if codeW < 8 {
		codeW = 8
	}
	origin := 0
	if onPlayhead && col >= 0 {
		origin = hOrigin(col, codeW)
	}

	for i := from + 1; i <= to; i++ {
		plain := buf.Line(i)
		active := onPlayhead && i == row
		line := m.renderLine(styled, i, plain, row, col, origin, codeW, blink, active)
		lines = append(lines, formatEditorRow(i, numW, codeW, width, nums, line, active))
	}
	return clip(strings.Join(lines, "\n"), width, height)
}

func formatEditorRow(i, numW, codeW, width int, nums lipgloss.Style, code string, active bool) string {
	n := strconv.Itoa(i)
	code = lipgloss.NewStyle().MaxWidth(codeW).MaxHeight(1).Render(code)
	if !active {
		row := nums.Render(n) + " │ " + code
		return lipgloss.NewStyle().MaxWidth(width).MaxHeight(1).Render(row)
	}
	row := playNum.Width(numW).Render(n) + playBar.Render(" ▌ ") + playRow.Width(codeW).MaxHeight(1).Render(code)
	return lipgloss.NewStyle().MaxWidth(width).MaxHeight(1).Render(row)
}

func (m Model) renderLine(styled [][]highlighter.Span, row int, plain string, caretRow, caretCol, origin, codeW int, blink, active bool) string {
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
	switch {
	case useHL:
		spans = highlighter.Slice(spans, origin, codeW)
		if active {
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
