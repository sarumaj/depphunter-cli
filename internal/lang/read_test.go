package lang

import (
	"bytes"
	"os"
	"path/filepath"
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
func TestReadCappedDropsWhatIsTooLarge(t *testing.T) {
	fixture := newReadFixture(t)
	for _, testCase := range []struct {
		name string
		ok   bool
	}{
		{"limit.lock", true},
		{"large.lock", false},
		{"a", false},
		{"missing.lock", false},
	} {
		data, ok := ReadCapped(filepath.Join(fixture.root, testCase.name))
		if ok != testCase.ok || !ok && data != nil {
			t.Errorf("ReadCapped(%s) = %d bytes, %v, want ok %v", testCase.name, len(data), ok, testCase.ok)
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

func TestSourceConfinesPathsToTheRoot(t *testing.T) {
	fixture := newReadFixture(t)
	outside, err := filepath.Rel(fixture.root, fixture.outside)
	if err != nil {
		t.Fatal(err)
	}
	outside = filepath.ToSlash(outside) // ../secret.lock
	confined := NewSource(fixture.root, SourceOptions{Confined: true})
	for _, relative := range []string{outside, "..", filepath.ToSlash(fixture.outside)} {
		if data, ok := confined.Read(relative); ok || data != nil {
			t.Errorf("confined Read(%q) = %q, %v, want nothing", relative, data, ok)
		}
	}
	if data, ok := confined.Read("a/pubspec.lock"); !ok || string(data) != "on disk" {
		t.Errorf("confined Read(a/pubspec.lock) = %q, %v, want the file on disk", data, ok)
	}
	// Without Confined, the resolver's own check is all there is: a path that
	// climbs out is read. An absolute path is still joined under the root.
	unconfined := NewSource(fixture.root, SourceOptions{})
	if data, ok := unconfined.Read(outside); !ok || string(data) != "outside" {
		t.Errorf("unconfined Read(%q) = %q, %v, want the file outside", outside, data, ok)
	}
	if data, ok := unconfined.Read(filepath.ToSlash(fixture.outside)); ok {
		t.Errorf("unconfined Read(%q) = %q, want nothing: it is looked for under the root", fixture.outside, data)
	}
}

// The confinement is on the path alone: a symbolic link under the root that
// points outside it is followed, as the resolvers always have for the files
// the scan leaves out (the scan itself never lists a link).
func TestSourceFollowsASymbolicLinkUnderTheRoot(t *testing.T) {
	fixture := newReadFixture(t)
	if err := os.Symlink(fixture.outside, filepath.Join(fixture.root, "linked.lock")); err != nil {
		t.Skipf("no symbolic links here: %v", err)
	}
	source := NewSource(fixture.root, SourceOptions{Confined: true, Bounded: true})
	if data, ok := source.Read("linked.lock"); !ok || string(data) != "outside" {
		t.Errorf("Read(linked.lock) = %q, %v, want the target's content", data, ok)
	}
}

func TestSourcePrefersTheScannedCopy(t *testing.T) {
	fixture := newReadFixture(t)
	scanned := filepath.Join(t.TempDir(), "copy.lock")
	if err := os.WriteFile(scanned, []byte("scanned"), 0o644); err != nil {
		t.Fatal(err)
	}
	source := NewSource(fixture.root, SourceOptions{Confined: true})
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
	rootless := NewSource("", SourceOptions{})
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
	bounded := NewSource(fixture.root, SourceOptions{Bounded: true})
	bounded.Add(&scan.File{Path: "listed/large.lock", AbsolutePath: large})
	for _, relative := range []string{"large.lock", "listed/large.lock", "a"} {
		if data, ok := bounded.Read(relative); ok || data != nil {
			t.Errorf("bounded Read(%s) = %d bytes, %v, want nothing", relative, len(data), ok)
		}
	}
	if _, ok := bounded.Read("limit.lock"); !ok {
		t.Error("bounded Read(limit.lock) failed on a file of exactly MaxParseSize")
	}
	// Without Bounded a Source reads whole, as the resolvers that use it did.
	if data, ok := NewSource(fixture.root, SourceOptions{}).Read("large.lock"); !ok || len(data) != MaxParseSize+1 {
		t.Errorf("unbounded Read(large.lock) = %d bytes, %v, want the whole file", len(data), ok)
	}
}
