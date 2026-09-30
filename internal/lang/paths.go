package lang

import (
	"os"
	"path"
	"strings"
)

// The helpers below work on the slash-separated paths relative to the
// repository root that scan.File.Path holds, where "." is the root itself.

// Depth is how many directories deep a directory is, "." being 0, so that
// resolvers can let the project nearest a file win over the ones around it.
func Depth(directory string) int {
	if directory == "." {
		return 0
	}
	return strings.Count(directory, "/") + 1
}

// DeepestFirst orders directories deepest first, then by name, so that the
// first one around a file is the nearest and ties do not depend on map order.
func DeepestFirst(a, b string) bool {
	if depthA, depthB := Depth(a), Depth(b); depthA != depthB {
		return depthA > depthB
	}
	return a < b
}

// Within reports whether file is below directory; "." holds everything.
// A directory is not within itself: see WithinOrEqual.
func Within(file, directory string) bool {
	return directory == "." || strings.HasPrefix(file, directory+"/")
}

// WithinOrEqual is Within for resolvers that match directories against
// directories, where the directory itself counts as inside.
func WithinOrEqual(p, directory string) bool {
	return directory == "." || p == directory || strings.HasPrefix(p, directory+"/")
}

// Inside reports whether a cleaned relative path stays in the repository. A
// path that a manifest or an import names can climb out with ".." or be
// absolute, and a resolver must not read or link anything outside the root.
func Inside(p string) bool {
	return p != ".." && !strings.HasPrefix(p, "../") && !path.IsAbs(p)
}

// CommonSegments counts the leading slash-separated segments two paths share.
func CommonSegments(a, b string) int {
	aParts, bParts := strings.Split(a, "/"), strings.Split(b, "/")
	n := 0
	for n < len(aParts) && n < len(bParts) && aParts[n] == bParts[n] {
		n++
	}
	return n
}

// CommonDirectories counts the directories two files share, so that among
// several candidates the one closest to an importing file can win. Two files
// at the top level share ".", which counts as one.
func CommonDirectories(a, b string) int {
	return CommonSegments(path.Dir(a), path.Dir(b))
}

// CommonSubdirectories is CommonDirectories without counting the top level:
// two files there share nothing, so a candidate at the root does not beat
// one in a subdirectory for an importer at the root.
func CommonSubdirectories(a, b string) int {
	if path.Dir(a) == "." {
		return 0
	}
	return CommonDirectories(a, b)
}

// IsDirectory reports whether an absolute path is a directory on disk, for
// the installed trees and caches that the scan does not list.
func IsDirectory(absolute string) bool {
	info, err := os.Stat(absolute)
	return err == nil && info.IsDir()
}
