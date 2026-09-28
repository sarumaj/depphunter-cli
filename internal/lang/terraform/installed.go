package terraform

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// installedModule is an entry of .terraform/modules/modules.json, which
// `terraform init` writes in the directory it ran in: a module call by its key
// (the call names from that root module down, "vpc" or "network.vpc"), its
// source address as Terraform normalized it, the registry version it installed
// and the directory it installed into.
type installedModule struct {
	Key     string `json:"Key"`
	Source  string `json:"Source"`
	Version string `json:"Version"`
	Dir     string `json:"Dir"`
}

// maxNested bounds how deep local module calls are followed for Dependencies.
const maxNested = 16

// readInstalled reads a module directory's .terraform/modules/modules.json from
// disk (Terraform's download directory is never scanned). A missing or garbage
// file lists nothing.
//
// Implements: REQ-TERRAFORM-008
func readInstalled(root, dir string) []installedModule {
	if root == "" {
		return nil
	}
	src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(dir), ".terraform", "modules", "modules.json"))
	if err != nil || len(src) > lang.MaxParseSize {
		return nil
	}
	var raw struct {
		Modules []installedModule `json:"Modules"`
	}
	if json.Unmarshal(src, &raw) != nil {
		return nil
	}
	out := raw.Modules[:0]
	for _, m := range raw.Modules {
		if m.Key != "" && m.Source != "" {
			out = append(out, m)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

// installedVersion is the version `terraform init` installed for the module call
// named call with the registry module pkg in dir: from the dir's own
// modules.json (key call), else from that of a module calling it, directly or
// not (a key ending in .call). "" when none installed it.
func (r *resolver) installedVersion(dir, call, pkg string) string {
	found := ""
	r.callersOf(dir, func(d string) bool {
		for _, m := range r.modules[d].installedList() {
			if m.Version == "" || m.Key != call && !strings.HasSuffix(m.Key, "."+call) {
				continue
			}
			if s, ok := parseSource(m.Source); ok && s.pkg == pkg {
				found = m.Version
				return true
			}
		}
		return false
	})
	return found
}

func (m *moduleDir) installedList() []installedModule {
	if m == nil {
		return nil
	}
	return m.installed
}

// installedCall is the modules.json entry that installed a registry module at
// a version, with the list it is in.
func (r *resolver) installedCall(t lang.Target) (installedModule, []installedModule, bool) {
	dirs := make([]string, 0, len(r.modules))
	for d := range r.modules {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)
	for _, d := range dirs {
		list := r.modules[d].installed
		for _, m := range list {
			if m.Version == "" || t.Version != "" && m.Version != t.Version {
				continue
			}
			if s, ok := parseSource(m.Source); ok && s.pkg == t.Package {
				return m, list, true
			}
		}
	}
	return installedModule{}, nil, false
}

// Dependencies lists the modules a registry module installed by `terraform
// init` calls: the entries of the same modules.json one key segment below it,
// through its local module calls. A registry module is pinned at the version
// installed; any other source follows the pin rule of its address.
//
// Implements: REQ-TERRAFORM-008
func (r *resolver) Dependencies(t lang.Target) []lang.Target {
	if t.Ecosystem != ecoModule {
		return nil
	}
	m, list, ok := r.installedCall(t)
	if !ok {
		return nil
	}
	var out []lang.Target
	seen := map[string]bool{}
	var children func(key string, depth int)
	children = func(key string, depth int) {
		for _, c := range list {
			rest, ok := strings.CutPrefix(c.Key, key+".")
			if !ok || strings.Contains(rest, ".") {
				continue
			}
			s, ok := parseSource(c.Source)
			switch {
			case !ok:
			case s.local != "":
				if depth < maxNested {
					children(c.Key, depth+1)
				}
			default:
				dt := r.moduleTarget(".", c.Source, "")
				if s.origin == "" && c.Version != "" {
					dt = lang.Target{Ecosystem: ecoModule, Package: s.pkg, Version: c.Version, Pinned: true}
				}
				if k := dt.Package + "@" + dt.Version; dt.Package != "" && !seen[k] {
					seen[k] = true
					out = append(out, dt)
				}
			}
		}
	}
	children(m.Key, 0)
	return out
}

// Installed reports whether a module's dependencies come from what `terraform
// init` installed.
func (r *resolver) Installed(t lang.Target) bool {
	if t.Ecosystem != ecoModule {
		return false
	}
	_, _, ok := r.installedCall(t)
	return ok
}

var _ lang.Installed = (*resolver)(nil)

// installedPin pins a registry module call that its version constraint leaves
// open to the version `terraform init` installed for it: the lock in effect,
// with the constraint as requested.
//
// Implements: REQ-TERRAFORM-009
func (r *resolver) installedPin(dir, call string, t lang.Target) lang.Target {
	if t.Ecosystem != ecoModule || t.Origin != "" || t.Pinned || call == "" {
		return t
	}
	if v := r.installedVersion(dir, call, t.Package); v != "" {
		t.Requested, t.Version, t.Pinned, t.Floating = t.Version, v, true, false
	}
	return t
}

// callName is the name of the module call an import's spec (`module "vpc"`)
// writes.
func callName(spec string) string {
	name, ok := strings.CutPrefix(spec, `module "`)
	if !ok {
		return ""
	}
	return strings.TrimSuffix(name, `"`)
}
