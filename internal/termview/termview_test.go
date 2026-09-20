package termview

import (
	"io"
	"testing"
	"time"
)

func TestPageSizeIsUsableAndKeepsTheAspect(t *testing.T) {
	g := geometry{cols: 140, rows: 40} // a terminal that reports no pixel size
	for _, proto := range []string{protoBlocks, protoKitty} {
		v := &view{renderer: newRenderer(proto), geom: g}
		rw, rh := v.resolution()
		w, h := v.pageSize()
		if w < minPageW || h < minPageH {
			t.Errorf("%s: page %dx%d is too small to lay the toolbar out in", proto, w, h)
		}
		// The browser scales frames down to the resolution, which only comes out
		// right when the page has the same shape.
		if w*rh != h*rw {
			t.Errorf("%s: page %dx%d does not have the aspect of %dx%d", proto, w, h, rw, rh)
		}
	}
}

func TestPixelMapsCellsIntoThePage(t *testing.T) {
	v := &view{renderer: blocksRenderer{}, geom: geometry{cols: 100, rows: 51}}
	w, h := v.pageSize()
	cw, ch := float64(w)/100, float64(h)/50

	x, y := v.pixel(1, 1)
	if x <= 0 || x >= cw || y <= 0 || y >= ch {
		t.Errorf("the top left cell landed at %.1f,%.1f, outside the first cell", x, y)
	}
	// The last cell of the map has to stay inside the page…
	if x, y = v.pixel(100, 50); x >= float64(w) || y >= float64(h) {
		t.Errorf("the last cell landed at %.1f,%.1f, outside a %dx%d page", x, y, w, h)
	}
	// …and cells in between in the middle of what they cover.
	if x, _ = v.pixel(51, 1); x != 50.5*cw {
		t.Errorf("cell 51 landed at %.1f, want %.1f", x, 50.5*cw)
	}
}

func TestClickCountCountsDoubleClicks(t *testing.T) {
	v := &view{}
	if got := v.clickCount(mouseReport{col: 4, row: 5}); got != 1 {
		t.Fatalf("first click counted as %d", got)
	}
	if got := v.clickCount(mouseReport{col: 4, row: 5}); got != 2 {
		t.Errorf("a second click on the same cell counted as %d: nothing would expand", got)
	}
	if got := v.clickCount(mouseReport{col: 9, row: 5}); got != 1 {
		t.Errorf("a click on another cell counted as %d", got)
	}
	// A click long after the last one starts over.
	v.lastDown.at = time.Now().Add(-time.Second)
	if got := v.clickCount(mouseReport{col: 9, row: 5}); got != 1 {
		t.Errorf("a slow second click counted as %d", got)
	}
}

func TestInputReaderKeepsWhatFollowsTheQueryReply(t *testing.T) {
	r, w := io.Pipe()
	in := newInputReader(r)
	defer in.stop()

	go func() {
		// The terminal answers the capability query, and the user types while it does.
		io.WriteString(w, "\x1b_Gi=31;OK\x1b\\\x1b[?62;4c")
		io.WriteString(w, "v")
	}()
	reply := in.query(io.Discard, probeQuery, 2*time.Second)
	if got := parseProbe(reply); !got.kitty || !got.sixel {
		t.Errorf("reply %q read as %+v", reply, got)
	}
	in.start()
	select {
	case ev := <-in.events:
		if ev.kind != evKey || ev.key.name != "v" {
			t.Errorf("got %+v, want the v that was typed during the query", ev)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the keystroke that followed the reply was lost")
	}
}

func TestInputReaderTreatsALoneEscapeAsAKey(t *testing.T) {
	r, w := io.Pipe()
	in := newInputReader(r)
	defer in.stop()
	in.start()

	io.WriteString(w, "\x1b") // nothing follows: the user pressed Escape
	select {
	case ev := <-in.events:
		if ev.kind != evKey || ev.key.name != "Escape" {
			t.Errorf("got %+v, want Escape", ev)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Escape never arrived, so a selection could not be cleared")
	}
}

func TestInputReaderSplitsASequenceAcrossReads(t *testing.T) {
	r, w := io.Pipe()
	in := newInputReader(r)
	defer in.stop()
	in.start()

	go func() {
		io.WriteString(w, "\x1b[<0;10")
		time.Sleep(20 * time.Millisecond)
		io.WriteString(w, ";5M")
	}()
	select {
	case ev := <-in.events:
		if ev.kind != evMouse || ev.mouse.col != 10 || ev.mouse.row != 5 {
			t.Errorf("got %+v, want a press at 10,5", ev)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the split mouse report never arrived")
	}
}
