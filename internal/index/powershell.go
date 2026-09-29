package index

import (
	"cmp"
	"context"
	"encoding/xml"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// ---------------------------------------------------------------- repositories

// parsePSResourceRepositories reads PSResourceGet's PSResourceRepository.xml,
// `<configuration><Repository Name Url Priority Trusted APIVersion .../>`, which
// Register-PSResourceRepository writes: PSResourceGet asks its repositories by
// priority (the lower first), then by name, and takes a module from the first
// that has it, so they are Listed in that order and the Gallery is asked only
// when it is one of them. A local (file system) repository and a container
// registry are not asked. A file PSResourceGet cannot read names nothing.
//
// Implements: REQ-SUP-078
func parsePSResourceRepositories(data []byte, k sink) {
	var doc struct {
		XMLName      xml.Name `xml:"configuration"`
		Repositories []struct {
			Name       string `xml:"Name,attr"`
			URL        string `xml:"Url,attr"`
			URI        string `xml:"Uri,attr"`
			Priority   string `xml:"Priority,attr"`
			APIVersion string `xml:"APIVersion,attr"`
		} `xml:"Repository"`
	}
	if xml.Unmarshal(data, &doc) != nil {
		return
	}
	priority := func(s string) int {
		n, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil {
			return 50 // PSResourceGet's default
		}
		return n
	}
	repositories := doc.Repositories
	sort.SliceStable(repositories, func(i, j int) bool {
		if a, b := priority(repositories[i].Priority), priority(repositories[j].Priority); a != b {
			return a < b
		}
		return strings.ToLower(repositories[i].Name) < strings.ToLower(repositories[j].Name)
	})
	k.off(PowerShell)
	for _, r := range repositories {
		switch strings.ToLower(r.APIVersion) {
		case "local", "containerregistry":
			continue
		}
		addPowerShellRepository(k, r.Name, cmp.Or(r.URL, r.URI))
	}
}

// parsePowerShellGetRepositories reads PowerShellGet 2's PSRepositories.xml, the
// repositories Register-PSRepository registered, serialized by PowerShell
// (CLIXML): a dictionary of PSRepository objects whose Name and SourceLocation
// are string properties. They are Listed in the file's order, the Gallery asked
// only when it is one of them.
//
// Implements: REQ-SUP-078
func parsePowerShellGetRepositories(data []byte, k sink) {
	type property struct {
		Name  string `xml:"N,attr"`
		Value string `xml:",chardata"`
	}
	var doc struct {
		XMLName xml.Name `xml:"Objs"`
		Objects []struct {
			Entries []struct {
				Value struct {
					Properties []property `xml:"MS>S"`
				} `xml:"Obj"`
			} `xml:"DCT>En"`
		} `xml:"Obj"`
	}
	if xml.Unmarshal(data, &doc) != nil {
		return
	}
	k.off(PowerShell)
	for _, o := range doc.Objects {
		for _, e := range o.Entries {
			var name, location string
			for _, p := range e.Value.Properties {
				switch p.Name {
				case "Name":
					name = strings.TrimSpace(p.Value)
				case "SourceLocation":
					location = strings.TrimSpace(p.Value)
				}
			}
			addPowerShellRepository(k, name, location)
		}
	}
}

// addPowerShellRepository records a registered repository served over HTTP.
func addPowerShellRepository(k sink, name, location string) {
	if u, err := url.Parse(location); err != nil || u.Scheme != "https" && u.Scheme != "http" || name == "" {
		return
	}
	k.put(PowerShell, Source{URL: location, Kind: Listed, powershellName: name})
}

// powershellRepository is the candidate for a module an install names the
// repository of (Install-Module -Repository, PSDepend's Repository): that
// registered repository alone, by name, as PowerShellGet asks it; PSGallery is
// the Gallery unless this machine's files unregistered it. A name nothing here
// registers has none: PowerShellGet itself would fail.
//
// Implements: REQ-SUP-078
func (c *Config) powershellRepository(name string) []candidate {
	for _, s := range c.sources[PowerShell] {
		if strings.EqualFold(s.powershellName, name) {
			return []candidate{{url: s.URL, primary: true, known: c.fetchable(PowerShell, s)}}
		}
	}
	if strings.EqualFold(name, "PSGallery") && c.off[PowerShell] == "" {
		return []candidate{{url: c.publicURL(PowerShell), primary: true, known: true}}
	}
	return nil
}

// ---------------------------------------------------------------- client

// powershellModule reads a module's dependencies from a repository. A NuGet v3
// feed (a service index, ".../index.json") is read by the NuGet client. The
// Gallery and any other repository speak NuGet's v2 OData API: one version is
// `GET <repository>/Packages(Id='<name>',Version='<version>')`, anything else
// `GET <repository>/FindPackagesById()?id='<name>'` (every version, followed
// through its pages), of which the newest release the requirement admits is
// taken - ModuleVersion's minimum or a NuGet range. The entry's Dependencies
// property, `Name:[range]:|Name:version:` (a framework after the second colon),
// is the answer.
//
// Implements: REQ-SUP-078
func (c *Client) powershellModule(ctx context.Context, index string, t lang.Target) ([]dependency, error) {
	if strings.HasSuffix(strings.ToLower(index), "/index.json") {
		return c.nugetPackage(ctx, index, t)
	}
	exact := powershellExact(t.Version)
	if !t.Pinned && !strings.HasPrefix(strings.TrimSpace(t.Version), "[") {
		exact = "" // ModuleVersion: a minimum
	}
	if exact != "" {
		body, err := c.accept(ctx, index+"/Packages(Id='"+odataString(t.Package)+"',Version='"+odataString(exact)+"')",
			"application/atom+xml, application/xml")
		if err != nil {
			return nil, err
		}
		entries, _, err := odataEntries(body)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if strings.EqualFold(e.name(), t.Package) {
				return powershellDependencies(e.Properties.Dependencies), nil
			}
		}
		return nil, errAbsent
	}
	address := index + "/FindPackagesById()?id='" + url.QueryEscape(odataString(t.Package)) + "'"
	var best *odataEntry
	for page := 0; address != "" && page < 20; page++ {
		body, err := c.accept(ctx, address, "application/atom+xml, application/xml")
		if err != nil {
			return nil, err
		}
		entries, next, err := odataEntries(body)
		if err != nil {
			return nil, err
		}
		for i := range entries {
			e := &entries[i]
			version := e.version()
			if !strings.EqualFold(e.name(), t.Package) || version == "" || e.prerelease() ||
				!powershellAdmits(t.Version, version) {
				continue
			}
			if best == nil || compareVersions(version, best.version()) > 0 {
				best = e
			}
		}
		address = next
	}
	if best == nil {
		return nil, errAbsent
	}
	return powershellDependencies(best.Properties.Dependencies), nil
}

// odataEntry is one package version of a NuGet v2 OData feed. The Gallery names the
// package in the Id property; Artifactory leaves it out and puts it in the title.
type odataEntry struct {
	Title      string `xml:"title"`
	Properties struct {
		ID                string `xml:"Id"`
		Version           string `xml:"Version"`
		NormalizedVersion string `xml:"NormalizedVersion"`
		Dependencies      string `xml:"Dependencies"`
		IsPrerelease      string `xml:"IsPrerelease"`
	} `xml:"properties"`
}

func (e odataEntry) name() string {
	return cmp.Or(strings.TrimSpace(e.Properties.ID), strings.TrimSpace(e.Title))
}

func (e odataEntry) version() string {
	return strings.TrimSpace(cmp.Or(e.Properties.NormalizedVersion, e.Properties.Version))
}

func (e odataEntry) prerelease() bool {
	return strings.EqualFold(strings.TrimSpace(e.Properties.IsPrerelease), "true") || strings.Contains(e.version(), "-")
}

// odataEntries reads a feed's entries and the address of its next page, or a
// single entry (what Packages(Id,Version) answers).
func odataEntries(body []byte) (entries []odataEntry, next string, err error) {
	var feed struct {
		XMLName xml.Name
		odataEntry
		Entries []odataEntry `xml:"entry"`
		Links   []struct {
			Rel  string `xml:"rel,attr"`
			Href string `xml:"href,attr"`
		} `xml:"link"`
	}
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, "", err
	}
	if feed.XMLName.Local == "entry" {
		return []odataEntry{feed.odataEntry}, "", nil
	}
	for _, l := range feed.Links {
		if l.Rel == "next" {
			next = l.Href
		}
	}
	return feed.Entries, next, nil
}

// odataString is a value inside an OData string literal, its quotes doubled.
func odataString(s string) string { return strings.ReplaceAll(s, "'", "''") }

// powershellDependencies reads a v2 entry's Dependencies property, each module
// once.
func powershellDependencies(property string) []dependency {
	seen := map[string]bool{}
	var out []dependency
	for _, item := range strings.Split(property, "|") {
		name, version, _ := strings.Cut(strings.TrimSpace(item), ":")
		version, _, _ = strings.Cut(version, ":") // a target framework follows
		if name = strings.TrimSpace(name); name == "" || seen[strings.ToLower(name)] {
			continue
		}
		seen[strings.ToLower(name)] = true
		out = append(out, dependency{Name: name, Version: strings.TrimSpace(version)})
	}
	return out
}

// powershellExact is the one version a requirement names: "[1.2.3]", or a bare
// version, which PowerShellGet takes as that version alone. "" for a range or
// nothing.
func powershellExact(requirement string) string {
	r := strings.TrimSpace(requirement)
	if inner, ok := strings.CutPrefix(r, "["); ok {
		if inner, ok := strings.CutSuffix(inner, "]"); ok && !strings.Contains(inner, ",") {
			return strings.TrimSpace(inner)
		}
		return ""
	}
	if r == "" || strings.ContainsAny(r, "(),*") {
		return ""
	}
	return r
}

// powershellAdmits reports whether a requirement admits a version: nothing admits
// every version, a bare version is a minimum (ModuleVersion), and a NuGet range
// is read as NuGet reads it - "[1.0,2.0)", "(,2.0]", "[1.0,)", "[1.2]".
func powershellAdmits(requirement, version string) bool {
	r := strings.TrimSpace(requirement)
	if r == "" || r == "*" {
		return true
	}
	if !strings.HasPrefix(r, "[") && !strings.HasPrefix(r, "(") {
		return compareVersions(version, r) >= 0
	}
	if len(r) < 2 {
		return false
	}
	low, high, isRange := strings.Cut(r[1:len(r)-1], ",")
	if !isRange {
		return compareVersions(version, strings.TrimSpace(low)) == 0
	}
	low, high = strings.TrimSpace(low), strings.TrimSpace(high)
	if low != "" {
		if c := compareVersions(version, low); c < 0 || c == 0 && r[0] == '(' {
			return false
		}
	}
	if high != "" {
		if c := compareVersions(version, high); c > 0 || c == 0 && r[len(r)-1] == ')' {
			return false
		}
	}
	return true
}
