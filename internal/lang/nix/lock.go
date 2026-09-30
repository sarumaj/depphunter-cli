package nix

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// flake.lock (versions 5 to 7): a graph of nodes, each with its "locked" source
// (the commit and content hash in use), its "original" reference as flake.nix
// wrote it, and its own "inputs", each the key of another node or, for
// `follows`, a path of input names from the root.
type lockFile struct {
	Nodes map[string]lockNode `json:"nodes"`
	Root  string              `json:"root"`
}

type lockNode struct {
	Inputs   map[string]json.RawMessage `json:"inputs"`
	Locked   map[string]any             `json:"locked"`
	Original map[string]any             `json:"original"`
}

func readLock(source []byte) *lockFile {
	var l lockFile
	if json.Unmarshal(source, &l) != nil || l.Nodes == nil {
		return nil
	}
	l.Root = cmp.Or(l.Root, "root")
	return &l
}

func stringMap(m map[string]any) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		switch v := v.(type) {
		case string:
			out[k] = v
		case float64:
			out[k] = fmt.Sprint(int64(v))
		case bool:
			out[k] = fmt.Sprint(v)
		}
	}
	return out
}

// input returns the node key an input of node names: its own key, or the node at
// the end of a follows path from the root.
func (l *lockFile) input(node, name string) (string, bool) {
	raw, ok := l.Nodes[node].Inputs[name]
	if !ok {
		return "", false
	}
	var key string
	if json.Unmarshal(raw, &key) == nil {
		return key, true
	}
	var follows []string
	if json.Unmarshal(raw, &follows) != nil {
		return "", false
	}
	return l.follow(follows, 0)
}

func (l *lockFile) follow(p []string, depth int) (string, bool) {
	if depth > 16 {
		return "", false
	}
	current := l.Root
	for _, name := range p {
		raw, ok := l.Nodes[current].Inputs[name]
		if !ok {
			return "", false
		}
		var key string
		if json.Unmarshal(raw, &key) == nil {
			current = key
			continue
		}
		var more []string
		if json.Unmarshal(raw, &more) != nil {
			return "", false
		}
		if current, ok = l.follow(more, depth+1); !ok {
			return "", false
		}
	}
	return current, true
}

// target is what a lock node fetches: named by its original reference (as flake.nix
// names it), versioned by the locked commit, pinned by the locked commit or content
// hash, with the original reference as the requested version. A path node returns its
// directory (relative to the root) instead.
//
// Implements: REQ-NIX-005
func (l *lockFile) target(directory, key string) (lang.Target, string) {
	n, ok := l.Nodes[key]
	if !ok {
		return lang.Target{}, ""
	}
	original, locked := attributesReference(stringMap(n.Original)), attributesReference(stringMap(n.Locked))
	if original.typeName == "path" || locked.typeName == "path" {
		return lang.Target{}, localDirectory(directory, first(original.url, locked.url))
	}
	if original.typeName == "" {
		original = locked
	}
	t, ok := original.target()
	if !ok {
		return lang.Target{}, ""
	}
	requested := ""
	if !t.Pinned {
		requested = t.Version
	}
	t.Floating, t.Requested = false, ""
	switch {
	case locked.rev != "":
		t.Version, t.Pinned, t.Git = short(locked.rev), true, locked.git()
	case locked.narHash != "":
		t.Version, t.Pinned = first(t.Version, short(locked.narHash)), true
	}
	if !t.Pinned {
		t.Floating = requested == "" || original.typeName == "indirect"
	}
	if requested != "" && requested != t.Version {
		t.Requested = requested
	}
	return t, ""
}

// keys lists the lock's nodes other than the root, sorted.
func (l *lockFile) keys() []string {
	var out []string
	for k := range l.Nodes {
		if k != l.Root {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// pinsFile is niv's nix/sources.json or npins' npins/sources.json.
type pinsFile map[string]reference

// readPins reads niv's sources.json (kind "niv": a map of sources) or npins'
// (kind "npins": {"pins": {...}}).
//
// Implements: REQ-NIX-008
func readPins(kind string, source []byte) pinsFile {
	out := pinsFile{}
	if kind == "niv" {
		var m map[string]map[string]any
		if json.Unmarshal(source, &m) != nil {
			return nil
		}
		for name, raw := range m {
			f := stringMap(raw)
			r := reference{rev: f["rev"], narHash: f["sha256"], reference: first(f["branch"], f["version"])}
			switch {
			case f["type"] == "git":
				r.typeName, r.url = "git", f["repo"]
			case f["owner"] != "" && f["repo"] != "" && (f["url"] == "" || strings.Contains(f["url"], "github.com")):
				r.typeName, r.owner, r.repository = "github", f["owner"], f["repo"]
			default:
				r.typeName, r.url = "tarball", f["url"]
			}
			out[name] = r
		}
		return out
	}
	var m struct {
		Pins map[string]struct {
			Type       string            `json:"type"`
			Repository map[string]string `json:"repository"`
			Branch     string            `json:"branch"`
			Version    string            `json:"version"`
			Revision   string            `json:"revision"`
			Name       string            `json:"name"`
			URL        string            `json:"url"`
			Hash       string            `json:"hash"`
		} `json:"pins"`
	}
	if json.Unmarshal(source, &m) != nil {
		return nil
	}
	for name, p := range m.Pins {
		r := reference{rev: p.Revision, narHash: p.Hash, reference: first(p.Branch, p.Version)}
		repository := p.Repository
		switch {
		case p.Type == "Channel":
			// A channel release: nixpkgs at that release. The tarball's directory is
			// the release (nixos-24.05.1234.abcdef).
			r = reference{typeName: "indirect", id: "nixpkgs", reference: p.Name, narHash: p.Hash, channel: p.Name}
			if u := strings.TrimSuffix(p.URL, "/nixexprs.tar.xz"); u != p.URL {
				r.reference = path.Base(u)
			}
		case p.Type == "PyPi":
			r = reference{typeName: "tarball", url: "https://pypi.org/project/" + p.Name, reference: p.Version, narHash: p.Hash}
		case repository["type"] == "GitHub":
			r.typeName, r.owner, r.repository = "github", repository["owner"], repository["repo"]
		case repository["type"] == "GitLab":
			r.typeName, r.url = "git", strings.TrimSuffix(first(repository["server"], "https://gitlab.com/"), "/")+"/"+repository["repo_path"]
		case repository["type"] == "Forgejo":
			r.typeName, r.url = "git", strings.TrimSuffix(repository["server"], "/")+"/"+repository["owner"]+"/"+repository["repo"]
		case repository["url"] != "":
			r.typeName, r.url = "git", repository["url"]
		default:
			r.typeName, r.url = "tarball", p.URL
		}
		out[name] = r
	}
	return out
}

// pinTarget is a niv or npins source as a package: pinned by its revision or hash.
func pinTarget(r reference) lang.Target {
	t, _ := r.target()
	if r.typeName == "indirect" && r.narHash != "" { // an npins channel, hashed
		t.Version, t.Pinned, t.Floating = r.reference, true, false
		if r.channel != r.reference {
			t.Requested = r.channel
		}
	}
	return t
}

// extractLock lists flake.lock's nodes, niv's or npins' sources as imports, each
// on the line naming it.
//
// Implements: REQ-NIX-005, REQ-NIX-008
func extractLock(class string, source []byte) *lang.Extraction {
	extraction := &lang.Extraction{Symbols: []lang.Symbol{}}
	var names []string
	kind := class
	switch class {
	case "lock":
		l := readLock(source)
		if l == nil {
			return extraction
		}
		names = l.keys()
	default:
		p := readPins(class, source)
		for k := range p {
			names = append(names, k)
		}
		sort.Strings(names)
	}
	for _, name := range names {
		extraction.Imports = append(extraction.Imports, lang.RawImport{Spec: name, Module: name, Name: kind, Line: keyLine(source, name)})
	}
	return extraction
}

// keyLine is the line of the first `"name": {` in src (1 when not found): in
// flake.lock the same name also appears as other nodes' input values.
func keyLine(source []byte, name string) int {
	q, _ := json.Marshal(name)
	for off := 0; ; {
		i := bytes.Index(source[off:], q)
		if i < 0 {
			return 1
		}
		j := off + i + len(q)
		rest := bytes.TrimLeft(source[j:min(len(source), j+16)], " \t")
		if bytes.HasPrefix(rest, []byte(":")) && bytes.HasPrefix(bytes.TrimLeft(rest[1:], " \t\r\n"), []byte("{")) {
			return bytes.Count(source[:off+i], []byte{'\n'}) + 1
		}
		off = j
	}
}
