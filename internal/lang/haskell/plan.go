package haskell

import (
	"encoding/json"
	"sort"
)

// planned is one package of cabal's build plan.
type planned struct {
	name, version string
	local         bool   // a package of the project itself (style local or inplace)
	origin        string // a source-repository-package's location
	tag           string
	depends       []string // package names
}

// buildPlan is dist-newstyle/cache/plan.json: the package versions cabal chose and
// what each depends on.
type buildPlan struct {
	packages map[string]*planned
}

// readPlan reads plan.json's install-plan. A package appears once per component
// (lib, exe:x) or once with all of them under "components"; depends name unit ids
// (aeson-2.2.1.0-abc123), mapped back to package names through the plan's ids.
//
// Implements: REQ-HASKELL-008
func readPlan(source []byte) *buildPlan {
	type dependencies struct {
		Depends    []string `json:"depends"`
		ExeDepends []string `json:"exe-depends"`
	}
	var doc struct {
		InstallPlan []struct {
			Type       string                  `json:"type"`
			ID         string                  `json:"id"`
			Name       string                  `json:"pkg-name"`
			Version    string                  `json:"pkg-version"`
			Style      string                  `json:"style"`
			Depends    []string                `json:"depends"`
			Components map[string]dependencies `json:"components"`
			Source     struct {
				Type       string `json:"type"`
				Repository struct {
					Location string `json:"location"`
					Tag      string `json:"tag"`
				} `json:"source-repo"`
			} `json:"pkg-src"`
		} `json:"install-plan"`
	}
	if json.Unmarshal(source, &doc) != nil || len(doc.InstallPlan) == 0 {
		return nil
	}
	ids := map[string]string{}
	for _, u := range doc.InstallPlan {
		ids[u.ID] = u.Name
	}
	p := &buildPlan{packages: map[string]*planned{}}
	for _, u := range doc.InstallPlan {
		if u.Name == "" {
			continue
		}
		plannedPackage := p.packages[u.Name]
		if plannedPackage == nil {
			plannedPackage = &planned{name: u.Name, version: u.Version}
			p.packages[u.Name] = plannedPackage
		}
		if u.Style == "local" || u.Style == "inplace" || u.Source.Type == "local" {
			plannedPackage.local = true
		}
		if u.Source.Type == "source-repo" {
			plannedPackage.origin, plannedPackage.tag = u.Source.Repository.Location, u.Source.Repository.Tag
		}
		all := append([]string(nil), u.Depends...)
		for _, c := range u.Components {
			all = append(all, c.Depends...)
		}
		for _, id := range all {
			if n := ids[id]; n != "" && n != u.Name {
				plannedPackage.depends = append(plannedPackage.depends, n)
			}
		}
	}
	for _, planned := range p.packages {
		sort.Strings(planned.depends)
		out := planned.depends[:0]
		for i, d := range planned.depends {
			if i == 0 || d != planned.depends[i-1] {
				out = append(out, d)
			}
		}
		planned.depends = out
	}
	return p
}
