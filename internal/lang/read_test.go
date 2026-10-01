package lang

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// readFixture is a repository root in a directory of its own, with a file
// beside it (outside the root) that no read under the root may reach through
// its path, and a file one byte over MaxParseSize inside it.
type readFixture struct {
	root, outside string
}

func newReadFixture(t *testing.T) readFixture {
	t.Helper()
	directory := t.TempDir()
	fixture := readFixture{root: filepath.Join(directory, "repository"), outside: filepath.Join(directory, "secret.lock")}
	for name, content := range map[string][]byte{
		fixture.outside: []byte("outside"),
		filepath.Join(fixture.root, "a", "pubspec.lock"): []byte("on disk"),
		filepath.Join(fixture.root, "a", "listed.lock"):  []byte("on disk"),
		filepath.Join(fixture.root, "large.lock"):        bytes.Repeat([]byte("x"), MaxParseSize+1),
		filepath.Join(fixture.root, "limit.lock"):        bytes.Repeat([]byte("x"), MaxParseSize),
	} {
		if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return fixture
}

// Verifies: REQ-LANG-012
func TestReadBoundedMeasuresBeforeReading(t *testing.T) {
	fixture := newReadFixture(t)
	for _, testCase := range []struct {
		name string
		ok   bool
	}{
		{"limit.lock", true},
		{"large.lock", false},   // over MaxParseSize
		{"a", false},            // a directory
		{"missing.lock", false}, // not there
	} {
		data, ok := ReadBounded(filepath.Join(fixture.root, testCase.name))
		if ok != testCase.ok || !ok && data != nil {
			t.Errorf("ReadBounded(%s) = %d bytes, %v, want ok %v", testCase.name, len(data), ok, testCase.ok)
		}
	}
}

// Verifies: REQ-LANG-012
func TestReadScannedSkipsWhatReadableRejects(t *testing.T) {
	fixture := newReadFixture(t)
	small := filepath.Join(fixture.root, "a", "pubspec.lock")
	for _, testCase := range []struct {
		f  scan.File
		ok bool
	}{
		{scan.File{Path: "a/pubspec.lock", AbsolutePath: small, Size: 7}, true},
		// The scan's measurements decide, before anything is read.
		{scan.File{Path: "a/pubspec.lock", AbsolutePath: small, Size: MaxParseSize + 1}, false},
		{scan.File{Path: "a/pubspec.lock", AbsolutePath: small, Size: 7, Binary: true}, false},
		{scan.File{Path: "a/pubspec.lock", AbsolutePath: small, Size: 7, TooLarge: true}, false},
		{scan.File{Path: "gone.lock", AbsolutePath: filepath.Join(fixture.root, "gone.lock")}, false},
	} {
		if data, ok := ReadScanned(&testCase.f); ok != testCase.ok || ok && string(data) != "on disk" {
			t.Errorf("ReadScanned(%+v) = %q, %v, want ok %v", testCase.f, data, ok, testCase.ok)
		}
	}
}

// symlink makes a symbolic link, or skips the test where none can be made (on
// Windows without the privilege or developer mode).
func symlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("no symbolic links here: %v", err)
	}
}

// links adds the symbolic links the confinement is about to a fixture: to
// files and directories inside the root (relative and absolute, as pnpm and
// shards make them) and outside it, and to files too large to parse.
func (fixture readFixture) links(t *testing.T) {
	t.Helper()
	outsideDirectory := filepath.Join(filepath.Dir(fixture.outside), "elsewhere")
	if err := os.MkdirAll(outsideDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outsideDirectory, "pubspec.lock"), []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	largeOutside := filepath.Join(outsideDirectory, "large.lock")
	if err := os.WriteFile(largeOutside, bytes.Repeat([]byte("x"), MaxParseSize+1), 0o644); err != nil {
		t.Fatal(err)
	}
	for link, target := range map[string]string{
		"linked.lock":          fixture.outside,                                     // a file outside
		"relative-out.lock":    filepath.Join("..", filepath.Base(fixture.outside)), // the same, relatively
		"inside.lock":          filepath.Join("a", "pubspec.lock"),                  // a file inside, relatively
		"absolute-inside.lock": filepath.Join(fixture.root, "a", "pubspec.lock"),    // and absolutely
		"linkdir":              "a",                                                 // a directory inside
		"outdir":               outsideDirectory,                                    // a directory outside
		"large-link.lock":      "large.lock",                                        // an oversize file inside
		"large-out.lock":       largeOutside,                                        // an oversize file outside
	} {
		symlink(t, target, filepath.Join(fixture.root, link))
	}
}

// Verifies: REQ-LANG-031, REQ-LANG-012
func TestRootConfinesReadsToTheRepository(t *testing.T) {
	fixture := newReadFixture(t)
	fixture.links(t)
	root := OpenRoot(fixture.root)
	for _, testCase := range []struct {
		name, path string
		want       string // "" when refused
	}{
		{"a file", "a/pubspec.lock", "on disk"},
		{"a symbolic link to a file outside", "linked.lock", ""},
		{"a relative symbolic link to a file outside", "relative-out.lock", ""},
		{"a symbolic link to a file inside", "inside.lock", "on disk"},
		{"an absolute symbolic link to a file inside", "absolute-inside.lock", "on disk"},
		{"a file under a symbolic link to a directory inside", "linkdir/pubspec.lock", "on disk"},
		{"a file under a symbolic link to a directory outside", "outdir/pubspec.lock", ""},
		{"..", "../secret.lock", ""},
		{"a path climbing out and back", "a/../../repository/a/pubspec.lock", "on disk"},
		{"an absolute path outside", fixture.outside, ""},
		{"a file of exactly MaxParseSize", "limit.lock", "limit"},
		{"a file over MaxParseSize", "large.lock", ""},
		{"a symbolic link to a file over MaxParseSize", "large-link.lock", ""},
		{"a symbolic link to a file outside over MaxParseSize", "large-out.lock", ""},
		{"a directory", "a", ""},
		{"a missing file", "missing.lock", ""},
	} {
		absolute := testCase.path
		if !filepath.IsAbs(absolute) {
			absolute = filepath.Join(fixture.root, filepath.FromSlash(testCase.path))
		}
		data, ok := root.ReadBounded(absolute)
		switch {
		case testCase.want == "" && (ok || data != nil):
			t.Errorf("%s: ReadBounded(%s) = %d bytes, want nothing", testCase.name, testCase.path, len(data))
		case testCase.want == "limit" && (!ok || len(data) != MaxParseSize):
			t.Errorf("%s: ReadBounded(%s) = %d bytes, %v, want the whole file", testCase.name, testCase.path, len(data), ok)
		case testCase.want != "" && testCase.want != "limit" && (!ok || string(data) != testCase.want):
			t.Errorf("%s: ReadBounded(%s) = %q, %v, want %q", testCase.name, testCase.path, data, ok, testCase.want)
		}
	}
	// The zero Root, and one without a directory, read nothing.
	for _, r := range []Root{{}, OpenRoot("")} {
		if data, ok := r.ReadBounded(filepath.Join(fixture.root, "a", "pubspec.lock")); ok || data != nil {
			t.Errorf("%+v read %q", r, data)
		}
	}
	// Machine reads anywhere, bounded as well.
	if data, ok := Machine.ReadBounded(fixture.outside); !ok || string(data) != "outside" {
		t.Errorf("Machine.ReadBounded(outside) = %q, %v", data, ok)
	}
	if _, ok := Machine.ReadBounded(filepath.Join(fixture.root, "large-out.lock")); ok {
		t.Error("Machine.ReadBounded read a file over MaxParseSize through a link")
	}
}

// Verifies: REQ-LANG-031
func TestRootListsAndWalksOnlyTheRepository(t *testing.T) {
	fixture := newReadFixture(t)
	fixture.links(t)
	root := OpenRoot(fixture.root)
	join := func(p string) string { return filepath.Join(fixture.root, filepath.FromSlash(p)) }
	if !root.IsDirectory(join("linkdir")) || root.IsDirectory(join("outdir")) || root.IsDirectory(join("..")) {
		t.Error("IsDirectory: want linkdir only")
	}
	if _, err := root.Stat(join("linked.lock")); err == nil {
		t.Error("Stat followed a link out of the root")
	}
	if entries, err := root.ReadDir(join("outdir")); err == nil || entries != nil {
		t.Errorf("ReadDir(outdir) = %v, %v, want an error", entries, err)
	}
	if entries, err := root.ReadDir(join("linkdir")); err != nil || len(entries) != 2 {
		t.Errorf("ReadDir(linkdir) = %v, %v, want a's two files", entries, err)
	}
	var walked []string
	root.WalkDir(fixture.root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			t.Errorf("WalkDir: %v", err)
			return nil
		}
		relative, _ := filepath.Rel(fixture.root, p)
		walked = append(walked, filepath.ToSlash(relative))
		return nil
	})
	for _, p := range walked {
		if strings.HasPrefix(p, "outdir/") || strings.HasPrefix(p, "linkdir/") {
			t.Errorf("WalkDir descended into the link %s", p)
		}
	}
	if !slices.Contains(walked, "a/pubspec.lock") || !slices.Contains(walked, "outdir") {
		t.Errorf("WalkDir = %v", walked)
	}
	// The top of a walk is not followed when it is a link, as with filepath.WalkDir.
	n := 0
	root.WalkDir(join("outdir"), func(string, fs.DirEntry, error) error { n++; return nil })
	if n != 1 {
		t.Errorf("WalkDir(outdir) visited %d entries, want the link alone", n)
	}
	if matches, _ := root.Glob(join("*dir/pubspec.lock")); !slices.Equal(matches, []string{join("linkdir/pubspec.lock")}) {
		t.Errorf("Glob = %v, want linkdir's file alone", matches)
	}
	if matches, _ := root.Glob(filepath.Join(filepath.Dir(fixture.root), "*", "pubspec.lock")); len(matches) != 0 {
		t.Errorf("Glob outside the root = %v", matches)
	}
}

// Verifies: REQ-LANG-031
func TestSourceConfinesPathsToTheRoot(t *testing.T) {
	fixture := newReadFixture(t)
	outside, err := filepath.Rel(fixture.root, fixture.outside)
	if err != nil {
		t.Fatal(err)
	}
	outside = filepath.ToSlash(outside) // ../secret.lock
	source := NewSource(fixture.root)
	for _, relative := range []string{outside, "..", "a/../../secret.lock", filepath.ToSlash(fixture.outside)} {
		if data, ok := source.Read(relative); ok || data != nil {
			t.Errorf("Read(%q) = %q, %v, want nothing", relative, data, ok)
		}
	}
	if data, ok := source.Read("a/pubspec.lock"); !ok || string(data) != "on disk" {
		t.Errorf("Read(a/pubspec.lock) = %q, %v, want the file on disk", data, ok)
	}
}

// A symbolic link under the root that points outside it is not followed: the
// scan never lists a link, and the files it leaves out are read through a Root.
//
// Verifies: REQ-LANG-031
func TestSourceRefusesASymbolicLinkOutOfTheRoot(t *testing.T) {
	fixture := newReadFixture(t)
	fixture.links(t)
	source := NewSource(fixture.root)
	for _, relative := range []string{"linked.lock", "relative-out.lock", "outdir/pubspec.lock"} {
		if data, ok := source.Read(relative); ok || data != nil {
			t.Errorf("Read(%s) = %q, %v, want nothing", relative, data, ok)
		}
	}
	for _, relative := range []string{"inside.lock", "absolute-inside.lock", "linkdir/pubspec.lock"} {
		if data, ok := source.Read(relative); !ok || string(data) != "on disk" {
			t.Errorf("Read(%s) = %q, %v, want the file it links to", relative, data, ok)
		}
	}
}

func TestSourcePrefersTheScannedCopy(t *testing.T) {
	fixture := newReadFixture(t)
	scanned := filepath.Join(t.TempDir(), "copy.lock")
	if err := os.WriteFile(scanned, []byte("scanned"), 0o644); err != nil {
		t.Fatal(err)
	}
	source := NewSource(fixture.root)
	source.Add(&scan.File{Path: "a/listed.lock", AbsolutePath: scanned})
	if data, ok := source.Read("a/listed.lock"); !ok || string(data) != "scanned" {
		t.Errorf("Read(a/listed.lock) = %q, %v, want the scanned copy", data, ok)
	}
	if !source.Listed("a/listed.lock") || source.Listed("a/pubspec.lock") {
		t.Error("Listed should hold a/listed.lock alone")
	}
	if a, ok := source.Absolute("a/listed.lock"); !ok || a != scanned {
		t.Errorf("Absolute(a/listed.lock) = %q, %v, want %q", a, ok, scanned)
	}
	if data, ok := source.Read("a/missing.lock"); ok || data != nil {
		t.Errorf("Read(a/missing.lock) = %q, %v, want nothing", data, ok)
	}
	// Without a root only the listed files are read.
	rootless := NewSource("")
	rootless.Add(&scan.File{Path: "a/listed.lock", AbsolutePath: scanned})
	if _, ok := rootless.Read("a/pubspec.lock"); ok {
		t.Error("a Source without a root read a file it does not list")
	}
	if data, ok := rootless.Read("a/listed.lock"); !ok || string(data) != "scanned" {
		t.Errorf("rootless Read(a/listed.lock) = %q, %v, want the scanned copy", data, ok)
	}
}

// Verifies: REQ-LANG-012
func TestSourceBoundsListedAndUnlistedFilesAlike(t *testing.T) {
	fixture := newReadFixture(t)
	large := filepath.Join(fixture.root, "large.lock")
	source := NewSource(fixture.root)
	source.Add(&scan.File{Path: "listed/large.lock", AbsolutePath: large})
	for _, relative := range []string{"large.lock", "listed/large.lock", "a"} {
		if data, ok := source.Read(relative); ok || data != nil {
			t.Errorf("Read(%s) = %d bytes, %v, want nothing", relative, len(data), ok)
		}
	}
	if _, ok := source.Read("limit.lock"); !ok {
		t.Error("Read(limit.lock) failed on a file of exactly MaxParseSize")
	}
}

// A directory from the environment: a relative one against the repository root and
// read through its Root, an absolute one elsewhere through Machine.
//
// Verifies: REQ-LANG-031
func TestFromEnvironment(t *testing.T) {
	root, elsewhere := t.TempDir(), filepath.Join(t.TempDir(), "cache")
	inside := filepath.Join(root, "x")
	for _, c := range []struct {
		value, want string
		repository  bool
	}{
		{"", "", false},
		{"cache", filepath.Join(root, "cache"), true},
		{filepath.Join(root, "cache"), filepath.Join(root, "cache"), true},
		{"../elsewhere", filepath.Join(filepath.Dir(root), "elsewhere"), false},
		{elsewhere, elsewhere, false},
	} {
		got, files := FromEnvironment(root, c.value)
		if got != c.want {
			t.Errorf("%q: %q, want %q", c.value, got, c.want)
		}
		if got != "" && files.Contains(inside) != c.repository {
			t.Errorf("%q: read through the repository's Root: %v, want %v", c.value, !c.repository, c.repository)
		}
	}
}
