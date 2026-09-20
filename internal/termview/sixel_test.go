package termview

import (
	"bufio"
	"bytes"
	"image"
	"image/color"
	"strconv"
	"strings"
	"testing"
)

// decodeSixel reads back what writeSixel produced, so the encoder is checked against
// the protocol rather than against itself.
func decodeSixel(t *testing.T, s string) *image.RGBA {
	t.Helper()
	body, ok := strings.CutPrefix(s, "\x1bP0;1;0q")
	if !ok {
		t.Fatalf("no sixel introducer: %.20q", s)
	}
	body, ok = strings.CutSuffix(body, "\x1b\\")
	if !ok {
		t.Fatal("unterminated sixel")
	}
	if body[0] != '"' {
		t.Fatalf("no raster attributes: %.20q", body)
	}
	end := strings.IndexFunc(body, func(r rune) bool { return r != '"' && r != ';' && (r < '0' || r > '9') })
	raster := strings.Split(body[1:end], ";")
	if len(raster) != 4 {
		t.Fatalf("raster attributes %q", raster)
	}
	width, _ := strconv.Atoi(raster[2])
	height, _ := strconv.Atoi(raster[3])
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	palette := map[int]color.RGBA{}
	var cur color.RGBA
	x, top := 0, 0
	for i := end; i < len(body); {
		switch c := body[i]; {
		case c == '#':
			j := i + 1
			for j < len(body) && body[j] >= '0' && body[j] <= '9' {
				j++
			}
			n, _ := strconv.Atoi(body[i+1 : j])
			if j < len(body) && body[j] == ';' { // a color definition
				k := j
				for k < len(body) && (body[k] == ';' || (body[k] >= '0' && body[k] <= '9')) {
					k++
				}
				f := strings.Split(body[j+1:k], ";")
				if len(f) != 4 || f[0] != "2" {
					t.Fatalf("color %d: %q", n, f)
				}
				var v [3]int
				for p := 0; p < 3; p++ {
					pct, _ := strconv.Atoi(f[p+1])
					v[p] = pct * 255 / 100
				}
				palette[n] = color.RGBA{R: uint8(v[0]), G: uint8(v[1]), B: uint8(v[2]), A: 255}
				i = k
				continue
			}
			cur, x, i = palette[n], 0, j
		case c == '$':
			x, i = 0, i+1
		case c == '-':
			top, x, i = top+6, 0, i+1
		case c == '!':
			j := i + 1
			for j < len(body) && body[j] >= '0' && body[j] <= '9' {
				j++
			}
			n, _ := strconv.Atoi(body[i+1 : j])
			for r := 0; r < n; r++ {
				putSixel(img, x, top, body[j], cur)
				x++
			}
			i = j + 1
		case c >= 0x3f && c <= 0x7e:
			putSixel(img, x, top, c, cur)
			x, i = x+1, i+1
		default:
			t.Fatalf("unexpected byte %q at %d", c, i)
		}
	}
	return img
}

func putSixel(img *image.RGBA, x, top int, b byte, c color.RGBA) {
	for dy := 0; dy < 6; dy++ {
		if (b-0x3f)&(1<<dy) != 0 {
			img.Set(x, top+dy, c)
		}
	}
}

func TestSixelRoundTrip(t *testing.T) {
	// Colors from the cube itself survive quantization exactly, so what comes back
	// can be compared pixel for pixel.
	want := color.RGBA{R: 51, G: 102, B: 153, A: 255}
	img := solid(17, 13, want)
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	writeSixel(w, img)
	w.Flush()

	got := decodeSixel(t, buf.String())
	if b := got.Bounds(); b.Dx() != 17 || b.Dy() != 13 {
		t.Fatalf("size %v, want 17x13", b)
	}
	for _, p := range []image.Point{{X: 0, Y: 0}, {X: 16, Y: 12}, {X: 8, Y: 6}, {X: 3, Y: 11}} {
		if c := rgba(got.At(p.X, p.Y)); c != want {
			t.Errorf("pixel %v is %v, want %v", p, c, want)
		}
	}
}

func TestSixelKeepsColorsApart(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 12, 12))
	left, right := color.RGBA{R: 255, A: 255}, color.RGBA{B: 255, A: 255}
	for y := 0; y < 12; y++ {
		for x := 0; x < 12; x++ {
			c := left
			if x >= 6 {
				c = right
			}
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	writeSixel(w, img)
	w.Flush()

	got := decodeSixel(t, buf.String())
	for y := 0; y < 12; y++ {
		if c := rgba(got.At(2, y)); c != left {
			t.Fatalf("pixel 2,%d is %v, want %v", y, c, left)
		}
		if c := rgba(got.At(9, y)); c != right {
			t.Fatalf("pixel 9,%d is %v, want %v", y, c, right)
		}
	}
}

func TestSixelRunLength(t *testing.T) {
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	writeRunLength(w, []byte("~~~~~~??~~"))
	w.Flush()
	// Long runs are counted, short ones are cheaper written out.
	if got := buf.String(); got != "!6~??~~" {
		t.Errorf("got %q, want %q", got, "!6~??~~")
	}
}

func TestQuantizeLevelDithers(t *testing.T) {
	// A value halfway between two cube levels has to land on both, or wide gradients
	// turn into bands.
	seen := map[int]bool{}
	for x := 0; x < 4; x++ {
		for y := 0; y < 4; y++ {
			seen[quantizeLevel(25, x, y)] = true
		}
	}
	if len(seen) < 2 {
		t.Errorf("levels %v, want a mix", seen)
	}
	// The ends of the range stay put whatever the dither says.
	for x := 0; x < 4; x++ {
		if got := quantizeLevel(0, x, 0); got != 0 {
			t.Errorf("black became level %d", got)
		}
		if got := quantizeLevel(255, x, 0); got != sixelLevels-1 {
			t.Errorf("white became level %d", got)
		}
	}
}
