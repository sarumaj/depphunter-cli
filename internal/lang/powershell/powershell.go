// Package powershell analyzes PowerShell scripts (.ps1), modules (.psm1) and module
// manifests (.psd1). Dependencies are `using module`, Import-Module, dot-sourced and
// &-invoked scripts, `#Requires -Modules`, and a manifest's RequiredModules,
// RootModule, NestedModules and ScriptsToProcess.
//
// Parsing uses the small scanner in scan.go, not tree-sitter: the pure-Go PowerShell
// grammar turns common syntax (e.g. `Import-Module -Name A, B`) into error nodes that
// swallow the following lines. Dependencies and definitions are statement-level, so
// a scanner that understands comments, strings, here-strings, continuations and
// braces is enough.
package powershell

import (
	"path"
	"regexp"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

const (
	ecosystemGallery = "psgallery"
	ecosystemBuiltin = "powershell"
)

// RawImport.Name values: how a reference is to be resolved.
const (
	referencePath     = "path"     // a script or module file path
	referenceModule   = "module"   // a module name (or path)
	referenceRequires = "requires" // a declared module requirement, optionally "requires=<version>"
)

// Implements: REQ-PS-011
type Plugin struct{}

func (Plugin) Name() string { return "powershell" }
func (Plugin) Version() int { return 2 }
func (Plugin) Claims(f *scan.File) bool {
	switch strings.ToLower(path.Ext(f.Path)) {
	case ".ps1", ".psm1", ".psd1":
		return !f.Binary
	}
	return false
}
func (Plugin) Ecosystems() []lang.Ecosystem {
	return []lang.Ecosystem{
		{ID: ecosystemGallery, Name: "PowerShell Gallery"},
		{ID: ecosystemBuiltin, Name: "PowerShell built-in modules", Std: true},
	}
}

func (Plugin) Resolver(root string, all []*scan.File) (lang.Resolver, error) {
	return newResolver(all), nil
}

var (
	requiresModule     = regexp.MustCompile(`(?i)^#requires\b.*?-modules\s+(.+?)(?:\s+-\w+\b.*)?$`)
	functionDefinition = regexp.MustCompile(`(?i)^(?:function|filter|workflow)\s+(?:[a-z]+:)?([\w\-.]+)`)
	classDefinition    = regexp.MustCompile(`(?i)^class\s+([A-Za-z_]\w*)`)
	methodDefinition   = regexp.MustCompile(`(?i)^(?:(?:static|hidden)\s+)*(?:\[[^\]]+\]\s*)?([A-Za-z_]\w*)\s*\(`)
	manifestKey        = regexp.MustCompile(`(?im)^\s*(RequiredModules|RootModule|NestedModules|ScriptsToProcess)\s*=\s*`)
)

// Words that look like method headers inside class bodies but are statements.
var keywords = map[string]bool{"if": true, "elseif": true, "foreach": true, "for": true, "while": true, "switch": true, "until": true, "catch": true, "return": true, "throw": true}

// Implements: REQ-PS-005, REQ-PS-006
func (Plugin) Extract(f *scan.File, source []byte) (*lang.Extraction, error) {
	extraction := &lang.Extraction{}
	var symbols lang.SymbolSet
	add := func(spec, module, how string, line int) {
		if module != "" {
			extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: spec, Module: module, Name: how, Line: line})
		}
	}
	statements, comments := scanStatements(string(source))
	for _, c := range comments {
		if m := requiresModule.FindStringSubmatch(c.text); m != nil {
			for _, module := range moduleSpecs(m[1]) {
				add("#Requires -Modules "+module.name, module.name, module.requires(), c.line)
			}
		}
	}
	for _, statement := range statements {
		for _, r := range commandReferences(statement.text) {
			add(r.spec, r.module, r.how, statement.line)
		}
		switch {
		case functionDefinition.MatchString(statement.text):
			symbols.Add(functionDefinition.FindStringSubmatch(statement.text)[1], "func", statement.line)
		case classDefinition.MatchString(statement.text):
			symbols.Add(classDefinition.FindStringSubmatch(statement.text)[1], "class", statement.line)
		case statement.class != "":
			if m := methodDefinition.FindStringSubmatch(statement.text); m != nil && !keywords[strings.ToLower(m[1])] {
				symbols.Add(statement.class+"."+m[1], "method", statement.line)
			}
		}
	}
	if strings.EqualFold(path.Ext(f.Path), ".psd1") {
		text := stripComments(string(source))
		for _, span := range manifestKey.FindAllStringSubmatchIndex(text, -1) {
			key := text[span[2]:span[3]]
			value := valueAt(text[span[1]:])
			line := strings.Count(text[:span[0]], "\n") + 1
			if strings.EqualFold(key, "RequiredModules") {
				for _, module := range moduleSpecs(value) {
					add(key+": "+module.name, module.name, module.requires(), line)
				}
				continue
			}
			for _, s := range quoted(value) {
				add(key+": "+s, s, referencePath, line)
			}
		}
	}
	extraction.Symbols = symbols.List()
	return extraction, nil
}

type reference struct{ spec, module, how string }

// commandReferences reads the dependencies a single command expresses.
var assignment = regexp.MustCompile(`^\$[\w:]+(?:\.\w+)*\s*[+]?=\s*`)

// Implements: REQ-PS-001, REQ-PS-002, REQ-PS-003
func commandReferences(text string) []reference {
	// $psGet = Import-Module PowerShellGet -PassThru
	fields := psFields(assignment.ReplaceAllString(text, ""))
	if len(fields) < 2 {
		return nil
	}
	switch command := strings.ToLower(fields[0]); command {
	case "using":
		if strings.EqualFold(fields[1], "module") && len(fields) > 2 {
			return []reference{{"using module " + fields[2], unquote(fields[2]), referenceModule}}
		}
	case "import-module", "ipmo":
		var out []reference
		for _, name := range importModuleNames(fields[1:]) {
			out = append(out, reference{"Import-Module " + name, name, referenceModule})
		}
		return out
	case ".", "&":
		target := unquote(fields[1])
		switch strings.ToLower(path.Ext(target)) {
		case ".ps1", ".psm1", ".psd1":
			return []reference{{command + " " + fields[1], target, referencePath}}
		}
	}
	return nil
}

// Import-Module parameters that take a value, so their value is not a module name.
var valueParameters = map[string]bool{
	"-requiredversion": true, "-minimumversion": true, "-maximumversion": true, "-prefix": true,
	"-argumentlist": true, "-scope": true, "-function": true, "-cmdlet": true, "-variable": true,
	"-alias": true, "-psession": true, "-cimsession": true, "-version": true,
}

// Implements: REQ-PS-002
func importModuleNames(arguments []string) []string {
	var names []string
	collecting := false
	for i := 0; i < len(arguments); i++ {
		a := arguments[i]
		if strings.HasPrefix(a, "-") {
			p := strings.ToLower(strings.TrimSuffix(a, ":"))
			collecting = p == "-name"
			if valueParameters[p] {
				i++
			}
			continue
		}
		if collecting || len(names) == 0 {
			for _, n := range strings.Split(a, ",") {
				if n = unquote(strings.TrimSpace(n)); n != "" {
					names = append(names, n)
				}
			}
			collecting = strings.HasSuffix(a, ",")
		}
	}
	return names
}

// moduleSpec is a module requirement: RequiredVersion names one version, while
// ModuleVersion is a minimum the installed module may exceed.
//
// Implements: REQ-PS-010
type moduleSpec struct {
	name, version string
	exact         bool
}

func (m moduleSpec) requires() string {
	if m.version != "" {
		if m.exact {
			return referenceRequires + "==" + m.version
		}
		return referenceRequires + "=" + m.version
	}
	return referenceRequires
}

var (
	hashtable    = regexp.MustCompile(`@\{[^}]*\}`)
	hashName     = regexp.MustCompile(`(?i)ModuleName\s*=\s*['"]([^'"]+)['"]`)
	hashVersion  = regexp.MustCompile(`(?i)(RequiredVersion|ModuleVersion)\s*=\s*['"]([^'"]+)['"]`)
	bareword     = regexp.MustCompile(`'([^']+)'|"([^"]+)"|([A-Za-z][\w.\-]*)`)
	quotedString = regexp.MustCompile(`'([^']+)'|"([^"]+)"`)
)

// moduleSpecs parses module specifications: names, quoted names and
// @{ModuleName='X'; ModuleVersion='1.0'} tables, in lists or @(...) arrays.
//
// Implements: REQ-PS-005, REQ-PS-006, REQ-PS-010
func moduleSpecs(s string) []moduleSpec {
	var out []moduleSpec
	for _, t := range hashtable.FindAllString(s, -1) {
		if n := hashName.FindStringSubmatch(t); n != nil {
			spec := moduleSpec{name: n[1]}
			for _, v := range hashVersion.FindAllStringSubmatch(t, -1) {
				// RequiredVersion wins: a table may carry both.
				if exact := strings.EqualFold(v[1], "RequiredVersion"); exact || !spec.exact {
					spec.version, spec.exact = v[2], exact
				}
			}
			out = append(out, spec)
		}
	}
	for _, m := range bareword.FindAllStringSubmatch(hashtable.ReplaceAllString(s, ""), -1) {
		name := m[1] + m[2] + m[3]
		if name != "" {
			out = append(out, moduleSpec{name: name})
		}
	}
	return out
}

func quoted(s string) []string {
	var out []string
	for _, m := range quotedString.FindAllStringSubmatch(s, -1) {
		out = append(out, m[1]+m[2])
	}
	return out
}

func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && (s[0] == '\'' || s[0] == '"') && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1]
	}
	return s
}

// psFields splits a command into words, keeping quoted strings and @{...}/@(...)/(...)
// groups intact.
func psFields(s string) []string {
	var out []string
	var current strings.Builder
	var quote rune
	depth := 0
	flush := func() {
		if current.Len() > 0 {
			out = append(out, current.String())
			current.Reset()
		}
	}
	for _, r := range s {
		switch {
		case quote != 0:
			current.WriteRune(r)
			if r == quote {
				quote = 0
			}
		case r == '\'' || r == '"':
			quote = r
			current.WriteRune(r)
		case r == '(' || r == '{':
			depth++
			current.WriteRune(r)
		case r == ')' || r == '}':
			depth--
			current.WriteRune(r)
		case (r == ' ' || r == '\t' || r == '\n' || r == '\r') && depth <= 0:
			flush()
		default:
			current.WriteRune(r)
		}
	}
	flush()
	return out
}
