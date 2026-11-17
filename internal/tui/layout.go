package tui

// Minimum terminal size for the two-panel TUI layout.
const (
	MinWidth  = 72
	MinHeight = 16
)

// Layout is the resolved geometry for one terminal size. All sizes are in
// terminal cells; panel widths include their borders.
type Layout struct {
	Width  int
	Height int

	TooSmall bool

	HeaderHeight  int
	StatusHeight  int
	HelpHeight    int
	ContentHeight int

	ListWidth int
	Gap       int
	LogWidth  int

	ListViewportWidth  int
	ListViewportHeight int
	LogViewportWidth   int
	LogViewportHeight  int
}

// ComputeLayout is pure so it can be tested at fixed terminal sizes.
func ComputeLayout(width, height int, showHelp bool) Layout {
	layout := Layout{
		Width:        width,
		Height:       height,
		HeaderHeight: 1,
		StatusHeight: 1,
		Gap:          1,
	}
	if showHelp {
		layout.HelpHeight = 5
	} else {
		layout.HelpHeight = 1
	}

	if width < MinWidth || height < MinHeight {
		layout.TooSmall = true
		return layout
	}

	layout.ContentHeight = height - layout.HeaderHeight - layout.StatusHeight - layout.HelpHeight
	if layout.ContentHeight < 5 {
		layout.TooSmall = true
		return layout
	}

	listWidth := width / 3
	if listWidth < 26 {
		listWidth = 26
	}
	if listWidth > 46 {
		listWidth = 46
	}
	if width-listWidth-layout.Gap < 34 {
		listWidth = width - layout.Gap - 34
	}
	if listWidth < 20 {
		layout.TooSmall = true
		return layout
	}

	layout.ListWidth = listWidth
	layout.LogWidth = width - listWidth - layout.Gap

	// Panels render borders plus one cell of horizontal padding per side.
	layout.ListViewportWidth = max(1, layout.ListWidth-4)
	layout.LogViewportWidth = max(1, layout.LogWidth-4)
	layout.ListViewportHeight = max(1, layout.ContentHeight-3)
	layout.LogViewportHeight = max(1, layout.ContentHeight-3)

	return layout
}

func max(left, right int) int {
	if left > right {
		return left
	}
	return right
}
