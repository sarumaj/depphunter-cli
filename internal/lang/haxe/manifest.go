package haxe

import (
	"bytes"
	"cmp"
	"encoding/json"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// hxmlArgument is one argument of an .hxml file with the line it is on.
type hxmlArgument struct {
	text string
	line int
}

// valueFlags are the compiler flags that take a value (the next argument).
var valueFlags = map[string]bool{
	"-cp": true, "-p": true, "--class-path": true, "-lib": true, "-L": true, "--library": true,
	"-main": true, "-m": true, "--main": true, "-D": true, "--define": true, "--run": true,
	"-js": true, "--js": true, "-swf": true, "--swf": true, "-neko": true, "--neko": true, "-php": true, "--php": true,
	"-cpp": true, "--cpp": true, "-cppia": true, "--cppia": true, "-cs": true, "--cs": true, "-java": true, "--java": true,
	"-jvm": true, "--jvm": true, "-python": true, "--python": true, "-lua": true, "--lua": true, "-hl": true, "--hl": true,
	"-x": true, "-resource": true, "--resource": true, "-r": true, "-xml": true, "--xml": true, "-C": true, "--cwd": true,
	"--cmd": true, "-dce": true, "--dce": true, "--macro": true, "--remap": true, "--swf-version": true,
	"--swf-header": true, "--swf-lib": true, "--swf-lib-extern": true, "--java-lib": true, "--java-lib-extern": true,
	"--net-lib": true, "--net-std": true, "--c-arg": true, "--display": true, "--wait": true, "--connect": true,
	"--server-listen": true, "--server-connect": true, "--error-format": true, "--json": true, "-w": true,
	"--custom-target": true, "--hxb": true, "--hxb-lib": true, "--neko-lib-path": true,
}

// hxmlArguments splits an .hxml file into arguments as the compiler does: a line
// starting with - is a flag and, after its first blank, its value; any other
// line is one argument; # starts a comment line.
func hxmlArguments(source []byte) []hxmlArgument {
	var out []hxmlArgument
	for n, line := range strings.Split(string(source), "\n") {
		s := strings.TrimSpace(line)
		if s == "" || s[0] == '#' {
			continue
		}
		if s[0] == '-' {
			if i := strings.IndexAny(s, " \t"); i > 0 {
				out = append(out, hxmlArgument{s[:i], n + 1}, hxmlArgument{strings.TrimSpace(s[i+1:]), n + 1})
				continue
			}
		}
		out = append(out, hxmlArgument{s, n + 1})
	}
	return out
}

// hxml is what an .hxml file says about the build: class paths (relative to
// the file, after --cwd), libraries, main classes, root modules and the .hxml
// files it includes, over all its --next sections.
type hxml struct {
	classPaths []string
	libraries  []hxmlLibrary
	defines    map[string]string // -D name=value
	// install is lix's `# @install: lix download "<url>" into <dir>` line.
	install    string
	rawImports []lang.RawImport
}

type hxmlLibrary struct {
	name, version string
	line          int
}

var installLine = regexp.MustCompile(`^#\s*@install:\s*lix\b.*?\bdownload\s+"?([^"\s]+)`)

// readHXML reads an .hxml file.
//
// Implements: REQ-HAXE-005
func readHXML(source []byte) *hxml {
	h := &hxml{defines: map[string]string{}}
	for _, line := range strings.SplitN(string(source), "\n", 64) {
		if m := installLine.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			h.install = m[1]
			break
		}
	}
	arguments := hxmlArguments(source)
	workingDirectory := ""
	add := func(spec, module, kind string, line int) {
		h.rawImports = append(h.rawImports, lang.RawImport{Spec: spec, Module: module, Name: kind, Line: line})
	}
	for i := 0; i < len(arguments); i++ {
		a := arguments[i]
		if !strings.HasPrefix(a.text, "-") {
			switch {
			case strings.HasSuffix(a.text, ".hxml"):
				add(a.text, path.Join(workingDirectory, a.text), kindHXML, a.line)
			case dotted(a.text):
				add(a.text, a.text, kindMain, a.line) // a root module to compile
			}
			continue
		}
		flag := a.text
		if !valueFlags[flag] {
			continue
		}
		if i+1 >= len(arguments) {
			break
		}
		i++
		v := arguments[i].text
		if strings.Contains(v, "::") {
			continue // a template's placeholder: -cp ::OUTPUT_DIR::/haxe
		}
		switch flag {
		case "-C", "--cwd":
			workingDirectory = path.Join(workingDirectory, v)
		case "-cp", "-p", "--class-path":
			h.classPaths = append(h.classPaths, path.Join(workingDirectory, v))
			add(flag+" "+v, path.Join(workingDirectory, v), kindCP, a.line)
		case "-lib", "-L", "--library":
			name, version, _ := strings.Cut(v, ":")
			if name == "" || strings.ContainsAny(name, "${} ") {
				continue
			}
			h.libraries = append(h.libraries, hxmlLibrary{name, version, a.line})
			add(flag+" "+v, v, kindLibrary, a.line)
		case "-main", "-m", "--main", "--run":
			if dotted(v) {
				add(flag+" "+v, v, kindMain, a.line)
			}
		case "-D", "--define":
			name, value, _ := strings.Cut(v, "=")
			h.defines[name] = value
		case "-resource", "--resource", "-r":
			file, _, _ := strings.Cut(v, "@")
			if file != "" && !strings.Contains(file, "$") {
				add(flag+" "+v, path.Join(workingDirectory, file), kindFile, a.line)
			}
		case "--macro":
			// A macro call names classes: openfl.utils.Macro.include().
			for _, rawImport := range extractSource([]byte(v)).Imports {
				if rawImport.Name == kindReference {
					add(flag+" "+rawImport.Module, rawImport.Module, kindReference, a.line)
				}
			}
		}
	}
	return h
}

// dotted reports whether s is a module path: a.b.Main or Main.
func dotted(s string) bool {
	if s == "" {
		return false
	}
	for _, segment := range strings.Split(s, ".") {
		if segment == "" || !isIdentifierStart(segment[0]) || segment[0] == '$' {
			return false
		}
		for i := 0; i < len(segment); i++ {
			if !isIdentifierCharacter(segment[i]) {
				return false
			}
		}
	}
	return true
}

// extractHXML turns an .hxml file into imports: its libraries, class paths,
// main classes, root modules, resources, included .hxml files and the classes
// its --macro calls name.
func extractHXML(source []byte) *lang.Extraction {
	return &lang.Extraction{Imports: readHXML(source).rawImports}
}

// extractLix is the library a lix haxe_libraries/<name>.hxml pins: the file's
// own name.
func extractLix(name string, source []byte) *lang.Extraction {
	h := readHXML(source)
	line := 1
	if h.install != "" {
		for n, l := range strings.Split(string(source), "\n") {
			if strings.Contains(l, "@install") {
				line = n + 1
				break
			}
		}
	}
	return &lang.Extraction{Imports: []lang.RawImport{{Spec: name, Module: name, Name: kindLix, Line: line}}}
}

// haxelibJSON is a haxelib.json.
type haxelibJSON struct {
	Name         string         `json:"name"`
	Version      string         `json:"version"`
	ClassPath    string         `json:"classPath"`
	Dependencies map[string]any `json:"dependencies"`
}

type dependency struct {
	name, version string
	line          int
}

// readHaxelib reads a haxelib.json: the library's name and class path and its
// dependencies in the order written.
//
// Implements: REQ-HAXE-005
func readHaxelib(source []byte) (h haxelibJSON, dependencies []dependency, ok bool) {
	if json.Unmarshal(source, &h) != nil {
		return h, nil, false
	}
	dependenciesAt := bytes.Index(source, []byte(`"dependencies"`))
	for name, v := range h.Dependencies {
		s, _ := v.(string)
		line := 0
		if dependenciesAt >= 0 {
			line = lang.LineOf(source, `"`+name+`"`, dependenciesAt)
		}
		dependencies = append(dependencies, dependency{name, strings.TrimSpace(s), line})
	}
	sort.Slice(dependencies, func(i, j int) bool {
		if dependencies[i].line != dependencies[j].line {
			return dependencies[i].line < dependencies[j].line
		}
		return dependencies[i].name < dependencies[j].name
	})
	return h, dependencies, true
}

func extractHaxelib(source []byte) *lang.Extraction {
	h, dependencies, ok := readHaxelib(source)
	extraction := &lang.Extraction{}
	if !ok {
		return extraction
	}
	for _, d := range dependencies {
		m := d.name
		if d.version != "" {
			m += ":" + d.version
		}
		extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: d.name, Module: m, Name: kindLibrary, Line: d.line})
	}
	if classPath := strings.TrimSpace(h.ClassPath); classPath != "" {
		extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: "classPath " + classPath, Module: classPath, Name: kindCP, Line: max(lang.LineOf(source, `"classPath"`, 0), 1)})
	}
	return extraction
}

// limeProject reports whether an XML file is a Lime/OpenFL project file: its
// root element is <project> (or <extension>, a library's include.xml) and it
// names a haxelib, a source path or a class path. A project.xml of another
// tool is not.
//
// Implements: REQ-HAXE-001
func limeProject(source []byte) bool {
	root := ""
	xmlTags(source, func(name string, _ map[string]string, _ int) bool {
		root = name
		return false
	})
	if root != "project" && root != "extension" {
		return false
	}
	return bytes.Contains(source, []byte("<haxelib")) || bytes.Contains(source, []byte("<source")) ||
		bytes.Contains(source, []byte("<classpath")) || bytes.Contains(source, []byte("<include haxelib"))
}

var entities = strings.NewReplacer("&amp;", "&", "&lt;", "<", "&gt;", ">", "&quot;", `"`, "&apos;", "'")

func xmlNameCharacter(c byte) bool {
	return isIdentifierCharacter(c) || c == '-' || c == ':' || c == '.'
}

// xmlTags calls function with every start tag's name, attributes and line, in order,
// until function returns false. It is a tag scanner, not a parser: project files
// hold what XML forbids (if="${a < b}"), which Lime reads anyway.
func xmlTags(source []byte, function func(name string, attributes map[string]string, line int) bool) {
	i, line, counted := 0, 1, 0
	for i < len(source) {
		j := bytes.IndexByte(source[i:], '<')
		if j < 0 {
			return
		}
		i += j
		rest := source[i:]
		switch {
		case bytes.HasPrefix(rest, []byte("<!--")):
			k := bytes.Index(rest[4:], []byte("-->"))
			if k < 0 {
				return
			}
			i += 4 + k + 3
			continue
		case bytes.HasPrefix(rest, []byte("<?")), bytes.HasPrefix(rest, []byte("<!")), bytes.HasPrefix(rest, []byte("</")):
			k := bytes.IndexByte(rest, '>')
			if k < 0 {
				return
			}
			i += k + 1
			continue
		}
		n := i + 1
		for n < len(source) && xmlNameCharacter(source[n]) {
			n++
		}
		if n == i+1 {
			i++
			continue
		}
		name := string(source[i+1 : n])
		attributes := map[string]string{}
		k := n
		for k < len(source) {
			c := source[k]
			if c == '>' {
				k++
				break
			}
			a := k
			for k < len(source) && xmlNameCharacter(source[k]) {
				k++
			}
			if k == a {
				k++ // a blank, a / or a stray character
				continue
			}
			key := string(source[a:k])
			for k < len(source) && (source[k] == ' ' || source[k] == '\t' || source[k] == '\n' || source[k] == '\r') {
				k++
			}
			if k >= len(source) || source[k] != '=' {
				attributes[key] = ""
				continue
			}
			k++
			for k < len(source) && (source[k] == ' ' || source[k] == '\t' || source[k] == '\n' || source[k] == '\r') {
				k++
			}
			if k < len(source) && (source[k] == '"' || source[k] == '\'') {
				q := source[k]
				e := bytes.IndexByte(source[k+1:], q)
				if e < 0 {
					e = len(source) - k - 1
				}
				attributes[key] = entities.Replace(string(source[k+1 : k+1+e]))
				k += e + 2
			} else {
				v := k
				for k < len(source) && source[k] != '>' && source[k] != ' ' && source[k] != '\t' && source[k] != '\n' && source[k] != '/' {
					k++
				}
				attributes[key] = string(source[v:k])
			}
		}
		line += bytes.Count(source[counted:i], []byte("\n"))
		counted = i
		if !function(name, attributes, line) {
			return
		}
		i = min(k, len(source))
	}
}

// project is what a Lime/OpenFL project file declares.
type project struct {
	libraries  []dependency
	sources    []string
	rawImports []lang.RawImport
}

// readProject reads a Lime/OpenFL project file: <haxelib name version>,
// <include haxelib>, <source path>, <classpath name|path>, <app main> and
// <include path>. Conditions (if, unless) are not evaluated; values with ${}
// are skipped.
//
// Implements: REQ-HAXE-005
func readProject(source []byte) *project {
	p := &project{}
	if !limeProject(source) {
		return p
	}
	add := func(spec, module, kind string, line int) {
		p.rawImports = append(p.rawImports, lang.RawImport{Spec: spec, Module: module, Name: kind, Line: line})
	}
	xmlTags(source, func(tag string, attributes map[string]string, line int) bool {
		attribute := func(name string) string {
			if v := strings.TrimSpace(attributes[name]); !strings.Contains(v, "$") && !strings.Contains(v, "::") {
				return v
			}
			return ""
		}
		switch tag {
		case "haxelib":
			if n := attribute("name"); n != "" {
				v := attribute("version")
				p.libraries = append(p.libraries, dependency{n, v, line})
				m := n
				if v != "" {
					m += ":" + v
				}
				add(n, m, kindLibrary, line)
			}
		case "include":
			if n := attribute("haxelib"); n != "" {
				p.libraries = append(p.libraries, dependency{n, "", line})
				add(n, n, kindLibrary, line)
			} else if f := attribute("path"); f != "" {
				add("include "+f, f, kindFile, line)
			}
		case "source", "classpath":
			f := cmp.Or(attribute("path"), attribute("name"))
			if f != "" {
				p.sources = append(p.sources, f)
				add(tag+" "+f, f, kindCP, line)
			}
		case "app":
			if m := attribute("main"); dotted(m) {
				add("main "+m, m, kindMain, line)
			}
		}
		return true
	})
	return p
}

func extractProject(source []byte) *lang.Extraction {
	return &lang.Extraction{Imports: readProject(source).rawImports}
}
