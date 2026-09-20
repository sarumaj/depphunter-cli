package termview

import (
	"bufio"
	"fmt"
	"image"
	"strconv"
)

// The sixel palette is a fixed 6×6×6 color cube. A palette computed per frame would
// look better, but the map's colors are stable and a shared cube keeps every frame
// cheap - which is what a stream of them needs.
const (
	sixelLevels = 6
	sixelColors = sixelLevels * sixelLevels * sixelLevels
)

// levelValue maps a cube level to its 8-bit value (0, 51, 102, …, 255).
func levelValue(level int) int { return level * 255 / (sixelLevels - 1) }

// bayer is the 4×4 ordered dither matrix. Without it the sky and the sea, which are
// wide gradients, break into visible bands on a 216-color palette.
var bayer = [4][4]int{
	{0, 8, 2, 10},
	{12, 4, 14, 6},
	{3, 11, 1, 9},
	{15, 7, 13, 5},
}

// quantizeLevel picks the cube level for one channel, nudged by the dither matrix.
func quantizeLevel(v, x, y int) int {
	const step = 255 / (sixelLevels - 1)
	v += (bayer[y&3][x&3]*step)/16 - step/2
	level := (v*(sixelLevels-1) + 127) / 255
	if level < 0 {
		return 0
	}
	if level >= sixelLevels {
		return sixelLevels - 1
	}
	return level
}

// sixelRenderer speaks the sixel protocol (xterm -ti vt340, foot, mlterm, contour,
// recent Windows Terminal and VTE).
type sixelRenderer struct{}

func (sixelRenderer) name() string                     { return "sixel" }
func (sixelRenderer) resolution(g geometry) (int, int) { return pixelSize(g) }

func (sixelRenderer) draw(w *bufio.Writer, f *frame, g geometry) error {
	img, err := f.image()
	if err != nil {
		return err
	}
	width, height := pixelSize(g)
	img = fit(img, width, height)
	writeSixel(w, img)
	return nil
}

// writeSixel encodes img as one sixel image. Sixels are written in bands of six pixel
// rows: within a band each color is drawn in its own pass, one bit per row.
func writeSixel(w *bufio.Writer, img image.Image) {
	b := img.Bounds()
	width, height := b.Dx(), b.Dy()

	// P1 = pixel aspect 1:1, P2 = 1 leaves the background untouched, P3 unused.
	w.WriteString("\x1bP0;1;0q")
	fmt.Fprintf(w, `"1;1;%d;%d`, width, height)

	indexed := make([]uint16, width*height)
	used := make([]bool, sixelColors)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			c := rgba(img.At(b.Min.X+x, b.Min.Y+y))
			idx := uint16(quantizeLevel(int(c.R), x, y)*sixelLevels*sixelLevels +
				quantizeLevel(int(c.G), x, y)*sixelLevels +
				quantizeLevel(int(c.B), x, y))
			indexed[y*width+x] = idx
			used[idx] = true
		}
	}
	for idx, ok := range used {
		if !ok {
			continue
		}
		r, g, bl := idx/(sixelLevels*sixelLevels), (idx/sixelLevels)%sixelLevels, idx%sixelLevels
		// Sixel color components are percentages, not bytes.
		fmt.Fprintf(w, "#%d;2;%d;%d;%d", idx,
			levelValue(r)*100/255, levelValue(g)*100/255, levelValue(bl)*100/255)
	}

	row := make([]byte, width)
	for top := 0; top < height; top += 6 {
		first := true
		for idx, ok := range used {
			if !ok {
				continue
			}
			if !bandRow(row, indexed, width, height, top, uint16(idx)) {
				continue
			}
			if !first {
				w.WriteByte('$') // back to the left edge, same band
			}
			first = false
			w.WriteByte('#')
			w.WriteString(strconv.Itoa(idx))
			writeRunLength(w, row)
		}
		if top+6 < height {
			w.WriteByte('-') // next band
		}
	}
	w.WriteString("\x1b\\")
}

// bandRow fills row with one sixel byte per column for color idx, and reports whether
// the color appears in this band at all.
func bandRow(row []byte, indexed []uint16, width, height, top int, idx uint16) bool {
	any := false
	for x := 0; x < width; x++ {
		var bits byte
		for dy := 0; dy < 6 && top+dy < height; dy++ {
			if indexed[(top+dy)*width+x] == idx {
				bits |= 1 << dy
			}
		}
		if bits != 0 {
			any = true
		}
		row[x] = 0x3f + bits
	}
	return any
}

// writeRunLength writes the band with sixel run-length encoding. Runs are what keeps
// the payload small: the map has large areas of one color.
func writeRunLength(w *bufio.Writer, row []byte) {
	for i := 0; i < len(row); {
		j := i + 1
		for j < len(row) && row[j] == row[i] {
			j++
		}
		// "!n c" pays for itself from four repetitions on.
		if n := j - i; n > 3 {
			w.WriteByte('!')
			w.WriteString(strconv.Itoa(n))
			w.WriteByte(row[i])
		} else {
			for k := 0; k < n; k++ {
				w.WriteByte(row[i])
			}
		}
		i = j
	}
}
