package lua

import (
	"bytes"
	"encoding/json"
	"path"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/tidwall/jsonc"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/luarocks"
)

// extractRockspec makes a rockspec's dependencies imports of the rocks they name
// (lua itself is the interpreter, not a rock) and its build.modules entries imports
// of the files that provide them.
//
// Implements: REQ-LUA-006
func extractRockspec(source []byte) *lang.Extraction {
	r := luarocks.ReadRockspec(source)
	extraction := &lang.Extraction{}
	seen := map[string]bool{}
	for _, d := range r.Dependencies {
		if d.Name == "lua" || seen[d.Spec] {
			continue
		}
		seen[d.Spec] = true
		extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: d.Spec, Module: d.Name, Name: kindDependency + ":" + d.Constraint, Line: d.Line})
	}
	for _, m := range r.Modules {
		extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: "modules[" + m.Name + "]", Module: m.File, Name: kindModule, Line: m.Line})
	}
	sortImports(extraction.Imports)
	return extraction
}

func sortImports(ims []lang.RawImport) {
	sort.SliceStable(ims, func(i, j int) bool { return ims[i].Line < ims[j].Line })
}

// wallyDependency is one dependency of a wally.toml: Roact = "roblox/roact@1.4.0".
type wallyDependency struct {
	alias, packageName, requirement, section string
	line                                     int
}

// wallySections are wally.toml's dependency tables.
var wallySections = []string{"dependencies", "server-dependencies", "dev-dependencies"}

// readWally reads a wally.toml's package name and dependencies.
func readWally(source []byte) (string, []wallyDependency) {
	var doc map[string]map[string]any
	if _, err := toml.Decode(string(source), &doc); err != nil {
		return "", nil
	}
	name, _ := doc["package"]["name"].(string)
	lines := strings.Split(string(source), "\n")
	var out []wallyDependency
	for _, section := range wallySections {
		keys := make([]string, 0, len(doc[section]))
		for k := range doc[section] {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, alias := range keys {
			v, _ := doc[section][alias].(string)
			packageName, requirement, _ := strings.Cut(v, "@")
			if packageName == "" {
				continue
			}
			out = append(out, wallyDependency{alias: alias, packageName: strings.ToLower(packageName), requirement: requirement, section: section, line: lineOf(lines, alias, section)})
		}
	}
	return strings.ToLower(name), out
}

// lineOf finds the line of a key in a TOML section, 1-based (0 when not found).
func lineOf(lines []string, key, section string) int {
	in := false
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "[") {
			in = strings.Trim(t, "[] ") == section
			continue
		}
		if in && (strings.HasPrefix(t, key+" ") || strings.HasPrefix(t, key+"=") || strings.HasPrefix(t, `"`+key+`"`)) {
			return i + 1
		}
	}
	return 0
}

// extractWally makes a wally.toml's dependencies imports of the packages they name.
//
// Implements: REQ-LUA-009
func extractWally(source []byte) *lang.Extraction {
	_, dependencies := readWally(source)
	extraction := &lang.Extraction{}
	for _, d := range dependencies {
		spec := d.alias + ` = "` + d.packageName + "@" + d.requirement + `"`
		extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: spec, Module: d.packageName, Name: kindWally + ":" + d.requirement, Line: d.line})
	}
	sortImports(extraction.Imports)
	return extraction
}

// wallyLock is a wally.lock: every package's locked version and dependencies.
type wallyLock struct {
	versions     map[string]string   // scope/name -> version
	dependencies map[string][]string // scope/name -> scope/name@version
}

func readWallyLock(source []byte) *wallyLock {
	var doc struct {
		Package []struct {
			Name         string     `toml:"name"`
			Version      string     `toml:"version"`
			Dependencies [][]string `toml:"dependencies"`
		} `toml:"package"`
	}
	l := &wallyLock{versions: map[string]string{}, dependencies: map[string][]string{}}
	if _, err := toml.Decode(string(source), &doc); err != nil {
		return l
	}
	for _, p := range doc.Package {
		name := strings.ToLower(p.Name)
		l.versions[name] = p.Version
		for _, d := range p.Dependencies {
			if len(d) == 2 {
				l.dependencies[name] = append(l.dependencies[name], strings.ToLower(d[1]))
			}
		}
	}
	return l
}

// rojoNode is a node of a Rojo project tree.
type rojoNode struct {
	name     string
	path     string // $path, relative to the project file
	class    string // $className
	children []*rojoNode
}

func readRojo(source []byte) (*rojoNode, bool) {
	var doc struct {
		Tree json.RawMessage `json:"tree"`
	}
	if err := json.Unmarshal(jsonc.ToJSON(source), &doc); err != nil || doc.Tree == nil {
		return nil, false
	}
	var build func(name string, raw json.RawMessage, depth int) *rojoNode
	build = func(name string, raw json.RawMessage, depth int) *rojoNode {
		var fields map[string]json.RawMessage
		if depth > 50 || json.Unmarshal(raw, &fields) != nil {
			return nil
		}
		n := &rojoNode{name: name}
		keys := make([]string, 0, len(fields))
		for k := range fields {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			switch {
			case k == "$path":
				var p any
				json.Unmarshal(fields[k], &p)
				switch v := p.(type) {
				case string:
					n.path = v
				case map[string]any: // { "optional": "path" }
					n.path, _ = v["optional"].(string)
				}
			case k == "$className":
				json.Unmarshal(fields[k], &n.class)
			case strings.HasPrefix(k, "$"):
			default:
				if c := build(k, fields[k], depth+1); c != nil {
					n.children = append(n.children, c)
				}
			}
		}
		return n
	}
	root := build("", doc.Tree, 0)
	return root, root != nil
}

// extractRojo makes a Rojo project's $path entries imports of the files and
// directories they put into the game.
//
// Implements: REQ-LUA-010
func extractRojo(source []byte) *lang.Extraction {
	extraction := &lang.Extraction{}
	root, ok := readRojo(source)
	if !ok {
		return extraction
	}
	lines := bytes.Split(source, []byte("\n"))
	seen := map[string]bool{}
	var walk func(n *rojoNode)
	walk = func(n *rojoNode) {
		if n.path != "" && !seen[n.path] {
			seen[n.path] = true
			line := 0
			quoted, _ := json.Marshal(n.path)
			for i, l := range lines {
				if bytes.Contains(l, []byte(`"$path"`)) && bytes.Contains(l, quoted) {
					line = i + 1
					break
				}
			}
			extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: "$path: " + n.path, Module: n.path, Name: kindRojo, Line: line})
		}
		for _, c := range n.children {
			walk(c)
		}
	}
	walk(root)
	sortImports(extraction.Imports)
	return extraction
}

// readLuarc reads what a lua-language-server configuration (.luarc.json) adds to
// the module search: the directories of its runtime.path templates ("lua/?.lua" ->
// lua) and its workspace.library entries, relative to the file's directory. Absolute
// and variable paths ($VIMRUNTIME, ${3rd}) are left out.
func readLuarc(source []byte, directory string) []string {
	var doc map[string]any
	if json.Unmarshal(jsonc.ToJSON(source), &doc) != nil {
		return nil
	}
	get := func(dotted string) []string {
		v, ok := doc[dotted]
		if !ok {
			first, rest, _ := strings.Cut(dotted, ".")
			if m, ok := doc[first].(map[string]any); ok {
				v = m[rest]
			}
		}
		var out []string
		switch l := v.(type) {
		case []any:
			for _, e := range l {
				if s, ok := e.(string); ok {
					out = append(out, s)
				}
			}
		case string:
			out = append(out, l)
		}
		return out
	}
	var roots []string
	add := func(p string) {
		if p == "" || strings.ContainsAny(p[:1], "/$~\\") || strings.Contains(p, ":") {
			return
		}
		roots = append(roots, path.Clean(path.Join(directory, p)))
	}
	for _, t := range get("runtime.path") {
		if i := strings.Index(t, "?"); i >= 0 {
			if d := strings.TrimSuffix(t[:i], "/"); d != "" {
				add(d)
			}
		}
	}
	for _, l := range get("workspace.library") {
		add(l)
	}
	return roots
}

// readLuaurc reads a .luaurc's require aliases ("aliases": {"pkg": "./Packages"}),
// keyed in lower case (aliases are case-insensitive).
func readLuaurc(source []byte) map[string]string {
	var doc struct {
		Aliases map[string]string `json:"aliases"`
	}
	if json.Unmarshal(jsonc.ToJSON(source), &doc) != nil {
		return nil
	}
	out := map[string]string{}
	for k, v := range doc.Aliases {
		out[strings.ToLower(strings.TrimPrefix(k, "@"))] = v
	}
	return out
}
