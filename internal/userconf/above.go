package userconf

import (
	"os"
	"path/filepath"
)

// DirectoriesAbove splits the directories above Directory, closest first, in two.
// The first are those of the checkout Directory belongs to: up to the nearest
// directory above it holding a .git, when Directory itself holds none. They are
// the repository's, only not analyzed, and what they hold is the repository's to
// say. The others lie outside the checkout, and what they hold is this machine's
// configuration - the files Yarn and NuGet look for in every directory up to the
// file system's root. Without a .git anywhere, every directory above Directory is
// outside. Without a Directory there are none.
//
// Implements: REQ-SUP-079
func (m Machine) DirectoriesAbove() (repository, outside []string) {
	if m.Directory == "" {
		return nil, nil
	}
	directory := filepath.Clean(m.Directory)
	var above []string
	for d := directory; filepath.Dir(d) != d; {
		d = filepath.Dir(d)
		above = append(above, d)
	}
	top := -1
	if !exists(filepath.Join(directory, ".git")) {
		for i, d := range above {
			if exists(filepath.Join(d, ".git")) {
				top = i
				break
			}
		}
	}
	if top < 0 {
		return nil, above
	}
	return above[:top+1], above[top+1:]
}

// exists reports whether a file or directory is at path; a checkout's .git is a
// directory, or a file in a worktree or a submodule.
func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
