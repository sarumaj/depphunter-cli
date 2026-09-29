package powershell

import (
	"path"
	"strings"
)

// psDependFile reports whether p is a PSDepend requirements file, which
// Invoke-PSDepend looks for by name: requirements.psd1 and *.depend.psd1.
func psDependFile(p string) bool {
	base := strings.ToLower(path.Base(p))
	return base == "requirements.psd1" || strings.HasSuffix(base, ".depend.psd1")
}

// installValueParameters are the parameters of Install-Module, Install-PSResource
// and their Save- twins (and the common ones) that take a value, so their value is
// not a module name.
var installValueParameters = map[string]bool{
	"-requiredversion": true, "-minimumversion": true, "-maximumversion": true, "-version": true,
	"-repository": true, "-scope": true, "-credential": true, "-proxy": true, "-proxycredential": true,
	"-temporarypath": true, "-requiredresource": true, "-requiredresourcefile": true, "-inputobject": true,
	"-path": true, "-literalpath": true, "-erroraction": true, "-warningaction": true,
	"-errorvariable": true, "-warningvariable": true, "-informationaction": true,
	"-informationvariable": true, "-outvariable": true, "-outbuffer": true, "-pipelinevariable": true,
}

// installReferences reads the modules an Install-Module or Install-PSResource
// command (or Save-Module, Save-PSResource) fetches from a repository: the names
// (-Name or the first positional argument, a comma-separated list), the version
// asked for and the repository named. Install-Module's -RequiredVersion is one
// version and -MinimumVersion a minimum; Install-PSResource's -Version is one
// version when bare and a NuGet range in brackets. A name held in a variable or
// holding a wildcard names no module.
//
// Implements: REQ-PS-012
func installReferences(command string, arguments []string) []reference {
	var names []string
	var required, minimum, version, repository string
	collecting := false
	for i := 0; i < len(arguments); i++ {
		a := arguments[i]
		if strings.HasPrefix(a, "-") {
			parameter, inline, hasInline := strings.Cut(a, ":")
			parameter = strings.ToLower(parameter)
			value := inline
			if !hasInline && installValueParameters[parameter] && i+1 < len(arguments) {
				i++
				value = arguments[i]
			}
			value = unquote(value)
			collecting = parameter == "-name"
			switch parameter {
			case "-name":
				if hasInline {
					names = appendNames(names, inline)
					collecting = strings.HasSuffix(inline, ",")
				}
			case "-requiredversion":
				required = value
			case "-minimumversion":
				minimum = value
			case "-version":
				version = value
			case "-repository":
				repository = value
			}
			continue
		}
		if collecting || len(names) == 0 {
			names = appendNames(names, a)
			collecting = strings.HasSuffix(a, ",")
		}
	}
	how := referenceRequires
	switch {
	case required != "" && !strings.HasPrefix(required, "$"):
		how += "==" + required
	case version != "" && !strings.HasPrefix(version, "$"):
		if strings.ContainsAny(version, "[(,") {
			how += "=" + version // a NuGet range
		} else {
			how += "==" + version
		}
	case minimum != "" && !strings.HasPrefix(minimum, "$"):
		how += "=" + minimum
	}
	if repository != "" && !strings.HasPrefix(repository, "$") {
		how += "\x00" + repository
	}
	var out []reference
	for _, name := range names {
		if strings.HasPrefix(name, "$") || strings.ContainsAny(name, "*?()@{}") {
			continue
		}
		out = append(out, reference{command + " " + name, name, how})
	}
	return out
}

// appendNames adds the names of a comma-separated argument.
func appendNames(names []string, argument string) []string {
	for _, n := range strings.Split(argument, ",") {
		if n = unquote(strings.TrimSpace(n)); n != "" {
			names = append(names, n)
		}
	}
	return names
}

// dependencyReference is a module a PSDepend file names, on its line.
type dependencyReference struct {
	reference
	line int
}

// psDependReferences reads the PowerShell Gallery modules of a PSDepend file: a
// hashtable whose keys name dependencies ("Name", or "Type::Name") and whose values
// are a version ('latest' or an empty string for the newest; a NuGet range; else
// one version) or a hashtable of DependencyType, Name, Version and Parameters
// (Repository). As PSDepend decides, a dependency without a type is a
// PSGalleryModule unless its name holds a "/" (a GitHub or git repository, which
// no gallery serves); PSGalleryNuget is fetched from the gallery too. Other types,
// and PSDependOptions, are left out.
//
// Implements: REQ-PS-012
func psDependReferences(source string) []dependencyReference {
	table, ok := parseHashtable(stripComments(source))
	if !ok {
		return nil
	}
	var out []dependencyReference
	for _, entry := range table.entries {
		key := entry.key
		if strings.EqualFold(key, "PSDependOptions") {
			continue
		}
		kind, name := "", key
		if t, n, ok := strings.Cut(key, "::"); ok {
			kind, name = t, n
		}
		version, repository := entry.value.text, ""
		if entry.value.isTable {
			version = ""
			for _, field := range entry.value.entries {
				switch strings.ToLower(field.key) {
				case "dependencytype":
					kind = field.value.text
				case "name":
					name = field.value.text
				case "version":
					version = field.value.text
				case "parameters":
					for _, parameter := range field.value.entries {
						if strings.EqualFold(parameter.key, "Repository") {
							repository = parameter.value.text
						}
					}
				}
			}
		}
		if kind == "" && strings.Contains(name, "/") ||
			kind != "" && !strings.EqualFold(kind, "PSGalleryModule") && !strings.EqualFold(kind, "PSGalleryNuget") ||
			name == "" || strings.HasPrefix(name, "$") {
			continue
		}
		how := referenceRequires
		switch {
		case version == "" || strings.EqualFold(version, "latest") || strings.HasPrefix(version, "$"):
		case strings.ContainsAny(version, "[(,"):
			how += "=" + version
		default:
			how += "==" + version
		}
		if repository != "" && !strings.HasPrefix(repository, "$") {
			how += "\x00" + repository
		}
		out = append(out, dependencyReference{reference{"PSDepend: " + name, name, how}, entry.line})
	}
	return out
}

// psValue is a value of a PowerShell data file: a scalar's text, or a hashtable's
// entries in order.
type psValue struct {
	text    string
	isTable bool
	entries []psEntry
}

type psEntry struct {
	key   string
	value psValue
	line  int
}

// parseHashtable reads the hashtable a data file holds, "@{ key = value ... }",
// with its entries separated by newlines or semicolons. Arrays and expressions are
// kept as their text; comments must already be stripped.
func parseHashtable(source string) (psValue, bool) {
	p := &hashtableParser{source: source, line: 1}
	p.space()
	if !strings.HasPrefix(p.source[p.position:], "@{") {
		return psValue{}, false
	}
	return p.value(), true
}

type hashtableParser struct {
	source   string
	position int
	line     int
}

// space skips blanks, newlines and entry separators.
func (p *hashtableParser) space() {
	for p.position < len(p.source) {
		switch p.source[p.position] {
		case '\n':
			p.line++
		case ' ', '\t', '\r', ';':
		default:
			return
		}
		p.position++
	}
}

func (p *hashtableParser) value() psValue {
	rest := p.source[p.position:]
	switch {
	case strings.HasPrefix(rest, "@{"):
		p.position += 2
		var table psValue
		table.isTable = true
		for {
			p.space()
			if p.position >= len(p.source) {
				return table
			}
			if p.source[p.position] == '}' {
				p.position++
				return table
			}
			line := p.line
			key := p.scalar("=}\n;")
			p.space()
			if p.position >= len(p.source) || p.source[p.position] != '=' {
				p.scalar("\n;}") // not an entry: skip what is left of it
				continue
			}
			p.position++
			for p.position < len(p.source) && (p.source[p.position] == ' ' || p.source[p.position] == '\t') {
				p.position++
			}
			table.entries = append(table.entries, psEntry{key: key, value: p.value(), line: line})
		}
	case strings.HasPrefix(rest, "@("), strings.HasPrefix(rest, "("):
		start := p.position
		depth := 0
		for ; p.position < len(p.source); p.position++ {
			switch p.source[p.position] {
			case '(':
				depth++
			case ')':
				depth--
			case '\n':
				p.line++
			}
			if depth == 0 && p.source[p.position] == ')' {
				p.position++
				break
			}
		}
		return psValue{text: p.source[start:p.position]}
	}
	return psValue{text: p.scalar("\n;}")}
}

// scalar reads a quoted string or a bare word ending at one of stops (or a blank
// before one).
func (p *hashtableParser) scalar(stops string) string {
	if p.position < len(p.source) && (p.source[p.position] == '\'' || p.source[p.position] == '"') {
		quote := p.source[p.position]
		var text strings.Builder
		for p.position++; p.position < len(p.source); p.position++ {
			c := p.source[p.position]
			if c == quote {
				if p.position+1 < len(p.source) && p.source[p.position+1] == quote {
					text.WriteByte(c) // a doubled quote is the quote itself
					p.position++
					continue
				}
				p.position++
				break
			}
			if c == '\n' {
				p.line++
			}
			text.WriteByte(c)
		}
		return text.String()
	}
	start := p.position
	for p.position < len(p.source) && !strings.ContainsRune(stops, rune(p.source[p.position])) {
		p.position++
	}
	return strings.TrimSpace(p.source[start:p.position])
}
