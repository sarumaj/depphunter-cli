package proto

import (
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// Buf's configuration files: buf.yaml (v1beta1, v1 and v2), buf.work.yaml, buf.lock
// (v1 and v2) and buf.gen.yaml (v1 and v2).

type entry struct {
	value string
	line  int
}

type locked struct {
	name   string // host/owner/repository, lower-cased
	commit string
	line   int
}

// bufFile is what one of Buf's files says.
type bufFile struct {
	version string
	names   []string // the modules it names (v1 name, v2 modules[].name), lower-cased
	roots   []entry  // v1beta1 build.roots, buf.work.yaml directories, v2 modules[].path
	deps    []entry  // deps, and a buf.gen.yaml's module inputs
	locks   []locked // buf.lock deps
	plugins []entry  // a buf.gen.yaml's remote plugins, with their versions
	inputs  []entry  // a buf.gen.yaml's directory inputs
	v2      bool     // a v2 buf.yaml, which is a workspace of its own
}

func mapping(n *yaml.Node) *yaml.Node {
	for n != nil && n.Kind == yaml.DocumentNode && len(n.Content) > 0 {
		n = n.Content[0]
	}
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	return n
}

func lookup(m *yaml.Node, key string) *yaml.Node {
	if m == nil {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

func scalar(m *yaml.Node, key string) string {
	if n := lookup(m, key); n != nil && n.Kind == yaml.ScalarNode {
		return n.Value
	}
	return ""
}

// items lists a sequence's elements; nil for anything else.
func items(n *yaml.Node) []*yaml.Node {
	if n == nil || n.Kind != yaml.SequenceNode {
		return nil
	}
	return n.Content
}

// readBuf reads one of Buf's files, class naming which (buf.yaml, buf.work.yaml,
// buf.lock, buf.gen.yaml). A file that is not YAML says nothing.
//
// Implements: REQ-PROTO-005, REQ-PROTO-007
func readBuf(class string, src []byte) *bufFile {
	b := &bufFile{}
	var doc yaml.Node
	if yaml.Unmarshal(src, &doc) != nil {
		return b
	}
	root := mapping(&doc)
	if root == nil {
		return b
	}
	b.version = scalar(root, "version")
	switch class {
	case classLock:
		for _, d := range items(lookup(root, "deps")) {
			var name string
			if n := scalar(d, "name"); n != "" { // v2
				name = n
			} else if owner, repo := scalar(d, "owner"), scalar(d, "repository"); owner != "" && repo != "" { // v1
				remote := scalar(d, "remote")
				if remote == "" {
					remote = "buf.build"
				}
				name = remote + "/" + owner + "/" + repo
			}
			if name != "" {
				b.locks = append(b.locks, locked{name: strings.ToLower(name), commit: scalar(d, "commit"), line: d.Line})
			}
		}
	case classWork:
		for _, d := range items(lookup(root, "directories")) {
			if d.Kind == yaml.ScalarNode && d.Value != "" {
				b.roots = append(b.roots, entry{d.Value, d.Line})
			}
		}
	case classGen:
		for _, pl := range items(lookup(root, "plugins")) {
			ref := scalar(pl, "remote")
			if ref == "" {
				ref = scalar(pl, "plugin") // v1: a remote plugin when it names a host
			}
			if host, _, ok := strings.Cut(ref, "/"); ok && strings.Contains(host, ".") && !strings.HasPrefix(host, ".") {
				b.plugins = append(b.plugins, entry{ref, pl.Line})
			}
		}
		for _, in := range items(lookup(root, "inputs")) {
			if m := scalar(in, "module"); m != "" {
				b.deps = append(b.deps, entry{m, in.Line})
			} else if d := scalar(in, "directory"); d != "" {
				b.inputs = append(b.inputs, entry{d, in.Line})
			}
		}
	default: // buf.yaml
		b.v2 = b.version == "v2"
		if n := scalar(root, "name"); n != "" {
			b.names = append(b.names, strings.ToLower(n))
		}
		for _, d := range items(lookup(root, "deps")) {
			if d.Kind == yaml.ScalarNode && d.Value != "" {
				b.deps = append(b.deps, entry{d.Value, d.Line})
			}
		}
		for _, r := range items(lookup(lookup(root, "build"), "roots")) { // v1beta1
			if r.Kind == yaml.ScalarNode && r.Value != "" {
				b.roots = append(b.roots, entry{r.Value, r.Line})
			}
		}
		for _, m := range items(lookup(root, "modules")) { // v2
			if p := scalar(m, "path"); p != "" {
				b.roots = append(b.roots, entry{p, m.Line})
			}
			if n := scalar(m, "name"); n != "" {
				b.names = append(b.names, strings.ToLower(n))
			}
		}
	}
	return b
}

// extraction lists what a Buf file depends on, as imports.
//
// Implements: REQ-PROTO-005, REQ-PROTO-007
func (b *bufFile) extraction() *lang.Extraction {
	ex := &lang.Extraction{}
	add := func(spec, module, kind string, line int) {
		ex.Imports = append(ex.Imports, lang.RawImport{Spec: spec, Module: module, Name: kind, Line: line})
	}
	for _, d := range b.deps {
		add(d.value, d.value, kindDep, d.line)
	}
	for _, l := range b.locks {
		add(l.name, l.name, kindLock, l.line)
	}
	for _, p := range b.plugins {
		add(p.value, p.value, kindPlugin, p.line)
	}
	for _, r := range b.roots {
		add(r.value+"/", r.value, kindDirectory, r.line)
	}
	for _, in := range b.inputs {
		add(in.value+"/", in.value, kindDirectory, in.line)
	}
	return ex
}

// moduleRef splits a BSR module or plugin reference into its lower-cased name and
// its ref: "buf.build/acme/pay:v1" -> ("buf.build/acme/pay", "v1").
func moduleRef(s string) (string, string) {
	s = strings.TrimSpace(s)
	slash := strings.LastIndex(s, "/")
	if i := strings.LastIndex(s, ":"); i > slash && slash >= 0 {
		return strings.ToLower(s[:i]), s[i+1:]
	}
	return strings.ToLower(s), ""
}
