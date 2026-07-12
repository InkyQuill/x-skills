package ui

// ClampIndex confines index to the valid range for count items.
func ClampIndex(index, count int) int {
	if count <= 0 || index < 0 {
		return 0
	}
	if index >= count {
		return count - 1
	}
	return index
}

// ClampScroll confines scroll to the valid starting line for a viewport.
func ClampScroll(scroll, bodyHeight, viewportHeight int) int {
	if scroll < 0 || bodyHeight <= 0 || viewportHeight <= 0 {
		return 0
	}
	maxScroll := bodyHeight - viewportHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	if scroll > maxScroll {
		return maxScroll
	}
	return scroll
}

// VisibleLines returns an independent copy of the lines visible in a viewport.
func VisibleLines(lines []string, scroll, height int) []string {
	if height <= 0 || len(lines) == 0 {
		return []string{}
	}
	scroll = ClampScroll(scroll, len(lines), height)
	end := scroll + height
	if end > len(lines) {
		end = len(lines)
	}
	return append([]string(nil), lines[scroll:end]...)
}
