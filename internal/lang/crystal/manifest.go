package crystal

import (
	"path"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// dependency is one entry of shard.yml's dependencies or development_dependencies,
// or of shard.override.yml: where the shard comes from (github:, gitlab:,
// bitbucket:, codeberg:, git: or path:) and what it asks for (version:, branch:,
// tag: or commit:).
type dependency struct {
	name, url, path              string
	version, branch, tag, commit string
	dev                          bool
	line                         int
}

// requirement is what the entry asks for, as shown for a locked shard.
func (d *dependency) requirement() string {
	switch {
	case d.commit != "":
		return d.commit
	case d.tag != "":
		return d.tag
	case d.branch != "":
		return d.branch
	}
	return d.version
}

// shard is what a shard.yml says: the shard's name, its dependencies and the
// main files of its targets.
type shard struct {
	name    string
	deps    map[string]*dependency
	targets map[string]string // target name -> main file, as written
	lines   map[string]int    // target name -> line
}

// locked is an entry of shard.lock: the shard's source and the version shards
// installed (1.2.3, or 0.3.1+git.commit.<sha> for a commit).
type locked struct {
	name, url, path, version string
	line                     int
}

// Implements: REQ-CRYSTAL-005
func readShard(src []byte) *shard {
	sh := &shard{deps: map[string]*dependency{}, targets: map[string]string{}, lines: map[string]int{}}
	root := document(src)
	if root == nil {
		return sh
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		key, val := root.Content[i].Value, root.Content[i+1]
		switch key {
		case "name":
			sh.name = val.Value
		case "dependencies", "development_dependencies":
			readDependencies(val, key != "dependencies", sh.deps)
		case "targets":
			if val.Kind != yaml.MappingNode {
				continue
			}
			for j := 0; j+1 < len(val.Content); j += 2 {
				if main := lookup(val.Content[j+1], "main"); main != nil && main.Value != "" {
					sh.targets[val.Content[j].Value] = main.Value
					sh.lines[val.Content[j].Value] = main.Line
				}
			}
		}
	}
	return sh
}

// readOverride reads shard.override.yml: dependencies that replace the entries
// of the same name wherever the dependency graph names them.
//
// Implements: REQ-CRYSTAL-005
func readOverride(src []byte) map[string]*dependency {
	out := map[string]*dependency{}
	if root := document(src); root != nil {
		readDependencies(lookup(root, "dependencies"), false, out)
	}
	return out
}

func readDependencies(n *yaml.Node, dev bool, into map[string]*dependency) {
	if n == nil || n.Kind != yaml.MappingNode {
		return
	}
	for j := 0; j+1 < len(n.Content); j += 2 {
		name := n.Content[j].Value
		if name == "" || into[name] != nil {
			continue
		}
		d := &dependency{name: name, dev: dev, line: n.Content[j].Line}
		if m := n.Content[j+1]; m.Kind == yaml.MappingNode {
			str := func(k string) string {
				if v := lookup(m, k); v != nil && v.Kind == yaml.ScalarNode {
					return strings.TrimSpace(v.Value)
				}
				return ""
			}
			d.url = sourceURL(str)
			d.path = str("path")
			d.version, d.branch, d.tag, d.commit = str("version"), str("branch"), str("tag"), str("commit")
		}
		into[name] = d
	}
}

// hosts are the shorthand sources shards knows, each a repository "owner/repo" on
// that host.
var hosts = []struct{ key, base string }{
	{"github", "https://github.com/"},
	{"gitlab", "https://gitlab.com/"},
	{"bitbucket", "https://bitbucket.org/"},
	{"codeberg", "https://codeberg.org/"},
}

func sourceURL(str func(string) string) string {
	for _, h := range hosts {
		if v := str(h.key); v != "" {
			return h.base + v + ".git"
		}
	}
	return str("git")
}

// readLock reads shard.lock (version 1.0 or 2.0). A version 1.0 entry pinned to a
// commit writes it as commit:.
//
// Implements: REQ-CRYSTAL-006
func readLock(src []byte) map[string]*locked {
	out := map[string]*locked{}
	root := document(src)
	shards := lookup(root, "shards")
	if shards == nil || shards.Kind != yaml.MappingNode {
		return out
	}
	for j := 0; j+1 < len(shards.Content); j += 2 {
		name, m := shards.Content[j].Value, shards.Content[j+1]
		if name == "" || m.Kind != yaml.MappingNode {
			continue
		}
		str := func(k string) string {
			if v := lookup(m, k); v != nil && v.Kind == yaml.ScalarNode {
				return strings.TrimSpace(v.Value)
			}
			return ""
		}
		l := &locked{name: name, url: sourceURL(str), path: str("path"), version: str("version"), line: shards.Content[j].Line}
		if c := str("commit"); c != "" {
			l.version = c
		}
		out[name] = l
	}
	return out
}

func document(src []byte) *yaml.Node {
	var doc yaml.Node
	if err := yaml.Unmarshal(src, &doc); err != nil || len(doc.Content) == 0 {
		return nil
	}
	if root := doc.Content[0]; root.Kind == yaml.MappingNode {
		return root
	}
	return nil
}

func lookup(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

// extractShard makes each dependency of shard.yml an import of the shard it
// names, and each target's main file an import of that file.
//
// Implements: REQ-CRYSTAL-005
func extractShard(src []byte) *lang.Extraction {
	sh := readShard(src)
	ex := &lang.Extraction{}
	for _, name := range sortedKeys(sh.deps) {
		d := sh.deps[name]
		kind := kindDep
		if d.dev {
			kind = kindDevDep
		}
		ex.Imports = append(ex.Imports, lang.RawImport{Spec: name, Module: name, Name: kind, Line: d.line})
	}
	for _, t := range sortedKeys(sh.targets) {
		main := path.Clean(sh.targets[t])
		ex.Imports = append(ex.Imports, lang.RawImport{Spec: "main: " + sh.targets[t], Module: main, Name: kindMain, Line: sh.lines[t]})
	}
	if sh.name != "" {
		ex.Symbols = []lang.Symbol{{Name: sh.name, Kind: "package", Line: nameLine(src)}}
	}
	return ex
}

// extractLock makes each shard of shard.lock an import, so what shards installs is
// on the map even when no file requires it.
//
// Implements: REQ-CRYSTAL-006
func extractLock(src []byte) *lang.Extraction {
	ex := &lang.Extraction{}
	lock := readLock(src)
	for _, name := range sortedKeys(lock) {
		ex.Imports = append(ex.Imports, lang.RawImport{Spec: name, Module: name, Name: kindLocked, Line: lock[name].line})
	}
	return ex
}

// Implements: REQ-CRYSTAL-005
func extractOverride(src []byte) *lang.Extraction {
	ex := &lang.Extraction{}
	over := readOverride(src)
	for _, name := range sortedKeys(over) {
		ex.Imports = append(ex.Imports, lang.RawImport{Spec: name, Module: name, Name: kindOverride, Line: over[name].line})
	}
	return ex
}

func nameLine(src []byte) int {
	for i, l := range strings.Split(string(src), "\n") {
		if strings.HasPrefix(l, "name:") {
			return i + 1
		}
	}
	return 1
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
