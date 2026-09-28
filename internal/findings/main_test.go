package findings

import (
	"os"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/userconf"
)

// TestMain keeps the system-wide configuration of the machine running the tests
// (/etc/pip.conf and the like) out of the credential stores the tests read with
// auth.Read, and pins their platform to Linux, where the fixtures sit.
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
