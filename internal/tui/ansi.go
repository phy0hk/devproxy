package tui

import (
	"strings"
)

const (
	ansiGround = iota
	ansiEscape
	ansiCSI
	ansiOSC
	ansiConsumeOne
)

// Sanitizer strips terminal control sequences and reports screen clears.
// Feed may be called with chunks; parser state is carried between calls.
type Sanitizer struct {
	state  int
	csi    string
	oscEsc bool
}

func NewSanitizer() *Sanitizer {
	return &Sanitizer{state: ansiGround}
}

// Feed returns printable text and whether a screen-clear sequence was seen.
func (s *Sanitizer) Feed(input string) (string, bool) {
	var out strings.Builder
	cleared := false

	for index := 0; index < len(input); index++ {
		value := input[index]

		switch s.state {
		case ansiGround:
			switch {
			case value == 0x1b:
				s.state = ansiEscape
			case value == '\r':
				if index+1 < len(input) && input[index+1] == '\n' {
					index++
				} else {
					out.Reset()
				}
			case value == '\t':
				out.WriteByte(value)
			case value >= 0x20 && value != 0x7f:
				out.WriteByte(value)
			}

		case ansiEscape:
			switch value {
			case '[':
				s.state = ansiCSI
				s.csi = ""
			case ']', 'P', 'X', '^', '_':
				s.state = ansiOSC
				s.oscEsc = false
			case '(', ')', '*', '+', '#', '%':
				s.state = ansiConsumeOne
			default:
				s.state = ansiGround
			}

		case ansiCSI:
			if value >= 0x40 && value <= 0x7e {
				params := strings.Trim(s.csi, "?<>=! ")
				switch value {
				case 'J':
					if params == "2" || params == "3" {
						cleared = true
					}
				case 'H', 'f':
					if params == "" || params == "1;1" {
						cleared = true
					}
				}
				s.csi = ""
				s.state = ansiGround
			} else {
				s.csi += string(value)
			}

		case ansiOSC:
			if s.oscEsc {
				s.oscEsc = false
				if value == '\\' {
					s.state = ansiGround
				}
				continue
			}
			if value == 0x07 {
				s.state = ansiGround
			} else if value == 0x1b {
				s.oscEsc = true
			}

		case ansiConsumeOne:
			s.state = ansiGround
		}
	}

	return out.String(), cleared
}

// CleanTerminalLine strips control sequences from one stored log line.
func CleanTerminalLine(input string) (string, bool) {
	input = strings.TrimRight(input, "\r")
	return NewSanitizer().Feed(input)
}

// SanitizeTerminalText strips control sequences and discards clear markers.
func SanitizeTerminalText(input string) string {
	text, _ := CleanTerminalLine(input)
	return text
}
