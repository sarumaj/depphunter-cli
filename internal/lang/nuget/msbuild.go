package nuget

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// Item is an MSBuild item a project file lists: PackageReference, PackageVersion,
// ProjectReference, Compile, ...
type Item struct {
	Kind string // the element name
	// Include is the item's Include attribute, or its Update attribute (a
	// `<PackageReference Update="FSharp.Core" Version="..." />` changes the implicit
	// reference); Update is set in the second case.
	Include string
	Update  bool
	Version string // Version (or VersionOverride) attribute or child element
	Line    int
}

// Project is what a project or props file says: its items in document order and
// the few properties the plugins read.
type Project struct {
	Sdk        string // the Sdk attribute of <Project>: an SDK-style project
	Items      []Item
	Properties map[string]string // last value of each property, by name
}

// ReadProject reads an MSBuild project (.csproj, .fsproj, Directory.Packages.props,
// Directory.Build.props) with the line of every item. Conditions are ignored: every
// item of every configuration counts. A document that stops parsing keeps what was
// read before the error.
//
// Implements: REQ-FSHARP-005
func ReadProject(data []byte) Project {
	p := Project{Properties: map[string]string{}}
	d := xml.NewDecoder(bytes.NewReader(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))))
	d.Strict = false
	d.CharsetReader = charset
	var stack []string
	var current *Item // the item whose children are being read
	var text string   // character data of the innermost element
	for {
		token, err := d.Token()
		if err != nil {
			break
		}
		switch t := token.(type) {
		case xml.StartElement:
			name := t.Name.Local
			text = ""
			if len(stack) == 0 && name == "Project" {
				for _, a := range t.Attr {
					if a.Name.Local == "Sdk" {
						p.Sdk = a.Value
					}
				}
			}
			stack = append(stack, name)
			if len(stack) >= 2 && stack[len(stack)-2] == "ItemGroup" {
				line, _ := d.InputPos()
				it := Item{Kind: name, Line: line}
				for _, a := range t.Attr {
					switch a.Name.Local {
					case "Include":
						it.Include = strings.TrimSpace(a.Value)
					case "Update":
						if it.Include == "" {
							it.Include, it.Update = strings.TrimSpace(a.Value), true
						}
					case "Version", "VersionOverride":
						it.Version = strings.TrimSpace(a.Value)
					}
				}
				p.Items = append(p.Items, it)
				current = &p.Items[len(p.Items)-1]
			}
		case xml.CharData:
			text += string(t)
		case xml.EndElement:
			if len(stack) == 0 {
				continue
			}
			name := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			switch {
			case len(stack) >= 2 && stack[len(stack)-2] == "ItemGroup" && current != nil && (name == "Version" || name == "VersionOverride"):
				if current.Version == "" {
					current.Version = strings.TrimSpace(text)
				}
			case len(stack) >= 1 && stack[len(stack)-1] == "ItemGroup":
				current = nil
			case len(stack) >= 1 && stack[len(stack)-1] == "PropertyGroup":
				p.Properties[name] = strings.TrimSpace(text)
			}
			text = ""
		}
	}
	return p
}

// charset reads the encodings project files are declared in besides UTF-8.
func charset(label string, input io.Reader) (io.Reader, error) {
	switch strings.ToLower(strings.TrimSpace(label)) {
	case "utf-8", "utf8", "us-ascii", "ascii":
		return input, nil
	case "iso-8859-1", "latin1", "windows-1252":
		raw, err := io.ReadAll(input)
		if err != nil {
			return nil, err
		}
		out := make([]byte, 0, len(raw))
		for _, b := range raw {
			out = utf8.AppendRune(out, rune(b))
		}
		return bytes.NewReader(out), nil
	}
	return nil, fmt.Errorf("unsupported encoding %q", label)
}

// ProjectPath turns an item's Include into a path relative to the project's
// directory: backslashes become slashes and the MSBuild properties naming the
// project's own directory are dropped. ok is false for what stays unknown (another
// property, a wildcard).
func ProjectPath(include string) (string, bool) {
	s := strings.ReplaceAll(strings.TrimSpace(include), `\`, "/")
	for _, p := range []string{"$(MSBuildThisFileDirectory)", "$(MSBuildProjectDirectory)/", "$(MSBuildProjectDirectory)"} {
		s = strings.ReplaceAll(s, p, "")
	}
	if s == "" || strings.ContainsAny(s, "$*?%") {
		return "", false
	}
	return s, true
}
