package puppet

import (
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// Import kinds, carried in RawImport.Name. A manifest's references name a
// class, a defined (or custom) resource type, a function, a type alias, or a
// file a module serves; the manifests' entries name modules.
const (
	kindClass    = "class"    // include/require/contain, class { 'x': }, Class['x'], inherits
	kindDefine   = "define"   // x::y { 'title': }, X::Y['title'], X::Y <| |>
	kindFunction = "function" // x::y(...)
	kindType     = "type"     // X::Y as a data type (a type alias)
	kindTemplate = "template" // template('mod/x.erb'), epp('mod/x.epp')
	kindFile     = "file"     // file('mod/x'), 'puppet:///modules/mod/x'
	kindModule   = "puppetfile"
	kindMetadata = "metadata"
	kindFixture  = "fixtures"
)

// keyword holds the words that are not names of classes, types or
// functions.
var keyword = map[string]bool{
	"and": true, "application": true, "attr": true, "case": true, "class": true, "consumes": true, "default": true,
	"define": true, "else": true, "elsif": true, "false": true, "function": true, "if": true, "import": true,
	"in": true, "inherits": true, "node": true, "or": true, "plan": true, "private": true, "produces": true,
	"site": true, "true": true, "type": true, "undef": true, "unless": true,
}

// Puppet's own resource types (with those the agent bundles from the *_core
// modules) and built-in classes: declaring one is not a dependency.
var coreType = map[string]bool{
	"augeas": true, "component": true, "cron": true, "exec": true, "file": true, "filebucket": true, "group": true,
	"host": true, "k5login": true, "mailalias": true, "maillist": true, "mount": true, "notify": true,
	"package": true, "resources": true, "schedule": true, "scheduled_task": true, "selboolean": true,
	"selmodule": true, "service": true, "ssh_authorized_key": true, "sshkey": true, "stage": true, "tidy": true,
	"user": true, "whit": true, "yumrepo": true, "zfs": true, "zone": true, "zpool": true,
	"class": true, "node": true, "main": true, "settings": true, "resource": true,
}

// The functions of puppetlabs-stdlib called without a namespace (its
// pre-4.x API): calling one is a dependency on stdlib.
var stdlibFunction = map[string]bool{
	"any2array": true, "any2bool": true, "assert_private": true, "base64": true, "basename": true, "bool2num": true,
	"bool2str": true, "clamp": true, "concat": true, "count": true, "deep_merge": true, "defined_with_params": true,
	"delete": true, "delete_at": true, "delete_regex": true, "delete_undef_values": true, "delete_values": true,
	"deprecation": true, "difference": true, "dirname": true, "dos2unix": true, "enclose_ipv6": true,
	"ensure_packages": true, "ensure_resource": true, "ensure_resources": true, "fqdn_rand_string": true,
	"fqdn_rotate": true, "fqdn_uuid": true, "get_module_path": true, "getparam": true, "glob": true, "grep": true,
	"has_interface_with": true, "has_ip_address": true, "has_ip_network": true, "has_key": true, "intersection": true,
	"is_absolute_path": true, "is_array": true, "is_bool": true, "is_hash": true, "is_integer": true,
	"is_string": true, "join_keys_to_values": true, "load_module_metadata": true, "loadjson": true,
	"loadyaml": true, "member": true, "merge": true, "num2bool": true, "parsejson": true, "parseyaml": true,
	"pick": true, "pick_default": true, "prefix": true, "pry": true, "pw_hash": true, "range": true,
	"regexpescape": true, "reject": true, "seeded_rand": true, "shell_escape": true, "shell_join": true,
	"shell_split": true, "shuffle": true, "squeeze": true, "str2bool": true, "str2saltedsha512": true,
	"suffix": true, "swapcase": true, "time": true, "to_bytes": true, "to_json": true, "to_json_pretty": true,
	"to_yaml": true, "try_get_value": true, "union": true, "unix2dos": true, "uriescape": true,
	"validate_absolute_path": true, "validate_array": true, "validate_augeas": true, "validate_bool": true,
	"validate_cmd": true, "validate_hash": true, "validate_integer": true, "validate_ip_address": true,
	"validate_legacy": true, "validate_re": true, "validate_slength": true, "validate_string": true,
	"values_at": true, "zip": true,
}

// extractSource reads a Puppet manifest: what it declares (classes, defined
// types, nodes, functions, type aliases, plans) and what it refers to.
//
// Implements: REQ-PUPPET-002, REQ-PUPPET-003
func extractSource(source []byte) *lang.Extraction {
	x := &extractor{tokens: lex(source), extraction: &lang.Extraction{}, seen: map[string]bool{}}
	if pascal(x.tokens) {
		return x.extraction
	}
	for x.i = 0; x.i < len(x.tokens); x.i++ {
		switch t := x.tokens[x.i]; t.kind {
		case tString:
			if rest, ok := strings.CutPrefix(t.text, "puppet:///modules/"); ok && !t.interpolate {
				x.reference(kindFile, rest, t.text, t.line)
			}
		case tReference:
			x.typeReference(t)
		case tName:
			if previous := x.get(x.i - 1); previous.kind != tPunctuation || previous.text != "." { // not $x.each, a method call
				x.name(t)
			}
			// tPunctuation: @name { } and @@name { }, virtual and exported resources,
			// are declarations like any other; the name is read next.
		}
	}
	x.extraction.Symbols = x.symbols.List()
	return x.extraction
}

// extractor is extractSource's walk over a manifest's tokens: where it is (i) and
// what it has found.
type extractor struct {
	tokens     []token
	i          int
	extraction *lang.Extraction
	symbols    lang.SymbolSet
	seen       map[string]bool
}

func (x *extractor) reference(kind, module, spec string, line int) {
	module = strings.TrimPrefix(module, "::")
	if module == "" || x.seen[kind+" "+module] {
		return
	}
	x.seen[kind+" "+module] = true
	x.extraction.Imports = append(x.extraction.Imports, lang.RawImport{Spec: spec, Module: module, Name: kind, Line: line})
}

func (x *extractor) get(i int) token {
	if i >= 0 && i < len(x.tokens) {
		return x.tokens[i]
	}
	return token{kind: tPunctuation}
}

func (x *extractor) is(i int, text string) bool {
	t := x.get(i)
	return t.kind == tPunctuation && t.text == text
}

// declares reports whether what follows i declares or refers to a resource:
// Name['title'], Name { }, and the collectors Name <| |> and Name <<| |>>.
func (x *extractor) declares(i int) bool {
	return x.is(i+1, "[") && x.get(i+2).kind == tString || x.is(i+1, "{") || x.is(i+1, "<|") || x.is(i+1, "<<|")
}

// typeReference reads a capitalized name: a type alias being declared, Class['a'],
// a defined type declared or collected, or a type referred to.
func (x *extractor) typeReference(t token) {
	if previous := x.get(x.i - 1); previous.kind == tName && previous.text == "type" {
		if x.is(x.i+1, "=") {
			x.symbols.Add(t.text, "type", t.line)
		}
		return
	}
	lower := strings.ToLower(strings.TrimPrefix(t.text, "::"))
	switch {
	case t.text == "Class" && x.is(x.i+1, "["):
		for _, n := range titles(x.tokens, x.i+2) {
			if !coreType[strings.ToLower(n.text)] {
				x.reference(kindClass, strings.ToLower(n.text), "Class['"+n.text+"']", n.line)
			}
		}
	case strings.Contains(lower, "::") && x.declares(x.i):
		x.reference(kindDefine, lower, t.text, t.line)
	case strings.Contains(lower, "::"):
		x.reference(kindType, lower, t.text, t.line)
	case !coreType[lower] && !dataType[t.text] && x.declares(x.i):
		x.reference(kindDefine, lower, t.text, t.line) // an unqualified custom type: Concat['x'], Firewall { }
	}
}

// name reads a lower-case name: a declaration, a node, inherits, include and its
// kin, a template or file call, or a function or resource it uses.
func (x *extractor) name(t token) {
	i, next := x.i, x.get(x.i+1)
	switch t.text {
	case "class":
		if x.is(i+1, "{") { // class { 'a::b': ... }
			for _, n := range titles(x.tokens, i+2) {
				x.reference(kindClass, n.text, "class { '"+n.text+"': }", n.line)
			}
		} else if next.kind == tName {
			x.symbols.Add(next.text, "class", next.line)
			x.i++
		}
		return
	case "define", "function", "plan":
		if next.kind == tName && !x.is(i-1, "=>") {
			x.symbols.Add(next.text, t.text, next.line)
			x.i++
		}
		return
	case "node":
		x.node()
		return
	case "inherits":
		if next.kind == tName {
			x.reference(kindClass, next.text, "inherits "+next.text, next.line)
		}
		return
	case "include", "require", "contain":
		if x.is(i+1, "=>") || x.is(i-1, ".") {
			return // the require metaparameter
		}
		for _, n := range classArguments(x.tokens, i+1) {
			x.reference(kindClass, n.text, t.text+" "+n.text, n.line)
		}
		return
	case "template", "epp", "file":
		if x.is(i+1, "(") {
			x.templateCall(t)
			return
		}
	}
	x.use(t)
}

// node reads the names a node definition lists, up to its body.
func (x *extractor) node() {
	for j := x.i + 1; j < len(x.tokens) && !x.is(j, "{"); j++ {
		switch n := x.tokens[j]; n.kind {
		case tString, tRegex, tName:
			x.symbols.Add(n.text, "node", n.line)
		}
		if x.is(j+1, ",") {
			j++
		} else {
			x.i = j
			break
		}
	}
}

// templateCall reads the module paths template(), epp() and file() are given.
func (x *extractor) templateCall(t token) {
	kind := kindTemplate
	if t.text == "file" {
		kind = kindFile
	}
	for j := x.i + 2; j < len(x.tokens) && x.tokens[j].kind == tString && !x.tokens[j].interpolate; j += 2 {
		if a := x.tokens[j].text; strings.Contains(a, "/") && !strings.HasPrefix(a, "/") {
			x.reference(kind, a, t.text+"('"+a+"')", x.tokens[j].line)
		}
		if !x.is(j+1, ",") {
			break
		}
	}
}

// use reads a name that is used rather than declared: a qualified function, a
// stdlib function, or a resource of a defined type.
func (x *extractor) use(t token) {
	i, previous, next := x.i, x.get(x.i-1), x.get(x.i+1)
	if keyword[t.text] || previous.kind == tName && (previous.text == "class" || previous.text == "define" || previous.text == "function" || previous.text == "plan") {
		return
	}
	qualified := strings.Contains(strings.TrimPrefix(t.text, "::"), "::")
	switch {
	case x.is(i+1, "(") && qualified:
		x.reference(kindFunction, t.text, t.text+"()", t.line)
	case x.is(i+1, "(") && next.adjacent && stdlibFunction[t.text]:
		x.reference(kindFunction, "stdlib::"+t.text, t.text+"()", t.line)
	case x.is(i+1, "{") && resourceBody(x.tokens, i+2):
		if qualified || !coreType[t.text] {
			x.reference(kindDefine, t.text, t.text, t.line)
		}
	}
}

// resourceBody reports whether the `{` before i opens a resource body: a
// title expression ('x', $x, $facts['x'], ['a', 'b']) and then a colon,
// before any `=>` or closing brace.
func resourceBody(tokens []token, i int) bool {
	depth := 0
	for end := min(i+64, len(tokens)); i < end; i++ {
		t := tokens[i]
		if t.kind != tPunctuation {
			continue
		}
		switch t.text {
		case "(", "[", "{":
			depth++
		case ")", "]", "}":
			if depth--; depth < 0 {
				return false
			}
		case ":":
			if depth == 0 {
				return true
			}
		case "=>", ";", "|":
			if depth == 0 {
				return false
			}
		}
	}
	return false
}

// titles are the string titles at i: 'a' or ['a', 'b'] (then `:` or `]`).
func titles(tokens []token, i int) []token {
	var out []token
	if i < len(tokens) && tokens[i].kind == tPunctuation && tokens[i].text == "[" {
		for i++; i < len(tokens) && tokens[i].kind != tPunctuation || i < len(tokens) && tokens[i].text == ","; i++ {
			if tokens[i].kind == tString && !tokens[i].interpolate || tokens[i].kind == tName {
				out = append(out, tokens[i])
			}
		}
		return out
	}
	if i < len(tokens) && (tokens[i].kind == tString && !tokens[i].interpolate || tokens[i].kind == tName) {
		out = append(out, tokens[i])
	}
	return out
}

// classArguments are the classes include, require and contain name: bare words
// and strings, separated by commas, optionally in parentheses or an array.
func classArguments(tokens []token, i int) []token {
	var out []token
	for ; i < len(tokens); i++ {
		t := tokens[i]
		switch {
		case t.kind == tName && !keyword[t.text] || t.kind == tString && !t.interpolate && t.text != "":
			if i+1 < len(tokens) && tokens[i+1].kind == tPunctuation && tokens[i+1].text == "(" {
				return out // a function call computing the name
			}
			out = append(out, token{kind: tName, text: strings.ToLower(t.text), line: t.line})
			if i+1 >= len(tokens) || tokens[i+1].kind != tPunctuation || tokens[i+1].text != "," {
				return out
			}
			i++
		case t.kind == tPunctuation && (t.text == "(" || t.text == "[") && len(out) == 0:
		default:
			return out
		}
	}
	return out
}

// pascal reports whether a .pp file is Free Pascal (unit, program or library
// and a name and `;`), which shares the extension.
func pascal(tokens []token) bool {
	if len(tokens) < 3 {
		return false
	}
	switch strings.ToLower(tokens[0].text) {
	case "unit", "program", "library":
		return tokens[2].kind == tPunctuation && (tokens[2].text == ";" || tokens[2].text == ".")
	}
	return false
}

// The capitalized names of Puppet's data types, which are no dependency.
var dataType = map[string]bool{
	"Any": true, "Array": true, "Binary": true, "Boolean": true, "Callable": true, "CatalogEntry": true,
	"Collection": true, "Data": true, "Default": true, "Deferred": true, "Enum": true, "Error": true,
	"Float": true, "Hash": true, "Init": true, "Integer": true, "Iterable": true, "Iterator": true,
	"NotUndef": true, "Numeric": true, "Object": true, "Optional": true, "Pattern": true, "Regexp": true,
	"Resource": true, "RichData": true, "Runtime": true, "Scalar": true, "ScalarData": true, "SemVer": true,
	"SemVerRange": true, "Sensitive": true, "String": true, "Struct": true, "Target": true, "Timespan": true,
	"Timestamp": true, "Tuple": true, "Type": true, "TypeSet": true, "Undef": true, "Variant": true,
}
