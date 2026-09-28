package auth

import (
	"os"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/userconf"
)

// TestMain keeps the system-wide configuration of the machine running the tests
// (/etc/pip.conf and the like) out of every test here, and pins the platform Read
// gives the machine to Linux: the fixtures sit where the tools keep their files
// there (~/.config/...), whatever platform runs the tests. The tests of another
// platform's locations pick it through onMachine.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "depphunter-system")
	if err != nil {
		panic(err)
	}
	userconf.SystemRoot = dir
	userconf.Platform = "linux"
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
