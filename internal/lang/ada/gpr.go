package ada

import (
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// item is a string of a project file with the line it was written on.
type item struct {
	s    string
	line int
}

// gpr is what a GNAT project file says that the map needs: the projects it
// imports and extends, its source directories, mains, aggregated projects and
// the Naming package's explicit unit-to-file entries.
type gpr struct {
	name, low         string
	line              int
	withs             []item // with "x.gpr", including limited with
	extends           *item
	sourceDirectories []item
	directoriesSet    bool // Source_Dirs is set (else the project's directory)
	mains             []item
	projectFiles      []item // an aggregate project's Project_Files
	specs, bodies     map[string]string
}

// value is a project-file expression's value: a string or a list of strings;
// known is false for what depends on other projects or the environment.
type value struct {
	items []item
	list  bool
	known bool
}

type gprReader struct {
	s         *stream
	i         int
	g         *gpr
	variables map[string]value
	// inCase counts the case constructs being read: an assignment inside one
	// adds to the previous value, since every alternative is taken.
	inCase int
}

// readGPR reads a project file without evaluating it: every alternative of a
// case construct counts, a variable holds what the file assigns it (an
// external's default), and what refers to other projects' variables or
// attributes is unknown.
//
// Implements: REQ-ADA-005
func readGPR(source []byte) *gpr {
	r := &gprReader{s: newStream(source), g: &gpr{specs: map[string]string{}, bodies: map[string]string{}}, variables: map[string]value{}}
	r.run()
	return r.g
}

func (r *gprReader) run() {
	s := r.s
	// context clause
	for {
		w := s.word(r.i)
		if w == "limited" && s.word(r.i+1) == "with" {
			r.i++
			w = "with"
		}
		if w != "with" {
			break
		}
		r.i++
		for n := 0; n < 1024; n++ {
			t, ok := s.at(r.i)
			if !ok {
				return
			}
			r.i++
			if t.kind == tString {
				r.g.withs = append(r.g.withs, item{t.text, t.line})
			} else if t.kind == tPunctuation && t.text == ";" {
				break
			}
		}
	}
	// project header
	for n := 0; n < 8; n++ {
		switch s.word(r.i) {
		case "aggregate", "library", "abstract", "configuration":
			r.i++
			continue
		}
		break
	}
	if s.word(r.i) != "project" {
		return
	}
	t, _ := s.at(r.i)
	written, low, next := r.name(r.i + 1)
	r.g.name, r.g.low, r.g.line = written, low, t.line
	r.i = next
	if s.word(r.i) == "extends" {
		r.i++
		if s.word(r.i) == "all" {
			r.i++
		}
		if u, ok := s.at(r.i); ok && u.kind == tString {
			r.g.extends = &item{u.text, u.line}
			r.i++
		}
	}
	if s.word(r.i) != "is" {
		return
	}
	r.i++
	r.body("")
}

// name reads a dotted name.
func (r *gprReader) name(i int) (written, low string, next int) {
	t, ok := r.s.at(i)
	if !ok || t.kind != tIdentifier {
		return "", "", i
	}
	written, low = t.text, t.low
	i++
	for n := 0; n < 64 && r.s.punctuation(i, "."); n++ {
		u, ok := r.s.at(i + 1)
		if !ok || u.kind != tIdentifier {
			break
		}
		written += "." + u.text
		low += "." + u.low
		i += 2
	}
	return written, low, i
}

// body reads declarations up to the end of the project or of package pkg.
func (r *gprReader) body(packageName string) {
	s := r.s
	for {
		start := r.i
		s.trim(r.i)
		t, ok := s.at(r.i)
		if !ok {
			return
		}
		switch {
		case t.kind != tIdentifier:
			r.i++
		case t.low == "end":
			if s.word(r.i+1) == "case" {
				if r.inCase > 0 {
					r.inCase--
				}
				r.skip()
				continue
			}
			r.skip()
			return
		case t.low == "for":
			r.attribute(packageName)
		case t.low == "package":
			r.packageBlock()
		case t.low == "case":
			r.inCase++
			for n := 0; n < maxHeader; n++ {
				if w := s.word(r.i); w == "is" {
					r.i++
					break
				}
				if _, ok := s.at(r.i); !ok {
					return
				}
				r.i++
			}
		case t.low == "when":
			for n := 0; n < maxHeader; n++ {
				if s.punctuation(r.i, "=>") {
					r.i++
					break
				}
				if _, ok := s.at(r.i); !ok {
					return
				}
				r.i++
			}
		case t.low == "type", t.low == "null":
			r.skip()
		default:
			r.assignment()
		}
		if r.i <= start {
			r.i = start + 1
		}
	}
}

// skip moves past the next ;.
func (r *gprReader) skip() {
	for {
		t, ok := r.s.at(r.i)
		if !ok {
			return
		}
		r.i++
		if t.kind == tPunctuation && t.text == ";" {
			return
		}
	}
}

// packageBlock reads a package: renames and extends forms are skipped, the Naming
// package's entries kept, the rest read for its variables.
func (r *gprReader) packageBlock() {
	written, low, next := r.name(r.i + 1)
	_ = written
	r.i = next
	switch r.s.word(r.i) {
	case "renames":
		r.skip()
		return
	case "extends":
		r.i++
		_, _, r.i = r.name(r.i)
	}
	if r.s.word(r.i) != "is" {
		r.skip()
		return
	}
	r.i++
	r.body(low)
}

// attribute reads for Name [(index)] use expression;
func (r *gprReader) attribute(packageName string) {
	s := r.s
	r.i++
	t, ok := s.at(r.i)
	if !ok || t.kind != tIdentifier {
		r.skip()
		return
	}
	attribute := t.low
	r.i++
	index := ""
	if s.punctuation(r.i, "(") {
		if u, ok := s.at(r.i + 1); ok && (u.kind == tString || u.kind == tIdentifier) {
			index = u.text
		}
		for n := 0; n < 64; n++ {
			if s.punctuation(r.i, ")") {
				r.i++
				break
			}
			if _, ok := s.at(r.i); !ok {
				return
			}
			r.i++
		}
	}
	if s.word(r.i) != "use" {
		r.skip()
		return
	}
	r.i++
	v := r.expression()
	r.skip()
	if !v.known {
		return
	}
	switch packageName {
	case "":
		switch attribute {
		case "source_dirs":
			r.g.directoriesSet = true
			r.g.sourceDirectories = r.assign(r.g.sourceDirectories, v.items)
		case "main":
			r.g.mains = r.assign(r.g.mains, v.items)
		case "project_files":
			r.g.projectFiles = r.assign(r.g.projectFiles, v.items)
		}
	case "naming":
		if index == "" || len(v.items) == 0 {
			return
		}
		switch attribute {
		case "spec", "specification":
			r.g.specs[lower(index)] = v.items[0].s
		case "body", "implementation":
			r.g.bodies[lower(index)] = v.items[0].s
		}
	}
}

// assign is an attribute's or variable's new value: inside a case construct
// every alternative's value is added.
func (r *gprReader) assign(old, v []item) []item {
	if r.inCase == 0 {
		return v
	}
	out := append([]item(nil), old...)
	for _, x := range v {
		duplicate := false
		for _, y := range out {
			if y.s == x.s {
				duplicate = true
				break
			}
		}
		if !duplicate {
			out = append(out, x)
		}
	}
	return out
}

// assignment reads Name [: Type] := expression;
func (r *gprReader) assignment() {
	s := r.s
	_, low, next := r.name(r.i)
	r.i = next
	if s.punctuation(r.i, ":") {
		r.i++
		_, _, r.i = r.name(r.i)
	}
	if !s.punctuation(r.i, ":=") {
		r.skip()
		return
	}
	r.i++
	v := r.expression()
	r.skip()
	if low == "" {
		return
	}
	if !v.known {
		if _, ok := r.variables[low]; !ok || r.inCase == 0 {
			r.variables[low] = v
		}
		return
	}
	old := r.variables[low]
	r.variables[low] = value{items: r.assign(old.items, v.items), list: v.list || old.list, known: true}
}

// expression reads terms joined by & up to ; (not consumed).
func (r *gprReader) expression() value {
	v := r.term()
	for n := 0; n < 4096 && r.s.punctuation(r.i, "&"); n++ {
		r.i++
		w := r.term()
		if !v.known || !w.known {
			v.known = false
			continue
		}
		if v.list || w.list {
			v.items = append(append([]item(nil), v.items...), w.items...)
			v.list = true
			continue
		}
		if len(v.items) == 1 && len(w.items) == 1 {
			v.items = []item{{v.items[0].s + w.items[0].s, v.items[0].line}}
		}
	}
	return v
}

// term reads a string, a list, a variable or external ("X", "default").
func (r *gprReader) term() value {
	s := r.s
	t, ok := s.at(r.i)
	if !ok {
		return value{}
	}
	switch {
	case t.kind == tString:
		r.i++
		return value{items: []item{{t.text, t.line}}, known: true}
	case t.kind == tPunctuation && t.text == "(":
		r.i++
		v := value{list: true, known: true}
		for n := 0; n < 4096; n++ {
			if s.punctuation(r.i, ")") {
				r.i++
				return v
			}
			e := r.expression()
			if e.known {
				v.items = append(v.items, e.items...)
			} else {
				v.known = false
			}
			if s.punctuation(r.i, ",") {
				r.i++
				continue
			}
			if s.punctuation(r.i, ")") {
				r.i++
				return v
			}
			return value{}
		}
		return value{}
	case t.kind == tIdentifier && (t.low == "external" || t.low == "external_as_list"):
		r.i++
		if !s.punctuation(r.i, "(") {
			return value{}
		}
		var arguments []token
		depth := 0
		for n := 0; n < 256; n++ {
			u, ok := s.at(r.i)
			if !ok {
				return value{}
			}
			r.i++
			if u.kind == tPunctuation {
				switch u.text {
				case "(":
					depth++
				case ")":
					depth--
				}
				if depth == 0 {
					break
				}
				continue
			}
			if depth == 1 {
				arguments = append(arguments, u)
			}
		}
		if t.low == "external" && len(arguments) >= 2 && arguments[1].kind == tString {
			return value{items: []item{{arguments[1].text, arguments[1].line}}, known: true}
		}
		return value{}
	case t.kind == tIdentifier:
		_, low, next := r.name(r.i)
		r.i = next
		if s.punctuation(r.i, "'") {
			// Project'Attribute or Pkg'Attribute ("Index")
			r.i += 2
			if s.punctuation(r.i, "(") {
				for n := 0; n < 64; n++ {
					if s.punctuation(r.i, ")") {
						r.i++
						break
					}
					if _, ok := s.at(r.i); !ok {
						break
					}
					r.i++
				}
			}
			return value{}
		}
		if v, ok := r.variables[low]; ok {
			return v
		}
		return value{}
	}
	r.i++
	return value{}
}

// extractGPR turns a project file into imports: the projects it withs and
// extends and aggregates, its source directories and its mains.
//
// Implements: REQ-ADA-005
func extractGPR(source []byte) *lang.Extraction {
	g := readGPR(source)
	extraction := &lang.Extraction{}
	seen := map[string]bool{}
	add := func(spec, module, kind string, line int) {
		if strings.TrimSpace(module) == "" || seen[kind+"\x00"+spec] {
			return
		}
		seen[kind+"\x00"+spec] = true
		extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: spec, Module: module, Name: kind, Line: line})
	}
	for _, w := range g.withs {
		add(`with "`+w.s+`"`, w.s, kindProject, w.line)
	}
	if g.extends != nil {
		add(`extends "`+g.extends.s+`"`, g.extends.s, kindProject, g.extends.line)
	}
	for _, d := range g.sourceDirectories {
		add(`Source_Dirs "`+d.s+`"`, d.s, kindDirectory, d.line)
	}
	for _, m := range g.mains {
		add(`Main "`+m.s+`"`, m.s, kindMain, m.line)
	}
	for _, f := range g.projectFiles {
		add(`Project_Files "`+f.s+`"`, f.s, kindProject, f.line)
	}
	if g.name != "" {
		var symbols lang.SymbolSet
		symbols.Add(g.name, "project", g.line)
		extraction.Symbols = symbols.List()
	}
	return extraction
}

// directorySpec splits a Source_Dirs entry into a directory and whether its
// subdirectories count ("src/**").
func directorySpec(s string) (directory string, recursive bool) {
	s = strings.ReplaceAll(s, "\\", "/")
	if d, ok := strings.CutSuffix(s, "**"); ok {
		s, recursive = d, true
	}
	s = strings.TrimSuffix(s, "/")
	if s == "" {
		s = "."
	}
	return s, recursive
}
