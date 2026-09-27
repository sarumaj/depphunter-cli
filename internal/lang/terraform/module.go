package terraform

import (
	"sort"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// Import kinds, carried in RawImport.Name (before a "|" and what follows it).
const (
	kindModule   = "module"   // a module call; Name "module|<version constraint>"
	kindProvider = "provider" // a provider by its local name in the module
	kindRef      = "ref"      // var.x, local.x, module.x, data.t.n, t.n
	kindFile     = "file"     // a path read by file() or templatefile()
	kindLock     = "lock"     // a provider address in .terraform.lock.hcl
	kindTGSource = "tgsource" // Terragrunt's terraform { source }
	kindTGDep    = "tgdep"    // a Terragrunt dependency's config_path
	kindTGParent = "tgparent" // find_in_parent_folders("x"): Name "tgparent|<rest of the path>"
)

// fileInfo is what one Terraform file says: its symbols and imports, and what the
// resolver needs of every file of a module (the providers it requires).
type fileInfo struct {
	symbols   lang.SymbolSet
	imports   []lang.RawImport
	seen      map[string]bool
	providers map[string]providerReq // local name -> requirement
	tg        *tgConfig              // a Terragrunt configuration's locals and includes
}

type providerReq struct {
	source     string // as written, "" for a legacy requirement without one
	constraint string
	line       int
}

func newFileInfo() *fileInfo {
	return &fileInfo{seen: map[string]bool{}, providers: map[string]providerReq{}}
}

func (fi *fileInfo) add(spec, module, name string, line int) {
	if module == "" || fi.seen[spec] {
		return
	}
	fi.seen[spec] = true
	fi.imports = append(fi.imports, lang.RawImport{Spec: spec, Module: module, Name: name, Line: line})
}

func (fi *fileInfo) extraction() *lang.Extraction {
	sort.SliceStable(fi.imports, func(i, j int) bool { return fi.imports[i].Line < fi.imports[j].Line })
	return &lang.Extraction{Imports: fi.imports, Symbols: fi.symbols.List()}
}

// builtin is the provider Terraform and OpenTofu carry inside themselves
// (terraform_remote_state, terraform_data): nothing to install.
const builtin = "terraform"

// readModule reads a configuration file (.tf, .tofu, or .tf.json made into the same
// blocks): its declarations as symbols, and as imports its module calls, the
// providers it requires, configures or uses, its references to what other files of
// the module declare, and the files it reads.
//
// Implements: REQ-TERRAFORM-002, REQ-TERRAFORM-003, REQ-TERRAFORM-005, REQ-TERRAFORM-006
func readModule(root *block) *fileInfo {
	fi := newFileInfo()
	own := map[string]bool{}
	used := map[string]string{} // provider local name -> first resource type using it
	usedLine := map[string]int{}
	use := func(local, typ string, line int) {
		if _, ok := used[local]; !ok && local != "" {
			used[local], usedLine[local] = typ, line
		}
	}
	for _, b := range root.blocks {
		switch {
		case (b.typ == "resource" || b.typ == "data" || b.typ == "ephemeral") && len(b.labels) == 2:
			name := b.labels[0] + "." + b.labels[1]
			if b.typ != "resource" {
				name = b.typ + "." + name
			}
			fi.symbols.Add(name, b.typ, b.line)
			own[name] = true
			local := providerPrefix(b.labels[0])
			if a, ok := b.get("provider"); ok && len(a.expr) > 0 && a.expr[0].kind == tIdent {
				local = a.expr[0].text // provider = aws.west
			}
			use(local, b.labels[0], b.line)
		case b.typ == "module" && len(b.labels) == 1:
			name := "module." + b.labels[0]
			fi.symbols.Add(name, "module", b.line)
			own[name] = true
			src, _ := b.get("source")
			version := ""
			if a, ok := b.get("version"); ok {
				version, _ = literal(a.expr)
			}
			if s, ok := literal(src.expr); ok {
				fi.add("module \""+b.labels[0]+"\"", s, kindModule+"|"+version, b.line)
			}
		case b.typ == "variable" && len(b.labels) == 1:
			fi.symbols.Add("var."+b.labels[0], "variable", b.line)
			own["var."+b.labels[0]] = true
		case b.typ == "output" && len(b.labels) == 1:
			fi.symbols.Add("output."+b.labels[0], "output", b.line)
		case b.typ == "check" && len(b.labels) == 1:
			fi.symbols.Add("check."+b.labels[0], "check", b.line)
		case b.typ == "locals":
			for _, a := range b.attrs {
				fi.symbols.Add("local."+a.name, "local", a.line)
				own["local."+a.name] = true
			}
		case b.typ == "provider" && len(b.labels) == 1:
			name := "provider." + b.labels[0]
			if a, ok := b.get("alias"); ok {
				if alias, ok := literal(a.expr); ok {
					name += "." + alias
				}
			}
			fi.symbols.Add(name, "provider", b.line)
			if a, ok := b.get("version"); ok { // before Terraform 0.13
				if c, ok := literal(a.expr); ok {
					if r, have := fi.providers[b.labels[0]]; !have || r.constraint == "" {
						fi.providers[b.labels[0]] = providerReq{source: r.source, constraint: c, line: a.line}
					}
				}
			}
			if b.labels[0] != builtin {
				fi.add("provider \""+b.labels[0]+"\"", b.labels[0], kindProvider, b.line)
			}
		case b.typ == "terraform":
			for _, rp := range b.blocks {
				if rp.typ != "required_providers" {
					continue
				}
				for _, a := range rp.attrs {
					req := providerReq{line: a.line}
					if c, ok := literal(a.expr); ok { // aws = "~> 2.0", before 0.13
						req.constraint = c
					}
					for _, it := range object(a.expr) {
						switch it.key {
						case "source":
							req.source, _ = literal(it.val)
						case "version":
							req.constraint, _ = literal(it.val)
						}
					}
					fi.providers[a.name] = req
					if a.name != builtin {
						fi.add("required_providers "+a.name, a.name, kindProvider, a.line)
					}
				}
			}
		}
	}
	locals := sortedKeys(used)
	for _, local := range locals {
		if local == builtin || fi.seen["required_providers "+local] || fi.seen["provider \""+local+"\""] {
			continue
		}
		fi.add("provider "+local+" ("+used[local]+")", local, kindProvider, usedLine[local])
	}
	iters := iterators(root)
	for _, b := range root.blocks {
		switch b.typ {
		case "moved", "removed", "import", "terraform":
			continue // addresses of what was or will be, not uses
		}
		scanBlock(b, func(expr []tok) {
			references(expr, own, iters, fi)
			fileReads(expr, fi)
		})
	}
	return fi
}

// iterators are the names a file binds to values in expressions: for expressions'
// variables and dynamic blocks' iterators (the block's label unless an iterator
// attribute renames it). var_x.y is not a resource when var_x is one of them.
func iterators(root *block) map[string]bool {
	out := map[string]bool{}
	var visit func(b *block)
	visit = func(b *block) {
		if b.typ == "dynamic" && len(b.labels) == 1 {
			out[b.labels[0]] = true
			if a, ok := b.get("iterator"); ok && len(a.expr) == 1 && a.expr[0].kind == tIdent {
				out[a.expr[0].text] = true
			}
		}
		for _, a := range b.attrs {
			walk(a.expr, func(tokens []tok, i int) {
				if tokens[i].kind != tIdent || tokens[i].text != "for" {
					return
				}
				for j := i + 1; j < len(tokens) && j <= i+3; j++ {
					if tokens[j].kind == tIdent && tokens[j].text != "in" {
						out[tokens[j].text] = true
					} else if !tokens[j].is(",") {
						break
					}
				}
			})
		}
		for _, c := range b.blocks {
			visit(c)
		}
	}
	visit(root)
	return out
}

// providerPrefix is the provider local name a resource type implies: the part
// before its first underscore (aws_instance: aws).
func providerPrefix(typ string) string {
	if i := strings.IndexByte(typ, '_'); i > 0 {
		return typ[:i]
	}
	return typ
}

func scanBlock(b *block, fn func([]tok)) {
	for _, a := range b.attrs {
		fn(a.expr)
	}
	for _, c := range b.blocks {
		scanBlock(c, fn)
	}
}

// references records the named values an expression uses that the file does not
// declare itself: var.x, local.x, module.x, data.t.n and managed resources t.n
// (a type with an underscore, which the resolver checks against the module's
// declarations).
//
// Implements: REQ-TERRAFORM-005
func references(expr []tok, own, iters map[string]bool, fi *fileInfo) {
	walk(expr, func(tokens []tok, i int) {
		t := tokens[i]
		if t.kind != tIdent || i+2 >= len(tokens) || !tokens[i+1].is(".") || tokens[i+2].kind != tIdent {
			return
		}
		if i > 0 && (tokens[i-1].is(".") || tokens[i-1].is("::")) {
			return // an attribute of something else
		}
		name := ""
		switch t.text {
		case "var", "local", "module":
			name = t.text + "." + tokens[i+2].text
		case "data", "ephemeral":
			if i+4 < len(tokens) && tokens[i+3].is(".") && tokens[i+4].kind == tIdent {
				name = t.text + "." + tokens[i+2].text + "." + tokens[i+4].text
			}
		default:
			if strings.Contains(t.text, "_") && !iters[t.text] {
				name = t.text + "." + tokens[i+2].text
			}
		}
		if name != "" && !own[name] {
			fi.add(name, name, kindRef, t.line)
		}
	})
}

// fileFuncs read a file by path.
var fileFuncs = map[string]bool{
	"file": true, "templatefile": true, "filebase64": true, "filemd5": true, "filesha1": true,
	"filesha256": true, "filesha512": true, "filebase64sha256": true, "filebase64sha512": true,
}

// fileReads records the paths file() and its kin read, when the path is a literal,
// possibly under ${path.module} (or path.root or path.cwd, taken as the module's
// own directory, which they are for a root module).
//
// Implements: REQ-TERRAFORM-006
func fileReads(expr []tok, fi *fileInfo) {
	walk(expr, func(tokens []tok, i int) {
		t := tokens[i]
		if t.kind != tIdent || !fileFuncs[t.text] || i+2 >= len(tokens) || !tokens[i+1].is("(") || tokens[i+2].kind != tString {
			return
		}
		if i > 0 && (tokens[i-1].is(".") || tokens[i-1].is("::")) {
			return
		}
		p, ok := modulePath(tokens[i+2].text)
		if !ok {
			return
		}
		fi.add(t.text+"(\""+tokens[i+2].text+"\")", p, kindFile, t.line)
	})
}

// modulePath reads a path relative to the module's directory out of a template:
// "x.tpl", "${path.module}/x.tpl". Anything else interpolated makes it unknown.
func modulePath(s string) (string, bool) {
	for _, prefix := range [...]string{"${path.module}", "${path.root}", "${path.cwd}"} {
		if rest, ok := strings.CutPrefix(s, prefix); ok {
			s = strings.TrimPrefix(rest, "/")
			break
		}
	}
	if s == "" || strings.Contains(s, "${") || strings.Contains(s, "%{") || strings.HasPrefix(s, "/") || strings.HasPrefix(s, "~") {
		return "", false
	}
	return s, true
}

// readVars reads a .tfvars file: the variables it sets.
//
// Implements: REQ-TERRAFORM-003
func readVars(root *block) *fileInfo {
	fi := newFileInfo()
	for _, a := range root.attrs {
		fi.symbols.Add(a.name, "value", a.line)
	}
	return fi
}

// readLock reads .terraform.lock.hcl: every provider it locks is an import of that
// provider, pinned to the locked version.
//
// Implements: REQ-TERRAFORM-008
func readLock(root *block) *fileInfo {
	fi := newFileInfo()
	for _, l := range lockEntries(root) {
		fi.add("provider \""+l.addr+"\"", l.addr, kindLock, l.line)
	}
	return fi
}

type lockEntry struct {
	addr, version, constraints string
	line                       int
}

func lockEntries(root *block) []lockEntry {
	var out []lockEntry
	for _, b := range root.blocks {
		if b.typ != "provider" || len(b.labels) != 1 {
			continue
		}
		e := lockEntry{addr: b.labels[0], line: b.line}
		if a, ok := b.get("version"); ok {
			e.version, _ = literal(a.expr)
		}
		if a, ok := b.get("constraints"); ok {
			e.constraints, _ = literal(a.expr)
		}
		out = append(out, e)
	}
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
