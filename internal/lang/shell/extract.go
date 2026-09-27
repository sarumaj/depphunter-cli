package shell

import (
	"path"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// A path is evaluated into a string that may start with one of these markers, for the
// directory it is relative to. The markers never appear in a file name.
const (
	mDir  = "\x01" // the script's own directory
	mRoot = "\x02" // the repository root (git rev-parse --show-toplevel)
	mSelf = "\x03" // the script itself ($0, ${BASH_SOURCE[0]})
	mCwd  = "\x04" // the working directory ($PWD, $(pwd))
	mAny  = "\x05" // a variable set outside the file ($PLUGIN_PATH/x/functions)
)

const markers = mDir + mRoot + mSelf + mCwd + mAny

// extractor turns the parser's commands into imports and symbols, in source order,
// keeping what variables hold so that "$SCRIPT_DIR/lib.sh" can be read.
type extractor struct {
	ext     string
	vars    map[string]string // evaluated values; a variable whose value is unknown is absent
	symbols lang.SymbolSet
	defined map[string]bool
	imports []lang.RawImport
	specs   map[string]bool
}

func (x *extractor) function(name, kind string, line int) {
	if !x.defined[name] {
		x.defined[name] = true
		x.symbols.Add(name, kind, line)
	}
}

func (x *extractor) symbol(name, kind string, line int) {
	if name != "" && !x.defined[name] {
		x.defined[name] = true
		x.symbols.Add(name, kind, line)
	}
}

// add records an import once per file and spec.
func (x *extractor) add(spec, module, kind string, line int) {
	key := spec + "\x00" + module + "\x00" + kind
	if module == "" || x.specs[key] {
		return
	}
	x.specs[key] = true
	x.imports = append(x.imports, lang.RawImport{Spec: spec, Module: module, Name: kind, Line: line})
}

// extract reads a shell script. ext is the file's lower-cased extension, which
// decides whether direnv's and bats' commands are read.
func extract(src []byte, ext string) *lang.Extraction {
	src = trimBOM(src)
	x := &extractor{ext: ext, vars: map[string]string{}, defined: map[string]bool{}, specs: map[string]bool{}}
	newParser(src, 1, x).script()
	return &lang.Extraction{Imports: x.imports, Symbols: x.symbols.List()}
}

func trimBOM(src []byte) []byte {
	if len(src) >= 3 && src[0] == 0xEF && src[1] == 0xBB && src[2] == 0xBF {
		return src[3:]
	}
	return src
}

// raw joins words as written, for an import's spec.
func raw(words []*word) string {
	var parts []string
	for _, w := range words {
		parts = append(parts, w.raw)
	}
	return strings.Join(parts, " ")
}

// name is a command word's plain text, "" when it has expansions.
func name(w *word) string {
	s, _ := w.text()
	return s
}

func upper(s string) bool {
	letter := false
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c >= 'A' && c <= 'Z':
			letter = true
		case c == '_' || c >= '0' && c <= '9':
		default:
			return false
		}
	}
	return letter
}

// command handles one simple command.
//
// Implements: REQ-SHELL-003, REQ-SHELL-004, REQ-SHELL-005, REQ-SHELL-006, REQ-SHELL-010
func (x *extractor) command(c *command) {
	top := !c.inFunc && !c.sub
	if len(c.words) == 0 {
		for _, a := range c.assigns {
			n, val, _ := splitAssign(a)
			x.assign(n, val)
			if top && upper(n) && n != "IFS" {
				x.symbol(n, "var", a.line)
			}
		}
		return
	}
	words := x.unwrap(c.words)
	if len(words) == 0 {
		return
	}
	cmd := name(words[0])
	args := words[1:]
	switch {
	case cmd == "source" || cmd == ".":
		if len(args) > 0 {
			x.path(raw(words[:2]), args[0], kindSource, c.line)
		}
		return
	case cmd == "load" && x.ext == ".bats":
		if len(args) > 0 {
			x.path(raw(words[:2]), args[0], kindBats, c.line)
		}
		return
	case x.ext == ".envrc" && x.direnv(cmd, words, c.line):
		return
	case cmd == "alias":
		for _, a := range args {
			if s, ok := a.text(); ok && !strings.HasPrefix(s, "-") {
				if n, _, ok := strings.Cut(s, "="); ok && !c.sub {
					x.symbol(n, "alias", a.line)
				}
			}
		}
		return
	case cmd == "export" || cmd == "readonly" || cmd == "declare" || cmd == "typeset" || cmd == "local":
		x.declare(cmd, args, top)
		return
	}
	if x.install(words, c.line) {
		return
	}
	x.invoke(words, c.line)
}

// assign records what a variable now holds.
func (x *extractor) assign(n string, val *word) {
	if v, ok := x.eval(val); ok {
		x.vars[n] = v
	} else {
		delete(x.vars, n)
	}
}

// declare handles export, readonly, declare, typeset and local: their assignments
// are tracked, and at the top level of the script exported variables, read-only
// ones and upper-case ones become symbols.
//
// Implements: REQ-SHELL-003
func (x *extractor) declare(cmd string, args []*word, top bool) {
	flags := ""
	for _, a := range args {
		s, _ := a.text()
		if strings.HasPrefix(s, "-") || strings.HasPrefix(s, "+") {
			if strings.HasPrefix(s, "-") {
				flags += s[1:]
			}
			continue
		}
		if strings.ContainsAny(flags, "fF") {
			return // declare -f: functions, not variables
		}
		n, val, ok := splitAssign(a)
		if ok {
			x.assign(n, val)
		} else if n = s; !validName(n) {
			continue
		}
		if !top || cmd == "local" {
			continue
		}
		switch {
		case cmd == "readonly" || strings.ContainsRune(flags, 'r'):
			x.symbol(n, "const", a.line)
		case cmd == "export" || strings.ContainsRune(flags, 'x'):
			x.symbol(n, "var", a.line)
		case ok && upper(n) && n != "IFS":
			x.symbol(n, "var", a.line)
		}
	}
}

// unwrap drops the commands that run another command - exec, command, env, sudo,
// nohup, nice, timeout, and bats' run - with their options, leaving the command
// they run.
func (x *extractor) unwrap(words []*word) []*word {
	for len(words) > 0 {
		n := name(words[0])
		if strings.HasPrefix(n, "/") {
			n = path.Base(n)
		}
		var valued map[string]bool
		switch n {
		case "exec":
			valued = map[string]bool{"-a": true}
		case "command", "builtin":
			if len(words) > 1 {
				if f := name(words[1]); f == "-v" || f == "-V" {
					return nil // command -v x only asks whether x exists
				}
			}
		case "env":
			valued = map[string]bool{"-u": true, "--unset": true, "-C": true, "--chdir": true, "-S": true}
		case "sudo", "doas":
			valued = map[string]bool{"-u": true, "-g": true, "-C": true, "-D": true, "-h": true, "-p": true, "-r": true, "-t": true, "-U": true}
		case "nohup", "time", "stdbuf":
		case "run": // bats: run [-N] [!] command
			if x.ext != ".bats" {
				return words
			}
			if len(words) > 1 && name(words[1]) == "!" {
				words = words[1:]
			}
		case "nice":
			valued = map[string]bool{"-n": true}
		case "timeout":
			valued = map[string]bool{"-s": true, "--signal": true, "-k": true, "--kill-after": true}
		default:
			return words
		}
		words = words[1:]
		for len(words) > 0 {
			s := name(words[0])
			if s == "--" {
				words = words[1:]
				break
			}
			if strings.HasPrefix(s, "-") {
				words = words[1:]
				if valued[s] && len(words) > 0 {
					words = words[1:]
				}
				continue
			}
			if n == "env" && isAssign(words[0]) {
				words = words[1:]
				continue
			}
			if n == "timeout" { // the duration
				words = words[1:]
			}
			break
		}
	}
	return words
}

// interpreters run the script named by their first argument that is not an option.
// valued are the options that take a value; stop the ones after which the script
// comes from elsewhere (a string, a module, standard input).
var interpreters = map[string]struct{ valued, stop string }{
	"sh": {"-o +o -O +O", "-c -s"}, "bash": {"-o +o -O +O --rcfile --init-file", "-c -s"},
	"zsh": {"-o +o", "-c -s"}, "dash": {"-o +o", "-c -s"}, "ksh": {"-o +o", "-c -s"},
	"mksh": {"-o +o", "-c -s"}, "ash": {"-o +o", "-c -s"}, "bats": {"-f --filter -j --jobs -F --formatter -o --output --filter-tags --filter-status --report-formatter --gather-test-outputs-in --line-reference-format --code-quote-style --setup-suite-file", ""},
	"python": {"-W -X", "-c -m -"}, "python2": {"-W -X", "-c -m -"}, "python3": {"-W -X", "-c -m -"},
	"node": {"-r --require --import --loader", "-e --eval -p --print -"}, "ruby": {"-I -r -C", "-e -"},
	"perl": {"-I -M", "-e -E -"}, "php": {"-c -d", "-r"}, "Rscript": {"", "-e"},
	"pwsh": {"-ExecutionPolicy -WorkingDirectory", "-Command -c -EncodedCommand"},
}

// interpreter is the interpreter a command name runs, "" for none.
func interpreter(n string) string {
	if strings.HasPrefix(n, "/") {
		n = path.Base(n)
	}
	if _, ok := interpreters[n]; ok {
		return n
	}
	if strings.HasPrefix(n, "python3.") {
		return "python3"
	}
	return ""
}

// invoke records a script the command runs: the command itself when it is a path
// (./build.sh, "$DIR/x.sh"), or the script an interpreter is given (bash x.sh,
// python3 tools/gen.py).
//
// Implements: REQ-SHELL-005
func (x *extractor) invoke(words []*word, line int) {
	if in := interpreter(name(words[0])); in != "" {
		opts := interpreters[in]
		valued := strings.Fields(opts.valued)
		stop := strings.Fields(opts.stop)
		for i := 1; i < len(words); i++ {
			s := name(words[i])
			switch {
			case contains(stop, s):
				return
			case contains(valued, s):
				i++
			case strings.HasPrefix(s, "-") || strings.HasPrefix(s, "+"):
				if in == "pwsh" && strings.EqualFold(s, "-File") && i+1 < len(words) {
					x.path(raw(words[:i+2]), words[i+1], kindExec, line)
					return
				}
			default:
				x.path(raw(words[:i+1]), words[i], kindExec, line)
				return
			}
		}
		return
	}
	v, ok := x.eval(words[0])
	if !ok || v == "" || strings.HasPrefix(v, "/") {
		return
	}
	// Only a command that names a path runs a file; a bare name is looked up on PATH.
	if strings.ContainsAny(v[:1], markers) || strings.Contains(v, "/") {
		x.path(words[0].raw, words[0], kindExec, line)
	}
}

func contains(list []string, s string) bool {
	for _, l := range list {
		if l == s {
			return true
		}
	}
	return false
}

// direnv reads direnv's stdlib commands in an .envrc.
//
// Implements: REQ-SHELL-006
func (x *extractor) direnv(cmd string, words []*word, line int) bool {
	switch cmd {
	case "source_env", "source_env_if_exists":
		if len(words) > 1 {
			x.path(raw(words[:2]), words[1], kindEnvrc, line)
		}
	case "source_up", "source_up_if_exists":
		file := ".envrc"
		spec := raw(words[:1])
		if len(words) > 1 {
			if s, ok := words[1].text(); ok && s != "" && !strings.Contains(s, "/") {
				file, spec = s, raw(words[:2])
			}
		}
		x.add(spec, file, kindUp, line)
	case "dotenv", "dotenv_if_exists":
		if len(words) > 1 {
			x.path(raw(words[:2]), words[1], kindDotenv, line)
		} else {
			x.add(cmd, "dir:.env", kindDotenv, line)
		}
	default:
		return false
	}
	return true
}

// path records an import of the file a word names, when the word can be evaluated
// to a path relative to something the resolver knows.
func (x *extractor) path(spec string, w *word, kind string, line int) {
	if m, ok := x.module(w); ok {
		x.add(spec, m, kind, line)
	}
}

// module turns a path word into what the resolver reads: "dir:rel" (relative to the
// script's directory), "root:rel" (to the repository root), "cwd:rel" (to the
// working directory, which the resolver guesses) or "any:rel" (below a directory
// the environment names, found by the path's end). Absolute paths, paths under the
// home directory, globs and paths with unknown parts after the start give nothing.
//
// Implements: REQ-SHELL-004, REQ-SHELL-009, REQ-SHELL-010
func (x *extractor) module(w *word) (string, bool) {
	v, ok := x.eval(w)
	if !ok || v == "" || strings.ContainsAny(v, "*?[") {
		return "", false
	}
	prefix := "cwd:"
	switch v[:1] {
	case mDir:
		prefix = "dir:"
	case mRoot:
		prefix = "root:"
	case mCwd:
	case mAny:
		// Only a path with directories below the variable is worth looking up by
		// its end: "$X/lib/functions", not "$X/functions".
		if rest := strings.TrimPrefix(v[1:], "/"); !strings.Contains(rest, "/") || strings.HasPrefix(rest, "../") {
			return "", false
		}
		prefix = "any:"
	case mSelf:
		return "", false
	default:
		if strings.HasPrefix(v, "/") {
			return "", false
		}
		return prefix + v, true
	}
	rest := strings.TrimPrefix(v[1:], "/")
	if rest == "" {
		rest = "."
	}
	return prefix + rest, true
}

// eval evaluates a word as the shell would, as far as it can be known without
// running anything: literals, variables assigned earlier in the file, the idioms
// for the script's own directory, the repository root and the working directory.
// ok is false when any part is unknown.
//
// Implements: REQ-SHELL-004
func (x *extractor) eval(w *word) (string, bool) {
	if w == nil {
		return "", false
	}
	var b strings.Builder
	parts := w.parts
	for i := 0; i < len(parts); i++ {
		pt := parts[i]
		switch pt.kind {
		case pLit:
			if i == 0 && !pt.quoted && strings.HasPrefix(pt.text, "~") {
				return "", false // the home directory
			}
			b.WriteString(pt.text)
		case pParam:
			v, ok := x.param(pt.text)
			if !ok && i == 0 && validName(pt.text) && pt.text != "HOME" {
				v, ok = mAny, true // set by whatever runs the script
			}
			if !ok {
				return "", false
			}
			// zsh: $0:A:h is the script's directory.
			if v == mSelf && i+1 < len(parts) && parts[i+1].kind == pLit {
				for _, m := range []string{":A:h", ":a:h", ":h"} {
					if strings.HasPrefix(parts[i+1].text, m) {
						v = mDir
						parts = append([]part(nil), parts...)
						parts[i+1].text = strings.TrimPrefix(parts[i+1].text, m)
						break
					}
				}
			}
			b.WriteString(v)
		case pSub:
			v, ok := x.subst(pt.cmds)
			if !ok {
				return "", false
			}
			b.WriteString(v)
		default:
			return "", false
		}
	}
	s := b.String()
	if len(s) > 1 && strings.ContainsAny(s[1:], markers) || strings.HasPrefix(s, mSelf) && len(s) > 1 {
		return "", false // a marker can only start a path
	}
	return s, true
}

// selfNames are the parameters that hold the script's own path.
var selfNames = map[string]bool{
	"0": true, "BASH_SOURCE": true, "BASH_SOURCE[0]": true, "(%):-%x": true, "(%):-%N": true,
	"${(%):-%x}": true, "${(%):-%N}": true, "BATS_TEST_FILENAME": true,
}

// param evaluates the inside of ${...} (or a plain $name).
func (x *extractor) param(inner string) (string, bool) {
	n, op := splitParam(inner)
	var v string
	known := true
	switch {
	case selfNames[n]:
		v = mSelf
	case n == "BATS_TEST_DIRNAME":
		v = mDir
	case n == "HOME":
		return "", false
	case n == "PWD":
		v = mCwd
	default:
		v, known = x.vars[n]
		if !known && strings.HasPrefix(n, "(%)") { // a whole zsh prompt expansion
			if selfNames[inner] {
				return mSelf, true
			}
			return "", false
		}
	}
	switch op {
	case "", ":a", ":A":
		return v, known
	case "%/*", ":h", ":a:h", ":A:h", ":h:a", ":h:A":
		if !known {
			return "", false
		}
		return dirOf(v)
	}
	for _, d := range []string{":-", ":=", "-", "="} {
		if def, ok := strings.CutPrefix(op, d); ok {
			if known && v != "" {
				return v, true
			}
			q := newParser([]byte(def), 1, discard{})
			if w := q.token(); w.kind == tWord {
				return x.eval(w.w)
			}
			return "", false
		}
	}
	return "", false
}

// splitParam splits ${name op} into the name (with an index, or a nested ${...})
// and the operator after it.
func splitParam(inner string) (string, string) {
	if strings.HasPrefix(inner, "${") {
		depth := 0
		for i := 0; i < len(inner); i++ {
			switch inner[i] {
			case '{':
				depth++
			case '}':
				if depth--; depth == 0 {
					return inner[:i+1], inner[i+1:]
				}
			}
		}
		return inner, ""
	}
	if strings.HasPrefix(inner, "(") {
		return inner, ""
	}
	i := 0
	for i < len(inner) && (inner[i] == '_' || isAlnum(inner[i])) {
		i++
	}
	if i == 0 && len(inner) > 0 {
		i = 1 // $@, $#, ...
	}
	if i < len(inner) && inner[i] == '[' {
		if j := strings.IndexByte(inner[i:], ']'); j > 0 {
			i += j + 1
		}
	}
	return inner[:i], inner[i:]
}

// dirOf is the directory of an evaluated path.
func dirOf(v string) (string, bool) {
	switch {
	case v == mSelf:
		return mDir, true
	case v == "":
		return "", false
	}
	return v + "/..", true // the resolver cleans the path: a/b.sh/.. is a
}

// subst evaluates a command substitution that computes a directory: dirname,
// realpath and readlink -f of a known path, "cd DIR && pwd", pwd, and
// git rev-parse --show-toplevel.
func (x *extractor) subst(cmds []*command) (string, bool) {
	if len(cmds) == 0 {
		return "", false
	}
	first, last := cmds[0], cmds[len(cmds)-1]
	if len(first.words) == 0 || len(last.words) == 0 {
		return "", false
	}
	args := func(c *command) []*word {
		var out []*word
		for _, w := range c.words[1:] {
			if s := name(w); !strings.HasPrefix(s, "-") || s == "-" {
				out = append(out, w)
			}
		}
		return out
	}
	cmd := name(first.words[0])
	if len(cmds) == 1 {
		a := args(first)
		switch cmd {
		case "pwd":
			return mCwd, true
		case "dirname":
			if len(a) == 1 {
				if v, ok := x.eval(a[0]); ok {
					return dirOf(v)
				}
			}
		case "realpath", "grealpath", "readlink", "greadlink":
			if len(a) == 1 && (!strings.HasSuffix(cmd, "readlink") || len(a) < len(first.words)-1) {
				if v, ok := x.eval(a[0]); ok && v != "" && strings.ContainsAny(v[:1], markers) {
					return v, true
				}
			}
		case "git":
			var rev, top bool
			for _, w := range first.words[1:] {
				rev = rev || name(w) == "rev-parse"
				top = top || name(w) == "--show-toplevel"
			}
			if rev && top {
				return mRoot, true
			}
		}
		return "", false
	}
	if cmd == "cd" && name(last.words[0]) == "pwd" {
		if a := args(first); len(a) == 1 {
			if v, ok := x.eval(a[0]); ok && v != "" && strings.ContainsAny(v[:1], markers) {
				return v, true
			}
		}
	}
	return "", false
}

// discard is a sink for parsing a fragment whose commands do not count.
type discard struct{}

func (discard) command(*command)             {}
func (discard) function(string, string, int) {}
