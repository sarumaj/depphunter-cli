package cpp

import (
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/kballard/go-shellquote"

	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// maxCompileDB bounds the compilation database read; a large project's runs to tens
// of megabytes, and it is read on every analysis.
const maxCompileDB = 64 << 20

// compileDB is the include path of a compilation database (compile_commands.json),
// as project-relative directories.
type compileDB struct {
	byFile map[string][]string // source file -> its include directories, in order
	all    []string            // every include directory, for files without an entry (headers)
	rel    map[string]string   // absolute path -> project-relative, "\x00" when outside
}

// command is one entry of compile_commands.json.
type command struct {
	Directory string   `json:"directory"`
	File      string   `json:"file"`
	Arguments []string `json:"arguments"`
	Command   string   `json:"command"`
}

// readCompileDB reads the compilation databases at the root and in build*/ and
// cmake-build-*/ directories. Those are usually ignored by git and so missing from
// the scanned files, which is why they are looked for on disk.
//
// Implements: REQ-CPP-004
func readCompileDB(root string, all []*scan.File) *compileDB {
	db := &compileDB{byFile: map[string][]string{}, rel: map[string]string{}}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return db
	}
	if real, err := filepath.EvalSymlinks(absRoot); err == nil {
		absRoot = real
	}
	candidates := []string{filepath.Join(absRoot, "compile_commands.json")}
	for _, pattern := range []string{"build*", "cmake-build-*"} {
		dirs, _ := filepath.Glob(filepath.Join(absRoot, pattern, "compile_commands.json"))
		candidates = append(candidates, dirs...)
	}
	for _, f := range all {
		if path.Base(f.Path) == "compile_commands.json" && strings.Count(f.Path, "/") == 1 {
			candidates = append(candidates, filepath.Join(absRoot, filepath.FromSlash(f.Path)))
		}
	}
	seen := map[string]bool{}
	for _, c := range candidates {
		if seen[c] {
			continue
		}
		seen[c] = true
		st, err := os.Stat(c)
		if err != nil || !st.Mode().IsRegular() || st.Size() > maxCompileDB {
			continue
		}
		data, err := os.ReadFile(c)
		if err != nil {
			continue
		}
		var entries []command
		if json.Unmarshal(data, &entries) != nil {
			continue
		}
		for _, e := range entries {
			db.add(absRoot, filepath.Dir(c), e)
		}
	}
	return db
}

// add reads one entry. A relative "directory" (tools write absolute ones) is taken
// relative to the database's own directory.
func (db *compileDB) add(root, dbDir string, e command) {
	args := e.Arguments
	if len(args) == 0 {
		args, _ = shellquote.Split(e.Command)
	}
	dir := e.Directory
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(dbDir, dir)
	}
	rel := func(p string) (string, bool) {
		if !filepath.IsAbs(p) {
			p = filepath.Join(dir, p)
		}
		if r, ok := db.rel[p]; ok {
			return r, r != "\x00"
		}
		abs := p
		if real, err := filepath.EvalSymlinks(p); err == nil {
			p = real
		}
		r, err := filepath.Rel(root, p)
		if err != nil || r == ".." || strings.HasPrefix(r, ".."+string(filepath.Separator)) {
			db.rel[abs] = "\x00"
			return "", false // outside the project: a system or SDK directory
		}
		r = filepath.ToSlash(r)
		if r == "." {
			r = ""
		}
		db.rel[abs] = r
		return r, true
	}
	var dirs []string
	for _, d := range includeDirs(args) {
		if r, ok := rel(d); ok && !slices.Contains(dirs, r) {
			dirs = append(dirs, r)
		}
	}
	if file, ok := rel(e.File); ok && e.File != "" {
		db.byFile[file] = dirs
	}
	for _, d := range dirs {
		if !slices.Contains(db.all, d) {
			db.all = append(db.all, d)
		}
	}
}

// dirs is the include path of a file: its own entry's, or every entry's.
func (db *compileDB) dirs(file string) []string {
	if d, ok := db.byFile[file]; ok {
		return d
	}
	return db.all
}

// includeDirs reads the include-path options of a compiler command line: GCC and
// Clang's -I, -iquote, -isystem and -idirafter, and MSVC's /I and /external:I,
// joined to their value or followed by it.
func includeDirs(args []string) []string {
	flags := []string{"-I", "-iquote", "-isystem", "-idirafter", "--include-directory="}
	if len(args) > 0 {
		// Only MSVC's command line has /-options; elsewhere /I… is a path.
		switch strings.ToLower(strings.TrimSuffix(path.Base(strings.ReplaceAll(args[0], `\`, "/")), ".exe")) {
		case "cl", "clang-cl":
			flags = append(flags, "/I", "-external:I", "/external:I")
		}
	}
	var out []string
	for i := 1; i < len(args); i++ {
		a := args[i]
		for _, flag := range flags {
			v, ok := strings.CutPrefix(a, flag)
			if !ok {
				continue
			}
			if v == "" && i+1 < len(args) {
				i++
				v = args[i]
			}
			if v != "" {
				out = append(out, v)
			}
			break
		}
	}
	return out
}
