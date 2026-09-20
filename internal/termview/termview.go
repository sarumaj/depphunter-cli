package termview

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Options configures the terminal view.
type Options struct {
	URL      string // the page to show, token and all
	Browser  string // a Chromium-based browser to drive; "" looks for one
	Graphics string // auto, kitty, iterm, sixel or blocks
	// Download allows fetching a browser when the machine has none. Without it, a
	// machine with no browser is asked before anything is fetched, and told what to
	// do when there is nobody to ask.
	Download bool
	// SHA256 is the digest the downloaded archive must have; empty accepts what the
	// publisher serves over TLS and reports the digest it got.
	SHA256 string
}

// Protocols lists the values Graphics takes, for the flag's help and validation.
func Protocols() []string { return append([]string(nil), protocols...) }

const (
	// The stream's JPEG quality: past this the frames grow faster than the picture
	// improves, and a terminal shows less than the browser does anyway.
	frameQuality = 70
	// How long a key stays down after its last repeat: long enough to bridge the gap
	// between a terminal's auto-repeats, short enough not to overshoot a step.
	keyHold = 220 * time.Millisecond
	// A second press within this long, on the same cell, is a double click.
	doubleClick = 400 * time.Millisecond
	// The smallest page the map is laid out in: the toolbar needs room, however few
	// pixels the terminal has to show it in.
	minPageW = 1200
	minPageH = 720
	// Mouse moves are reported per cell; the page only needs the latest one.
	moveInterval = 30 * time.Millisecond
)

// escape sequences that frame the view.
const (
	altScreenOn  = "\x1b[?1049h"
	altScreenOff = "\x1b[?1049l"
	cursorHide   = "\x1b[?25l"
	cursorShow   = "\x1b[?25h"
	cursorHome   = "\x1b[H"
	clearScreen  = "\x1b[2J"
	// Any-motion tracking with SGR coordinates: hovering shows tooltips, dragging
	// pans, and coordinates stay right past column 223.
	mouseOn  = "\x1b[?1003h\x1b[?1006h"
	mouseOff = "\x1b[?1006l\x1b[?1003l"
)

// Run shows opt.URL in the terminal until the user quits with Ctrl+C or ctx is done.
func Run(ctx context.Context, opt Options) error {
	// The browser is settled before the terminal is taken over: a question about
	// downloading one, and the progress of doing so, belong on an ordinary screen.
	bin, err := browserFor(ctx, opt)
	if err != nil {
		return err
	}

	t, err := openTTY()
	if err != nil {
		return fmt.Errorf("terminal view: %w", err)
	}
	defer t.close()
	if err := t.raw(); err != nil {
		return fmt.Errorf("terminal view: %w", err)
	}
	defer t.restore()

	in := newInputReader(t.input())
	defer in.stop()
	proto := pickProtocol(opt.Graphics, envOrEmpty, func() probeResult {
		return parseProbe(in.query(t.file(), probeQuery, 300*time.Millisecond))
	})
	in.start()
	v := &view{
		out:      bufio.NewWriterSize(t.file(), 1<<20),
		renderer: newRenderer(proto),
		holder:   newHolder(keyHold),
	}
	if v.geom, err = t.size(); err != nil {
		return fmt.Errorf("terminal size: %w", err)
	}
	return v.run(ctx, bin, opt.URL, t, in)
}

// browserFor returns the browser to drive: the one named, the first one installed,
// or - with permission - one fetched from Chrome for Testing.
func browserFor(ctx context.Context, opt Options) (string, error) {
	if opt.Browser != "" {
		return opt.Browser, nil
	}
	bin, err := findBrowser(exec.LookPath, os.Stat)
	if err == nil || !errors.Is(err, errNoBrowser) {
		return bin, err
	}
	if !opt.Download && !askToDownload() {
		return "", err
	}
	dir, dirErr := browserDir()
	if dirErr != nil {
		return "", err
	}
	if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
		return "", mkErr
	}
	return download(ctx, dir, versionsURL, opt.SHA256, func(msg string) {
		fmt.Fprintln(os.Stderr, "depphunter: "+msg)
	})
}

// askToDownload asks for permission when there is someone to ask. With no terminal on
// the standard input - a script, a pipeline - nothing is downloaded and the caller
// reports what to pass instead.
func askToDownload() bool {
	if !isTerminal(int(os.Stdin.Fd())) {
		return false
	}
	fmt.Fprint(os.Stderr, "depphunter: no browser found. Download the Chrome for Testing headless shell "+
		"(about 90 MB) into the cache directory? [y/N] ")
	answer, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "y", "yes":
		return true
	}
	return false
}

// view holds everything the drawing loop needs.
type view struct {
	out      *bufio.Writer
	renderer renderer
	holder   *holder
	geom     geometry
	session  string
	browser  *browserProcess

	mu      sync.Mutex
	pending *frame // the newest frame not drawn yet

	buttons  int // mouse buttons currently down, as a DevTools bit set
	lastDown struct {
		at       time.Time
		col, row int
		count    int
	}
	lastMove time.Time
}

// screencastFrame is the event the browser sends for every repainted frame.
type screencastFrame struct {
	Data      string `json:"data"` // base64 JPEG
	SessionID int    `json:"sessionId"`
}

func (v *view) run(ctx context.Context, bin, url string, t *tty, in *inputReader) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	frames := make(chan screencastFrame, 8)
	redraw := make(chan struct{}, 1)
	onEvent := func(sessionID, method string, params json.RawMessage) {
		if method != "Page.screencastFrame" {
			return
		}
		var f screencastFrame
		if json.Unmarshal(params, &f) == nil {
			select {
			case frames <- f:
			default: // the terminal is behind; the next frame will be newer anyway
			}
		}
	}

	w, h := v.pageSize()
	b, err := startBrowser(ctx, bin, w, h, onEvent)
	if err != nil {
		return err
	}
	v.browser = b
	defer b.stop()

	// A browser too old for --remote-debugging-pipe answers nothing at all, which
	// would leave the view waiting in a terminal it has already taken over.
	start, cancelStart := context.WithTimeout(ctx, 30*time.Second)
	defer cancelStart()
	if v.session, err = b.open(start, url); err != nil {
		return fmt.Errorf("open page: %w", err)
	}
	if err := v.configurePage(start); err != nil {
		return err
	}

	// Frames are acknowledged off the protocol's read loop: acknowledging from it
	// would wait for a reply the same loop has to deliver.
	go v.acknowledge(ctx, frames, redraw)

	writeAll(v.out, altScreenOn+cursorHide+mouseOn+clearScreen)
	v.out.Flush()
	defer func() {
		v.releaseKeys()
		writeAll(v.out, mouseOff+cursorShow+altScreenOff)
		v.out.Flush()
	}()
	v.status("starting the browser…")

	sizes, stopSizes := resizes()
	defer stopSizes()
	keyTick := time.NewTicker(keyHold / 4)
	defer keyTick.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-b.exited:
			return errors.New("the browser stopped")
		case <-redraw:
			if err := v.paint(); err != nil {
				return err
			}
		case <-sizes:
			if err := v.resize(ctx, t); err != nil {
				return err
			}
		case <-keyTick.C:
			v.releaseExpired(ctx)
		case ev, ok := <-in.events:
			if !ok {
				return nil
			}
			if quit := v.handle(ctx, ev); quit {
				return nil
			}
		}
	}
}

// configurePage sizes the page to the terminal and starts the frame stream.
func (v *view) configurePage(ctx context.Context) error {
	w, h := v.pageSize()
	c := v.browser.client
	if err := c.call(ctx, v.session, "Emulation.setDeviceMetricsOverride", map[string]any{
		"width": w, "height": h, "deviceScaleFactor": 1, "mobile": false,
	}, nil); err != nil {
		return err
	}
	// The frames come back at the terminal's resolution: the browser scales them,
	// which costs nothing here and keeps the stream small.
	rw, rh := v.resolution()
	return c.call(ctx, v.session, "Page.startScreencast", map[string]any{
		"format": "jpeg", "quality": frameQuality, "maxWidth": rw, "maxHeight": rh, "everyNthFrame": 1,
	}, nil)
}

// acknowledge confirms every frame - the browser stops sending until it is - and keeps
// the newest one for the drawing loop.
func (v *view) acknowledge(ctx context.Context, frames <-chan screencastFrame, redraw chan<- struct{}) {
	for {
		select {
		case <-ctx.Done():
			return
		case f, ok := <-frames:
			if !ok {
				return
			}
			v.browser.client.call(ctx, v.session, "Page.screencastFrameAck",
				map[string]any{"sessionId": f.SessionID}, nil)
			data, err := base64.StdEncoding.DecodeString(f.Data)
			if err != nil {
				continue
			}
			v.mu.Lock()
			v.pending = &frame{data: data}
			v.mu.Unlock()
			select {
			case redraw <- struct{}{}:
			default:
			}
		}
	}
}

// resolution is the size of the picture the terminal shows.
func (v *view) resolution() (int, int) { return v.renderer.resolution(v.geom) }

// pageSize is the size the page is laid out at, in CSS pixels. Drawing the page at the
// terminal's own resolution would work for the graphics protocols but leave half
// blocks with a window a hundred pixels wide, where the toolbar alone fills the
// screen. The page is laid out at a usable size with the aspect ratio of the picture
// instead, and the browser scales the frames down to it (see configurePage).
func (v *view) pageSize() (int, int) {
	w, h := v.resolution()
	if w < 1 || h < 1 {
		return minPageW, minPageH
	}
	scale := 1
	for w*scale < minPageW || h*scale < minPageH {
		scale++
	}
	return w * scale, h * scale
}

// paint draws the newest frame and the status line.
func (v *view) paint() error {
	v.mu.Lock()
	f := v.pending
	v.pending = nil
	v.mu.Unlock()
	if f == nil {
		return nil
	}
	writeAll(v.out, cursorHome)
	if err := v.renderer.draw(v.out, f, v.geom); err != nil {
		return err
	}
	v.statusLine(fmt.Sprintf("%s · %dx%d · Ctrl+C quit · ? help · V walk · drag pans, right-drag orbits",
		v.renderer.name(), v.geom.cols, v.geom.mapRows()))
	return v.out.Flush()
}

// status shows a message before the first frame arrives.
func (v *view) status(msg string) {
	v.statusLine(msg)
	v.out.Flush()
}

func (v *view) statusLine(msg string) {
	if runes := []rune(msg); len(runes) > v.geom.cols {
		msg = string(runes[:max(0, v.geom.cols)])
	}
	fmt.Fprintf(v.out, "\x1b[%d;1H\x1b[7m%s\x1b[0m\x1b[K", v.geom.rows, msg)
}

// resize re-measures the terminal and has the page re-render at the new size.
func (v *view) resize(ctx context.Context, t *tty) error {
	g, err := t.size()
	if err != nil || (g == v.geom) {
		return nil
	}
	v.geom = g
	writeAll(v.out, clearScreen)
	v.out.Flush()
	if err := v.browser.client.call(ctx, v.session, "Page.stopScreencast", nil, nil); err != nil {
		return err
	}
	return v.configurePage(ctx)
}

// handle acts on one terminal event and reports whether the view should close.
func (v *view) handle(ctx context.Context, ev event) bool {
	switch ev.kind {
	case evKey:
		if ev.key.mods.ctrl && ev.key.name == "c" {
			return true
		}
		v.sendKey(ctx, ev.key)
	case evMouse:
		v.sendMouse(ctx, ev.mouse)
	}
	return false
}

func (v *view) sendKey(ctx context.Context, p keyPress) {
	k, mods, ok := toCDPKey(p)
	if !ok {
		return
	}
	now := time.Now()
	if mods.shift && k.code != "" && strings.HasPrefix(k.code, "Key") {
		// Walk mode runs while shift is down, which a terminal never reports on its
		// own: an upper-case letter holds it instead.
		if v.holder.press(shiftKey, modifiers{shift: true}, now) {
			v.dispatchKey(ctx, "keyDown", shiftKey, modifiers{shift: true})
		}
	}
	if v.holder.press(k, mods, now) {
		v.dispatchKey(ctx, "keyDown", k, mods)
	}
}

// releaseExpired sends the key-ups for keys whose repeats have stopped.
func (v *view) releaseExpired(ctx context.Context) {
	for _, h := range v.holder.release(time.Now()) {
		v.dispatchKey(ctx, "keyUp", h.key, h.mods)
	}
}

func (v *view) releaseKeys() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	for _, h := range v.holder.releaseAll() {
		v.dispatchKey(ctx, "keyUp", h.key, h.mods)
	}
}

func (v *view) dispatchKey(ctx context.Context, kind string, k cdpKey, mods modifiers) {
	params := map[string]any{
		"type": kind, "key": k.key, "code": k.code, "modifiers": mods.bits(),
		"windowsVirtualKeyCode": k.vk, "nativeVirtualKeyCode": k.vk,
	}
	if kind == "keyDown" && k.text != "" {
		params["text"] = k.text
		params["unmodifiedText"] = k.text
	} else if kind == "keyDown" && k.text == "" {
		params["type"] = "rawKeyDown" // a key that types nothing
	}
	v.browser.client.call(ctx, v.session, "Input.dispatchKeyEvent", params, nil)
}

// buttonNames are the names the protocol uses, by the terminal's button number.
var buttonNames = map[int]string{0: "left", 1: "middle", 2: "right"}

// buttonBits are the bits the protocol's "buttons" set uses.
var buttonBits = map[int]int{0: 1, 1: 4, 2: 2}

func (v *view) sendMouse(ctx context.Context, m mouseReport) {
	if m.row > v.geom.mapRows() {
		return // the status line is not part of the page
	}
	x, y := v.pixel(m.col, m.row)
	params := map[string]any{"x": x, "y": y, "modifiers": m.mods.bits()}
	switch m.action {
	case mouseWheel:
		delta := 100.0
		if m.wheelUp {
			delta = -delta
		}
		params["type"], params["deltaX"], params["deltaY"] = "mouseWheel", 0, delta
		params["buttons"] = v.buttons
	case mousePress:
		v.buttons |= buttonBits[m.button]
		params["type"], params["button"] = "mousePressed", buttonNames[m.button]
		params["buttons"], params["clickCount"] = v.buttons, v.clickCount(m)
	case mouseRelease:
		params["type"], params["button"] = "mouseReleased", buttonNames[m.button]
		v.buttons &^= buttonBits[m.button]
		params["buttons"], params["clickCount"] = v.buttons, v.lastDown.count
	case mouseMove:
		now := time.Now()
		if v.buttons == 0 && now.Sub(v.lastMove) < moveInterval {
			return // hovering: the page only needs the newest position
		}
		v.lastMove = now
		params["type"], params["button"] = "mouseMoved", "none"
		params["buttons"] = v.buttons
	default:
		return
	}
	v.browser.client.call(ctx, v.session, "Input.dispatchMouseEvent", params, nil)
}

// clickCount counts a quick second press on the same cell as a double click, which is
// how the map expands and collapses.
func (v *view) clickCount(m mouseReport) int {
	now := time.Now()
	if now.Sub(v.lastDown.at) < doubleClick && v.lastDown.col == m.col && v.lastDown.row == m.row {
		v.lastDown.count++
	} else {
		v.lastDown.count = 1
	}
	v.lastDown.at, v.lastDown.col, v.lastDown.row = now, m.col, m.row
	return v.lastDown.count
}

// pixel maps a character cell to the middle of the page pixels it covers.
func (v *view) pixel(col, row int) (float64, float64) {
	w, h := v.pageSize()
	sx, sy := float64(w)/float64(max(v.geom.cols, 1)), float64(h)/float64(max(v.geom.mapRows(), 1))
	return (float64(col-1) + 0.5) * sx, (float64(row-1) + 0.5) * sy
}

// inputReader reads the terminal without blocking the drawing loop: raw bytes go to
// the parser, which turns complete sequences into events.
type inputReader struct {
	events chan event
	raw    chan []byte
	done   chan struct{}
	once   sync.Once
	spare  []byte // bytes read while probing but not part of the reply
}

func newInputReader(f io.Reader) *inputReader {
	in := &inputReader{events: make(chan event, 64), raw: make(chan []byte, 8), done: make(chan struct{})}
	go in.read(f)
	return in
}

// start begins turning the bytes into events. It comes after the capability query,
// which reads the same stream and leaves behind whatever was typed while it ran.
func (in *inputReader) start() { go in.parse() }

func (in *inputReader) read(f io.Reader) {
	buf := make([]byte, 4096)
	for {
		n, err := f.Read(buf)
		if n > 0 {
			chunk := make([]byte, n)
			copy(chunk, buf[:n])
			select {
			case in.raw <- chunk:
			case <-in.done:
				return
			}
		}
		if err != nil {
			close(in.raw)
			return
		}
	}
}

// query writes a terminal query and collects the reply, which ends with the device
// attributes response ("…c"). Whatever arrives after it is kept for the parser: it is
// the user typing.
func (in *inputReader) query(w io.Writer, q string, timeout time.Duration) string {
	if _, err := io.WriteString(w, q); err != nil {
		return ""
	}
	deadline := time.After(timeout)
	var reply []byte
	for {
		select {
		case chunk, ok := <-in.raw:
			if !ok {
				return string(reply)
			}
			reply = append(reply, chunk...)
			if i := endOfAttributes(reply); i >= 0 {
				in.spare = append(in.spare, reply[i:]...)
				return string(reply[:i])
			}
		case <-deadline:
			return string(reply)
		}
	}
}

// endOfAttributes finds the end of a primary device attributes reply, which is the
// first "c" that follows a CSI.
func endOfAttributes(b []byte) int {
	start := -1
	for i := 0; i < len(b); i++ {
		switch {
		case b[i] == 0x1b && i+1 < len(b) && b[i+1] == '[':
			start = i
		case b[i] == 'c' && start >= 0:
			return i + 1
		}
	}
	return -1
}

// parse turns the byte stream into events, waiting a moment on a partial escape
// sequence: a lone ESC is the Escape key, which the page uses to clear a selection.
func (in *inputReader) parse() {
	defer close(in.events)
	buf := in.spare
	var wait <-chan time.Time
	for {
		for len(buf) > 0 {
			ev, n, incomplete := parseEvent(buf)
			if incomplete {
				break
			}
			buf = buf[n:]
			if ev.kind != evNone {
				select {
				case in.events <- ev:
				case <-in.done:
					return
				}
			}
		}
		wait = nil
		if len(buf) > 0 {
			wait = time.After(50 * time.Millisecond)
		}
		select {
		case chunk, ok := <-in.raw:
			if !ok {
				return
			}
			buf = append(buf, chunk...)
		case <-wait: // no more bytes came: take what is buffered at face value
			ev, n := parseRune(buf)
			buf = buf[n:]
			select {
			case in.events <- ev:
			case <-in.done:
				return
			}
		case <-in.done:
			return
		}
	}
}

func (in *inputReader) stop() { in.once.Do(func() { close(in.done) }) }
