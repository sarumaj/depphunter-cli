package java

import (
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
// and what it needs sits in plain sight - the package clauses, and the definitions
// outside every class, object and function body (declarationscan.go).

// sourceExtensions are the files whose declarations other files import. Scripts (.kts,
// .sc) are run, never imported, and are not read.
var sourceExtensions = map[string]bool{".kt": true, ".scala": true}

// topLevel matches a top-level line that is a definition: modifiers and
// annotations, the keyword, type parameters, and a name - for a Kotlin extension
// function, a receiver and a name (String.shout, List<T>.second).
var topLevel = regexp.MustCompile(`^(?:(?:@[\w.]+(?:\([^)]*\))?|(?:private|protected)(?:\[\w+\])?|` +
	`public|internal|abstract|sealed|open|final|data|value|inline|annotation|enum|case|` +
	`implicit|lazy|override|suspend|tailrec|operator|infix|external|const|expect|actual|` +
	`opaque|transparent)\s+)*` +
	`(?:fun\s+interface|class|interface|object|trait|enum|typealias|type|fun|def|val|var)\s+` +
	`(?:<[^>]*>\s*)?([\w.]+(?:<[^>]*>\.\w+)?)`)

// readSource adds a Kotlin or Scala file to the index of what each package declares.
// A Kotlin file also declares its facade class (utils.kt holds UtilsKt), which is
// the name Java imports its top-level functions by.
//
// Implements: REQ-KT-003, REQ-KT-007, REQ-SCALA-003
func (r *resolver) readSource(f *scan.File) {
	data, ok := lang.ReadScanned(f)
	if !ok {
		return
	}
	found := declarations(data, path.Ext(f.Path) == ".scala")
	var packageNames []string
	for _, each := range found {
		if each.name == "" {
			continue // the default package, which no import can name
		}
		packageNames = append(packageNames, each.name)
		r.packages[each.name] = append(r.packages[each.name], f.Path)
		names := each.names
		if extension := path.Ext(f.Path); extension == ".kt" {
			base := []rune(strings.TrimSuffix(path.Base(f.Path), extension))
			if len(base) > 0 {
				base[0] = unicode.ToUpper(base[0])
				names = append(names, string(base)+"Kt")
			}
		}
		for _, name := range names {
			r.declarations[each.name+"."+name] = append(r.declarations[each.name+"."+name], f.Path)
		}
	}
	if len(packageNames) > 0 {
		r.filePackage[f.Path] = commonPackage(packageNames)
	}
}

// commonPackage is the package the imports of a file with several package blocks
// are relative to: the one all its packages lie in.
func commonPackage(packageNames []string) string {
	common := strings.Split(packageNames[0], ".")
	for _, name := range packageNames[1:] {
		segments := strings.Split(name, ".")
		count := 0
		for count < len(common) && count < len(segments) && common[count] == segments[count] {
			count++
		}
		common = common[:count]
	}
	return strings.Join(common, ".")
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
		if files := r.declarations[strings.Join(segments[:n], ".")]; len(files) > 0 {
			return files[0]
		}
	}
	files := r.packages[strings.Join(segments, ".")]
	if len(files) == 0 {
		return ""
	}
	directory := path.Dir(files[0])
	for _, f := range files[1:] {
		if path.Dir(f) != directory {
			return files[0]
		}
	}
	return directory
}
