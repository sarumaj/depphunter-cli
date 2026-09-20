//go:build !windows

package termview

import (
	"errors"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/sys/unix"
)

// tty is the terminal the view draws on. It is opened as /dev/tty rather than taken
// from the standard streams, so the view still works when they are redirected; where
// there is no /dev/tty, the standard streams have to be a terminal themselves.
type tty struct {
	f    *os.File // written to, and read from when it is /dev/tty
	in   *os.File
	own  bool // f was opened here, rather than borrowed from the standard streams
	orig *unix.Termios
}

func openTTY() (*tty, error) {
	if f, err := os.OpenFile("/dev/tty", os.O_RDWR, 0); err == nil {
		return &tty{f: f, in: f, own: true}, nil
	}
	if !isTerminal(int(os.Stdout.Fd())) || !isTerminal(int(os.Stdin.Fd())) {
		return nil, errors.New("not a terminal")
	}
	return &tty{f: os.Stdout, in: os.Stdin}, nil
}

func isTerminal(fd int) bool {
	_, err := unix.IoctlGetTermios(fd, ioctlGetTermios)
	return err == nil
}

// file is where frames are written, input where keys are read.
func (t *tty) file() *os.File  { return t.f }
func (t *tty) input() *os.File { return t.in }

// raw turns off line buffering, echo and signal keys, so every keystroke reaches the
// page unchanged. Ctrl+C is read as a byte and quits the view (see run).
func (t *tty) raw() error {
	fd := int(t.in.Fd())
	orig, err := unix.IoctlGetTermios(fd, ioctlGetTermios)
	if err != nil {
		return err
	}
	t.orig = orig
	raw := *orig
	raw.Iflag &^= unix.IGNBRK | unix.BRKINT | unix.PARMRK | unix.ISTRIP | unix.INLCR | unix.IGNCR | unix.ICRNL | unix.IXON
	raw.Oflag &^= unix.OPOST
	raw.Lflag &^= unix.ECHO | unix.ECHONL | unix.ICANON | unix.ISIG | unix.IEXTEN
	raw.Cflag &^= unix.CSIZE | unix.PARENB
	raw.Cflag |= unix.CS8
	raw.Cc[unix.VMIN], raw.Cc[unix.VTIME] = 1, 0
	return unix.IoctlSetTermios(fd, ioctlSetTermios, &raw)
}

func (t *tty) restore() {
	if t.orig != nil {
		unix.IoctlSetTermios(int(t.in.Fd()), ioctlSetTermios, t.orig)
		t.orig = nil
	}
}

func (t *tty) close() {
	t.restore()
	if t.own {
		t.f.Close()
	}
}

// size reports the terminal in character cells and, when the terminal tells us, in
// pixels. Pixel sizes of zero mean it did not: probeCellSize then asks it directly.
func (t *tty) size() (geometry, error) {
	ws, err := unix.IoctlGetWinsize(int(t.f.Fd()), unix.TIOCGWINSZ)
	if err != nil {
		return geometry{}, err
	}
	return geometry{
		cols: int(ws.Col), rows: int(ws.Row),
		pixW: int(ws.Xpixel), pixH: int(ws.Ypixel),
	}, nil
}

// resizes reports terminal size changes.
func resizes() (<-chan os.Signal, func()) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	return ch, func() { signal.Stop(ch) }
}
