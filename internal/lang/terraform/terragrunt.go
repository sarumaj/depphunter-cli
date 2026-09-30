package terraform

import (
	"cmp"
	"regexp"
	"strings"
)

// readTerragrunt reads a Terragrunt configuration (terragrunt.hcl, root.hcl, the
// .hcl files they include): the module its terraform block's source names, the
// configurations its dependency blocks and dependencies list point at, and the
// files it includes or reads from a parent folder. Its locals, dependencies and
// includes are its symbols.
//
// Implements: REQ-TERRAFORM-010
func readTerragrunt(root *block) *fileInfo {
	fileInfo := newFileInfo()
	fileInfo.terragrunt = &tgConfig{locals: map[string]string{}, includes: map[string]string{}}
	locals := fileInfo.terragrunt.locals // string locals, substituted into sources
	for _, b := range root.blocks {
		if b.typeName != "locals" {
			continue
		}
		for _, a := range b.attributes {
			fileInfo.symbols.Add("local."+a.name, "local", a.line)
			if s, ok := stringExpression(a.expression); ok {
				locals[a.name] = s
			}
		}
	}
	for _, b := range root.blocks {
		switch b.typeName {
		case "terraform":
			if a, ok := b.get("source"); ok {
				if s, ok := stringExpression(a.expression); ok {
					// What is still interpolated may come from an included file's
					// locals (include.envcommon.locals.x), which the resolver reads.
					fileInfo.add("terraform.source", expandLocals(s, locals), kindTGSource, a.line)
				}
			}
		case "dependency":
			if len(b.labels) != 1 {
				continue
			}
			fileInfo.symbols.Add("dependency."+b.labels[0], "dependency", b.line)
			if a, ok := b.get("config_path"); ok {
				if p, ok := terragruntPath(a.expression); ok {
					fileInfo.add("dependency \""+b.labels[0]+"\"", p, kindTGDependency, a.line)
				}
			}
		case "dependencies":
			if a, ok := b.get("paths"); ok {
				for _, t := range a.expression {
					if p, ok := terragruntPath([]token{t}); ok {
						fileInfo.add("dependencies \""+t.text+"\"", p, kindTGDependency, t.line)
					}
				}
			}
		case "include":
			name, label := "include", ""
			if len(b.labels) == 1 {
				label = b.labels[0]
				name += "." + label
			}
			fileInfo.symbols.Add(name, "include", b.line)
			if a, ok := b.get("path"); ok {
				fileInfo.terragrunt.includes[label] = includePath(a.expression)
			}
		}
	}
	// Includes and read_terragrunt_config: anything found in a parent folder.
	var all []attribute
	collectAttributes(root, &all)
	for _, a := range all {
		parentReads(a.expression, fileInfo)
	}
	return fileInfo
}

// tgConfig is what the resolver needs to expand a Terragrunt source built from an
// included file's locals.
type tgConfig struct {
	locals   map[string]string // string locals as written, own locals expanded
	includes map[string]string // include label -> path, as includePath writes it
}

// includePath reads an include's path: "parent:<name>" for
// find_in_parent_folders("name"), "parent:<name>|/rest" for a path under that file's
// directory, else the path as written; "" when it cannot be read.
func includePath(expression []token) string {
	if s, ok := stringExpression(expression); ok {
		if m := parentDirectory.FindStringSubmatch(s); m != nil {
			return "parent:" + parentName(m[1]) + "|" + m[2]
		}
		if p, ok := terragruntPath(expression); ok {
			return p
		}
		return ""
	}
	if len(expression) >= 3 && expression[0].kind == tIdentifier && expression[0].text == "find_in_parent_folders" && expression[1].is("(") {
		if expression[2].is(")") {
			return "parent:" + parentName("")
		}
		if expression[2].kind == tString && expression[2].literal {
			return "parent:" + parentName(expression[2].text)
		}
	}
	return ""
}

func collectAttributes(b *block, out *[]attribute) {
	*out = append(*out, b.attributes...)
	for _, c := range b.blocks {
		collectAttributes(c, out)
	}
}

var localReference = regexp.MustCompile(`\$\{\s*local\.([A-Za-z_][A-Za-z0-9_-]*)\s*\}`)

// expandLocals substitutes literal locals into a template, as far as they go.
func expandLocals(s string, locals map[string]string) string {
	for range 4 { // locals naming locals, a few levels deep
		next := localReference.ReplaceAllStringFunc(s, func(m string) string {
			if v, ok := locals[localReference.FindStringSubmatch(m)[1]]; ok {
				return v
			}
			return m
		})
		if next == s {
			break
		}
		s = next
	}
	return s
}

// terragruntPath reads a directory a dependency names: "../vpc", or
// "${get_terragrunt_dir()}/../vpc".
func terragruntPath(expression []token) (string, bool) {
	s, ok := stringExpression(expression)
	if !ok {
		return "", false
	}
	if rest, ok := strings.CutPrefix(s, "${get_terragrunt_dir()}"); ok {
		s = strings.TrimPrefix(rest, "/")
		s = cmp.Or(s, ".")
	}
	if s == "" || strings.Contains(s, "${") || strings.HasPrefix(s, "/") {
		return "", false
	}
	return s, true
}

// parentDirectory matches "${dirname(find_in_parent_folders("root.hcl"))}/rest".
var parentDirectory = regexp.MustCompile(`^\$\{\s*dirname\(\s*find_in_parent_folders\(\s*(?:"([^"]*)")?\s*\)\s*\)\s*\}(/[^$%]*)$`)

// parentReads records find_in_parent_folders("name") calls, which name the nearest
// file of that name above the configuration (terragrunt.hcl without an argument),
// and paths built on the directory one finds.
func parentReads(expression []token, info *fileInfo) {
	walk(expression, func(tokens []token, i int) {
		t := tokens[i]
		if t.kind == tString {
			if m := parentDirectory.FindStringSubmatch(t.text); m != nil {
				info.add(t.text, parentName(m[1]), kindTGParent+"|"+m[2], t.line)
			}
			return
		}
		if t.kind != tIdentifier || t.text != "find_in_parent_folders" || i+1 >= len(tokens) || !tokens[i+1].is("(") {
			return
		}
		if i >= 2 && tokens[i-1].is("(") && tokens[i-2].kind == tIdentifier && tokens[i-2].text == "dirname" {
			return // a directory, read with the rest of its path above
		}
		name := ""
		if i+2 < len(tokens) && tokens[i+2].kind == tString {
			if !tokens[i+2].literal {
				return
			}
			name = tokens[i+2].text
		} else if i+2 >= len(tokens) || !tokens[i+2].is(")") {
			return
		}
		spec := "find_in_parent_folders(" + quoteIf(name) + ")"
		info.add(spec, parentName(name), kindTGParent, t.line)
	})
}

func parentName(name string) string {
	if name == "" {
		return "terragrunt.hcl"
	}
	return name
}

func quoteIf(s string) string {
	if s == "" {
		return ""
	}
	return "\"" + s + "\""
}
