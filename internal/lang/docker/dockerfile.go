package docker

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// A Dockerfile is read by a small line parser rather than the tree-sitter grammar.
// The format is line-oriented and its syntax is the easy part; what decides which
// image a FROM names is its meaning - which ARGs are declared before the first FROM
// and with what default, which stage names exist by then, what the escape directive
// makes of a trailing backslash - and a syntax tree would still leave all of that to
// be done by hand, over nodes, in source order. The parser follows BuildKit's rules
// where they matter here: parser directives, comment lines (dropped even inside a
// continued instruction), line continuations, case-insensitive instructions and
// heredoc bodies, which are skipped so that a script's text is never read as an
// instruction.

// instruction is one logical line of a Dockerfile, its continuations joined.
type instruction struct {
	keyword string // upper case: FROM, ARG, COPY, …
	args    string
	line    int // where it starts, 1-based
}

var directive = regexp.MustCompile(`^#\s*([a-zA-Z][a-zA-Z0-9_-]*)\s*=\s*(\S*)\s*$`)

// heredoc finds the heredoc markers of an instruction, "<<EOF", "<<-EOF", "<<'EOF'".
// The marker must start a word, so a shift in a shell expression is not one.
var heredoc = regexp.MustCompile(`(?:^|\s)<<(-?)\s*(["']?)([A-Za-z_][A-Za-z0-9_]*)(["']?)`)

// instructions splits a Dockerfile into its instructions. The syntax directive, when
// there is one, is returned apart: it names the frontend image BuildKit pulls to read
// the rest of the file.
//
// Implements: REQ-DOCKER-002
func instructions(src []byte) (out []instruction, syntax instruction) {
	lines := strings.Split(strings.ReplaceAll(string(src), "\r\n", "\n"), "\n")
	escape := byte('\\')
	i := 0
	// Parser directives are comments of the form "# key=value" at the very top; the
	// first line that is anything else ends them. Only escape changes how the rest is
	// read.
	for ; i < len(lines); i++ {
		m := directive.FindStringSubmatch(strings.TrimSpace(lines[i]))
		if m == nil {
			break
		}
		switch {
		case strings.EqualFold(m[1], "escape") && (m[2] == "`" || m[2] == `\`):
			escape = m[2][0]
		case strings.EqualFold(m[1], "syntax") && m[2] != "":
			syntax = instruction{keyword: "SYNTAX", args: m[2], line: i + 1}
		}
	}
	var cur *instruction
	var text strings.Builder
	for ; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" || line[0] == '#' {
			continue // comments go, even between continued lines, and so do blank lines
		}
		if cur == nil {
			cur = &instruction{line: i + 1}
			text.Reset()
		}
		continued := line[len(line)-1] == escape
		if continued {
			line = strings.TrimSpace(line[:len(line)-1])
		}
		if text.Len() > 0 {
			text.WriteByte(' ')
		}
		text.WriteString(line)
		if continued {
			continue
		}
		keyword, args, _ := strings.Cut(text.String(), " ")
		cur.keyword, cur.args = strings.ToUpper(keyword), strings.TrimSpace(args)
		out = append(out, *cur)
		cur = nil
		i = skipHeredocs(lines, i, keyword, args)
	}
	return out, syntax
}

// skipHeredocs returns the index of the last line of the heredoc bodies an instruction
// opens, or at when it opens none. A marker whose terminator never comes is taken for
// something other than a heredoc, and nothing is skipped.
func skipHeredocs(lines []string, at int, keyword, args string) int {
	switch strings.ToUpper(keyword) {
	case "RUN", "COPY", "ADD", "ONBUILD":
	default:
		return at
	}
	end := at
	for _, m := range heredoc.FindAllStringSubmatch(args, -1) {
		if m[2] != m[4] {
			continue // mismatched quotes
		}
		found := false
		for j := end + 1; j < len(lines); j++ {
			l := lines[j]
			if m[1] == "-" {
				l = strings.TrimLeft(l, "\t")
			}
			if strings.TrimRight(l, " \t\r") == m[3] {
				end, found = j, true
				break
			}
		}
		if !found {
			return at
		}
	}
	return end
}

// extractDockerfile reads a Dockerfile's images: the frontend a syntax directive
// names, and every FROM, COPY --from and RUN --mount from= that names an image rather
// than an earlier stage. Named stages become the file's symbols.
//
// Implements: REQ-DOCKER-002, REQ-DOCKER-003, REQ-DOCKER-005
func extractDockerfile(src []byte) *lang.Extraction {
	ex := &lang.Extraction{}
	var symbols lang.SymbolSet
	global := map[string]string{} // ARGs declared before the first FROM, with a value
	var stageArgs map[string]string
	stages := map[string]bool{} // earlier stages, by lower-cased name and by index
	count := 0
	add := func(spec, ref string, line int) {
		ex.Imports = append(ex.Imports, lang.RawImport{Spec: spec, Module: ref, Name: kindImage, Line: line})
	}
	all, syntax := instructions(src)
	if syntax.args != "" {
		add("# syntax="+syntax.args, syntax.args, syntax.line)
	}
	for _, in := range all {
		keyword, args := in.keyword, in.args
		if keyword == "ONBUILD" { // the instruction runs in a later build, from this file
			k, a, _ := strings.Cut(args, " ")
			keyword, args = strings.ToUpper(k), strings.TrimSpace(a)
		}
		switch keyword {
		case "ARG":
			if count == 0 {
				declare(global, global, args)
			} else {
				declare(stageArgs, global, args)
			}
		case "FROM":
			_, rest := splitFlags(args) // --platform says which variant, not which image
			fields := strings.Fields(rest)
			if len(fields) == 0 {
				continue
			}
			// A FROM sees only the ARGs declared before the first FROM.
			ref, ok := expand(fields[0], global, false)
			if ok && !stages[strings.ToLower(ref)] && !strings.EqualFold(ref, "scratch") {
				add("FROM "+ref, ref, in.line)
			} else if !ok {
				add("FROM "+fields[0], ref, in.line)
			}
			if len(fields) >= 3 && strings.EqualFold(fields[1], "AS") {
				stages[strings.ToLower(fields[2])] = true
				symbols.Add(fields[2], "stage", in.line)
			}
			stages[strconv.Itoa(count)] = true
			count++
			stageArgs = map[string]string{}
		case "COPY", "RUN":
			flags, _ := splitFlags(args)
			for _, f := range flags {
				name, value, _ := strings.Cut(f, "=")
				var from string
				switch {
				case keyword == "COPY" && name == "--from":
					from = value
				case keyword == "RUN" && name == "--mount":
					from = option(value, "from")
				}
				if from == "" {
					continue
				}
				ref, ok := expand(from, stageArgs, false)
				if ok && stages[strings.ToLower(ref)] {
					continue // an earlier stage of this file
				}
				spec := keyword + " " + name + "=" + from
				if keyword == "RUN" {
					spec = "RUN --mount from=" + from
				}
				add(spec, ref, in.line)
			}
		}
	}
	ex.Symbols = symbols.List()
	return ex
}

// declare records the variables an ARG instruction declares, "NAME" or "NAME=value",
// several to a line. A default may use variables declared before it. An ARG without a
// value inside a stage takes the value the same ARG had before the first FROM.
func declare(vars, global map[string]string, args string) {
	if vars == nil {
		return
	}
	for _, word := range strings.Fields(args) {
		name, value, hasValue := strings.Cut(word, "=")
		if !hasValue {
			if v, ok := global[name]; ok {
				vars[name] = v
			}
			continue
		}
		value = unquote(value)
		if v, ok := expand(value, vars, false); ok {
			vars[name] = v
		} else {
			delete(vars, name) // what it becomes is only known at build time
		}
	}
}

// splitFlags separates an instruction's leading --flags from the rest.
func splitFlags(args string) (flags []string, rest string) {
	rest = args
	for {
		rest = strings.TrimLeft(rest, " \t")
		if !strings.HasPrefix(rest, "--") {
			return flags, rest
		}
		flag, after, _ := strings.Cut(rest, " ")
		flags = append(flags, unquote(flag))
		rest = after
	}
}

// option reads one key out of a comma-separated flag value, "type=bind,from=x,target=/y".
func option(value, key string) string {
	for _, part := range strings.Split(unquote(value), ",") {
		if k, v, ok := strings.Cut(part, "="); ok && strings.EqualFold(strings.TrimSpace(k), key) {
			return unquote(strings.TrimSpace(v))
		}
	}
	return ""
}

func unquote(s string) string {
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1]
	}
	return s
}

// expand substitutes variables in s: $NAME, ${NAME}, ${NAME:-word}, ${NAME-word} and
// ${NAME:+word}. A variable with no value - declared without a default, or not at all,
// or set only by --build-arg or the environment - is left as written and ok is false:
// what it becomes is only known when the build runs, and an image is not made up for
// it. In Compose, "$$" is a literal dollar sign (compose true); in a Dockerfile, "\$"
// is.
//
// Implements: REQ-DOCKER-003
func expand(s string, vars map[string]string, compose bool) (string, bool) {
	var b strings.Builder
	ok := true
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case !compose && c == '\\' && i+1 < len(s) && s[i+1] == '$':
			b.WriteByte('$')
			i++
			continue
		case compose && c == '$' && i+1 < len(s) && s[i+1] == '$':
			b.WriteByte('$')
			i++
			continue
		case c != '$' || i+1 == len(s):
			b.WriteByte(c)
			continue
		}
		if s[i+1] == '{' {
			end := closing(s, i+2)
			if end < 0 {
				b.WriteString(s[i:])
				return b.String(), false
			}
			v, good := substitute(s[i+2:end], vars, compose)
			if !good {
				v, ok = s[i:end+1], false
			}
			b.WriteString(v)
			i = end
			continue
		}
		j := i + 1
		for j < len(s) && (s[j] == '_' || isAlnum(s[j])) {
			j++
		}
		if j == i+1 {
			b.WriteByte(c) // a lone dollar sign
			continue
		}
		if v, set := vars[s[i+1:j]]; set {
			b.WriteString(v)
		} else {
			b.WriteString(s[i:j])
			ok = false
		}
		i = j - 1
	}
	return b.String(), ok
}

// substitute evaluates the inside of a ${…}.
func substitute(body string, vars map[string]string, compose bool) (string, bool) {
	k := 0
	for k < len(body) && (body[k] == '_' || isAlnum(body[k])) {
		k++
	}
	name, rest := body[:k], body[k:]
	op, word := "", ""
	for _, o := range []string{":-", ":+", "-", "+"} {
		if w, found := strings.CutPrefix(rest, o); found {
			op, word = o, w
			break
		}
	}
	if name == "" || (rest != "" && op == "") {
		return "", false // ${NAME:?error}, ${NAME#pattern} and the like
	}
	v, set := vars[name]
	switch op {
	case ":-":
		if !set || v == "" {
			return expand(word, vars, compose)
		}
	case "-":
		if !set {
			return expand(word, vars, compose)
		}
	case ":+":
		if set && v != "" {
			return expand(word, vars, compose)
		}
		return "", true
	case "+":
		if set {
			return expand(word, vars, compose)
		}
		return "", true
	}
	return v, set
}

// closing finds the brace that closes a ${ opened before from, allowing nested ones.
func closing(s string, from int) int {
	depth := 1
	for i := from; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			if depth--; depth == 0 {
				return i
			}
		}
	}
	return -1
}

func isAlnum(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}
