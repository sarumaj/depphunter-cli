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
	byFile   map[string][]string // source file -> its include directories, in order
	all      []string            // every include directory, for files without an entry (headers)
	relative map[string]string   // absolute path -> project-relative, "\x00" when outside
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
	database := &compileDB{byFile: map[string][]string{}, relative: map[string]string{}}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return database
	}
	if real, err := filepath.EvalSymlinks(absoluteRoot); err == nil {
		absoluteRoot = real
	}
	candidates := []string{filepath.Join(absoluteRoot, "compile_commands.json")}
	for _, pattern := range []string{"build*", "cmake-build-*"} {
		directories, _ := filepath.Glob(filepath.Join(absoluteRoot, pattern, "compile_commands.json"))
		candidates = append(candidates, directories...)
	}
	for _, f := range all {
		if path.Base(f.Path) == "compile_commands.json" && strings.Count(f.Path, "/") == 1 {
			candidates = append(candidates, filepath.Join(absoluteRoot, filepath.FromSlash(f.Path)))
		}
	}
	seen := map[string]bool{}
	for _, c := range candidates {
		if seen[c] {
			continue
		}
		seen[c] = true
		fileInfo, err := os.Stat(c)
		if err != nil || !fileInfo.Mode().IsRegular() || fileInfo.Size() > maxCompileDB {
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
			database.add(absoluteRoot, filepath.Dir(c), e)
		}
	}
	return database
}

// add reads one entry. A relative "directory" (tools write absolute ones) is taken
// relative to the database's own directory.
func (database *compileDB) add(root, databaseDirectory string, e command) {
	arguments := e.Arguments
	if len(arguments) == 0 {
		arguments, _ = shellquote.Split(e.Command)
	}
	directory := e.Directory
	if !filepath.IsAbs(directory) {
		directory = filepath.Join(databaseDirectory, directory)
	}
	relative := func(p string) (string, bool) {
		if !filepath.IsAbs(p) {
			p = filepath.Join(directory, p)
		}
		if r, ok := database.relative[p]; ok {
			return r, r != "\x00"
		}
		absolute := p
		if real, err := filepath.EvalSymlinks(p); err == nil {
			p = real
		}
		r, err := filepath.Rel(root, p)
		if err != nil || r == ".." || strings.HasPrefix(r, ".."+string(filepath.Separator)) {
			database.relative[absolute] = "\x00"
			return "", false // outside the project: a system or SDK directory
		}
		r = filepath.ToSlash(r)
		if r == "." {
			r = ""
		}
		database.relative[absolute] = r
		return r, true
	}
	var directories []string
	for _, d := range includeDirectories(arguments) {
		if r, ok := relative(d); ok && !slices.Contains(directories, r) {
			directories = append(directories, r)
		}
	}
	if file, ok := relative(e.File); ok && e.File != "" {
		database.byFile[file] = directories
	}
	for _, d := range directories {
		if !slices.Contains(database.all, d) {
			database.all = append(database.all, d)
		}
	}
}

// directories is the include path of a file: its own entry's, or every entry's.
func (database *compileDB) directories(file string) []string {
	if d, ok := database.byFile[file]; ok {
		return d
	}
	return database.all
}

// includeDirectories reads the include-path options of a compiler command line: GCC and
// Clang's -I, -iquote, -isystem and -idirafter, and MSVC's /I and /external:I,
// joined to their value or followed by it.
func includeDirectories(arguments []string) []string {
	flags := []string{"-I", "-iquote", "-isystem", "-idirafter", "--include-directory="}
	if len(arguments) > 0 {
		// Only MSVC's command line has /-options; elsewhere /I… is a path.
		switch strings.ToLower(strings.TrimSuffix(path.Base(strings.ReplaceAll(arguments[0], `\`, "/")), ".exe")) {
		case "cl", "clang-cl":
			flags = append(flags, "/I", "-external:I", "/external:I")
		}
	}
	var out []string
	for i := 1; i < len(arguments); i++ {
		a := arguments[i]
		for _, flag := range flags {
			v, ok := strings.CutPrefix(a, flag)
			if !ok {
				continue
			}
			if v == "" && i+1 < len(arguments) {
				i++
				v = arguments[i]
			}
			if v != "" {
				out = append(out, v)
			}
			break
		}
	}
	return out
}
