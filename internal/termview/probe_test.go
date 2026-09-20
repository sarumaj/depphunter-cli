package termview

import "testing"

func TestParseProbe(t *testing.T) {
	for _, tt := range []struct {
		name         string
		reply        string
		kitty, sixel bool
	}{
		{"kitty answers its query", "\x1b_Gi=31;OK\x1b\\\x1b[?62;22c", true, false},
		{"sixel is feature 4", "\x1b[?62;4;22c", false, true},
		{"both", "\x1b_Gi=31;OK\x1b\\\x1b[?62;4c", true, true},
		{"plain terminal", "\x1b[?1;2c", false, false},
		{"no reply at all", "", false, false},
		// 14 is not 4: a terminal that reports other features must not be mistaken.
		{"other features", "\x1b[?64;1;9;14;21c", false, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseProbe(tt.reply); got.kitty != tt.kitty || got.sixel != tt.sixel {
				t.Errorf("got %+v, want kitty %v sixel %v", got, tt.kitty, tt.sixel)
			}
		})
	}
}

func TestPickProtocol(t *testing.T) {
	env := func(m map[string]string) func(string) string {
		return func(k string) string { return m[k] }
	}
	answers := func(r probeResult) func() probeResult { return func() probeResult { return r } }
	none := answers(probeResult{})

	for _, tt := range []struct {
		name     string
		override string
		env      map[string]string
		probe    func() probeResult
		want     string
	}{
		{"an explicit choice wins", protoSixel, map[string]string{"TERM": "xterm-kitty"}, none, protoSixel},
		{"kitty", protoAuto, map[string]string{"TERM": "xterm-kitty"}, none, protoKitty},
		{"ghostty", protoAuto, map[string]string{"TERM_PROGRAM": "ghostty"}, none, protoKitty},
		{"wezterm", protoAuto, map[string]string{"TERM_PROGRAM": "WezTerm"}, none, protoKitty},
		{"iterm", protoAuto, map[string]string{"TERM_PROGRAM": "iTerm.app"}, none, protoITerm},
		{"asked and answered", "", map[string]string{"TERM": "foot"}, answers(probeResult{sixel: true}), protoSixel},
		{"kitty by query", "", map[string]string{"TERM": "xterm-256color"}, answers(probeResult{kitty: true}), protoKitty},
		{"nothing works", "", map[string]string{"TERM": "vt100"}, none, protoBlocks},
		// Inside a multiplexer the graphics protocols need passthrough we do not do.
		{"tmux", protoAuto, map[string]string{"TMUX": "/tmp/x", "TERM": "xterm-kitty"}, answers(probeResult{kitty: true}), protoBlocks},
		{"screen", protoAuto, map[string]string{"TERM": "screen.xterm-256color"}, none, protoBlocks},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := pickProtocol(tt.override, env(tt.env), tt.probe); got != tt.want {
				t.Errorf("got %s, want %s", got, tt.want)
			}
		})
	}
}

func TestProtocolsAreRenderable(t *testing.T) {
	// Every name the flag takes has to map to something that can draw (auto aside,
	// which is decided before a renderer is made).
	for _, p := range Protocols() {
		if p == protoAuto {
			continue
		}
		if got := newRenderer(p).name(); got != p {
			t.Errorf("%s renders as %s", p, got)
		}
	}
}
