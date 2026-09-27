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
	pkgs map[string]*planned
}

// readPlan reads plan.json's install-plan. A package appears once per component
// (lib, exe:x) or once with all of them under "components"; depends name unit ids
// (aeson-2.2.1.0-abc123), mapped back to package names through the plan's ids.
//
// Implements: REQ-HASKELL-008
func readPlan(src []byte) *buildPlan {
	type deps struct {
		Depends    []string `json:"depends"`
		ExeDepends []string `json:"exe-depends"`
	}
	var doc struct {
		InstallPlan []struct {
			Type       string          `json:"type"`
			ID         string          `json:"id"`
			Name       string          `json:"pkg-name"`
			Version    string          `json:"pkg-version"`
			Style      string          `json:"style"`
			Depends    []string        `json:"depends"`
			Components map[string]deps `json:"components"`
			Src        struct {
				Type string `json:"type"`
				Repo struct {
					Location string `json:"location"`
					Tag      string `json:"tag"`
				} `json:"source-repo"`
			} `json:"pkg-src"`
		} `json:"install-plan"`
	}
	if json.Unmarshal(src, &doc) != nil || len(doc.InstallPlan) == 0 {
		return nil
	}
	ids := map[string]string{}
	for _, u := range doc.InstallPlan {
		ids[u.ID] = u.Name
	}
	p := &buildPlan{pkgs: map[string]*planned{}}
	for _, u := range doc.InstallPlan {
		if u.Name == "" {
			continue
		}
		pk := p.pkgs[u.Name]
		if pk == nil {
			pk = &planned{name: u.Name, version: u.Version}
			p.pkgs[u.Name] = pk
		}
		if u.Style == "local" || u.Style == "inplace" || u.Src.Type == "local" {
			pk.local = true
		}
		if u.Src.Type == "source-repo" {
			pk.origin, pk.tag = u.Src.Repo.Location, u.Src.Repo.Tag
		}
		all := append([]string(nil), u.Depends...)
		for _, c := range u.Components {
			all = append(all, c.Depends...)
		}
		for _, id := range all {
			if n := ids[id]; n != "" && n != u.Name {
				pk.depends = append(pk.depends, n)
			}
		}
	}
	for _, pk := range p.pkgs {
		sort.Strings(pk.depends)
		out := pk.depends[:0]
		for i, d := range pk.depends {
			if i == 0 || d != pk.depends[i-1] {
				out = append(out, d)
			}
		}
		pk.depends = out
	}
	return p
}
