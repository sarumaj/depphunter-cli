package terraform

import (
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
	fi := newFileInfo()
	fi.tg = &tgConfig{locals: map[string]string{}, includes: map[string]string{}}
	locals := fi.tg.locals // string locals, substituted into sources
	for _, b := range root.blocks {
		if b.typ != "locals" {
			continue
		}
		for _, a := range b.attrs {
			fi.symbols.Add("local."+a.name, "local", a.line)
			if s, ok := stringExpr(a.expr); ok {
				locals[a.name] = s
			}
		}
	}
	for _, b := range root.blocks {
		switch b.typ {
		case "terraform":
			if a, ok := b.get("source"); ok {
				if s, ok := stringExpr(a.expr); ok {
					// What is still interpolated may come from an included file's
					// locals (include.envcommon.locals.x), which the resolver reads.
					fi.add("terraform.source", expandLocals(s, locals), kindTGSource, a.line)
				}
			}
		case "dependency":
			if len(b.labels) != 1 {
				continue
			}
			fi.symbols.Add("dependency."+b.labels[0], "dependency", b.line)
			if a, ok := b.get("config_path"); ok {
				if p, ok := terragruntPath(a.expr); ok {
					fi.add("dependency \""+b.labels[0]+"\"", p, kindTGDep, a.line)
				}
			}
		case "dependencies":
			if a, ok := b.get("paths"); ok {
				for _, t := range a.expr {
					if p, ok := terragruntPath([]tok{t}); ok {
						fi.add("dependencies \""+t.text+"\"", p, kindTGDep, t.line)
					}
				}
			}
		case "include":
			name, label := "include", ""
			if len(b.labels) == 1 {
				label = b.labels[0]
				name += "." + label
			}
			fi.symbols.Add(name, "include", b.line)
			if a, ok := b.get("path"); ok {
				fi.tg.includes[label] = includePath(a.expr)
			}
		}
	}
	// Includes and read_terragrunt_config: anything found in a parent folder.
	var all []attr
	collectAttrs(root, &all)
	for _, a := range all {
		parentReads(a.expr, fi)
	}
	return fi
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
func includePath(expr []tok) string {
	if s, ok := stringExpr(expr); ok {
		if m := parentDir.FindStringSubmatch(s); m != nil {
			return "parent:" + parentName(m[1]) + "|" + m[2]
		}
		if p, ok := terragruntPath(expr); ok {
			return p
		}
		return ""
	}
	if len(expr) >= 3 && expr[0].kind == tIdent && expr[0].text == "find_in_parent_folders" && expr[1].is("(") {
		if expr[2].is(")") {
			return "parent:" + parentName("")
		}
		if expr[2].kind == tString && expr[2].lit {
			return "parent:" + parentName(expr[2].text)
		}
	}
	return ""
}

func collectAttrs(b *block, out *[]attr) {
	*out = append(*out, b.attrs...)
	for _, c := range b.blocks {
		collectAttrs(c, out)
	}
}

var localRef = regexp.MustCompile(`\$\{\s*local\.([A-Za-z_][A-Za-z0-9_-]*)\s*\}`)

// expandLocals substitutes literal locals into a template, as far as they go.
func expandLocals(s string, locals map[string]string) string {
	for range 4 { // locals naming locals, a few levels deep
		next := localRef.ReplaceAllStringFunc(s, func(m string) string {
			if v, ok := locals[localRef.FindStringSubmatch(m)[1]]; ok {
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
func terragruntPath(expr []tok) (string, bool) {
	s, ok := stringExpr(expr)
	if !ok {
		return "", false
	}
	if rest, ok := strings.CutPrefix(s, "${get_terragrunt_dir()}"); ok {
		s = strings.TrimPrefix(rest, "/")
		if s == "" {
			s = "."
		}
	}
	if s == "" || strings.Contains(s, "${") || strings.HasPrefix(s, "/") {
		return "", false
	}
	return s, true
}

// parentDir matches "${dirname(find_in_parent_folders("root.hcl"))}/rest".
var parentDir = regexp.MustCompile(`^\$\{\s*dirname\(\s*find_in_parent_folders\(\s*(?:"([^"]*)")?\s*\)\s*\)\s*\}(/[^$%]*)$`)

// parentReads records find_in_parent_folders("name") calls, which name the nearest
// file of that name above the configuration (terragrunt.hcl without an argument),
// and paths built on the directory one finds.
func parentReads(expr []tok, fi *fileInfo) {
	walk(expr, func(tokens []tok, i int) {
		t := tokens[i]
		if t.kind == tString {
			if m := parentDir.FindStringSubmatch(t.text); m != nil {
				fi.add(t.text, parentName(m[1]), kindTGParent+"|"+m[2], t.line)
			}
			return
		}
		if t.kind != tIdent || t.text != "find_in_parent_folders" || i+1 >= len(tokens) || !tokens[i+1].is("(") {
			return
		}
		if i >= 2 && tokens[i-1].is("(") && tokens[i-2].kind == tIdent && tokens[i-2].text == "dirname" {
			return // a directory, read with the rest of its path above
		}
		name := ""
		if i+2 < len(tokens) && tokens[i+2].kind == tString {
			if !tokens[i+2].lit {
				return
			}
			name = tokens[i+2].text
		} else if i+2 >= len(tokens) || !tokens[i+2].is(")") {
			return
		}
		spec := "find_in_parent_folders(" + quoteIf(name) + ")"
		fi.add(spec, parentName(name), kindTGParent, t.line)
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
