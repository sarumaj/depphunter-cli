// Package scan enumerates the files of a project and measures them.
package scan

import (
	"bufio"
	"bytes"
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"golang.org/x/sync/errgroup"
)

// File is a project file with paths relative to the project root, always slash-separated.
//
// Implements: REQ-LANG-020
type File struct {
	Path   string
	Abs    string
	Lang   string
	LOC    int
	Binary bool
	// Size is the file's size in bytes when it was measured, so that a file too
	// large to be worth reading can be passed over without being read.
	Size int64
	// TooLarge says the file is over Options.MaxFileSize, which promises it is not
	// read: not measured here, and not parsed by any plugin.
	TooLarge bool
	// Interpreter is the program a script's "#!" line runs ("bash", "python3"), read
	// through /usr/bin/env; "" when the file has no such line.
	Interpreter string
}

type Options struct {
	Exclude     []string // glob patterns matched against the relative path and each of its segments
	MaxFileSize int64    // files larger than this are listed but not read
}

// defaultIgnore applies when git is unavailable, so a plain walk does not descend into
// dependency caches and build output.
//
// Implements: REQ-LANG-018
var defaultIgnore = map[string]bool{
	".git": true, ".hg": true, ".svn": true, "node_modules": true, "vendor": true,
	"dist": true, "build": true, "target": true, "bin": true, "obj": true,
	".venv": true, "venv": true, "__pycache__": true, ".idea": true, ".vscode": true,
	".next": true, ".cache": true, ".gradle": true, ".tox": true, ".mypy_cache": true,
	".build": true, ".dart_tool": true, "_build": true, "dist-newstyle": true, ".stack-work": true,
	".terraform": true, ".terragrunt-cache": true, "lua_modules": true, "_opam": true,
	".zig-cache": true, "zig-cache": true, "zig-out": true, "zig-pkg": true,
	".cpcache": true, ".shadow-cljs": true, "elm-stuff": true, ".spago": true, "bower_components": true,
	".crystal": true, ".fake": true, ".dub": true, ".haxelib": true, "compiled": true, ".qlot": true,
	"nimbledeps": true, "nimcache": true,
}

func Scan(ctx context.Context, root string, opts Options) ([]*File, error) {
	paths, err := gitFiles(ctx, root)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if paths, err = walkFiles(ctx, root); err != nil {
			return nil, err
		}
	}

	files := make([]*File, 0, len(paths))
	for _, p := range paths {
		if !excluded(p, opts.Exclude) {
			files = append(files, &File{Path: p, Abs: filepath.Join(root, filepath.FromSlash(p)), Lang: Language(p)})
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })

	// Implements: REQ-DIST-016
	var g errgroup.Group
	g.SetLimit(runtime.NumCPU())
	for _, f := range files {
		if ctx.Err() != nil {
			break
		}
		g.Go(func() error { measure(f, opts.MaxFileSize); return nil })
	}
	g.Wait()
	return files, ctx.Err()
}

// gitFiles lists tracked and untracked-but-not-ignored files, which honours every
// .gitignore, .git/info/exclude and the global excludes file for free.
//
// Duplicates - a file with merge conflicts is listed once per stage - are dropped
// here rather than with --deduplicate, which needs git 2.31; an older git refuses
// the option, and the walk it would fall back to knows none of the ignore files.
//
// Implements: REQ-LANG-017
func gitFiles(ctx context.Context, root string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", root, "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var paths []string
	seen := map[string]bool{}
	for _, p := range bytes.Split(out, []byte{0}) {
		if len(p) == 0 || seen[string(p)] {
			continue
		}
		seen[string(p)] = true
		// Deleted-but-tracked files and submodule entries are listed but are not regular
		// files. Nor is a symbolic link, deliberately (Lstat): a committed link may point
		// anywhere on this machine, and a file node's content is served by /api/file and
		// written into the HTML export. walkFiles skips links too.
		if st, err := os.Lstat(filepath.Join(root, string(p))); err == nil && st.Mode().IsRegular() {
			paths = append(paths, string(p))
		}
	}
	return paths, nil
}

// Implements: REQ-LANG-018
func walkFiles(ctx context.Context, root string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			// One directory that cannot be read - a volume owned by another user, say -
			// is left out rather than failing the scan of everything else.
			if p != root {
				if d != nil && d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			return err
		}
		if d.IsDir() {
			if p != root && (defaultIgnore[d.Name()] || besideManifest(d.Name(), filepath.Dir(p))) {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type().IsRegular() {
			rel, _ := filepath.Rel(root, p)
			paths = append(paths, filepath.ToSlash(rel))
		}
		return nil
	})
	return paths, err
}

// generatedBeside names directories that hold what a tool wrote only when its
// manifest sits next to them: the PureScript compiler's output/ beside a
// spago.yaml or spago.dhall, the shards shards installs into lib/ beside a
// shard.yml, Paket's packages/ and paket-files/ beside a paket.dependencies,
// Alire's alire/ beside an alire.toml, ocicl's systems/ beside an
// ocicl.csv, Foundry's lib/, dependencies/ (Soldeer's), out/ and cache/
// beside a foundry.toml, and Hardhat's artifacts/, cache/ and
// typechain-types/ beside a hardhat.config.*, and the packages Atlas clones
// into deps/ beside a .nimble file or an Atlas configuration (a marker with
// a wildcard is a glob), and CUE's dependency trees pkg/, gen/ and usr/ in a
// cue.mod directory (beside its module.cue).
// Elsewhere an output/, lib/ or packages/ directory may well be source.
var generatedBeside = map[string][]string{
	"output": {"spago.yaml", "spago.dhall"},
	"lib":    {"shard.yml", "foundry.toml"},
	// Paket installs packages into packages/ and fetches remote files into
	// paket-files/ beside paket.dependencies.
	"packages":    {"paket.dependencies"},
	"paket-files": {"paket.dependencies"},
	// Alire keeps its lock file, build cache and the crates it fetches in
	// alire/ beside alire.toml.
	"alire": {"alire.toml"},
	// ocicl downloads the systems it installs into systems/ beside ocicl.csv.
	"systems": {"ocicl.csv"},
	// Foundry installs libraries into lib/ (git submodules) and Soldeer its
	// dependencies into dependencies/; forge builds into out/ and cache/.
	// Hardhat compiles into artifacts/ and cache/ and TypeChain writes
	// typechain-types/.
	"dependencies":    {"foundry.toml"},
	"out":             {"foundry.toml"},
	"cache":           {"foundry.toml", "hardhat.config.js", "hardhat.config.ts", "hardhat.config.cjs", "hardhat.config.mjs", "hardhat.config.cts", "hardhat.config.mts"},
	"artifacts":       {"hardhat.config.js", "hardhat.config.ts", "hardhat.config.cjs", "hardhat.config.mjs", "hardhat.config.cts", "hardhat.config.mts"},
	"typechain-types": {"hardhat.config.js", "hardhat.config.ts", "hardhat.config.cjs", "hardhat.config.mjs", "hardhat.config.cts", "hardhat.config.mts"},
	// Atlas clones a Nim project's dependencies into deps/ (its atlas.config
	// in the project or in deps/).
	"deps": {"*.nimble", "atlas.config", "atlas.workspace", "deps/atlas.config"},
	// CUE vendors modules into cue.mod/pkg, `cue get go` generates into
	// cue.mod/gen, and cue.mod/usr holds what augments them.
	"pkg": {"module.cue"},
	"gen": {"module.cue"},
	"usr": {"module.cue"},
}

// besideManifest reports whether the directory name in dir is such a directory.
//
// Implements: REQ-LANG-018
func besideManifest(name, dir string) bool {
	for _, m := range generatedBeside[name] {
		if strings.Contains(m, "*") {
			entries, _ := os.ReadDir(dir)
			for _, e := range entries {
				if ok, _ := path.Match(m, e.Name()); ok {
					return true
				}
			}
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, m)); err == nil {
			return true
		}
	}
	return false
}

// Implements: REQ-LANG-019
func excluded(rel string, patterns []string) bool {
	for _, pat := range patterns {
		if ok, _ := path.Match(pat, rel); ok {
			return true
		}
		for _, seg := range strings.Split(rel, "/") {
			if ok, _ := path.Match(pat, seg); ok {
				return true
			}
		}
	}
	return false
}

// Implements: REQ-LANG-016, REQ-LANG-020, REQ-LANG-021, REQ-LANG-022
func measure(f *File, maxSize int64) {
	st, err := os.Stat(f.Abs)
	if err != nil {
		return
	}
	f.Size = st.Size()
	if maxSize > 0 && f.Size > maxSize {
		f.TooLarge = true
		return
	}
	fh, err := os.Open(f.Abs)
	if err != nil {
		return
	}
	defer fh.Close()

	r := bufio.NewReader(fh)
	head, _ := r.Peek(8000)
	if bytes.IndexByte(head, 0) >= 0 {
		f.Binary = true
		return
	}
	// Qt Linguist keeps its translations in .ts files too: XML, which the
	// TypeScript grammar can only fail on, slowly (REQ-LANG-011's bound each).
	if f.Lang == "TypeScript" && strings.EqualFold(path.Ext(f.Path), ".ts") && xmlDocument(head) {
		f.Lang = "XML"
	}
	f.Interpreter = interpreter(head)
	// ".m" is MATLAB's and Mercury's too, and ".h" C's: Objective-C says which by
	// its keywords and directives, within the head already read. ".pl" is
	// Prolog's too, and ".t" is Perl's only by convention: Perl says which by its
	// #! line and statements. ".fs" is F#'s, a GLSL fragment shader's and Forth's.
	// ".d" is D's, a make dependency file's and a DTrace script's. ".f" and ".for"
	// are fixed-form Fortran's and sometimes Forth's. ".scm" and ".ss" are
	// Scheme's, and Racket's when a #lang line starts them. ".cl" is Common
	// Lisp's and OpenCL's.
	switch ext := strings.ToLower(path.Ext(f.Path)); {
	case ext == ".m" && f.Lang == "Objective-C" && !objcMarker(head, true):
		f.Lang = notObjC(head)
	case ext == ".h" && f.Lang == "C" && objcMarker(head, false):
		f.Lang = "Objective-C"
	case ext == ".pl" && f.Lang == "Perl" && !PerlInterpreter(f.Interpreter) && !perlMarker(head) && prologClause(head):
		f.Lang = "Prolog"
	case ext == ".t" && f.Lang == "Perl" && !PerlInterpreter(f.Interpreter) && !perlMarker(head):
		f.Lang = ""
	case ext == ".fs" && f.Lang == "F#" && glslSource(head):
		f.Lang = "GLSL"
	case ext == ".fs" && f.Lang == "F#" && forthSource(head):
		f.Lang = "Forth"
	case (ext == ".f" || ext == ".for") && f.Lang == "Fortran" && forthSource(head):
		f.Lang = "Forth"
	case ext == ".d" && f.Lang == "D" && dependencyFile(head):
		f.Lang = "Make"
	case ext == ".d" && f.Lang == "D" && (f.Interpreter == "dtrace" || dtraceSource(head)):
		f.Lang = "DTrace"
	case (ext == ".scm" || ext == ".ss") && f.Lang == "Scheme" && racketSource(head):
		f.Lang = "Racket"
	case ext == ".cl" && f.Lang == "Common Lisp" && openclSource(head):
		f.Lang = "OpenCL"
	}
	// A script without a language is labelled by the shell or perl its "#!" line
	// runs.
	switch {
	case f.Lang == "" && ShellInterpreter(f.Interpreter):
		f.Lang = "Shell"
	case f.Lang == "" && PerlInterpreter(f.Interpreter):
		f.Lang = "Perl"
	case f.Lang == "" && f.Interpreter == "bb":
		f.Lang = "Clojure" // a babashka script
	case f.Lang == "" && f.Interpreter == "racket":
		f.Lang = "Racket"
	}
	var lines, last int
	buf := make([]byte, 32*1024)
	for {
		n, err := r.Read(buf)
		lines += bytes.Count(buf[:n], []byte{'\n'})
		if n > 0 {
			last = int(buf[n-1])
		}
		if err != nil {
			break
		}
	}
	if last != 0 && last != '\n' {
		lines++ // final line without trailing newline
	}
	f.LOC = lines
}
