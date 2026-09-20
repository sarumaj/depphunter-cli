package termview

import (
	"errors"
	"os"
)

// The view needs a terminal in raw mode with pixel-addressable graphics. The Windows
// console can do neither through the interfaces used here, so the flag fails with an
// explanation instead of drawing something unusable.
type tty struct{ f *os.File }

var errUnsupported = errors.New("--terminal is not available on Windows; " +
	"run depphunter without it to use the browser")

func openTTY() (*tty, error)           { return nil, errUnsupported }
func isTerminal(int) bool              { return false }
func (t *tty) file() *os.File          { return t.f }
func (t *tty) input() *os.File         { return t.f }
func (t *tty) raw() error              { return errUnsupported }
func (t *tty) restore()                {}
func (t *tty) close()                  {}
func (t *tty) size() (geometry, error) { return geometry{}, errUnsupported }

func resizes() (<-chan os.Signal, func()) { return make(chan os.Signal), func() {} }
