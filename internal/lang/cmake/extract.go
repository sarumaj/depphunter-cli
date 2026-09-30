package cmake

import (
	"cmp"
	"maps"
	"path"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// Import kinds, carried in RawImport.Name.
const (
	kindSubdirectory = "subdir"  // add_subdirectory(dir): dir/CMakeLists.txt
	kindInclude      = "include" // include(): a file, or a module on CMAKE_MODULE_PATH or CMake's
	kindSource       = "source"  // a target's source file
	kindFile         = "file"    // configure_file()'s input; a presets file's include or toolchain
	kindFind         = "find"    // find_package(): Module "name" or "name/component"
	kindGit          = "git"     // fetched from git: Module "url\nref"
	kindURL          = "url"     // downloaded: Module "url\nhash"
	kindUse          = "use"     // FetchContent_MakeAvailable(name): Module is the name, lower case
	kindPackage      = "pkg"     // pkg_check_modules(): Module "name>=version"
)

// fileInfo is what one CMake file says: its imports and symbols for Extract, and for
// the resolver the variables it leaves set in its directory's scope, the projects
// it declares and the content it declares for fetching.
type fileInfo struct {
	imports   []lang.RawImport
	symbols   lang.SymbolSet
	variables map[string]string
	projects  []string
	fetches   map[string]lang.RawImport // lower-case name -> its git or url import
}

// extractor walks a file's commands in order, keeping the variables set so far.
type extractor struct {
	info      *fileInfo
	variables map[string]string
	saved     []map[string]string // the scopes a function or macro body hides
	seen      map[string]bool     // symbol names already added
}

// analyze reads a CMake file's commands. Variables are those `set()`, `list(APPEND)`
// and project() give, in source order and whatever the if() around them; a
// reference to one the file does not set is kept as written (${NAME}) for the
// resolver, which knows the directories above (REQ-CMAKE-003).
//
// Implements: REQ-CMAKE-003, REQ-CMAKE-004, REQ-CMAKE-010
func analyze(commands []command) *fileInfo {
	e := &extractor{
		info: &fileInfo{fetches: map[string]lang.RawImport{}},
		// CMAKE_MODULE_PATH starts empty here, so that the common
		// set(CMAKE_MODULE_PATH ${CMAKE_MODULE_PATH} dir) keeps just dir; the
		// directories above add theirs in the resolver.
		variables: map[string]string{"CMAKE_MODULE_PATH": ""},
		seen:      map[string]bool{},
	}
	for _, c := range commands {
		e.command(c)
	}
	for len(e.saved) > 0 { // an unclosed function body
		e.variables, e.saved = e.saved[len(e.saved)-1], e.saved[:len(e.saved)-1]
	}
	e.info.variables = e.variables
	return e.info
}

// value is one evaluated argument: its text, and as written - the argument, or the
// list element when the argument expanded to several.
type value struct {
	text, written string
	line          int
}

// values evaluates arguments: variable references expanded as far as the file
// knows, an unquoted argument split into its list elements (empty ones dropped).
func (e *extractor) values(arguments []argument) []value {
	var out []value
	for _, a := range arguments {
		if a.raw {
			out = append(out, value{a.text, a.text, a.line})
			continue
		}
		s, _ := expand(a.text, e.lookup, true)
		if a.quoted {
			out = append(out, value{s, a.text, a.line})
			continue
		}
		els := splitList(s)
		for _, el := range els {
			if el == "" {
				continue
			}
			written := el
			if len(els) == 1 {
				written = a.text
			}
			out = append(out, value{el, written, a.line})
		}
	}
	return out
}

func (e *extractor) lookup(name string) (string, bool) {
	v, ok := e.variables[name]
	return v, ok
}

// splitList splits a CMake list on the semicolons not escaped (\;).
func splitList(s string) []string {
	if !strings.Contains(s, ";") {
		return []string{s}
	}
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' {
			i++
			continue
		}
		if s[i] == ';' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}

func texts(values []value) []string {
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = v.text
	}
	return out
}

// known reports whether a name has no reference left the file could not expand.
func known(s string) bool {
	return !strings.Contains(s, "${") && !strings.Contains(s, "$<") && !strings.Contains(s, "$ENV{")
}

func (e *extractor) symbol(name, kind string, line int) {
	if name == "" || !known(name) || e.seen[kind+"\x00"+name] {
		return
	}
	e.seen[kind+"\x00"+name] = true
	e.info.symbols.Add(name, kind, line)
}

func (e *extractor) resolveImport(spec, module, kind string, line int) {
	if module == "" {
		return
	}
	if len(e.saved) > 0 && !known(module) {
		return // a function's parameter: known only where it is called
	}
	e.info.imports = append(e.info.imports, lang.RawImport{Spec: spec, Module: module, Name: kind, Line: line})
}

// Implements: REQ-CMAKE-003, REQ-CMAKE-004, REQ-CMAKE-005, REQ-CMAKE-006, REQ-CMAKE-007
// Implements: REQ-CMAKE-008, REQ-CMAKE-011
func (e *extractor) command(c command) {
	switch c.name {
	case "function", "macro":
		if values := e.values(c.arguments); len(values) > 0 {
			e.symbol(values[0].text, c.name, c.line)
		}
		e.saved = append(e.saved, e.variables)
		e.variables = maps.Clone(e.variables)
		return
	case "endfunction", "endmacro":
		if n := len(e.saved); n > 0 {
			e.variables, e.saved = e.saved[n-1], e.saved[:n-1]
		}
		return
	}
	values := e.values(c.arguments)
	if len(values) == 0 {
		return
	}
	first := values[0].text
	spec := func(s string) string { return c.cased + "(" + s + ")" }
	written := spec(values[0].written)
	switch c.name {
	case "set":
		e.set(values, c.line)
	case "unset":
		delete(e.variables, first)
	case "list":
		if len(values) >= 2 && (first == "APPEND" || first == "PREPEND") {
			v := values[1].text
			items := texts(values[2:])
			if current := e.variables[v]; current != "" {
				if first == "APPEND" {
					items = append([]string{current}, items...)
				} else {
					items = append(items, current)
				}
			}
			e.variables[v] = strings.Join(items, ";")
		}
	case "get_filename_component":
		// get_filename_component(ROOT ${CMAKE_CURRENT_SOURCE_DIR}/.. ABSOLUTE)
		if len(values) >= 3 {
			switch values[2].text {
			case "DIRECTORY", "PATH":
				e.variables[first] = values[1].text + "/.."
			case "ABSOLUTE", "REALPATH":
				e.variables[first] = values[1].text
			}
		}
	case "project":
		e.info.projects = append(e.info.projects, first)
		e.symbol(first, "project", c.line)
		// The directory is the resolver's to fill in (CMAKE_CURRENT_SOURCE_DIR).
		for _, v := range []string{"PROJECT_SOURCE_DIR", first + "_SOURCE_DIR"} {
			e.variables[v] = "${CMAKE_CURRENT_SOURCE_DIR}"
		}
		e.variables["PROJECT_NAME"] = first
		if _, ok := e.variables["CMAKE_PROJECT_NAME"]; !ok {
			e.variables["CMAKE_PROJECT_NAME"] = first
		}
		for i := 1; i+1 < len(values); i++ {
			if values[i].text == "VERSION" {
				e.variables["PROJECT_VERSION"] = values[i+1].text
				e.variables[first+"_VERSION"] = values[i+1].text
			}
		}
	case "option":
		e.symbol(first, "option", c.line)
	case "add_subdirectory":
		e.resolveImport(written, first, kindSubdirectory, c.line)
	case "include":
		e.resolveImport(written, first, kindInclude, c.line)
	case "configure_file":
		e.resolveImport(written, first, kindFile, c.line)
	case "find_package", "find_dependency":
		e.find(c, values)
	case "add_library", "add_executable", "qt_add_executable", "qt_add_library", "qt6_add_executable",
		"qt6_add_library", "pybind11_add_module", "nanobind_add_module", "cuda_add_executable", "cuda_add_library":
		e.target(c, values)
	case "add_custom_target":
		e.symbol(first, "target", c.line)
		e.sources(c, first, values[1:], "SOURCES")
	case "target_sources":
		e.sources(c, first, values[1:], "")
	case "fetchcontent_declare", "externalproject_add":
		e.fetch(c, first, keyValues(texts(values[1:])), spec(first), values[0].line)
	case "fetchcontent_populate":
		if len(values) > 1 { // the old form, declaring what it populates
			e.fetch(c, first, keyValues(texts(values[1:])), spec(first), values[0].line)
		} else {
			e.resolveImport(spec(first), strings.ToLower(first), kindUse, c.line)
		}
	case "fetchcontent_makeavailable":
		for _, v := range values {
			e.resolveImport(spec(v.text), strings.ToLower(v.text), kindUse, v.line)
		}
	case "cpmaddpackage", "cpmfindpackage", "cpmdeclarepackage":
		e.cpm(c, values)
	case "cpmgetpackage":
		e.resolveImport(spec(first), strings.ToLower(first), kindUse, c.line)
	case "pkg_check_modules", "pkg_search_module":
		for _, v := range values[1:] {
			if !isKeyword(v.text) {
				e.resolveImport(c.cased+"("+first+" "+v.text+")", v.text, kindPackage, v.line)
			}
		}
	}
}

// set records set(VAR values...), set(VAR value CACHE TYPE doc) - a cache entry is
// also a symbol - and set(VAR) unsetting VAR. A value for the parent scope or the
// environment is not the file's.
func (e *extractor) set(values []value, line int) {
	name := values[0].text
	if strings.HasPrefix(name, "ENV{") {
		return
	}
	var items []string
	for _, v := range values[1:] {
		switch v.text {
		case "PARENT_SCOPE":
			return
		case "CACHE":
			e.symbol(name, "cache", line)
			if _, ok := e.variables[name]; !ok { // a normal variable hides the cache entry
				e.variables[name] = strings.Join(items, ";")
			}
			return
		}
		items = append(items, v.text)
	}
	if len(items) == 0 {
		delete(e.variables, name)
		return
	}
	e.variables[name] = strings.Join(items, ";")
}

// isKeyword reports whether an argument is one of the upper-case keywords commands
// take (REQUIRED, GIT_TAG, PRIVATE) rather than a value.
func isKeyword(s string) bool {
	if len(s) < 2 || s[0] < 'A' || s[0] > 'Z' {
		return false
	}
	for i := 0; i < len(s); i++ {
		if c := s[i]; !(c >= 'A' && c <= 'Z') && !(c >= '0' && c <= '9') && c != '_' {
			return false
		}
	}
	return true
}

// keyValues groups arguments under the keyword before them.
func keyValues(values []string) map[string][]string {
	keyValue := map[string][]string{}
	key := ""
	for _, v := range values {
		if isKeyword(v) {
			key = v
			if _, ok := keyValue[key]; !ok {
				keyValue[key] = nil
			}
			continue
		}
		keyValue[key] = append(keyValue[key], v)
	}
	return keyValue
}

func firstOf(keyValue map[string][]string, key string) string {
	if v := keyValue[key]; len(v) > 0 {
		return v[0]
	}
	return ""
}

// find records find_package(Name [version] [REQUIRED] [COMPONENTS c...]) and the
// find_dependency() of package configuration files. Boost's and Qt's components are
// libraries of their own (vcpkg's boost-filesystem, <QtWidgets/...>): one import
// each.
//
// Implements: REQ-CMAKE-006
func (e *extractor) find(c command, values []value) {
	name := values[0].text
	var comps []string
	mode := ""
	for i, v := range values[1:] {
		switch {
		case isKeyword(v.text):
			mode = v.text
		case i == 0 && v.text != "" && v.text[0] >= '0' && v.text[0] <= '9':
			// the version
		case mode == "NAMES":
			name, mode = v.text, "" // the first of the names it goes by
		case mode == "REQUIRED" || mode == "COMPONENTS" || mode == "OPTIONAL_COMPONENTS":
			comps = append(comps, v.text)
		}
	}
	lower := strings.ToLower(name)
	// Qt${QT_VERSION_MAJOR}: the major version is often only known at configure time.
	if len(comps) > 0 && (lower == "boost" || qtLike(name) || strings.HasPrefix(name, "Qt${")) {
		for _, component := range comps {
			e.resolveImport(c.cased+"("+name+" "+component+")", name+"/"+component, kindFind, c.line)
		}
		return
	}
	e.resolveImport(c.cased+"("+name+")", name, kindFind, c.line)
}

// targetKeywords are the options of the commands defining a target that are not
// source files.
var targetKeywords = map[string]bool{
	"STATIC": true, "SHARED": true, "MODULE": true, "OBJECT": true, "INTERFACE": true, "UNKNOWN": true,
	"EXCLUDE_FROM_ALL": true, "WIN32": true, "MACOSX_BUNDLE": true, "GLOBAL": true,
	"MANUAL_FINALIZATION": true, "NO_EXTRAS": true, "THIN_LTO": true, "SYSTEM": true,
}

// target records add_library(), add_executable() and their framework variants: the
// target is a symbol, its sources are imports. An ALIAS is a symbol of its own; an
// IMPORTED target is defined elsewhere.
//
// Implements: REQ-CMAKE-004, REQ-CMAKE-008
func (e *extractor) target(c command, values []value) {
	name := values[0].text
	for _, v := range values[1:] {
		switch v.text {
		case "IMPORTED":
			return
		case "ALIAS":
			e.symbol(name, "alias", c.line)
			return
		}
	}
	kind := "library"
	if strings.HasSuffix(c.name, "executable") {
		kind = "executable"
	}
	e.symbol(name, kind, c.line)
	var sources []value
	for _, v := range values[1:] {
		if !targetKeywords[v.text] {
			sources = append(sources, v)
		}
	}
	e.sources(c, name, sources, "")
}

// sources records the files of a target: values that look like a file (an
// extension, no generator expression), after the keyword `only` when one is given,
// skipping target_sources' scope keywords and FILE_SET's name, TYPE and BASE_DIRS.
func (e *extractor) sources(c command, target string, values []value, only string) {
	on := only == ""
	skip := 0
	for _, v := range values {
		if isKeyword(v.text) {
			switch v.text {
			case "FILE_SET", "TYPE":
				skip = 1
			case "BASE_DIRS":
				skip = -1 // up to the next keyword
			default:
				skip = 0
			}
			if only != "" {
				on = v.text == only
			}
			continue
		}
		if skip != 0 {
			if skip > 0 {
				skip--
			}
			continue
		}
		if !on || strings.Contains(v.text, "$<") || path.Ext(v.text) == "" || strings.HasSuffix(v.text, "/") {
			continue
		}
		e.resolveImport(c.cased+"("+target+" "+v.written+")", v.text, kindSource, v.line)
	}
}

// fetch records a FetchContent_Declare() or ExternalProject_Add(): content from a git
// repository at a GIT_TAG, or downloaded from a URL with an optional URL_HASH (or
// URL_MD5). Content already in a SOURCE_DIR is that directory's CMakeLists.txt;
// Subversion and Mercurial content is not recorded.
//
// Implements: REQ-CMAKE-007
func (e *extractor) fetch(c command, name string, keyValue map[string][]string, spec string, line int) {
	var rawImport lang.RawImport
	switch {
	case firstOf(keyValue, "GIT_REPOSITORY") != "":
		rawImport = lang.RawImport{Spec: spec, Module: firstOf(keyValue, "GIT_REPOSITORY") + "\n" + firstOf(keyValue, "GIT_TAG"), Name: kindGit, Line: line}
	case firstOf(keyValue, "URL") != "":
		hash := firstOf(keyValue, "URL_HASH")
		if md5 := firstOf(keyValue, "URL_MD5"); hash == "" && md5 != "" {
			hash = "MD5=" + md5
		}
		rawImport = lang.RawImport{Spec: spec, Module: firstOf(keyValue, "URL") + "\n" + hash, Name: kindURL, Line: line}
	default:
		if directory := firstOf(keyValue, "SOURCE_DIR"); directory != "" && c.name == "fetchcontent_declare" {
			e.resolveImport(spec, directory, kindSubdirectory, line) // content already in the tree
		}
		return
	}
	e.declare(name, rawImport)
}

func (e *extractor) declare(name string, rawImport lang.RawImport) {
	if len(e.saved) > 0 && !known(rawImport.Module) {
		return
	}
	e.info.imports = append(e.info.imports, rawImport)
	if _, ok := e.info.fetches[strings.ToLower(name)]; !ok && name != "" {
		e.info.fetches[strings.ToLower(name)] = rawImport
	}
}

// cpmHosts are CPM.cmake's shorthand prefixes.
var cpmHosts = map[string]string{"gh": "github.com", "gl": "gitlab.com", "bb": "bitbucket.org"}

// cpm records CPM.cmake's CPMAddPackage(), CPMFindPackage() and
// CPMDeclarePackage(), in the single-argument shorthand
// ("gh:owner/repo@1.2.3", "gh:owner/repo#tag", "https://host/repo.git@1.2.3", an
// archive URL) or with keywords (NAME, VERSION, GIT_TAG, GITHUB_REPOSITORY,
// GITLAB_REPOSITORY, BITBUCKET_REPOSITORY, GIT_REPOSITORY, URL, URL_HASH). As in
// CPM, a VERSION without a GIT_TAG fetches the tag v<VERSION>. A SOURCE_DIR is
// that directory's CMakeLists.txt; a keyword form naming no source uses a package
// declared elsewhere.
//
// Implements: REQ-CMAKE-007
func (e *extractor) cpm(c command, values []value) {
	first := values[0].text
	line := values[0].line
	var keyValue map[string][]string
	name, repository, url, tag, hash, version := "", "", "", "", "", ""
	if c.name == "cpmdeclarepackage" {
		name, keyValue = first, keyValues(texts(values[1:]))
	} else if !isKeyword(first) {
		keyValue = keyValues(texts(values[1:]))
		repository, url, tag, version = cpmShorthand(first)
	} else {
		keyValue = keyValues(texts(values))
	}
	if n := firstOf(keyValue, "NAME"); n != "" {
		name = n
	}
	if v := firstOf(keyValue, "VERSION"); v != "" {
		version = v
	}
	if t := firstOf(keyValue, "GIT_TAG"); t != "" {
		tag = t
	}
	for _, h := range [][2]string{{"GITHUB", "gh"}, {"GITLAB", "gl"}, {"BITBUCKET", "bb"}} {
		if r := firstOf(keyValue, h[0]+"_REPOSITORY"); r != "" {
			repository = "https://" + cpmHosts[h[1]] + "/" + r
		}
	}
	if r := firstOf(keyValue, "GIT_REPOSITORY"); r != "" {
		repository = r
	}
	if u := firstOf(keyValue, "URL"); u != "" {
		url = u
	}
	hash = firstOf(keyValue, "URL_HASH")
	name = cmp.Or(name, path.Base(strings.TrimSuffix(strings.TrimSuffix(repository, "/"), ".git")))
	shown := first
	if isKeyword(first) || c.name == "cpmdeclarepackage" {
		shown = name
	}
	spec := c.cased + "(" + shown + ")"
	switch {
	case repository != "":
		if tag == "" && version != "" {
			tag = "v" + version
		}
		e.declare(name, lang.RawImport{Spec: spec, Module: repository + "\n" + tag, Name: kindGit, Line: line})
	case url != "":
		e.declare(name, lang.RawImport{Spec: spec, Module: url + "\n" + hash, Name: kindURL, Line: line})
	case firstOf(keyValue, "SOURCE_DIR") != "":
		e.resolveImport(spec, firstOf(keyValue, "SOURCE_DIR"), kindSubdirectory, line) // a package in the tree
	case name != "" && c.name != "cpmdeclarepackage":
		e.resolveImport(spec, strings.ToLower(name), kindUse, line)
	}
}

// cpmShorthand reads CPM's single-argument form: [gh|gl|bb]:owner/repo, or a git
// URL, with @version and/or #tag; any other URL is an archive.
func cpmShorthand(s string) (repository, url, tag, version string) {
	rest, t, _ := strings.Cut(s, "#")
	tag = t
	if scheme, r, ok := strings.Cut(rest, ":"); ok && cpmHosts[scheme] != "" {
		r, version, _ = strings.Cut(r, "@")
		return "https://" + cpmHosts[scheme] + "/" + r, "", tag, version
	}
	if i := strings.LastIndex(rest, "@"); i > strings.Index(rest, "://")+3 && strings.Contains(rest, "://") {
		// https://host/repo.git@1.2.3 - an @ after the host part, not user@host
		if strings.Contains(rest[i:], "/") {
			return "", rest, tag, "" // user@host/path: an archive
		}
		return rest[:i], "", tag, rest[i+1:]
	}
	lower := strings.ToLower(rest)
	if strings.HasSuffix(lower, ".git") || tag != "" {
		return rest, "", tag, ""
	}
	return "", rest, "", ""
}
