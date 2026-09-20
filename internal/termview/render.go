package termview

import (
	"bufio"
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
)

// geometry is the terminal's size: character cells always, pixels when the terminal
// reports them (some do not, and then the cell size is assumed).
type geometry struct {
	cols, rows int
	pixW, pixH int
}

const (
	defaultCellW = 10 // assumed when the terminal will not say
	defaultCellH = 20
	statusRows   = 1 // the bottom line holds the key hints, not the map
)

// cell reports the size of one character cell in pixels.
func (g geometry) cell() (w, h int) {
	w, h = defaultCellW, defaultCellH
	if g.cols > 0 && g.pixW > 0 {
		w = g.pixW / g.cols
	}
	if g.rows > 0 && g.pixH > 0 {
		h = g.pixH / g.rows
	}
	if w < 1 {
		w = defaultCellW
	}
	if h < 1 {
		h = defaultCellH
	}
	return w, h
}

// mapRows is the number of cell rows the map is drawn in.
func (g geometry) mapRows() int {
	if g.rows > statusRows {
		return g.rows - statusRows
	}
	return 1
}

// frame is one screencast frame. The picture is decoded only for the renderers that
// need pixels: the iTerm2 protocol takes the browser's JPEG as it is.
type frame struct {
	data []byte
	img  image.Image
}

func (f *frame) image() (image.Image, error) {
	if f.img == nil {
		img, err := jpeg.Decode(bytes.NewReader(f.data))
		if err != nil {
			return nil, err
		}
		f.img = img
	}
	return f.img, nil
}

// renderer paints frames with one terminal graphics protocol.
type renderer interface {
	name() string
	// resolution is the size, in pixels, of the picture this protocol draws in a
	// terminal of this geometry: the protocols that address pixels ask for the
	// terminal's own, half-blocks for two pixels per character cell.
	resolution(g geometry) (w, h int)
	// draw writes one frame at the cursor's position.
	draw(w *bufio.Writer, f *frame, g geometry) error
}

// pixelSize is the pixel area the map occupies for protocols that draw real pixels.
func pixelSize(g geometry) (int, int) {
	cw, ch := g.cell()
	return g.cols * cw, g.mapRows() * ch
}

// kittyRenderer speaks the kitty graphics protocol (kitty, ghostty, WezTerm, konsole).
// Frames go out as raw RGB compressed with zlib, which costs far less per frame than
// re-encoding a picture the browser has already compressed once.
type kittyRenderer struct{ id int }

func (kittyRenderer) name() string                     { return "kitty" }
func (kittyRenderer) resolution(g geometry) (int, int) { return pixelSize(g) }

func (k kittyRenderer) draw(w *bufio.Writer, f *frame, g geometry) error {
	img, err := f.image()
	if err != nil {
		return err
	}
	b := img.Bounds()
	rgb := make([]byte, 0, b.Dx()*b.Dy()*3)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			rgb = append(rgb, byte(r>>8), byte(g>>8), byte(bl>>8))
		}
	}
	var zBuf bytes.Buffer
	zw := zlib.NewWriter(&zBuf)
	if _, err := zw.Write(rgb); err != nil {
		return err
	}
	if err := zw.Close(); err != nil {
		return err
	}
	id := k.id
	if id == 0 {
		id = 1
	}
	// Replacing the image under the same id keeps the terminal from filling up with
	// one picture per frame.
	fmt.Fprintf(w, "\x1b_Ga=d,d=i,i=%d,q=2\x1b\\", id)

	payload := base64.StdEncoding.EncodeToString(zBuf.Bytes())
	const chunk = 4096
	first := true
	for len(payload) > 0 {
		n := min(chunk, len(payload))
		more := 0
		if n < len(payload) {
			more = 1
		}
		if first {
			fmt.Fprintf(w, "\x1b_Ga=T,f=24,o=z,i=%d,s=%d,v=%d,C=1,q=2,m=%d;%s\x1b\\",
				id, b.Dx(), b.Dy(), more, payload[:n])
			first = false
		} else {
			fmt.Fprintf(w, "\x1b_Gm=%d,q=2;%s\x1b\\", more, payload[:n])
		}
		payload = payload[n:]
	}
	return nil
}

// itermRenderer speaks the iTerm2 inline-image protocol, which takes an image file as
// it is: the browser's JPEG is forwarded without touching the pixels.
type itermRenderer struct{}

func (itermRenderer) name() string                     { return "iterm" }
func (itermRenderer) resolution(g geometry) (int, int) { return pixelSize(g) }

func (itermRenderer) draw(w *bufio.Writer, f *frame, g geometry) error {
	cols, rows := g.cols, g.mapRows()
	fmt.Fprintf(w, "\x1b]1337;File=inline=1;size=%d;width=%d;height=%d;preserveAspectRatio=0:%s\a",
		len(f.data), cols, rows, base64.StdEncoding.EncodeToString(f.data))
	return nil
}

// blocksRenderer needs no graphics protocol at all, only 24-bit color and the Block
// Elements of Unicode. Each character cell carries four pixels - one per quadrant -
// drawn with the glyph whose filled corners match, in the two colors a cell can
// hold. That is twice the width and twice the height of a half block, which is what
// this map needs: streets, labels and island edges survive the fallback far better.
type blocksRenderer struct {
	// half draws one pixel above another instead, in exactly the cell's two colors.
	// Quadrant glyphs are rarer in old fonts, and four pixels in a cell have to be
	// forced into two colors; this is the way out when either shows.
	half bool
}

func (r blocksRenderer) name() string {
	if r.half {
		return "halfblocks"
	}
	return "blocks"
}

func (r blocksRenderer) resolution(g geometry) (int, int) {
	if r.half {
		return g.cols, g.mapRows() * 2
	}
	return g.cols * 2, g.mapRows() * 2
}

// quadrants are the glyphs for the sixteen ways four corners can be filled, indexed
// by the bits upper-left, upper-right, lower-left, lower-right.
var quadrants = [16]rune{
	' ', '\u2598', '\u259d', '\u2580', '\u2596', '\u258c', '\u259e', '\u259b',
	'\u2597', '\u259a', '\u2590', '\u259c', '\u2584', '\u2599', '\u259f', '\u2588',
}

func (r blocksRenderer) draw(w *bufio.Writer, f *frame, g geometry) error {
	img, err := f.image()
	if err != nil {
		return err
	}
	width, height := r.resolution(g)
	img = fit(img, width, height)
	b := img.Bounds()
	step := 2 // pixels per cell across; a half block covers one
	if r.half {
		step = 1
	}
	at := func(x, y int) color.RGBA {
		return rgba(img.At(b.Min.X+min(x, b.Dx()-1), b.Min.Y+min(y, b.Dy()-1)))
	}
	var last struct {
		fg, bg color.RGBA
		set    bool
	}
	for y := 0; y < b.Dy(); y += 2 {
		if y > 0 {
			w.WriteString("\r\n")
		}
		last.set = false
		for x := 0; x < b.Dx(); x += step {
			var fg, bg color.RGBA
			var glyph rune
			if r.half {
				fg, bg, glyph = at(x, y), at(x, y+1), '\u2580'
			} else {
				fg, bg, glyph = quadrant(at(x, y), at(x+1, y), at(x, y+1), at(x+1, y+1))
			}
			if !last.set || last.fg != fg {
				fmt.Fprintf(w, "\x1b[38;2;%d;%d;%dm", fg.R, fg.G, fg.B)
			}
			if !last.set || last.bg != bg {
				fmt.Fprintf(w, "\x1b[48;2;%d;%d;%dm", bg.R, bg.G, bg.B)
			}
			last.fg, last.bg, last.set = fg, bg, true
			w.WriteRune(glyph)
		}
		w.WriteString("\x1b[0m")
	}
	return nil
}

// quadrant picks the glyph and the two colors for one cell of four pixels: those
// brighter than the cell's mean are drawn in the foreground, the rest in the
// background, and each color is the average of the pixels it stands for. Splitting
// on brightness keeps an edge - a street against grass, a label against a facade -
// where averaging all four would smear it.
func quadrant(ul, ur, ll, lr color.RGBA) (fg, bg color.RGBA, glyph rune) {
	pixels := [4]color.RGBA{ul, ur, ll, lr}
	mean := 0
	for _, p := range pixels {
		mean += luma(p)
	}
	mean /= len(pixels)

	bits := 0
	var light, dark []color.RGBA
	for i, p := range pixels {
		if luma(p) > mean {
			bits |= 1 << i
			light = append(light, p)
		} else {
			dark = append(dark, p)
		}
	}
	if len(light) == 0 { // a flat cell: one color, and no edge to keep
		return pixels[0], pixels[0], '\u2588'
	}
	return average(light), average(dark), quadrants[bits]
}

// luma weights the channels the way the eye does.
func luma(c color.RGBA) int { return (int(c.R)*299 + int(c.G)*587 + int(c.B)*114) / 1000 }

func average(cs []color.RGBA) color.RGBA {
	var r, g, b int
	for _, c := range cs {
		r, g, b = r+int(c.R), g+int(c.G), b+int(c.B)
	}
	n := len(cs)
	return color.RGBA{R: uint8(r / n), G: uint8(g / n), B: uint8(b / n), A: 0xff}
}

func rgba(c color.Color) color.RGBA {
	r, g, b, _ := c.RGBA()
	return color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: 0xff}
}

// fit scales img to w×h by nearest neighbor. Frames normally arrive at the size the
// page was told to render at; this covers the moment after a resize, when one frame of
// the old size may still be in flight.
func fit(img image.Image, w, h int) image.Image {
	b := img.Bounds()
	if b.Dx() == w && b.Dy() == h {
		return img
	}
	if w < 1 || h < 1 || b.Dx() < 1 || b.Dy() < 1 {
		return img
	}
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		// The middle of the source area the pixel stands for, not its corner: on the
		// map's thin lines a corner sample drops whole streets.
		sy := b.Min.Y + (2*y+1)*b.Dy()/(2*h)
		for x := 0; x < w; x++ {
			sx := b.Min.X + (2*x+1)*b.Dx()/(2*w)
			out.Set(x, y, img.At(sx, sy))
		}
	}
	return out
}

// newRenderer returns the renderer for a protocol name.
func newRenderer(proto string) renderer {
	switch proto {
	case "kitty":
		return kittyRenderer{id: 1}
	case "iterm":
		return itermRenderer{}
	case "sixel":
		return sixelRenderer{}
	case "halfblocks":
		return blocksRenderer{half: true}
	default:
		return blocksRenderer{}
	}
}

// writeAll is io.Writer.Write without the short-write case, for the escape sequences
// that frame a redraw.
func writeAll(w io.Writer, s string) error {
	_, err := io.WriteString(w, s)
	return err
}
