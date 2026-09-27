// Package nuget reads what a .NET repository says about NuGet packages, for every
// plugin that resolves to them (C#, F#): PackageReference items of project files,
// central versions (Directory.Packages.props), Directory.Build.props, Paket's
// paket.dependencies, paket.lock and paket.references, and NuGet's
// packages.lock.json. It is not a plugin.
//
// Both plugins build a Store from the same file list, so a package both use has one
// spelling and one version and lands on one node: NuGet ids are case-insensitive,
// and the spelling a lock file records (the package's own) wins over what a
// manifest wrote.
package nuget

import (
	"encoding/json"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

const (
	Ecosystem = "nuget"
	Dotnet    = "dotnet"
	// Paket is the island of what Paket fetches from outside NuGet: files and
	// repositories from GitHub, gists, git servers and HTTP.
	Paket = "paket"
)

// Ecosystems are the NuGet and .NET base library islands, as every .NET plugin
// declares them.
func Ecosystems() []lang.Ecosystem {
	return []lang.Ecosystem{
		{ID: Ecosystem, Name: "NuGet"},
		{ID: Dotnet, Name: ".NET base library", Std: true},
	}
}

// entry is everything one set knows about one package id.
type entry struct {
	declared string // version or constraint as a manifest wrote it
	paket    bool   // declared by paket.dependencies (Paket's pin rule applies)
	listed   bool   // a manifest names it (not only a lock)
	locked   string // version a lock file resolved it to
	deps     []LockDep
}

// set is the packages of one Paket root (the directory of a paket.dependencies,
// with its paket.lock and the paket.references below it), or of the MSBuild files
// and packages.lock.json files of the whole repository.
type set struct {
	dir    string
	pkgs   map[string]*entry            // lower-case id -> over all groups, Main first
	groups map[string]map[string]*entry // lower-case group -> lower-case id -> entry
}

func newSet(dir string) *set {
	return &set{dir: dir, pkgs: map[string]*entry{}, groups: map[string]map[string]*entry{}}
}

func (s *set) entry(group, key string) []*entry {
	g := strings.ToLower(group)
	if s.groups[g] == nil {
		s.groups[g] = map[string]*entry{}
	}
	out := make([]*entry, 0, 2)
	for _, m := range []map[string]*entry{s.groups[g], s.pkgs} {
		e := m[key]
		if e == nil {
			e = &entry{}
			m[key] = e
		}
		out = append(out, e)
	}
	return out
}

// remote is a Paket github/gist/git/http dependency.
type remote struct {
	name   string // RemoteName
	ref    string // what paket.dependencies asked for
	commit string // what paket.lock resolved
	kind   string
	files  []string // file names it provides (for paket.references File: lines)
}

// Store is the repository's NuGet knowledge.
//
// Implements: REQ-FSHARP-008, REQ-CS-002, REQ-CS-003, REQ-CS-004
type Store struct {
	ids map[string]string // lower-case id -> spelling
	// lockSpelled marks the ids whose spelling came from a lock file, which records
	// the package's own and so replaces what a manifest wrote.
	lockSpelled map[string]bool
	msbuild     *set
	roots       []*set // Paket roots, shallowest first
	remotes     map[string]*remote
	order       []string // remote names in the order first seen
}

// Read builds the store from the project files, props files, Paket files and lock
// files among all.
func Read(all []*scan.File) *Store {
	s := &Store{ids: map[string]string{}, lockSpelled: map[string]bool{}, msbuild: newSet(""), remotes: map[string]*remote{}}
	central := map[string]string{}
	var locks, paketLocks, references []*scan.File
	byDir := map[string]*set{}
	for _, f := range all {
		if f.Binary || f.TooLarge {
			continue
		}
		if path.Base(f.Path) == "paket.dependencies" {
			d := path.Dir(f.Path)
			byDir[d] = newSet(d)
			s.roots = append(s.roots, byDir[d])
		}
	}
	sort.SliceStable(s.roots, func(i, j int) bool { return depth(s.roots[i].dir) < depth(s.roots[j].dir) })
	for _, f := range all {
		if f.Binary || f.TooLarge {
			continue
		}
		base := path.Base(f.Path)
		switch {
		case IsProject(base) || base == "Directory.Packages.props" || base == "Directory.Build.props":
			data, err := os.ReadFile(f.Abs)
			if err != nil {
				continue
			}
			for _, it := range ReadProject(data).Items {
				switch it.Kind {
				case "PackageVersion":
					if it.Include != "" {
						s.spell(it.Include)
						central[strings.ToLower(it.Include)] = it.Version
					}
				case "PackageReference", "GlobalPackageReference":
					if it.Include != "" {
						s.declare(s.msbuild, MainGroup, it.Include, it.Version, false)
					}
				}
			}
		case base == "paket.dependencies":
			data, err := os.ReadFile(f.Abs)
			if err != nil {
				continue
			}
			deps, _ := ParseDependencies(data)
			for _, d := range deps {
				if d.Kind == "nuget" {
					s.declare(byDir[path.Dir(f.Path)], d.Group, d.Name, d.Constraint, true)
					continue
				}
				if r := s.remote(d.Kind, d.Name); r != nil {
					if r.ref == "" {
						r.ref = d.Constraint
					}
					if d.File != "" {
						r.files = append(r.files, path.Base(d.File))
					}
				}
			}
		case base == "paket.lock":
			paketLocks = append(paketLocks, f)
		case base == "paket.references":
			references = append(references, f)
		case base == "packages.lock.json":
			locks = append(locks, f)
		}
	}
	for key, e := range s.msbuild.pkgs {
		if e.declared == "" {
			e.declared = central[key]
		}
	}
	for _, f := range paketLocks {
		data, err := os.ReadFile(f.Abs)
		if err != nil {
			continue
		}
		root := byDir[path.Dir(f.Path)]
		if root == nil { // a lock without its paket.dependencies still pins
			root = newSet(path.Dir(f.Path))
			byDir[root.dir] = root
			s.roots = append(s.roots, root)
		}
		// The Main group first, so a package several groups lock takes Main's version.
		entries := ParseLock(data)
		sort.SliceStable(entries, func(i, j int) bool {
			return strings.EqualFold(entries[i].Group, MainGroup) && !strings.EqualFold(entries[j].Group, MainGroup)
		})
		for _, l := range entries {
			if l.Kind == "nuget" {
				s.lock(root, l.Group, l.Name, l.Version, l.Deps)
				continue
			}
			if r := s.remote(l.Kind, lockRemote(l)); r != nil {
				if r.commit == "" {
					r.commit = l.Version
				}
				if l.Name != "" {
					r.files = append(r.files, path.Base(l.Name))
				}
			}
		}
	}
	sort.SliceStable(s.roots, func(i, j int) bool { return depth(s.roots[i].dir) < depth(s.roots[j].dir) })
	// paket.references names what a project uses: a package it names is declared
	// for the namespace rule even when only a lock lists it.
	for _, f := range references {
		data, err := os.ReadFile(f.Abs)
		if err != nil {
			continue
		}
		root := s.root(f.Path)
		if root == nil {
			continue
		}
		for _, ref := range ParseReferences(data) {
			key := strings.ToLower(ref.Name)
			if ref.File || root.pkgs[key] == nil {
				continue // a package Paket does not know stays undeclared
			}
			for _, e := range root.entry(ref.Group, key) {
				e.listed = true
			}
		}
	}
	for _, f := range locks {
		readPackagesLock(s, f)
	}
	return s
}

func depth(dir string) int {
	if dir == "." || dir == "" {
		return 0
	}
	return strings.Count(dir, "/") + 1
}

// root is the Paket root governing a file: the nearest directory above it with a
// paket.dependencies (or paket.lock).
func (s *Store) root(file string) *set {
	var best *set
	for _, r := range s.roots {
		if r.dir == "." || strings.HasPrefix(file, r.dir+"/") {
			if best == nil || depth(r.dir) > depth(best.dir) {
				best = r
			}
		}
	}
	return best
}

// IsProject reports whether base names an MSBuild project file of a .NET language.
func IsProject(base string) bool {
	for _, ext := range []string{".csproj", ".fsproj", ".vbproj"} {
		if strings.HasSuffix(base, ext) {
			return true
		}
	}
	return false
}

func lockRemote(l Locked) string {
	if l.Kind == "http" && l.Name != "" && !strings.Contains(l.Remote, l.Name) {
		return strings.TrimRight(l.Remote, "/") + "/" + strings.TrimLeft(l.Name, "/")
	}
	return l.Remote
}

// spell records the first spelling of an id (a lock file's replaces it later).
func (s *Store) spell(id string) string {
	key := strings.ToLower(id)
	if _, ok := s.ids[key]; !ok {
		s.ids[key] = id
	}
	return key
}

func (s *Store) declare(in *set, group, id, version string, paket bool) {
	if in == nil {
		return
	}
	key := s.spell(id)
	for _, e := range in.entry(group, key) {
		if !e.listed || e.declared == "" && version != "" {
			e.declared, e.paket, e.listed = version, paket, true
		}
	}
}

func (s *Store) lock(in *set, group, id, version string, deps []LockDep) {
	if id == "" || version == "" {
		return
	}
	key := strings.ToLower(id)
	if !s.lockSpelled[key] {
		s.ids[key], s.lockSpelled[key] = id, true
	}
	for _, e := range in.entry(group, key) {
		if e.locked == "" {
			e.locked, e.deps = version, deps
		}
	}
}

func (s *Store) remote(kind, name string) *remote {
	n := RemoteName(kind, name)
	if n == "" {
		return nil
	}
	r := s.remotes[n]
	if r == nil {
		r = &remote{name: n, kind: kind}
		s.remotes[n] = r
		s.order = append(s.order, n)
	}
	return r
}

// packagesLock is NuGet's packages.lock.json: per target framework, every package
// restored with what was requested, what was resolved and what it depends on.
type packagesLock struct {
	Dependencies map[string]map[string]struct {
		Type         string            `json:"type"`
		Requested    string            `json:"requested"`
		Resolved     string            `json:"resolved"`
		Dependencies map[string]string `json:"dependencies"`
	} `json:"dependencies"`
}

// Implements: REQ-FSHARP-008
func readPackagesLock(s *Store, f *scan.File) {
	data, err := os.ReadFile(f.Abs)
	if err != nil {
		return
	}
	var doc packagesLock
	if json.Unmarshal(data, &doc) != nil {
		return
	}
	frameworks := make([]string, 0, len(doc.Dependencies))
	for fw := range doc.Dependencies {
		frameworks = append(frameworks, fw)
	}
	sort.Strings(frameworks)
	for _, fw := range frameworks {
		pkgs := doc.Dependencies[fw]
		ids := make([]string, 0, len(pkgs))
		for id := range pkgs {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			p := pkgs[id]
			if strings.EqualFold(p.Type, "Project") || p.Resolved == "" {
				continue
			}
			var deps []LockDep
			names := make([]string, 0, len(p.Dependencies))
			for n := range p.Dependencies {
				names = append(names, n)
			}
			sort.Strings(names)
			for _, n := range names {
				deps = append(deps, LockDep{Name: n, Constraint: p.Dependencies[n]})
			}
			s.lock(s.msbuild, MainGroup, id, p.Resolved, deps)
			if e := s.msbuild.pkgs[strings.ToLower(id)]; e != nil && !e.listed && e.declared == "" && p.Requested != "" {
				e.declared = p.Requested
			}
		}
	}
}

// ID is the store's spelling of a package id, or id itself when nothing names it.
func (s *Store) ID(id string) string {
	if v, ok := s.ids[strings.ToLower(id)]; ok {
		return v
	}
	return id
}

// sets lists where to look a package up for a file, most authoritative first: the
// Paket root governing the file, the MSBuild files, the other Paket roots
// (shallowest first). file "" skips the first.
func (s *Store) sets(file string) []*set {
	out := make([]*set, 0, len(s.roots)+2)
	own := s.root(file)
	if file != "" && own != nil {
		out = append(out, own)
	}
	out = append(out, s.msbuild)
	for _, r := range s.roots {
		if r != own || file == "" {
			out = append(out, r)
		}
	}
	return out
}

// Package is the target of a package id the repository knows (a manifest names it
// or a lock file resolved it) as seen from file ("" for the repository as a whole),
// with the version of the given Paket group when that group has one.
//
// Implements: REQ-FSHARP-008
func (s *Store) Package(file, group, id string) (lang.Target, bool) {
	key := strings.ToLower(id)
	for _, in := range s.sets(file) {
		e := in.pkgs[key]
		if group != "" {
			if g := in.groups[strings.ToLower(group)][key]; g != nil {
				e = g
			}
		}
		if e != nil {
			return s.target(key, e), true
		}
	}
	return lang.Target{}, false
}

// Version is the target of a package whose version the importer states itself (an
// F# script's `#r "nuget: X, 1.2.3"`): a lock file of the repository still pins it,
// else the version pins when exact and floats otherwise, and no version floats.
func (s *Store) Version(file, id, version string) lang.Target {
	if t, ok := s.Package(file, "", id); ok && t.Pinned && (version == "" || version == t.Version) {
		return t
	}
	t := lang.Target{Ecosystem: Ecosystem, Package: s.ID(id), Version: version, Pinned: lang.Pinned(version)}
	if version == "" {
		t.Floating = true
	}
	return t
}

// Implements: REQ-CS-007, REQ-FSHARP-008
func (s *Store) target(key string, e *entry) lang.Target {
	t := lang.Target{Ecosystem: Ecosystem, Package: s.ID(key)}
	switch {
	case e.locked != "":
		t.Version, t.Pinned = e.locked, true
		if e.declared != "" && e.declared != e.locked {
			if v, _ := PaketVersion(e.declared); !e.paket || v != e.locked {
				t.Requested = e.declared
			}
		}
	case e.paket:
		t.Version, t.Pinned = PaketVersion(e.declared)
		t.Floating = e.declared == ""
	default:
		// A PackageReference version is a minimum, but restore installs exactly it
		// when it exists: the versions that move are the wildcards and the ranges.
		t.Version, t.Pinned = e.declared, lang.Pinned(e.declared)
	}
	return t
}

// match finds the listed package for a namespace by rule: longest id prefixing it,
// or shortest id below it.
func (s *Store) match(file string, pick func(key string, best string) bool) (lang.Target, bool) {
	for _, in := range s.sets(file) {
		best := ""
		for key, e := range in.pkgs {
			if e.listed && pick(key, best) {
				best = key
			}
		}
		if best != "" {
			return s.target(best, in.pkgs[best]), true
		}
	}
	return lang.Target{}, false
}

// Declared is the package whose id is the longest case-insensitive prefix of the
// namespace ns among the packages manifests name (xunit provides Xunit.*): a lock
// file's transitive packages do not count, or System.Net.Http from a lock would take
// the base library's namespace.
//
// Implements: REQ-CS-002, REQ-CS-004
func (s *Store) Declared(file, ns string) (lang.Target, bool) {
	lower := strings.ToLower(ns)
	return s.match(file, func(key, best string) bool {
		return len(key) > len(best) && (lower == key || strings.HasPrefix(lower, key+"."))
	})
}

// Under is the declared package with the shortest id below the namespace ns: FAKE's
// modules share the namespace Fake.Core across Fake.Core.Target, Fake.Core.Process
// and more.
func (s *Store) Under(file, ns string) (lang.Target, bool) {
	prefix := strings.ToLower(ns) + "."
	return s.match(file, func(key, best string) bool {
		return strings.HasPrefix(key, prefix) && (best == "" || len(key) < len(best) || len(key) == len(best) && key < best)
	})
}

// Namespace resolves a namespace no project declares: a declared package, else the
// .NET base library (System.*, Microsoft.*, Windows.*, named by the first two
// segments), else an unresolved NuGet package named by the first two segments.
//
// Implements: REQ-CS-002, REQ-CS-007
func (s *Store) Namespace(ns string) lang.Target {
	if t, ok := s.Declared("", ns); ok {
		return t
	}
	return Undeclared(ns)
}

// Undeclared is Namespace without the declared packages.
func Undeclared(ns string) lang.Target {
	segments := strings.Split(ns, ".")
	top := strings.Join(segments[:min(2, len(segments))], ".")
	switch segments[0] {
	case "System", "Microsoft", "Windows":
		return lang.Target{Ecosystem: Dotnet, Package: top}
	}
	return lang.Target{Ecosystem: Ecosystem, Package: top, Unresolved: true}
}

// Remote is the target of a Paket github/gist/git/http dependency by kind and name
// (owner/repo or URL): pinned by paket.lock's commit, or by a commit
// paket.dependencies names; a branch, tag or tag range is shown and floats; no ref
// and every HTTP file float.
//
// Implements: REQ-FSHARP-007
func (s *Store) Remote(kind, name, ref string) (lang.Target, bool) {
	n := RemoteName(kind, name)
	if n == "" {
		return lang.Target{}, false
	}
	t := lang.Target{Ecosystem: Paket, Package: n}
	r := s.remotes[n]
	if r != nil && ref == "" {
		ref = r.ref
	}
	switch {
	case kind == "http":
		t.Floating = true
	case r != nil && r.commit != "":
		t.Version, t.Pinned = r.commit, true
		if ref != "" && ref != r.commit {
			t.Requested = ref
		}
	case ref != "":
		t.Version, t.Pinned = ref, lang.Commit(ref)
	default:
		t.Floating = true
	}
	return t, true
}

// RemoteFile is the Paket remote that provides a file a paket.references `File:`
// line (or a project's Compile item under paket-files/) names.
func (s *Store) RemoteFile(file string) (lang.Target, bool) {
	base := path.Base(strings.ReplaceAll(file, `\`, "/"))
	for _, n := range s.order {
		r := s.remotes[n]
		for _, f := range r.files {
			if strings.EqualFold(f, base) {
				return s.remoteTarget(r)
			}
		}
	}
	return lang.Target{}, false
}

// RemoteRepo is the Paket remote a path under paket-files/ comes from:
// paket-files/<owner>/<repo>/... for GitHub, paket-files/<host>/<path>... otherwise.
func (s *Store) RemoteRepo(rel string) (lang.Target, bool) {
	segs := strings.Split(rel, "/")
	for _, n := range s.order {
		r := s.remotes[n]
		parts := strings.Split(n, "/")
		switch {
		case len(segs) >= 2 && len(parts) == 3 && (r.kind == "github" || r.kind == "gist") && strings.EqualFold(segs[0], parts[1]) && strings.EqualFold(segs[1], parts[2]):
			return s.remoteTarget(r)
		case len(segs) >= 3 && len(parts) >= 3 && strings.EqualFold(segs[0], parts[0]) && strings.EqualFold(segs[1], parts[1]) && strings.EqualFold(segs[2], parts[2]):
			return s.remoteTarget(r)
		}
	}
	return lang.Target{}, false
}

func (s *Store) remoteTarget(r *remote) (lang.Target, bool) {
	if r.kind == "github" || r.kind == "gist" {
		return s.Remote(r.kind, strings.SplitN(r.name, "/", 2)[1], "")
	}
	return s.Remote(r.kind, r.name, "")
}

// Dependencies is what paket.lock or packages.lock.json say a NuGet package depends
// on, each at the version the same lock resolved it to.
//
// Implements: REQ-FSHARP-007, REQ-SUP-011
func (s *Store) Dependencies(t lang.Target) []lang.Target {
	if t.Ecosystem != Ecosystem {
		return nil
	}
	key := strings.ToLower(t.Package)
	for _, in := range s.sets("") {
		e := in.pkgs[key]
		if e == nil || e.locked == "" {
			continue
		}
		var out []lang.Target
		seen := map[string]bool{}
		for _, d := range e.deps {
			dk := strings.ToLower(d.Name)
			if seen[dk] {
				continue
			}
			seen[dk] = true
			if de := in.pkgs[dk]; de != nil {
				out = append(out, s.target(dk, de))
				continue
			}
			out = append(out, lang.Target{Ecosystem: Ecosystem, Package: s.ID(d.Name), Version: d.Constraint})
		}
		return out
	}
	return nil
}
