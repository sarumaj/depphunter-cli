package termview

import (
	"reflect"
	"testing"
)

func TestParseEventKeys(t *testing.T) {
	for _, tt := range []struct {
		name string
		in   string
		want keyPress
		n    int
	}{
		{"letter", "a", keyPress{name: "a"}, 1},
		{"upper", "W", keyPress{name: "W", mods: modifiers{shift: true}}, 1},
		{"digit", "7", keyPress{name: "7"}, 1},
		{"slash", "/", keyPress{name: "/"}, 1},
		{"enter", "\r", keyPress{name: "Enter"}, 1},
		{"tab", "\t", keyPress{name: "Tab"}, 1},
		{"backspace", "\x7f", keyPress{name: "Backspace"}, 1},
		{"ctrl+c", "\x03", keyPress{name: "c", mods: modifiers{ctrl: true}}, 1},
		{"escape twice", "\x1b\x1b", keyPress{name: "Escape"}, 1},
		{"alt+f", "\x1bf", keyPress{name: "f", mods: modifiers{alt: true}}, 2},
		{"arrow up", "\x1b[A", keyPress{name: "ArrowUp"}, 3},
		{"arrow left ss3", "\x1bOD", keyPress{name: "ArrowLeft"}, 3},
		{"shift+arrow", "\x1b[1;2C", keyPress{name: "ArrowRight", mods: modifiers{shift: true}}, 6},
		{"ctrl+arrow", "\x1b[1;5B", keyPress{name: "ArrowDown", mods: modifiers{ctrl: true}}, 6},
		{"delete", "\x1b[3~", keyPress{name: "Delete"}, 4},
		{"page down", "\x1b[6~", keyPress{name: "PageDown"}, 4},
		{"f5", "\x1b[15~", keyPress{name: "F5"}, 5},
		{"two-byte rune", "ä", keyPress{name: "ä"}, 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ev, n, incomplete := parseEvent([]byte(tt.in))
			if incomplete {
				t.Fatalf("reported incomplete")
			}
			if ev.kind != evKey || !reflect.DeepEqual(ev.key, tt.want) {
				t.Errorf("got %+v, want key %+v", ev, tt.want)
			}
			if n != tt.n {
				t.Errorf("consumed %d bytes, want %d", n, tt.n)
			}
		})
	}
}

func TestParseEventIncomplete(t *testing.T) {
	// A lone ESC may still grow into a sequence, so the caller has to wait; the same
	// goes for a CSI that has not reached its final byte yet.
	for _, in := range []string{"\x1b", "\x1b[", "\x1b[1;2", "\x1bO"} {
		if _, _, incomplete := parseEvent([]byte(in)); !incomplete {
			t.Errorf("%q: not reported incomplete", in)
		}
	}
}

func TestParseEventDropsUnknown(t *testing.T) {
	// The reply to a capability query must not turn into keystrokes, but it has to be
	// consumed so the keys after it still parse.
	ev, n, incomplete := parseEvent([]byte("\x1b[?62;4;22cX"))
	if incomplete || ev.kind != evNone {
		t.Fatalf("got %+v (incomplete %v)", ev, incomplete)
	}
	if n != 11 {
		t.Fatalf("consumed %d bytes, want 11", n)
	}
	if ev, _, _ := parseEvent([]byte("X")); ev.key.name != "X" {
		t.Errorf("the key after the reply was lost: %+v", ev)
	}
}

func TestParseMouse(t *testing.T) {
	for _, tt := range []struct {
		name string
		in   string
		want mouseReport
	}{
		{"left press", "\x1b[<0;10;5M", mouseReport{action: mousePress, button: 0, col: 10, row: 5}},
		{"left release", "\x1b[<0;10;5m", mouseReport{action: mouseRelease, button: 0, col: 10, row: 5}},
		{"right press", "\x1b[<2;3;4M", mouseReport{action: mousePress, button: 2, col: 3, row: 4}},
		{"drag", "\x1b[<32;7;8M", mouseReport{action: mouseMove, button: 0, col: 7, row: 8}},
		{"hover", "\x1b[<35;7;8M", mouseReport{action: mouseMove, button: -1, col: 7, row: 8}},
		{"wheel up", "\x1b[<64;1;1M", mouseReport{action: mouseWheel, col: 1, row: 1, wheelUp: true}},
		{"wheel down", "\x1b[<65;1;1M", mouseReport{action: mouseWheel, col: 1, row: 1}},
		{"shift+click", "\x1b[<4;2;2M", mouseReport{action: mousePress, col: 2, row: 2, mods: modifiers{shift: true}}},
		{"ctrl+wheel", "\x1b[<80;2;2M", mouseReport{action: mouseWheel, col: 2, row: 2, wheelUp: true, mods: modifiers{ctrl: true}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ev, n, incomplete := parseEvent([]byte(tt.in))
			if incomplete || n != len(tt.in) {
				t.Fatalf("consumed %d of %d bytes (incomplete %v)", n, len(tt.in), incomplete)
			}
			if ev.kind != evMouse || !reflect.DeepEqual(ev.mouse, tt.want) {
				t.Errorf("got %+v, want mouse %+v", ev, tt.want)
			}
		})
	}
}

func TestEndOfAttributes(t *testing.T) {
	reply := "\x1b_Gi=31;OK\x1b\\\x1b[?62;4c"
	if got := endOfAttributes([]byte(reply + "vv")); got != len(reply) {
		t.Errorf("end at %d, want %d", got, len(reply))
	}
	if got := endOfAttributes([]byte("\x1b[?62;4")); got != -1 {
		t.Errorf("incomplete reply ended at %d", got)
	}
}
