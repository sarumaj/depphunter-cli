package termview

import (
	"testing"
	"time"
)

func TestToCDPKey(t *testing.T) {
	for _, tt := range []struct {
		name      string
		in        keyPress
		key, code string
		vk        int
		text      string
		shift     bool
	}{
		// Walk mode reads e.code, the toolbar reads e.key: both have to be right.
		{"letter", keyPress{name: "w"}, "w", "KeyW", 87, "w", false},
		{"upper", keyPress{name: "W", mods: modifiers{shift: true}}, "W", "KeyW", 87, "W", true},
		{"digit", keyPress{name: "3"}, "3", "Digit3", 51, "3", false},
		{"space", keyPress{name: " "}, " ", "Space", 32, " ", false},
		{"plus", keyPress{name: "+"}, "+", "Equal", 187, "+", true},
		{"minus", keyPress{name: "-"}, "-", "Minus", 189, "-", false},
		{"bracket", keyPress{name: "["}, "[", "BracketLeft", 219, "[", false},
		{"question", keyPress{name: "?"}, "?", "Slash", 191, "?", true},
		{"escape", keyPress{name: "Escape"}, "Escape", "Escape", 27, "", false},
		{"enter", keyPress{name: "Enter"}, "Enter", "Enter", 13, "\r", false},
		{"arrow", keyPress{name: "ArrowUp"}, "ArrowUp", "ArrowUp", 38, "", false},
		// A shortcut types nothing: the search box must not fill up with letters.
		{"ctrl+a", keyPress{name: "a", mods: modifiers{ctrl: true}}, "a", "KeyA", 65, "", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			k, mods, ok := toCDPKey(tt.in)
			if !ok {
				t.Fatal("not translated")
			}
			if k.key != tt.key || k.code != tt.code || k.vk != tt.vk || k.text != tt.text {
				t.Errorf("got %+v, want key %q code %q vk %d text %q", k, tt.key, tt.code, tt.vk, tt.text)
			}
			if mods.shift != tt.shift {
				t.Errorf("shift %v, want %v", mods.shift, tt.shift)
			}
		})
	}
}

func TestModifierBits(t *testing.T) {
	if got := (modifiers{alt: true, ctrl: true, shift: true}).bits(); got != modAlt|modCtrl|modShift {
		t.Errorf("bits %d", got)
	}
	if got := (modifiers{}).bits(); got != 0 {
		t.Errorf("bits %d, want 0", got)
	}
}

func TestHolderKeepsKeysDownBetweenRepeats(t *testing.T) {
	h := newHolder(200 * time.Millisecond)
	now := time.Now()
	w, _, _ := toCDPKey(keyPress{name: "w"})

	if !h.press(w, modifiers{}, now) {
		t.Fatal("first press did not ask for a key-down")
	}
	// A terminal repeats a held key; the page must not see it go up in between.
	if h.press(w, modifiers{}, now.Add(100*time.Millisecond)) {
		t.Error("a repeat asked for a second key-down")
	}
	if got := h.release(now.Add(150 * time.Millisecond)); len(got) != 0 {
		t.Errorf("released %d keys while the repeats were still coming", len(got))
	}
	released := h.release(now.Add(350 * time.Millisecond))
	if len(released) != 1 || released[0].key.code != "KeyW" {
		t.Fatalf("released %+v, want KeyW", released)
	}
	if got := h.release(now.Add(time.Second)); len(got) != 0 {
		t.Errorf("released %+v twice", got)
	}
}

func TestHolderReleaseAll(t *testing.T) {
	h := newHolder(time.Second)
	now := time.Now()
	for _, name := range []string{"w", "a"} {
		k, _, _ := toCDPKey(keyPress{name: name})
		h.press(k, modifiers{}, now)
	}
	if got := h.releaseAll(); len(got) != 2 {
		t.Errorf("released %d keys, want 2", len(got))
	}
	if got := h.releaseAll(); len(got) != 0 {
		t.Errorf("released %d keys after the first sweep", len(got))
	}
}
