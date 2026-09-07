package scheduler

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// FrameMsg is the 60 FPS heartbeat delivered to tea.Model.Update.
type FrameMsg struct {
	Time time.Time
}

// Tick schedules one FrameMsg after Interval(fps). The Model must
// reschedule Tick on every frame — that is the Bubble Tea idiom.
func Tick(fps int) tea.Cmd {
	d := Interval(fps)
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return FrameMsg{Time: t}
	})
}
