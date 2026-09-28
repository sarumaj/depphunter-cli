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
	Key       string `json:"Key"`
	Source    string `json:"Source"`
	Version   string `json:"Version"`
	Directory string `json:"Dir"`
}

// maxNested bounds how deep local module calls are followed for Dependencies.
const maxNested = 16

// readInstalled reads a module directory's .terraform/modules/modules.json from
// disk (Terraform's download directory is never scanned). A missing or garbage
// file lists nothing.
//
// Implements: REQ-TERRAFORM-008
func readInstalled(root, directory string) []installedModule {
	if root == "" {
		return nil
	}
	source, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(directory), ".terraform", "modules", "modules.json"))
	if err != nil || len(source) > lang.MaxParseSize {
		return nil
	}
	var raw struct {
		Modules []installedModule `json:"Modules"`
	}
	if json.Unmarshal(source, &raw) != nil {
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
// named call with the registry module packageName in directory: from the dir's own
// modules.json (key call), else from that of a module calling it, directly or
// not (a key ending in .call). "" when none installed it.
func (r *resolver) installedVersion(directory, call, packageName string) string {
	found := ""
	r.callersOf(directory, func(d string) bool {
		for _, m := range r.modules[d].installedList() {
			if m.Version == "" || m.Key != call && !strings.HasSuffix(m.Key, "."+call) {
				continue
			}
			if s, ok := parseSource(m.Source); ok && s.packageName == packageName {
				found = m.Version
				return true
			}
		}
		return false
	})
	return found
}

func (m *moduleDirectory) installedList() []installedModule {
	if m == nil {
		return nil
	}
	return m.installed
}

// installedCall is the modules.json entry that installed a registry module at
// a version, with the list it is in.
func (r *resolver) installedCall(t lang.Target) (installedModule, []installedModule, bool) {
	directories := make([]string, 0, len(r.modules))
	for d := range r.modules {
		directories = append(directories, d)
	}
	sort.Strings(directories)
	for _, d := range directories {
		list := r.modules[d].installed
		for _, m := range list {
			if m.Version == "" || t.Version != "" && m.Version != t.Version {
				continue
			}
			if s, ok := parseSource(m.Source); ok && s.packageName == t.Package {
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
	if t.Ecosystem != ecosystemModule {
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
				dependencyTarget := r.moduleTarget(".", c.Source, "")
				if s.origin == "" && c.Version != "" {
					dependencyTarget = lang.Target{Ecosystem: ecosystemModule, Package: s.packageName, Version: c.Version, Pinned: true}
				}
				if k := dependencyTarget.Package + "@" + dependencyTarget.Version; dependencyTarget.Package != "" && !seen[k] {
					seen[k] = true
					out = append(out, dependencyTarget)
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
	if t.Ecosystem != ecosystemModule {
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
func (r *resolver) installedPin(directory, call string, t lang.Target) lang.Target {
	if t.Ecosystem != ecosystemModule || t.Origin != "" || t.Pinned || call == "" {
		return t
	}
	if v := r.installedVersion(directory, call, t.Package); v != "" {
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
