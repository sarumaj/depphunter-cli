package termview

import (
	"strconv"
	"strings"
)

// Events the terminal reports. Keys arrive as bytes or escape sequences; the mouse
// arrives as SGR reports (see mouseTracking) once tracking is on.
type eventKind int

const (
	evNone eventKind = iota
	evKey
	evMouse
	evPaste // bracketed paste, ignored: the map has no text to paste into
)

type mouseAction int

const (
	mousePress mouseAction = iota
	mouseRelease
	mouseMove
	mouseWheel
)

type modifiers struct{ shift, alt, ctrl bool }

type keyPress struct {
	name string // a DOM key name ("ArrowLeft", "Escape") or the character typed
	mods modifiers
}

type mouseReport struct {
	action  mouseAction
	button  int // 0 left, 1 middle, 2 right
	col     int // 1-based character cell
	row     int
	wheelUp bool
	mods    modifiers
}

type event struct {
	kind  eventKind
	key   keyPress
	mouse mouseReport
}

// parseEvent decodes the first event in b. It reports how many bytes it consumed and,
// when b ends inside an escape sequence, that more input is needed (the caller waits
// briefly before deciding a lone ESC was the Escape key).
func parseEvent(b []byte) (ev event, n int, incomplete bool) {
	if len(b) == 0 {
		return event{}, 0, true
	}
	if b[0] != 0x1b {
		ev, n := parseRune(b)
		return ev, n, false
	}
	if len(b) == 1 {
		return event{}, 0, true // ESC, or the start of a sequence still arriving
	}
	switch b[1] {
	case '[':
		return parseCSI(b)
	case 'O': // SS3: the keypad-mode arrows and F1-F4 some terminals send
		if len(b) < 3 {
			return event{}, 0, true
		}
		if name, ok := ss3Keys[b[2]]; ok {
			return event{kind: evKey, key: keyPress{name: name}}, 3, false
		}
		return event{}, 3, false
	case 0x1b:
		return event{kind: evKey, key: keyPress{name: "Escape"}}, 1, false
	default: // ESC x: the terminal's way of saying Alt+x
		ev, n := parseRune(b[1:])
		ev.key.mods.alt = true
		return ev, n + 1, false
	}
}

// parseRune turns one plain character into a key. Control bytes below space are
// Ctrl+<letter> except for the few that have keys of their own.
func parseRune(b []byte) (event, int) {
	c := b[0]
	switch {
	case c == '\r' || c == '\n':
		return event{kind: evKey, key: keyPress{name: "Enter"}}, 1
	case c == '\t':
		return event{kind: evKey, key: keyPress{name: "Tab"}}, 1
	case c == 0x7f || c == 8:
		return event{kind: evKey, key: keyPress{name: "Backspace"}}, 1
	case c == 0x1b:
		return event{kind: evKey, key: keyPress{name: "Escape"}}, 1
	case c < 0x20:
		return event{kind: evKey, key: keyPress{
			name: string(rune('a' + c - 1)),
			mods: modifiers{ctrl: true},
		}}, 1
	case c < 0x80:
		return event{kind: evKey, key: keyPress{name: string(rune(c)), mods: modifiers{shift: c >= 'A' && c <= 'Z'}}}, 1
	}
	// A multi-byte character: pass it on whole, however long it is.
	n := 1
	for n < len(b) && b[n]&0xc0 == 0x80 {
		n++
	}
	return event{kind: evKey, key: keyPress{name: string(b[:n])}}, n
}

// csiKeys maps the final byte of a CSI sequence without parameters to a key name.
var csiKeys = map[byte]string{
	'A': "ArrowUp", 'B': "ArrowDown", 'C': "ArrowRight", 'D': "ArrowLeft",
	'H': "Home", 'F': "End", 'Z': "Tab",
}

var ss3Keys = map[byte]string{
	'A': "ArrowUp", 'B': "ArrowDown", 'C': "ArrowRight", 'D': "ArrowLeft",
	'H': "Home", 'F': "End", 'P': "F1", 'Q': "F2", 'R': "F3", 'S': "F4",
}

// tildeKeys maps the numeric CSI sequences ("ESC [ 5 ~" and friends).
var tildeKeys = map[int]string{
	1: "Home", 2: "Insert", 3: "Delete", 4: "End", 5: "PageUp", 6: "PageDown",
	7: "Home", 8: "End", 11: "F1", 12: "F2", 13: "F3", 14: "F4", 15: "F5",
	17: "F6", 18: "F7", 19: "F8", 20: "F9", 21: "F10", 23: "F11", 24: "F12",
}

// parseCSI decodes "ESC [ … final". Unknown sequences are consumed and dropped: a
// terminal answering a query we did not make must not turn into keystrokes.
func parseCSI(b []byte) (event, int, bool) {
	end := 2
	for end < len(b) && (b[end] < 0x40 || b[end] > 0x7e) {
		end++
	}
	if end >= len(b) {
		return event{}, 0, true
	}
	final, body, n := b[end], string(b[2:end]), end+1
	if strings.HasPrefix(body, "<") { // SGR mouse report
		if ev, ok := parseMouse(body[1:], final); ok {
			return ev, n, false
		}
		return event{}, n, false
	}
	if body == "200" && final == '~' {
		return event{kind: evPaste}, n, false
	}
	params := splitParams(body)
	mods := csiModifiers(params)
	if name, ok := csiKeys[final]; ok {
		return event{kind: evKey, key: keyPress{name: name, mods: mods}}, n, false
	}
	if final == '~' && len(params) > 0 {
		if name, ok := tildeKeys[params[0]]; ok {
			return event{kind: evKey, key: keyPress{name: name, mods: mods}}, n, false
		}
	}
	return event{}, n, false
}

// csiModifiers reads the modifier parameter xterm appends to a key sequence: it is a
// bit set of shift, alt and ctrl, offset by one.
func csiModifiers(params []int) modifiers {
	if len(params) < 2 || params[1] < 1 {
		return modifiers{}
	}
	m := params[1] - 1
	return modifiers{shift: m&1 != 0, alt: m&2 != 0, ctrl: m&4 != 0}
}

func splitParams(body string) []int {
	body = strings.TrimLeft(body, "?>")
	if body == "" {
		return nil
	}
	parts := strings.Split(body, ";")
	out := make([]int, len(parts))
	for i, p := range parts {
		out[i], _ = strconv.Atoi(p)
	}
	return out
}

// parseMouse decodes an SGR mouse report: "ESC [ < button ; col ; row M|m", where M
// presses and m releases. The button field carries the modifiers, whether the pointer
// is being dragged, and which way a wheel turned.
func parseMouse(body string, final byte) (event, bool) {
	p := splitParams(body)
	if len(p) != 3 || (final != 'M' && final != 'm') {
		return event{}, false
	}
	code, col, row := p[0], p[1], p[2]
	m := mouseReport{
		col: col, row: row,
		mods: modifiers{shift: code&4 != 0, alt: code&8 != 0, ctrl: code&16 != 0},
	}
	switch {
	case code&64 != 0: // wheel
		m.action, m.wheelUp = mouseWheel, code&1 == 0
	case code&32 != 0: // motion, with or without a button held
		m.action, m.button = mouseMove, code&3
		if code&3 == 3 {
			m.button = -1 // no button: a plain hover
		}
	case final == 'm':
		m.action, m.button = mouseRelease, code&3
	default:
		m.action, m.button = mousePress, code&3
	}
	return event{kind: evMouse, mouse: m}, true
}
