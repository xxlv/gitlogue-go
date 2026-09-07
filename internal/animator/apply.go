package animator

import "strings"

func splitFile(s string) [][]rune {
	if s == "" {
		return nil
	}
	s = strings.TrimSuffix(s, "\n")
	parts := strings.Split(s, "\n")
	lines := make([][]rune, len(parts))
	for i, p := range parts {
		lines[i] = []rune(p)
	}
	return lines
}

func joinFile(lines [][]rune) string {
	if len(lines) == 0 {
		return ""
	}
	parts := make([]string, len(lines))
	for i, l := range lines {
		parts[i] = string(l)
	}
	return strings.Join(parts, "\n") + "\n"
}

func insertLineAt(lines [][]rune, row int) [][]rune {
	if row < 1 {
		row = 1
	}
	empty := []rune{}
	idx := row - 1
	if idx >= len(lines) {
		return append(lines, empty)
	}
	lines = append(lines, nil)
	copy(lines[idx+1:], lines[idx:])
	lines[idx] = empty
	return lines
}

func deleteLineAt(lines [][]rune, row int) [][]rune {
	idx := row - 1
	if idx < 0 || idx >= len(lines) {
		return lines
	}
	return append(lines[:idx], lines[idx+1:]...)
}

func insertRuneAt(lines [][]rune, row, col int, r rune) [][]rune {
	idx := row - 1
	for idx >= len(lines) {
		lines = append(lines, nil)
	}
	if idx < 0 {
		return lines
	}
	line := lines[idx]
	if col > len(line) {
		col = len(line)
	}
	if col < 0 {
		col = 0
	}
	line = append(line, 0)
	copy(line[col+1:], line[col:])
	line[col] = r
	lines[idx] = line
	return lines
}

func deleteRuneBefore(lines [][]rune, row, col int) [][]rune {
	idx := row - 1
	if idx < 0 || idx >= len(lines) || col < 1 {
		return lines
	}
	line := lines[idx]
	if col-1 >= len(line) {
		return lines
	}
	lines[idx] = append(line[:col-1], line[col:]...)
	return lines
}

// Apply replays a single-file script onto old content. The result is the
// contract the TUI buffer must match after a full playback.
func Apply(old string, actions []Action) string {
	path := ""
	for _, a := range actions {
		if a.File != "" {
			path = a.File
			break
		}
	}
	st := NewStage(map[string]string{path: old})
	for _, a := range actions {
		st.Apply(a)
	}
	if buf := st.File(path); buf != nil {
		return buf.String()
	}
	return joinFile(splitFile(old))
}
