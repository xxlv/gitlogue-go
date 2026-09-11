package ui

// stepSpeed doubles or halves the current multiplier. There is no cap:
// the user can keep tapping +/- (or pass any --speed > 0).
func stepSpeed(cur float64, dir int) float64 {
	if dir == 0 {
		return cur
	}
	if cur <= 0 {
		cur = 1
	}
	if dir > 0 {
		return cur * 2
	}
	return cur / 2
}
