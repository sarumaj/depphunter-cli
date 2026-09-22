// Package findings collects what other tools already say about a repository - known
// vulnerabilities in the packages it depends on, and what its linters complain about -
// and puts each one where it belongs on the map: on a file, or on an external package.
//
// Nothing here runs a scanner. Reports are read from files the user names (govulncheck,
// npm audit, trivy, golangci-lint, eslint, osv-scanner), which is the form those tools
// already produce in CI. With --online, pinned package versions are additionally looked
// up in the OSV database, which is the one question a repository's own files cannot
// answer.
package findings

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Severity is the shared vocabulary every report is mapped onto.
type Severity string

const (
	Critical Severity = "critical"
	High     Severity = "high"
	Medium   Severity = "medium"
	Low      Severity = "low"
	Info     Severity = "info"
	Unknown  Severity = "unknown"
)

var rank = map[Severity]int{Critical: 5, High: 4, Medium: 3, Low: 2, Info: 1, Unknown: 0}

// Rank orders severities, most serious first.
func (s Severity) Rank() int { return rank[s] }

// severity maps what a report calls a severity onto the shared vocabulary.
func severity(s string) Severity {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "critical":
		return Critical
	case "high", "important", "error":
		return High
	case "medium", "moderate", "warning":
		return Medium
	case "low", "minor":
		return Low
	case "info", "informational", "note", "unknown", "negligible", "none":
		return Info
	}
	return Unknown
}

// Kinds of finding: a dependency known to be vulnerable, a linter's objection to code
// in this repository, and a link in its documentation that leads nowhere.
const (
	KindVulnerability = "vulnerability"
	KindLint          = "lint"
	KindLink          = "link"
)

// A Finding is one thing a tool reported, placed on the map. A finding carries a Path
// when it is about code in this repository, a Package when it is about a dependency,
// and both when a scanner could say which file pulls the dependency in.
type Finding struct {
	ID       string   `json:"id"`
	Kind     string   `json:"kind"`
	Source   string   `json:"source"` // the tool that reported it
	Ref      string   `json:"ref"`    // CVE / GHSA / GO-… id, or the linter's rule
	Severity Severity `json:"severity"`
	Title    string   `json:"title"`
	Detail   string   `json:"detail,omitempty"`
	URL      string   `json:"url,omitempty"`

	Path   string `json:"path,omitempty"` // repository-relative, slash-separated
	Line   int    `json:"line,omitempty"`
	Column int    `json:"column,omitempty"`

	Ecosystem string `json:"ecosystem,omitempty"`
	Package   string `json:"package,omitempty"`
	Version   string `json:"version,omitempty"`
	Fixed     string `json:"fixed,omitempty"` // the first version that is not affected
}

// key identifies a finding across reports, so the same vulnerability read from two
// tools is one entry on the map.
func (f *Finding) key() string {
	return strings.Join([]string{
		f.Ref, f.Ecosystem, f.Package, f.Version, f.Path,
		strconv.Itoa(f.Line), strconv.Itoa(f.Column),
	}, "|")
}

// Set is the /api/findings document: everything that was reported, and who reported it.
type Set struct {
	Findings []*Finding `json:"findings"`
	Sources  []string   `json:"sources"`
	// Partial marks a set that is missing something it was asked for: a report that
	// would not parse, or a database that would not answer.
	Partial bool `json:"partial,omitempty"`
}

// Add appends findings, keeping the first report of each. The tool a finding came from
// is remembered even when the finding itself is a repeat, so the UI can say what ran.
func (s *Set) Add(source string, fs []*Finding) {
	if source != "" && !contains(s.Sources, source) {
		s.Sources = append(s.Sources, source)
	}
	seen := map[string]bool{}
	for _, f := range s.Findings {
		seen[f.key()] = true
	}
	for _, f := range fs {
		if f == nil || seen[f.key()] {
			continue
		}
		seen[f.key()] = true
		if f.Source == "" {
			f.Source = source
		}
		s.Findings = append(s.Findings, f)
	}
}

// Localize rewrites absolute report paths to repository-relative ones and drops the
// path of anything outside the repository: such a finding is still about a package,
// and the map has somewhere to put it.
func (s *Set) Localize(root string) {
	root = filepath.Clean(root)
	for _, f := range s.Findings {
		f.Path = localize(root, f.Path)
	}
}

func localize(root, path string) string {
	if path == "" {
		return ""
	}
	p := filepath.FromSlash(path)
	if !filepath.IsAbs(p) {
		// Already relative: only its spelling may need fixing.
		p = filepath.Clean(p)
		if p == "." || strings.HasPrefix(p, "..") {
			return ""
		}
		return filepath.ToSlash(p)
	}
	rel, err := filepath.Rel(root, p)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return ""
	}
	return filepath.ToSlash(rel)
}

// Finish gives every finding a stable id and orders the set: most serious first, then
// by where it is, so the panel and the streets agree on which bug matters most.
func (s *Set) Finish() {
	sort.Strings(s.Sources)
	sort.SliceStable(s.Findings, func(i, j int) bool {
		a, b := s.Findings[i], s.Findings[j]
		if a.Severity != b.Severity {
			return a.Severity.Rank() > b.Severity.Rank()
		}
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Package != b.Package {
			return a.Package < b.Package
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Ref < b.Ref
	})
	for _, f := range s.Findings {
		sum := sha256.Sum256([]byte(f.Source + "|" + f.key()))
		f.ID = hex.EncodeToString(sum[:8])
		if f.Kind == "" {
			f.Kind = KindVulnerability
		}
		if f.Severity == "" {
			f.Severity = Unknown
		}
	}
}

// Empty reports whether the set is worth publishing.
func (s *Set) Empty() bool { return s == nil || len(s.Findings) == 0 }

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

// trim keeps a description short enough to read in a side panel.
func trim(s string, max int) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\r\n", "\n"))
	if len(s) <= max {
		return s
	}
	cut := s[:max]
	if i := strings.LastIndexAny(cut, " \n"); i > max/2 {
		cut = cut[:i]
	}
	return strings.TrimSpace(cut) + "…"
}
