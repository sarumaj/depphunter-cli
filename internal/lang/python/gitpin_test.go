package python

import (
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
)

// A direct reference to a git commit installs that commit and nothing else: it pins,
// and the distribution carries the checkout for the vulnerability database. A
// reference to a branch or a tag floats, as before.
//
// Verifies: REQ-PY-013, REQ-FND-026
func TestDirectReferenceToACommitPins(t *testing.T) {
	const sha = "0123456789abcdef0123456789abcdef01234567"
	root := langtest.Write(t, map[string]string{
		"requirements.txt": "foo @ git+https://github.com/o/foo@" + sha + "#egg=foo\n" +
			"bar @ git+https://github.com/o/bar.git@v1.0\nbaz @ git+ssh://git@git.corp.example/o/baz.git@" + sha + " ; python_version >= '3'\n",
		"app.py": "import foo\nimport bar\nimport baz\n",
	})
	res := langtest.Analyze(t, Plugin{}, root)
	langtest.CheckImports(t, res["app.py"], map[string]lang.Target{
		"foo": {Ecosystem: "pypi", Package: "foo", Version: "@ git+https://github.com/o/foo@" + sha, Pinned: true,
			Git: "https://github.com/o/foo#" + sha},
		"bar": {Ecosystem: "pypi", Package: "bar", Version: "@ git+https://github.com/o/bar.git@v1.0"},
		"baz": {Ecosystem: "pypi", Package: "baz", Version: "@ git+ssh://git@git.corp.example/o/baz.git@" + sha, Pinned: true,
			Git: "ssh://git@git.corp.example/o/baz.git#" + sha},
	})
}
