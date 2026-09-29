package javascript

import (
	"encoding/json"
	"path"
	"sort"
	"strings"

	"github.com/tidwall/jsonc"
)

// Bun has written two lock files. bun.lockb, the binary one of Bun before 1.2,
// is not read: its layout is Bun's in-memory one and undocumented. A project
// that keeps one usually has `bun install --yarn` (or bunfig's `print =
// "yarn"`) write a yarn.lock beside it, and that one is read as any yarn.lock.
// bun.lock, the text lock file of Bun 1.1.39 onwards and the default since 1.2,
// is JSON with trailing commas:
//
//	"workspaces": { "": { "name": "app", "dependencies": { "react": "^18" } },
//	                "packages/ui": { "name": "ui", ... } },
//	"packages": {
//	  "react": ["react@18.3.1", "", { "dependencies": { ... } }, "sha512-…"],
//	  "ui/react": ["react@17.0.2", ...],       // installed under ui, not hoisted
//	  "forge-std": ["forge-std@github:foundry-rs/forge-std#bf647bd", {}, "…"],
//	  "ui": ["ui@workspace:packages/ui"],
//	}
//
// A package key is the node_modules path with the node_modules left out: "a/b"
// is b installed under a (or under the workspace package named a), "b" is b
// hoisted to the top. The first element of the tuple is "name@resolution";
// the object among the others carries the package's own dependencies.

// bunLock is bun.lock, as far as the resolver reads it; bun.lockb has no reader.
//
// Implements: REQ-JS-016, REQ-JS-017
type bunLock struct {
	Workspaces map[string]bunWorkspace `json:"workspaces"`
	Packages   map[string]json.RawMessage
}

type bunWorkspace struct {
	Name                                                                  string
	Dependencies, DevDependencies, OptionalDependencies, PeerDependencies map[string]string
}

// bunEntry is one package of bun.lock.
type bunEntry struct {
	name         string   // the name it is installed (and imported) under
	parent       string   // the key before the name: "" when hoisted
	version      string   // what pins it, "" when nothing does (a workspace, link, tarball)
	dependencies []string // what it requires
	platform     string   // the platforms it installs on, "os=linux & cpu=x64"; "" for every one
}

// readBunLock decodes a bun.lock, tolerating the trailing commas Bun writes
// and comments a person may have added.
//
// Implements: REQ-JS-016
func readBunLock(data []byte) (*bunLock, error) {
	var lock bunLock
	if err := json.Unmarshal(jsonc.ToJSON(data), &lock); err != nil {
		return nil, err
	}
	return &lock, nil
}

// entries reads the package tuples, keyed as the lock keys them. A tuple that is
// not an array of a "name@resolution" and optional objects is left out.
func (lock *bunLock) entries() map[string]bunEntry {
	out := make(map[string]bunEntry, len(lock.Packages))
	for key, raw := range lock.Packages {
		var tuple []json.RawMessage
		var identifier string
		if json.Unmarshal(raw, &tuple) != nil || len(tuple) == 0 || json.Unmarshal(tuple[0], &identifier) != nil {
			continue
		}
		parent, name := bunKey(key)
		if name == "" {
			continue
		}
		e := bunEntry{name: name, parent: parent, version: bunVersion(bunResolution(identifier))}
		for _, el := range tuple[1:] {
			var info struct {
				Dependencies, OptionalDependencies map[string]string
				// A single platform is a string, several an array.
				OS, CPU, Libc platformList
			}
			if len(el) > 0 && el[0] == '{' && json.Unmarshal(el, &info) == nil {
				for _, m := range []map[string]string{info.Dependencies, info.OptionalDependencies} {
					for dependency := range m {
						e.dependencies = append(e.dependencies, dependency)
					}
				}
				e.platform = platformCondition(info.OS, info.CPU, info.Libc)
				break
			}
		}
		out[key] = e
	}
	return out
}

// bunKey splits a package key into the path it is installed under and its own
// name: "@a/b/@c/d" is @c/d under @a/b, "x/y/z" is z under x/y.
func bunKey(key string) (parent, name string) {
	segments := strings.Split(key, "/")
	last := len(segments) - 1
	if last >= 1 && strings.HasPrefix(segments[last-1], "@") {
		last-- // a scoped name is two segments
	}
	name = strings.Join(segments[last:], "/")
	if last > 0 {
		parent = strings.Join(segments[:last], "/")
	}
	return parent, name
}

// bunResolution is the part of "name@resolution" after the name: the version
// of a registry package, "github:owner/repo#<commit>", "workspace:packages/x"…
// The name is the package's real one (an alias's key names the alias) and may
// be scoped, so the separator is the first "@" after the first character.
func bunResolution(identifier string) string {
	if len(identifier) < 2 {
		return ""
	}
	i := strings.IndexByte(identifier[1:], '@')
	if i < 0 {
		return ""
	}
	return identifier[i+2:]
}

// bunVersion is what a resolution pins the package to, or "" when it pins
// nothing: a registry version pins, and so does a git or GitHub resolution to
// a commit (Bun records the commit it checked out, abbreviated, beside the
// integrity of what it downloaded). Workspaces, links and folders are the
// project's own; a tarball URL names no version.
//
// Implements: REQ-JS-016
func bunVersion(resolution string) string {
	switch {
	case startsWithDigit(resolution):
		return resolution
	case strings.HasPrefix(resolution, "github:"), strings.HasPrefix(resolution, "git+"), strings.HasPrefix(resolution, "git@"),
		strings.HasPrefix(resolution, "git://"):
		if _, reference, ok := strings.Cut(resolution, "#"); ok && bunCommit(reference) {
			return resolution
		}
	}
	return ""
}

// bunCommit reports whether reference is a git commit, full or abbreviated as Bun
// writes it (at least 7 hex digits).
func bunCommit(reference string) bool {
	if len(reference) < 7 || len(reference) > 64 {
		return false
	}
	for _, c := range reference {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// versions returns the pinned versions per directory relative to the lock
// file. The lock's own directory gets every hoisted package, as
// package-lock.json's top-level node_modules entries do; each workspace gets
// the packages it declares, where one installed under the workspace's name
// ("ui/react") wins over the hoisted one.
//
// Implements: REQ-JS-016
func (lock *bunLock) versions(entries map[string]bunEntry) map[string]map[string]string {
	root := map[string]string{}
	for _, e := range entries {
		if e.parent == "" && e.version != "" {
			root[e.name] = e.version
		}
	}
	out := map[string]map[string]string{".": root}
	for directory, workspace := range lock.Workspaces {
		directory = path.Clean("./" + directory)
		m := out[directory]
		if m == nil {
			m = map[string]string{}
			out[directory] = m
		}
		for _, declared := range []map[string]string{workspace.OptionalDependencies, workspace.PeerDependencies, workspace.DevDependencies, workspace.Dependencies} {
			for dependency := range declared {
				e, ok := entries[workspace.Name+"/"+dependency]
				if !ok {
					e, ok = entries[dependency]
				}
				if ok && e.version != "" {
					m[dependency] = e.version
				}
			}
		}
	}
	return out
}

// addBunTree reads the dependency edges of bun.lock. The version a name stands
// for is the hoisted one; one installed under a single package ("a/b") is
// remembered as what a requires, as node_modules/a/node_modules/b would be
// found first by a's own require. bun.lock comes after every other lock file,
// so a version one of those already gave is kept.
//
// Implements: REQ-SUP-009
func (t *tree) addBunTree(entries map[string]bunEntry) {
	for _, e := range entries {
		if e.parent == "" && e.version != "" && t.locked[e.name] == "" {
			t.locked[e.name] = e.version
		}
	}
	keys := make([]string, 0, len(entries))
	for key := range entries {
		keys = append(keys, key)
	}
	sort.Strings(keys) // which nested version stands for an unhoisted name
	for _, key := range keys {
		e := entries[key]
		t.add(e.name, e.dependencies...)
		// Implements: REQ-JS-018
		t.addPlatform([]string{e.name}, e.platform)
		if e.parent == "" {
			continue
		}
		// Installed under one package (not deeper): that package's own answer.
		if grand, _ := bunKey(e.parent); grand == "" {
			if _, ok := t.nested[e.parent+"/"+e.name]; !ok {
				t.nested[e.parent+"/"+e.name] = e.version
			}
		}
		// Nothing hoisted under the name: a nested version is still one it has.
		if e.version != "" && t.locked[e.name] == "" {
			t.locked[e.name] = e.version
		}
	}
}
