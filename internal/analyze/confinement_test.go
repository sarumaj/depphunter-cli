package analyze

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/beam"
	"github.com/sarumaj/depphunter-cli/internal/lang/crystal"
	"github.com/sarumaj/depphunter-cli/internal/lang/dart"
	"github.com/sarumaj/depphunter-cli/internal/lang/objc"
	"github.com/sarumaj/depphunter-cli/internal/lang/ruby"
	"github.com/sarumaj/depphunter-cli/internal/lang/swift"
)

// outsideVersion is the version the lock file outside the repository pins: it
// is in no fixture, so the graph holds it only if that file was read.
const outsideVersion = "97.53.1"

// copyTree copies a fixture directory, its symbolic links as links.
func copyTree(t *testing.T, from, to string) {
	t.Helper()
	err := filepath.WalkDir(from, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, _ := filepath.Rel(from, p)
		target := filepath.Join(to, relative)
		switch {
		case d.IsDir():
			return os.MkdirAll(target, 0o755)
		case d.Type()&fs.ModeSymlink != 0:
			link, err := os.Readlink(p)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// git runs git in directory, skipping the test where there is no git.
func git(t *testing.T, directory string, arguments ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", directory, "-c", "user.name=t", "-c", "user.email=t@example.com",
		"-c", "commit.gpgsign=false", "-c", "core.symlinks=true"}, arguments...)...)
	if out, err := command.CombinedOutput(); err != nil {
		t.Skipf("git %v: %v\n%s", arguments, err, out)
	}
}

// A repository that commits its lock file as a symbolic link to a file outside
// it - anywhere on this machine - must not get that file read: the scan skips
// the link, and the resolvers that read lock files from disk (as they do for a
// lock git ignores) must not follow it either. Each case copies a fixture into
// a fresh git repository with its lock file rewritten to pin outsideVersion:
// committed as a regular file the version is in the graph, which shows the
// case exercises the read, and so it is through a relative link to a file in
// the repository; committed as a link to the same content outside the
// repository, it is not. The outside link is relative ("../secret.lock"), the
// form a committed link takes to leave any clone.
//
// Verifies: REQ-LANG-031
func TestLockFileLinkedOutOfTheRepositoryIsNotRead(t *testing.T) {
	for _, testCase := range []struct {
		ecosystem string
		plugin    lang.Plugin
		fixture   string // under internal/lang
		lock      string // relative to the fixture
		old, new  string // the pinned version the lock file is rewritten from, to
	}{
		{"dart", dart.Plugin{}, "dart/testdata/repo", "app/pubspec.lock", `version: "1.9.0"`, `version: "` + outsideVersion + `"`},
		{"beam", beam.Plugin{}, "beam/testdata/repo", "mix.lock", `:jason, "1.4.1"`, `:jason, "` + outsideVersion + `"`},
		{"swift", swift.Plugin{}, "swift/testdata/repo", "Package.resolved", `"version" : "1.3.0"`, `"version" : "` + outsideVersion + `"`},
		{"crystal", crystal.Plugin{}, "crystal/testdata/repo", "shard.lock", "version: 1.4.0", "version: " + outsideVersion},
		{"ruby", ruby.Plugin{}, "ruby/testdata/repo", "Gemfile.lock", "pg (1.5.4)", "pg (" + outsideVersion + ")"},
		{"cocoapods", objc.Plugin{}, "objc/testdata/repo", "Podfile.lock", "AFNetworking (4.0.1):", "AFNetworking (" + outsideVersion + "):"},
	} {
		t.Run(testCase.ecosystem, func(t *testing.T) {
			original, err := os.ReadFile(filepath.Join("..", "lang", testCase.fixture, testCase.lock))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(original), testCase.old) {
				t.Fatalf("%s holds no %q", testCase.lock, testCase.old)
			}
			rewritten := []byte(strings.Replace(string(original), testCase.old, testCase.new, 1))
			for _, mode := range []string{"file", "link inside", "link outside"} {
				directory := t.TempDir()
				root := filepath.Join(directory, "repository")
				copyTree(t, filepath.Join("..", "lang", testCase.fixture), root)
				lock := filepath.Join(root, testCase.lock)
				if err := os.Remove(lock); err != nil {
					t.Fatal(err)
				}
				switch mode {
				case "file":
					if err := os.WriteFile(lock, rewritten, 0o644); err != nil {
						t.Fatal(err)
					}
				default:
					// A link that stays in the repository (a relative one, as
					// pnpm and workspaces make) is followed; one out of it is not.
					target := filepath.Join(directory, "secret.lock")
					if mode == "link inside" {
						target = filepath.Join(root, "pinned", "lock.txt")
					}
					if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(target, rewritten, 0o644); err != nil {
						t.Fatal(err)
					}
					relative, err := filepath.Rel(filepath.Dir(lock), target)
					if err != nil {
						t.Fatal(err)
					}
					if err := os.Symlink(relative, lock); err != nil {
						t.Skipf("no symbolic links here: %v", err)
					}
				}
				git(t, root, "init", "-q")
				git(t, root, "add", "-A")
				git(t, root, "commit", "-q", "-m", "fixture")
				g, _, err := Run(context.Background(), root, Options{Plugins: []lang.Plugin{testCase.plugin}})
				if err != nil {
					t.Fatal(err)
				}
				data, err := json.Marshal(g)
				if err != nil {
					t.Fatal(err)
				}
				switch read := strings.Contains(string(data), outsideVersion); {
				case mode == "link outside" && read:
					t.Errorf("%s linked out of the repository was read: the graph pins %s", testCase.lock, outsideVersion)
				case mode != "link outside" && !read:
					t.Errorf("%s committed as a %s pins no %s", testCase.lock, mode, outsideVersion)
				}
			}
		})
	}
}
