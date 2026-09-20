package termview

import (
	"os"
	"strings"
)

// Graphics protocols, by the name --terminal-graphics takes.
const (
	protoAuto   = "auto"
	protoKitty  = "kitty"
	protoITerm  = "iterm"
	protoSixel  = "sixel"
	protoBlocks = "blocks"
)

var protocols = []string{protoAuto, protoKitty, protoITerm, protoSixel, protoBlocks}

// probeQuery asks the terminal what it can do: the first sequence is a one-pixel
// kitty transmission, which kitty-capable terminals answer and the rest ignore, and
// the second is the primary device attributes request, which every terminal answers -
// so the answer to it also marks the end of the reply.
const probeQuery = "\x1b_Gi=31,s=1,v=1,a=q,t=d,f=24;AAAA\x1b\\\x1b[c"

// probeResult is what the reply to probeQuery says the terminal supports.
type probeResult struct{ kitty, sixel bool }

// parseProbe reads the terminal's reply. The device attributes list features by
// number, and 4 is sixel graphics.
func parseProbe(reply string) probeResult {
	var r probeResult
	if i := strings.Index(reply, "\x1b_G"); i >= 0 && strings.Contains(reply[i:], "OK") {
		r.kitty = true
	}
	if i := strings.Index(reply, "\x1b[?"); i >= 0 {
		if end := strings.IndexByte(reply[i:], 'c'); end > 0 {
			for _, f := range strings.Split(reply[i+3:i+end], ";") {
				if f == "4" {
					r.sixel = true
				}
			}
		}
	}
	return r
}

// pickProtocol decides how to draw. An explicit choice always wins; otherwise the
// terminals that identify themselves in the environment are taken at their word and
// the rest are asked (see parseProbe).
func pickProtocol(override string, getenv func(string) string, probe func() probeResult) string {
	if override != "" && override != protoAuto {
		return override
	}
	term, program := getenv("TERM"), getenv("TERM_PROGRAM")
	// Inside tmux or screen the graphics protocols need passthrough that differs per
	// multiplexer and version; half blocks are ordinary text and always arrive.
	if getenv("TMUX") != "" || strings.HasPrefix(term, "screen") || strings.HasPrefix(term, "tmux") {
		return protoBlocks
	}
	switch {
	case getenv("KITTY_WINDOW_ID") != "", strings.Contains(term, "kitty"), strings.Contains(term, "ghostty"),
		program == "ghostty", program == "WezTerm":
		return protoKitty
	case program == "iTerm.app":
		return protoITerm
	}
	if probe != nil {
		switch r := probe(); {
		case r.kitty:
			return protoKitty
		case r.sixel:
			return protoSixel
		}
	}
	return protoBlocks
}

// envOrEmpty is os.Getenv, as a value pickProtocol can be tested without.
var envOrEmpty = os.Getenv
