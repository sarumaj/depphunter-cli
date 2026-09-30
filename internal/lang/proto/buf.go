package proto

import (
	"cmp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/yamlnode"
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
	version      string
	names        []string // the modules it names (v1 name, v2 modules[].name), lower-cased
	roots        []entry  // v1beta1 build.roots, buf.work.yaml directories, v2 modules[].path
	dependencies []entry  // deps, and a buf.gen.yaml's module inputs
	locks        []locked // buf.lock deps
	plugins      []entry  // a buf.gen.yaml's remote plugins, with their versions
	inputs       []entry  // a buf.gen.yaml's directory inputs
	v2           bool     // a v2 buf.yaml, which is a workspace of its own
}

// readBuf reads one of Buf's files, class naming which (buf.yaml, buf.work.yaml,
// buf.lock, buf.gen.yaml). A file that is not YAML says nothing.
//
// Implements: REQ-PROTO-005, REQ-PROTO-007
func readBuf(class string, source []byte) *bufFile {
	b := &bufFile{}
	root := yamlnode.Mapping(yamlnode.Parse(source))
	if root == nil {
		return b
	}
	b.version = yamlnode.Scalar(root, "version")
	switch class {
	case classLock:
		for _, d := range yamlnode.Items(yamlnode.Get(root, "deps")) {
			var name string
			if n := yamlnode.Scalar(d, "name"); n != "" { // v2
				name = n
			} else if owner, repository := yamlnode.Scalar(d, "owner"), yamlnode.Scalar(d, "repository"); owner != "" && repository != "" { // v1
				remote := cmp.Or(yamlnode.Scalar(d, "remote"), "buf.build")
				name = remote + "/" + owner + "/" + repository
			}
			if name != "" {
				b.locks = append(b.locks, locked{name: strings.ToLower(name), commit: yamlnode.Scalar(d, "commit"), line: d.Line})
			}
		}
	case classWork:
		for _, d := range yamlnode.Items(yamlnode.Get(root, "directories")) {
			if d.Kind == yaml.ScalarNode && d.Value != "" {
				b.roots = append(b.roots, entry{d.Value, d.Line})
			}
		}
	case classGen:
		for _, pl := range yamlnode.Items(yamlnode.Get(root, "plugins")) {
			reference := yamlnode.Scalar(pl, "remote")
			reference = cmp.Or(reference, yamlnode.Scalar(pl, "plugin")) // v1: a remote plugin when it names a host
			if host, _, ok := strings.Cut(reference, "/"); ok && strings.Contains(host, ".") && !strings.HasPrefix(host, ".") {
				b.plugins = append(b.plugins, entry{reference, pl.Line})
			}
		}
		for _, in := range yamlnode.Items(yamlnode.Get(root, "inputs")) {
			if m := yamlnode.Scalar(in, "module"); m != "" {
				b.dependencies = append(b.dependencies, entry{m, in.Line})
			} else if d := yamlnode.Scalar(in, "directory"); d != "" {
				b.inputs = append(b.inputs, entry{d, in.Line})
			}
		}
	default: // buf.yaml
		b.v2 = b.version == "v2"
		if n := yamlnode.Scalar(root, "name"); n != "" {
			b.names = append(b.names, strings.ToLower(n))
		}
		for _, d := range yamlnode.Items(yamlnode.Get(root, "deps")) {
			if d.Kind == yaml.ScalarNode && d.Value != "" {
				b.dependencies = append(b.dependencies, entry{d.Value, d.Line})
			}
		}
		for _, r := range yamlnode.Items(yamlnode.Get(root, "build", "roots")) { // v1beta1
			if r.Kind == yaml.ScalarNode && r.Value != "" {
				b.roots = append(b.roots, entry{r.Value, r.Line})
			}
		}
		for _, m := range yamlnode.Items(yamlnode.Get(root, "modules")) { // v2
			if p := yamlnode.Scalar(m, "path"); p != "" {
				b.roots = append(b.roots, entry{p, m.Line})
			}
			if n := yamlnode.Scalar(m, "name"); n != "" {
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
	extraction := &lang.Extraction{}
	add := func(spec, module, kind string, line int) {
		extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: spec, Module: module, Name: kind, Line: line})
	}
	for _, d := range b.dependencies {
		add(d.value, d.value, kindDependency, d.line)
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
	return extraction
}

// moduleReference splits a BSR module or plugin reference into its lower-cased name and
// its ref: "buf.build/acme/pay:v1" -> ("buf.build/acme/pay", "v1").
func moduleReference(s string) (string, string) {
	s = strings.TrimSpace(s)
	slash := strings.LastIndex(s, "/")
	if i := strings.LastIndex(s, ":"); i > slash && slash >= 0 {
		return strings.ToLower(s[:i]), s[i+1:]
	}
	return strings.ToLower(s), ""
}
