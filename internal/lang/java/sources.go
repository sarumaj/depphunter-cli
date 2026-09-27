package java

import (
	"os"
	"path"
	"regexp"
	"strings"
	"unicode"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// Kotlin and Scala files declare their package rather than live in it: a Kotlin file
// of package com.example.app may sit anywhere, and one Scala file may hold a dozen
// classes. Their imports - and Java's imports of them - cannot be found by path the
// way Java's own are, so the resolver reads what each such file declares. It reads
// text, not a syntax tree: the resolver is rebuilt on every run and must stay cheap,
// and what it needs sits in plain sight - the package clause at the top of the file,
// and the definitions that start at its first column.

// sourceExts are the files whose declarations other files import. Scripts (.kts,
// .sc) are run, never imported, and are not read.
var sourceExts = map[string]bool{".kt": true, ".scala": true}

// topLevel matches a definition starting at the first column: modifiers and
// annotations, the keyword, type parameters, and a name - for a Kotlin extension
// function, a receiver and a name (String.shout, List<T>.second).
var topLevel = regexp.MustCompile(`^(?:(?:@[\w.]+(?:\([^)]*\))?|(?:private|protected)(?:\[\w+\])?|` +
	`public|internal|abstract|sealed|open|final|data|value|inline|annotation|enum|case|` +
	`implicit|lazy|override|suspend|tailrec|operator|infix|external|const|expect|actual|` +
	`opaque|transparent)\s+)*` +
	`(?:fun\s+interface|class|interface|object|trait|enum|typealias|type|fun|def|val|var)\s+` +
	`(?:<[^>]*>\s*)?([\w.]+(?:<[^>]*>\.\w+)?)`)

// declarations reads a Kotlin or Scala file's package and the names it defines at
// the top level. Scala's chained package clauses (package com.example, then package
// app) make one package; a clause after the first definition opens a block that a
// line scan cannot follow, and is ignored.
//
// Implements: REQ-KT-003, REQ-KT-005, REQ-SCALA-003
func declarations(src []byte) (pkg string, names []string) {
	var parts []string
	comment := false
	for _, line := range strings.Split(string(src), "\n") {
		line = strings.TrimRight(line, " \t\r")
		trimmed := strings.TrimSpace(line)
		if comment {
			comment = !strings.Contains(trimmed, "*/")
			continue
		}
		if strings.HasPrefix(trimmed, "/*") {
			comment = !strings.Contains(trimmed[2:], "*/")
			continue
		}
		if line == "" || line != trimmed || strings.HasPrefix(line, "//") {
			continue // indented lines are members, not top-level definitions
		}
		if rest, ok := strings.CutPrefix(line, "package "); ok {
			rest = strings.TrimSpace(rest)
			if name, ok := strings.CutPrefix(rest, "object "); ok { // Scala package object
				if fields := strings.Fields(name); len(fields) > 0 {
					names = append(names, strings.Trim(fields[0], "{:"))
				}
			} else if len(names) == 0 {
				rest = strings.TrimSpace(strings.TrimRight(rest, ";{:"))
				parts = append(parts, strings.Join(strings.Fields(rest), ""))
			}
			continue
		}
		if m := topLevel.FindStringSubmatch(line); m != nil {
			name := m[1][strings.LastIndex(m[1], ".")+1:]
			if name != "" && name != "_" {
				names = append(names, name)
			}
		}
	}
	return strings.Join(parts, "."), names
}

// readSource adds a Kotlin or Scala file to the index of what each package declares.
// A Kotlin file also declares its facade class (utils.kt holds UtilsKt), which is
// the name Java imports its top-level functions by.
//
// Implements: REQ-KT-003, REQ-SCALA-003
func (r *resolver) readSource(f *scan.File) {
	if f.Binary || f.TooLarge || f.Size > lang.MaxParseSize {
		return
	}
	data, err := os.ReadFile(f.Abs)
	if err != nil {
		return
	}
	pkg, names := declarations(data)
	if pkg == "" {
		return // the default package, which no import can name
	}
	r.packages[pkg] = append(r.packages[pkg], f.Path)
	r.filePkg[f.Path] = pkg
	if ext := path.Ext(f.Path); ext == ".kt" {
		base := []rune(strings.TrimSuffix(path.Base(f.Path), ext))
		if len(base) > 0 {
			base[0] = unicode.ToUpper(base[0])
			names = append(names, string(base)+"Kt")
		}
	}
	for _, n := range names {
		r.decls[pkg+"."+n] = append(r.decls[pkg+"."+n], f.Path)
	}
}

// declared finds the project file an import names through the index: the longest
// prefix of the import that is a declared name (an import of a member or a nested
// class names its top-level owner first), else the files of a package it names.
// A package whose files share one directory resolves to that directory, as a Java
// wildcard import does; one spread over several resolves to its first file.
//
// Implements: REQ-KT-003, REQ-SCALA-003
func (r *resolver) declared(segments []string) string {
	for n := len(segments); n >= 2; n-- {
		if files := r.decls[strings.Join(segments[:n], ".")]; len(files) > 0 {
			return files[0]
		}
	}
	files := r.packages[strings.Join(segments, ".")]
	if len(files) == 0 {
		return ""
	}
	dir := path.Dir(files[0])
	for _, f := range files[1:] {
		if path.Dir(f) != dir {
			return files[0]
		}
	}
	return dir
}
