package termview

import (
	"strings"
	"time"
)

// cdpKey is a key as the DevTools protocol (and through it the DOM) wants it: the
// page reads both key and code - the toolbar switches on e.key, walk mode on e.code.
type cdpKey struct {
	key  string // DOM KeyboardEvent.key
	code string // DOM KeyboardEvent.code
	vk   int    // Windows virtual key code
	text string // the character typed, empty for keys that produce none
}

// modifier bits, as the protocol numbers them.
const (
	modAlt   = 1
	modCtrl  = 2
	modMeta  = 4
	modShift = 8
)

func (m modifiers) bits() int {
	b := 0
	if m.alt {
		b |= modAlt
	}
	if m.ctrl {
		b |= modCtrl
	}
	if m.shift {
		b |= modShift
	}
	return b
}

// shiftKey is held alongside an upper-case letter, so that walk mode - which reads
// ShiftLeft from the set of keys currently down - can be made to run.
var shiftKey = cdpKey{key: "Shift", code: "ShiftLeft", vk: 16}

// namedKeys are the keys the input parser reports by name.
var namedKeys = map[string]cdpKey{
	"ArrowUp":    {key: "ArrowUp", code: "ArrowUp", vk: 38},
	"ArrowDown":  {key: "ArrowDown", code: "ArrowDown", vk: 40},
	"ArrowLeft":  {key: "ArrowLeft", code: "ArrowLeft", vk: 37},
	"ArrowRight": {key: "ArrowRight", code: "ArrowRight", vk: 39},
	"Enter":      {key: "Enter", code: "Enter", vk: 13, text: "\r"},
	"Escape":     {key: "Escape", code: "Escape", vk: 27},
	"Backspace":  {key: "Backspace", code: "Backspace", vk: 8},
	"Tab":        {key: "Tab", code: "Tab", vk: 9, text: "\t"},
	"Delete":     {key: "Delete", code: "Delete", vk: 46},
	"Insert":     {key: "Insert", code: "Insert", vk: 45},
	"Home":       {key: "Home", code: "Home", vk: 36},
	"End":        {key: "End", code: "End", vk: 35},
	"PageUp":     {key: "PageUp", code: "PageUp", vk: 33},
	"PageDown":   {key: "PageDown", code: "PageDown", vk: 34},
}

// punctuation maps a character to the key it is typed with; shifted says whether
// typing it means holding shift.
var punctuation = map[rune]struct {
	code    string
	vk      int
	shifted bool
}{
	' ': {"Space", 32, false},
	'-': {"Minus", 189, false}, '_': {"Minus", 189, true},
	'=': {"Equal", 187, false}, '+': {"Equal", 187, true},
	'[': {"BracketLeft", 219, false}, '{': {"BracketLeft", 219, true},
	']': {"BracketRight", 221, false}, '}': {"BracketRight", 221, true},
	'\\': {"Backslash", 220, false}, '|': {"Backslash", 220, true},
	';': {"Semicolon", 186, false}, ':': {"Semicolon", 186, true},
	'\'': {"Quote", 222, false}, '"': {"Quote", 222, true},
	',': {"Comma", 188, false}, '<': {"Comma", 188, true},
	'.': {"Period", 190, false}, '>': {"Period", 190, true},
	'/': {"Slash", 191, false}, '?': {"Slash", 191, true},
	'`': {"Backquote", 192, false}, '~': {"Backquote", 192, true},
	')': {"Digit0", 48, true}, '!': {"Digit1", 49, true}, '@': {"Digit2", 50, true},
	'#': {"Digit3", 51, true}, '$': {"Digit4", 52, true}, '%': {"Digit5", 53, true},
	'^': {"Digit6", 54, true}, '&': {"Digit7", 55, true}, '*': {"Digit8", 56, true},
	'(': {"Digit9", 57, true},
}

// toCDPKey translates a parsed key press. mods carries what the terminal reported plus
// whatever the character itself implies (an upper-case letter means shift).
func toCDPKey(p keyPress) (k cdpKey, mods modifiers, ok bool) {
	mods = p.mods
	if k, found := namedKeys[p.name]; found {
		if mods.ctrl || mods.alt {
			k.text = ""
		}
		return k, mods, true
	}
	r := []rune(p.name)
	if len(r) != 1 {
		return cdpKey{}, mods, false
	}
	c := r[0]
	switch {
	case c >= 'a' && c <= 'z':
		k = cdpKey{key: string(c), code: "Key" + strings.ToUpper(string(c)), vk: int(c - 32), text: string(c)}
	case c >= 'A' && c <= 'Z':
		mods.shift = true
		k = cdpKey{key: string(c), code: "Key" + string(c), vk: int(c), text: string(c)}
	case c >= '0' && c <= '9':
		k = cdpKey{key: string(c), code: "Digit" + string(c), vk: int(c), text: string(c)}
	default:
		if p, found := punctuation[c]; found {
			mods.shift = mods.shift || p.shifted
			k = cdpKey{key: string(c), code: p.code, vk: p.vk, text: string(c)}
			break
		}
		// Anything else is still typeable: the search box takes it as text.
		k = cdpKey{key: string(c), text: string(c)}
	}
	if mods.ctrl || mods.alt {
		k.text = "" // a shortcut, not a character
	}
	return k, mods, true
}

// held is one key the view is pretending is still down.
type held struct {
	key   cdpKey
	mods  modifiers
	until time.Time
}

// holder turns keystrokes into key-down/key-up pairs that last long enough to be
// useful. A terminal reports a key once per press and repeats it while it is held, so
// releasing at once would make walk mode step rather than walk: instead a key stays
// down until its repeats stop.
type holder struct {
	hold time.Duration
	keys map[string]*held // by DOM code, or by key for the codeless ones
}

func newHolder(hold time.Duration) *holder {
	return &holder{hold: hold, keys: map[string]*held{}}
}

func holdID(k cdpKey) string {
	if k.code != "" {
		return k.code
	}
	return "key:" + k.key
}

// press registers a key press. It reports whether the key was not already down, which
// is when a key-down has to be sent.
func (h *holder) press(k cdpKey, mods modifiers, now time.Time) bool {
	id := holdID(k)
	if e, ok := h.keys[id]; ok {
		e.until = now.Add(h.hold)
		e.mods = mods
		return false
	}
	h.keys[id] = &held{key: k, mods: mods, until: now.Add(h.hold)}
	return true
}

// release lists the keys whose repeats have stopped, and forgets them.
func (h *holder) release(now time.Time) []held {
	var out []held
	for id, e := range h.keys {
		if !now.Before(e.until) {
			out = append(out, *e)
			delete(h.keys, id)
		}
	}
	return out
}

// releaseAll lists every key still down, for shutting the view down cleanly.
func (h *holder) releaseAll() []held {
	var out []held
	for id, e := range h.keys {
		out = append(out, *e)
		delete(h.keys, id)
	}
	return out
}
