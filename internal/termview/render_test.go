package termview

import (
	"bufio"
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"regexp"
	"strings"
	"testing"
)

// testFrame encodes a picture the way the browser does, so the renderers get what they
// get in practice.
func testFrame(t *testing.T, img image.Image) *frame {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	return &frame{data: buf.Bytes()}
}

func solid(w, h int, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	return img
}

func TestGeometryCell(t *testing.T) {
	g := geometry{cols: 100, rows: 40, pixW: 800, pixH: 800}
	if w, h := g.cell(); w != 8 || h != 20 {
		t.Errorf("cell %dx%d, want 8x20", w, h)
	}
	// A terminal that reports no pixel size gets the assumed one.
	g = geometry{cols: 100, rows: 40}
	if w, h := g.cell(); w != defaultCellW || h != defaultCellH {
		t.Errorf("cell %dx%d, want the default", w, h)
	}
	if got := g.mapRows(); got != 39 {
		t.Errorf("map rows %d, want 39 (one row is the status line)", got)
	}
	if got := (geometry{cols: 10, rows: 1}).mapRows(); got != 1 {
		t.Errorf("map rows %d in a one-row terminal, want 1", got)
	}
}

func TestResolutionsFollowTheProtocol(t *testing.T) {
	g := geometry{cols: 80, rows: 25, pixW: 640, pixH: 500}
	// Pixel protocols draw in the terminal's own pixels…
	for _, r := range []renderer{kittyRenderer{}, itermRenderer{}, sixelRenderer{}} {
		if w, h := r.resolution(g); w != 640 || h != 480 {
			t.Errorf("%s: %dx%d, want 640x480", r.name(), w, h)
		}
	}
	// …a quadrant carries four pixels per cell, a half block two.
	if w, h := (blocksRenderer{}).resolution(g); w != 160 || h != 48 {
		t.Errorf("blocks: %dx%d, want 160x48", w, h)
	}
	if w, h := (blocksRenderer{half: true}).resolution(g); w != 80 || h != 48 {
		t.Errorf("halfblocks: %dx%d, want 80x48", w, h)
	}
}

// drawCell renders a single character cell from four pixels and returns what was
// written, so the glyph and its two colors can be read back.
func drawCell(t *testing.T, r blocksRenderer, pixels [4]color.RGBA) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, pixels[0])
	img.Set(1, 0, pixels[1])
	img.Set(0, 1, pixels[2])
	img.Set(1, 1, pixels[3])
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	// One cell: two rows of pixels, and as many columns as the renderer takes.
	cols := 1
	if r.half {
		cols = 2
	}
	if err := r.draw(w, &frame{img: img}, geometry{cols: cols, rows: 2}); err != nil {
		t.Fatal(err)
	}
	w.Flush()
	return buf.String()
}

func TestQuadrantsKeepTheEdgeInACell(t *testing.T) {
	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	black := color.RGBA{A: 255}
	for _, c := range []struct {
		name   string
		pixels [4]color.RGBA
		glyph  string
	}{
		// Only the bright corners are drawn in the foreground, so the glyph says
		// which of the four pixels they were.
		{"upper left", [4]color.RGBA{white, black, black, black}, "\u2598"},
		{"lower right", [4]color.RGBA{black, black, black, white}, "\u2597"},
		{"left half", [4]color.RGBA{white, black, white, black}, "\u258c"},
		{"diagonal", [4]color.RGBA{black, white, white, black}, "\u259e"},
		{"upper half", [4]color.RGBA{white, white, black, black}, "\u2580"},
		// Four pixels of one color have no edge to keep.
		{"flat", [4]color.RGBA{white, white, white, white}, "\u2588"},
	} {
		t.Run(c.name, func(t *testing.T) {
			out := drawCell(t, blocksRenderer{}, c.pixels)
			if !strings.Contains(out, c.glyph) {
				t.Errorf("got %q, want the glyph %q", out, c.glyph)
			}
			if c.name != "flat" {
				if !strings.Contains(out, "\x1b[38;2;255;255;255m") || !strings.Contains(out, "\x1b[48;2;0;0;0m") {
					t.Errorf("got %q, want white on black", out)
				}
			}
		})
	}
}

func TestQuadrantsAverageWithinEachHalf(t *testing.T) {
	// Two bright pixels and two dark ones: each side is drawn in its own average
	// rather than one color for the whole cell.
	out := drawCell(t, blocksRenderer{}, [4]color.RGBA{
		{R: 200, G: 200, B: 200, A: 255}, {R: 220, G: 220, B: 220, A: 255},
		{R: 20, G: 20, B: 20, A: 255}, {R: 40, G: 40, B: 40, A: 255},
	})
	if !strings.Contains(out, "\x1b[38;2;210;210;210m") || !strings.Contains(out, "\x1b[48;2;30;30;30m") {
		t.Errorf("got %q, want the mean of each half", out)
	}
}

func TestHalfBlocksPaintPixelPairs(t *testing.T) {
	out := drawCell(t, blocksRenderer{half: true}, [4]color.RGBA{
		{R: 255, A: 255}, {G: 255, A: 255}, {B: 255, A: 255}, {R: 255, G: 255, B: 255, A: 255},
	})
	if n := strings.Count(out, "\u2580"); n != 2 {
		t.Errorf("%d half blocks, want one per column: %q", n, out)
	}
	for _, want := range []string{"\x1b[38;2;255;0;0m", "\x1b[48;2;0;0;255m", "\x1b[38;2;0;255;0m"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing color %q in %q", want, out)
		}
	}
}

func TestKittyRendererSendsTheFrameWhole(t *testing.T) {
	img := solid(40, 20, color.RGBA{R: 1, G: 2, B: 3, A: 255})
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	if err := (kittyRenderer{id: 1}).draw(w, &frame{img: img}, geometry{cols: 40, rows: 21, pixW: 40, pixH: 21}); err != nil {
		t.Fatal(err)
	}
	w.Flush()
	out := buf.String()
	if !strings.HasPrefix(out, "\x1b_Ga=d,d=i,i=1,q=2\x1b\\") {
		t.Error("the previous frame is not deleted, so the terminal fills up with images")
	}
	if !strings.Contains(out, "s=40,v=20") {
		t.Errorf("the frame's size is not declared: %.120q", out)
	}
	// Every chunk but the last says another follows.
	chunks := regexp.MustCompile(`\x1b_G[^;]*;([^\x1b]*)\x1b\\`).FindAllStringSubmatch(out, -1)
	if len(chunks) == 0 {
		t.Fatalf("no image data: %.120q", out)
	}
	if !strings.Contains(out, "m=0;") {
		t.Error("no chunk is marked as the last")
	}
	var payload strings.Builder
	for _, c := range chunks {
		payload.WriteString(c[1])
	}
	raw, err := base64.StdEncoding.DecodeString(payload.String())
	if err != nil {
		t.Fatalf("payload is not base64: %v", err)
	}
	zr, err := zlib.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("payload is not zlib: %v", err)
	}
	rgb, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	if len(rgb) != 40*20*3 {
		t.Fatalf("%d bytes of pixels, want %d", len(rgb), 40*20*3)
	}
	if rgb[0] != 1 || rgb[1] != 2 || rgb[2] != 3 {
		t.Errorf("first pixel %v, want 1 2 3", rgb[:3])
	}
}

func TestItermRendererForwardsTheJPEG(t *testing.T) {
	f := testFrame(t, solid(20, 10, color.RGBA{R: 200, A: 255}))
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	if err := (itermRenderer{}).draw(w, f, geometry{cols: 20, rows: 11, pixW: 20, pixH: 11}); err != nil {
		t.Fatal(err)
	}
	w.Flush()
	out := buf.String()
	i, j := strings.Index(out, ":"), strings.Index(out, "\a")
	if i < 0 || j < 0 {
		t.Fatalf("not an inline image: %.80q", out)
	}
	raw, err := base64.StdEncoding.DecodeString(out[i+1 : j])
	if err != nil {
		t.Fatal(err)
	}
	// The browser's JPEG is passed on untouched: no pixel is decoded on the way.
	if !bytes.Equal(raw, f.data) {
		t.Error("the frame was re-encoded")
	}
	if f.img != nil {
		t.Error("the frame was decoded although the protocol takes a file")
	}
}

func TestFitScales(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			c := color.RGBA{R: 255, A: 255}
			if x >= 2 && y >= 2 {
				c = color.RGBA{B: 255, A: 255}
			}
			img.Set(x, y, c)
		}
	}
	out := fit(img, 2, 2)
	if b := out.Bounds(); b.Dx() != 2 || b.Dy() != 2 {
		t.Fatalf("size %v", b)
	}
	if rgba(out.At(0, 0)).R != 255 || rgba(out.At(1, 1)).B != 255 {
		t.Error("the corners did not survive scaling")
	}
	// A frame that already fits is left alone.
	if got := fit(img, 4, 4); got != image.Image(img) {
		t.Error("a frame of the right size was copied")
	}
}

func TestNewRenderer(t *testing.T) {
	for proto, want := range map[string]string{
		protoKitty: "kitty", protoITerm: "iterm", protoSixel: "sixel",
		protoBlocks: "blocks", "nonsense": "blocks",
	} {
		if got := newRenderer(proto).name(); got != want {
			t.Errorf("%s: got %s, want %s", proto, got, want)
		}
	}
}
