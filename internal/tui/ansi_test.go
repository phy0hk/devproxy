package tui

import "testing"

func TestCleanTerminalLineStripsSGR(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"basic", "\x1b[31mred\x1b[0m plain", "red plain"},
		{"bold-underline", "\x1b[1;4mstyled\x1b[22;24m", "styled"},
		{"256-color", "\x1b[38;5;196mred\x1b[0m", "red"},
		{"truecolor", "\x1b[38;2;255;0;128mpink\x1b[0m", "pink"},
		{"private-mode", "\x1b[?25lhidden cursor\x1b[?25h", "hidden cursor"},
		{"cursor-move", "one\x1b[10;20Htwo", "onetwo"},
		{"erase-line", "abc\x1b[Kdef", "abcdef"},
		{"osc-bel", "\x1b]0;window title\x07visible", "visible"},
		{"osc-st", "\x1b]0;window title\x1b\\visible", "visible"},
		{"osc-hyperlink", "\x1b]8;;https://example.com\x1b\\link\x1b]8;;\x1b\\", "link"},
		{"charset", "\x1b(Bplain", "plain"},
		{"control-bytes", "a\x00b\x07c\x7fd", "abcd"},
		{"tab-preserved", "a\tb", "a\tb"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, cleared := CleanTerminalLine(testCase.input)
			if got != testCase.want {
				t.Fatalf("CleanTerminalLine(%q) = %q, want %q", testCase.input, got, testCase.want)
			}
			if cleared {
				t.Fatalf("CleanTerminalLine(%q) reported an unexpected clear", testCase.input)
			}
		})
	}
}

func TestCleanTerminalLineReportsClears(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		cleared  bool
		wantText string
	}{
		{"erase-display", "before\x1b[2Jafter", true, "beforeafter"},
		{"erase-scrollback", "before\x1b[3Jafter", true, "beforeafter"},
		{"home", "before\x1b[Hafter", true, "beforeafter"},
		{"home-1-1", "before\x1b[1;1Hafter", true, "beforeafter"},
		{"position-middle", "before\x1b[10;10Hafter", false, "beforeafter"},
		{"erase-line-no-clear", "before\x1b[Kafter", false, "beforeafter"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, cleared := CleanTerminalLine(testCase.input)
			if cleared != testCase.cleared {
				t.Fatalf("cleared = %v, want %v for %q", cleared, testCase.cleared, testCase.input)
			}
			if got != testCase.wantText {
				t.Fatalf("text = %q, want %q", got, testCase.wantText)
			}
		})
	}
}

func TestSanitizerHandlesSplitChunks(t *testing.T) {
	sanitizer := NewSanitizer()

	text, cleared := sanitizer.Feed("\x1b[38;2;1")
	if text != "" || cleared {
		t.Fatalf("partial truecolor emitted %q (cleared=%v)", text, cleared)
	}

	text, cleared = sanitizer.Feed("2;3mhello")
	if text != "hello" || cleared {
		t.Fatalf("split truecolor = %q (cleared=%v), want hello", text, cleared)
	}

	text, cleared = sanitizer.Feed("\x1b[2")
	if text != "" || cleared {
		t.Fatalf("partial clear emitted %q (cleared=%v)", text, cleared)
	}

	text, cleared = sanitizer.Feed("J")
	if text != "" || !cleared {
		t.Fatalf("split clear = %q (cleared=%v), want cleared", text, cleared)
	}
}

func TestSanitizerHandlesSplitOSC(t *testing.T) {
	sanitizer := NewSanitizer()

	text, _ := sanitizer.Feed("\x1b]8;;https://example.com")
	if text != "" {
		t.Fatalf("partial OSC emitted %q", text)
	}

	text, _ = sanitizer.Feed("\x1b\\link")
	if text != "link" {
		t.Fatalf("OSC link text = %q, want link", text)
	}

	text, _ = sanitizer.Feed("\x1b]8;;\x1b\\")
	if text != "" {
		t.Fatalf("OSC terminator emitted %q", text)
	}
}

func TestSanitizerCarriageReturn(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"overwrite", "progress 10%\rprogress 100%", "progress 100%"},
		{"crlf-collapses", "line\r\n", "line"},
		{"multiple", "a\rb\rc", "c"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			sanitizer := NewSanitizer()
			got, _ := sanitizer.Feed(testCase.input)
			if got != testCase.want {
				t.Fatalf("Feed(%q) = %q, want %q", testCase.input, got, testCase.want)
			}
		})
	}
}

func TestSanitizerHostileIncompleteInput(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"incomplete-csi", "text\x1b[", "text"},
		{"incomplete-osc", "text\x1b]0;title", "text"},
		{"dangling-escape", "text\x1b", "text"},
		{"unknown-escape", "\x1bZtext", "text"},
		{"dcs", "\x1bPq#0;2;0;0;0\x1b\\text", "text"},
		{"empty", "", ""},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, cleared := CleanTerminalLine(testCase.input)
			if got != testCase.want {
				t.Fatalf("CleanTerminalLine(%q) = %q, want %q", testCase.input, got, testCase.want)
			}
			if cleared {
				t.Fatalf("hostile input %q reported a clear", testCase.input)
			}
		})
	}
}

func TestSanitizeTerminalTextDropsClearMarker(t *testing.T) {
	got := SanitizeTerminalText("before\x1b[2Jafter")
	if got != "beforeafter" {
		t.Fatalf("SanitizeTerminalText = %q, want beforeafter", got)
	}
}
