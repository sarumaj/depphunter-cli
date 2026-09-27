package haxe

import (
	"bytes"
	"encoding/json"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// hxmlArg is one argument of an .hxml file with the line it is on.
type hxmlArg struct {
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

// hxmlArgs splits an .hxml file into arguments as the compiler does: a line
// starting with - is a flag and, after its first blank, its value; any other
// line is one argument; # starts a comment line.
func hxmlArgs(src []byte) []hxmlArg {
	var out []hxmlArg
	for n, line := range strings.Split(string(src), "\n") {
		s := strings.TrimSpace(line)
		if s == "" || s[0] == '#' {
			continue
		}
		if s[0] == '-' {
			if i := strings.IndexAny(s, " \t"); i > 0 {
				out = append(out, hxmlArg{s[:i], n + 1}, hxmlArg{strings.TrimSpace(s[i+1:]), n + 1})
				continue
			}
		}
		out = append(out, hxmlArg{s, n + 1})
	}
	return out
}

// hxml is what an .hxml file says about the build: class paths (relative to
// the file, after --cwd), libraries, main classes, root modules and the .hxml
// files it includes, over all its --next sections.
type hxml struct {
	cps  []string
	libs []hxmlLib
	defs map[string]string // -D name=value
	// install is lix's `# @install: lix download "<url>" into <dir>` line.
	install string
	imps    []lang.RawImport
}

type hxmlLib struct {
	name, version string
	line          int
}

var installLine = regexp.MustCompile(`^#\s*@install:\s*lix\b.*?\bdownload\s+"?([^"\s]+)`)

// readHXML reads an .hxml file.
//
// Implements: REQ-HAXE-005
func readHXML(src []byte) *hxml {
	h := &hxml{defs: map[string]string{}}
	for _, line := range strings.SplitN(string(src), "\n", 64) {
		if m := installLine.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			h.install = m[1]
			break
		}
	}
	args := hxmlArgs(src)
	cwd := ""
	add := func(spec, module, kind string, line int) {
		h.imps = append(h.imps, lang.RawImport{Spec: spec, Module: module, Name: kind, Line: line})
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a.text, "-") {
			switch {
			case strings.HasSuffix(a.text, ".hxml"):
				add(a.text, path.Join(cwd, a.text), kindHXML, a.line)
			case dotted(a.text):
				add(a.text, a.text, kindMain, a.line) // a root module to compile
			}
			continue
		}
		flag := a.text
		if !valueFlags[flag] {
			continue
		}
		if i+1 >= len(args) {
			break
		}
		i++
		v := args[i].text
		if strings.Contains(v, "::") {
			continue // a template's placeholder: -cp ::OUTPUT_DIR::/haxe
		}
		switch flag {
		case "-C", "--cwd":
			cwd = path.Join(cwd, v)
		case "-cp", "-p", "--class-path":
			h.cps = append(h.cps, path.Join(cwd, v))
			add(flag+" "+v, path.Join(cwd, v), kindCP, a.line)
		case "-lib", "-L", "--library":
			name, version, _ := strings.Cut(v, ":")
			if name == "" || strings.ContainsAny(name, "${} ") {
				continue
			}
			h.libs = append(h.libs, hxmlLib{name, version, a.line})
			add(flag+" "+v, v, kindLib, a.line)
		case "-main", "-m", "--main", "--run":
			if dotted(v) {
				add(flag+" "+v, v, kindMain, a.line)
			}
		case "-D", "--define":
			name, value, _ := strings.Cut(v, "=")
			h.defs[name] = value
		case "-resource", "--resource", "-r":
			file, _, _ := strings.Cut(v, "@")
			if file != "" && !strings.Contains(file, "$") {
				add(flag+" "+v, path.Join(cwd, file), kindFile, a.line)
			}
		case "--macro":
			// A macro call names classes: openfl.utils.Macro.include().
			for _, im := range extractSource([]byte(v)).Imports {
				if im.Name == kindRef {
					add(flag+" "+im.Module, im.Module, kindRef, a.line)
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
	for _, seg := range strings.Split(s, ".") {
		if seg == "" || !isIdentStart(seg[0]) || seg[0] == '$' {
			return false
		}
		for i := 0; i < len(seg); i++ {
			if !isIdentChar(seg[i]) {
				return false
			}
		}
	}
	return true
}

// extractHXML turns an .hxml file into imports: its libraries, class paths,
// main classes, root modules, resources, included .hxml files and the classes
// its --macro calls name.
func extractHXML(src []byte) *lang.Extraction {
	return &lang.Extraction{Imports: readHXML(src).imps}
}

// extractLix is the library a lix haxe_libraries/<name>.hxml pins: the file's
// own name.
func extractLix(name string, src []byte) *lang.Extraction {
	h := readHXML(src)
	line := 1
	if h.install != "" {
		for n, l := range strings.Split(string(src), "\n") {
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

type dep struct {
	name, version string
	line          int
}

// readHaxelib reads a haxelib.json: the library's name and class path and its
// dependencies in the order written.
//
// Implements: REQ-HAXE-005
func readHaxelib(src []byte) (h haxelibJSON, deps []dep, ok bool) {
	if json.Unmarshal(src, &h) != nil {
		return h, nil, false
	}
	text := string(src)
	depsAt := strings.Index(text, `"dependencies"`)
	for name, v := range h.Dependencies {
		s, _ := v.(string)
		line := 0
		if depsAt >= 0 {
			if i := strings.Index(text[depsAt:], `"`+name+`"`); i >= 0 {
				line = 1 + strings.Count(text[:depsAt+i], "\n")
			}
		}
		deps = append(deps, dep{name, strings.TrimSpace(s), line})
	}
	sort.Slice(deps, func(i, j int) bool {
		if deps[i].line != deps[j].line {
			return deps[i].line < deps[j].line
		}
		return deps[i].name < deps[j].name
	})
	return h, deps, true
}

func extractHaxelib(src []byte) *lang.Extraction {
	h, deps, ok := readHaxelib(src)
	ex := &lang.Extraction{}
	if !ok {
		return ex
	}
	for _, d := range deps {
		m := d.name
		if d.version != "" {
			m += ":" + d.version
		}
		ex.Imports = append(ex.Imports, lang.RawImport{Spec: d.name, Module: m, Name: kindLib, Line: d.line})
	}
	if cp := strings.TrimSpace(h.ClassPath); cp != "" {
		line := 1 + strings.Count(string(src[:max(0, bytes.Index(src, []byte(`"classPath"`)))]), "\n")
		ex.Imports = append(ex.Imports, lang.RawImport{Spec: "classPath " + cp, Module: cp, Name: kindCP, Line: line})
	}
	return ex
}

// limeProject reports whether an XML file is a Lime/OpenFL project file: its
// root element is <project> (or <extension>, a library's include.xml) and it
// names a haxelib, a source path or a class path. A project.xml of another
// tool is not.
//
// Implements: REQ-HAXE-001
func limeProject(src []byte) bool {
	root := ""
	xmlTags(src, func(name string, _ map[string]string, _ int) bool {
		root = name
		return false
	})
	if root != "project" && root != "extension" {
		return false
	}
	return bytes.Contains(src, []byte("<haxelib")) || bytes.Contains(src, []byte("<source")) ||
		bytes.Contains(src, []byte("<classpath")) || bytes.Contains(src, []byte("<include haxelib"))
}

var entities = strings.NewReplacer("&amp;", "&", "&lt;", "<", "&gt;", ">", "&quot;", `"`, "&apos;", "'")

func xmlNameChar(c byte) bool { return isIdentChar(c) || c == '-' || c == ':' || c == '.' }

// xmlTags calls fn with every start tag's name, attributes and line, in order,
// until fn returns false. It is a tag scanner, not a parser: project files
// hold what XML forbids (if="${a < b}"), which Lime reads anyway.
func xmlTags(src []byte, fn func(name string, attrs map[string]string, line int) bool) {
	i, line, counted := 0, 1, 0
	for i < len(src) {
		j := bytes.IndexByte(src[i:], '<')
		if j < 0 {
			return
		}
		i += j
		rest := src[i:]
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
		for n < len(src) && xmlNameChar(src[n]) {
			n++
		}
		if n == i+1 {
			i++
			continue
		}
		name := string(src[i+1 : n])
		attrs := map[string]string{}
		k := n
		for k < len(src) {
			c := src[k]
			if c == '>' {
				k++
				break
			}
			a := k
			for k < len(src) && xmlNameChar(src[k]) {
				k++
			}
			if k == a {
				k++ // a blank, a / or a stray character
				continue
			}
			key := string(src[a:k])
			for k < len(src) && (src[k] == ' ' || src[k] == '\t' || src[k] == '\n' || src[k] == '\r') {
				k++
			}
			if k >= len(src) || src[k] != '=' {
				attrs[key] = ""
				continue
			}
			k++
			for k < len(src) && (src[k] == ' ' || src[k] == '\t' || src[k] == '\n' || src[k] == '\r') {
				k++
			}
			if k < len(src) && (src[k] == '"' || src[k] == '\'') {
				q := src[k]
				e := bytes.IndexByte(src[k+1:], q)
				if e < 0 {
					e = len(src) - k - 1
				}
				attrs[key] = entities.Replace(string(src[k+1 : k+1+e]))
				k += e + 2
			} else {
				v := k
				for k < len(src) && src[k] != '>' && src[k] != ' ' && src[k] != '\t' && src[k] != '\n' && src[k] != '/' {
					k++
				}
				attrs[key] = string(src[v:k])
			}
		}
		line += bytes.Count(src[counted:i], []byte("\n"))
		counted = i
		if !fn(name, attrs, line) {
			return
		}
		i = min(k, len(src))
	}
}

// project is what a Lime/OpenFL project file declares.
type project struct {
	libs    []dep
	sources []string
	imps    []lang.RawImport
}

// readProject reads a Lime/OpenFL project file: <haxelib name version>,
// <include haxelib>, <source path>, <classpath name|path>, <app main> and
// <include path>. Conditions (if, unless) are not evaluated; values with ${}
// are skipped.
//
// Implements: REQ-HAXE-005
func readProject(src []byte) *project {
	p := &project{}
	if !limeProject(src) {
		return p
	}
	add := func(spec, module, kind string, line int) {
		p.imps = append(p.imps, lang.RawImport{Spec: spec, Module: module, Name: kind, Line: line})
	}
	xmlTags(src, func(tag string, attrs map[string]string, line int) bool {
		attr := func(name string) string {
			if v := strings.TrimSpace(attrs[name]); !strings.Contains(v, "$") && !strings.Contains(v, "::") {
				return v
			}
			return ""
		}
		switch tag {
		case "haxelib":
			if n := attr("name"); n != "" {
				v := attr("version")
				p.libs = append(p.libs, dep{n, v, line})
				m := n
				if v != "" {
					m += ":" + v
				}
				add(n, m, kindLib, line)
			}
		case "include":
			if n := attr("haxelib"); n != "" {
				p.libs = append(p.libs, dep{n, "", line})
				add(n, n, kindLib, line)
			} else if f := attr("path"); f != "" {
				add("include "+f, f, kindFile, line)
			}
		case "source", "classpath":
			f := attr("path")
			if f == "" {
				f = attr("name")
			}
			if f != "" {
				p.sources = append(p.sources, f)
				add(tag+" "+f, f, kindCP, line)
			}
		case "app":
			if m := attr("main"); dotted(m) {
				add("main "+m, m, kindMain, line)
			}
		}
		return true
	})
	return p
}

func extractProject(src []byte) *lang.Extraction {
	return &lang.Extraction{Imports: readProject(src).imps}
}
