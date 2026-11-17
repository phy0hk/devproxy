package tui

import "testing"

func TestComputeLayoutAtSupportedSizes(t *testing.T) {
	cases := []struct {
		name   string
		width  int
		height int
	}{
		{"wide", 120, 40},
		{"standard", 80, 24},
		{"minimum", MinWidth, MinHeight},
		{"tall-narrow", 90, 50},
		{"very-wide", 200, 40},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			layout := ComputeLayout(testCase.width, testCase.height, false)

			if layout.TooSmall {
				t.Fatalf("layout for %dx%d reported too small", testCase.width, testCase.height)
			}
			if layout.Width != testCase.width || layout.Height != testCase.height {
				t.Fatalf("layout size = %dx%d, want %dx%d", layout.Width, layout.Height, testCase.width, testCase.height)
			}
			if layout.ListWidth+layout.Gap+layout.LogWidth != testCase.width {
				t.Fatalf(
					"panels %d + gap %d + log %d != width %d",
					layout.ListWidth,
					layout.Gap,
					layout.LogWidth,
					testCase.width,
				)
			}
			if layout.ListWidth < 20 {
				t.Fatalf("list width %d is unusable", layout.ListWidth)
			}
			if layout.LogWidth < 34 {
				t.Fatalf("log width %d is below the minimum", layout.LogWidth)
			}
			if layout.ContentHeight != testCase.height-3 {
				t.Fatalf("content height = %d, want %d", layout.ContentHeight, testCase.height-3)
			}
			if layout.ListViewportHeight <= 0 || layout.LogViewportHeight <= 0 {
				t.Fatalf("non-positive viewport heights: list=%d log=%d", layout.ListViewportHeight, layout.LogViewportHeight)
			}
			if layout.ListViewportWidth >= layout.ListWidth || layout.LogViewportWidth >= layout.LogWidth {
				t.Fatalf(
					"viewport widths must fit inside panels: list %d/%d log %d/%d",
					layout.ListViewportWidth,
					layout.ListWidth,
					layout.LogViewportWidth,
					layout.LogWidth,
				)
			}
		})
	}
}

func TestComputeLayoutTooSmall(t *testing.T) {
	cases := []struct {
		name   string
		width  int
		height int
	}{
		{"narrow", MinWidth - 1, 24},
		{"short", 100, MinHeight - 1},
		{"tiny", 40, 10},
		{"zero", 0, 0},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			layout := ComputeLayout(testCase.width, testCase.height, false)
			if !layout.TooSmall {
				t.Fatalf("layout for %dx%d should be too small", testCase.width, testCase.height)
			}
		})
	}
}

func TestComputeLayoutHelpReducesContent(t *testing.T) {
	withoutHelp := ComputeLayout(100, 30, false)
	withHelp := ComputeLayout(100, 30, true)

	if withoutHelp.HelpHeight != 1 {
		t.Fatalf("compact help height = %d, want 1", withoutHelp.HelpHeight)
	}
	if withHelp.HelpHeight != 5 {
		t.Fatalf("expanded help height = %d, want 5", withHelp.HelpHeight)
	}
	if withHelp.ContentHeight != withoutHelp.ContentHeight-4 {
		t.Fatalf(
			"expanded content height = %d, want compact %d - 4",
			withHelp.ContentHeight,
			withoutHelp.ContentHeight,
		)
	}
	if withHelp.LogViewportHeight != withoutHelp.LogViewportHeight-4 {
		t.Fatalf(
			"expanded log viewport = %d, want compact %d - 4",
			withHelp.LogViewportHeight,
			withoutHelp.LogViewportHeight,
		)
	}
}

func TestComputeLayoutClampsListWidth(t *testing.T) {
	narrow := ComputeLayout(72, 24, false)
	if narrow.ListWidth != 26 {
		t.Fatalf("narrow list width = %d, want 26", narrow.ListWidth)
	}

	wide := ComputeLayout(200, 40, false)
	if wide.ListWidth != 46 {
		t.Fatalf("wide list width = %d, want clamp at 46", wide.ListWidth)
	}
}
