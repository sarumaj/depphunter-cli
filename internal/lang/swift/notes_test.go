package swift

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
)

// Package.resolved is flat: without SwiftPM's checkouts the walk has nothing past
// it, which is noted, as is one read from disk that the scan left out.
//
// Verifies: REQ-SWIFT-009, REQ-SWIFT-012, REQ-TRC-017
func TestResolvedNotes(t *testing.T) {
	resolved := `{"pins":[{"identity":"swift-log","kind":"remoteSourceControl",` +
		`"location":"https://github.com/apple/swift-log.git","state":{"revision":"abc","version":"1.5.3"}}],"version":2}`
	root := langtest.Write(t, map[string]string{
		"Package.swift": `// swift-tools-version:5.9
import PackageDescription
let package = Package(name: "app",
  dependencies: [.package(url: "https://github.com/apple/swift-log.git", from: "1.5.0")],
  targets: [.target(name: "App", dependencies: [.product(name: "Logging", package: "swift-log")])])
`,
		"Package.resolved":      resolved,
		"Sources/App/App.swift": "import Logging\n",
	})
	files := langtest.Files(t, root)
	if got, want := langtest.Notes(newResolver(root, files)), []string{"Package.resolved lock-flat"}; !slices.Equal(got, want) {
		t.Errorf("notes %q, want %q", got, want)
	}
	got := langtest.Notes(newResolver(root, langtest.Without(files, "Package.resolved")))
	if want := []string{"Package.resolved lock-flat", "Package.resolved lock-ignored"}; !slices.Equal(got, want) {
		t.Errorf("ignored: notes %q, want %q", got, want)
	}
	// With the checkouts on disk the walk reads edges from them: nothing to say.
	if err := os.MkdirAll(filepath.Join(root, ".build", "checkouts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := langtest.Notes(newResolver(root, files)); len(got) != 0 {
		t.Errorf("with checkouts: notes %q", got)
	}
}
