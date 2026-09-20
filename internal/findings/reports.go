package findings

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

// Read imports one report. The format is recognized from its own shape rather than
// from the file's name: a report is whatever a tool wrote, and CI pipelines name them
// anything. The returned name is the tool the report came from.
func Read(r io.Reader) (name string, out []*Finding, err error) {
	data, err := io.ReadAll(io.LimitReader(r, 256<<20))
	if err != nil {
		return "", nil, err
	}
	trimmed := bytes.TrimLeft(data, " \t\r\n")
	if len(trimmed) == 0 {
		return "", nil, fmt.Errorf("empty report")
	}
	if trimmed[0] == '[' {
		out, err = readESLint(trimmed)
		return "eslint", out, err
	}
	if trimmed[0] != '{' {
		return "", nil, fmt.Errorf("not a JSON report")
	}

	// One object, or a stream of them: govulncheck writes a stream, everything else
	// writes a single document.
	var probe struct {
		Results         json.RawMessage `json:"Results"`
		Issues          json.RawMessage `json:"Issues"`
		Vulnerabilities json.RawMessage `json:"vulnerabilities"`
		Advisories      json.RawMessage `json:"advisories"`
		OSVResults      json.RawMessage `json:"results"`
		// govulncheck writes a stream whose first message is one of these.
		Config   json.RawMessage `json:"config"`
		Progress json.RawMessage `json:"progress"`
		OSV      json.RawMessage `json:"osv"`
		Finding  json.RawMessage `json:"finding"`
	}
	if err := json.NewDecoder(bytes.NewReader(trimmed)).Decode(&probe); err != nil {
		return "", nil, err
	}
	var read func([]byte) ([]*Finding, error)
	switch {
	case probe.Results != nil:
		name, read = "trivy", readTrivy
	case probe.Issues != nil:
		name, read = "golangci-lint", readGolangCI
	case probe.Vulnerabilities != nil:
		name, read = "npm-audit", readNPMAudit
	case probe.Advisories != nil:
		name, read = "npm-audit", readNPMAuditV6
	case probe.OSVResults != nil:
		name, read = "osv-scanner", readOSVScanner
	case probe.Config != nil || probe.Progress != nil || probe.OSV != nil || probe.Finding != nil:
		name, read = "govulncheck", readGovulncheck
	default:
		return "", nil, fmt.Errorf("not a report depphunter recognizes")
	}
	out, err = read(trimmed)
	return name, out, err
}

// ------------------------------------------------------------------ govulncheck

// govulncheck -json writes a stream of one-key objects: the advisories it knows, then
// the places in this repository that reach them. An advisory without a reachable call
// is reported too, and says so.
func readGovulncheck(data []byte) ([]*Finding, error) {
	type trace struct {
		Module   string `json:"module"`
		Version  string `json:"version"`
		Package  string `json:"package"`
		Function string `json:"function"`
		Position *struct {
			Filename string `json:"filename"`
			Line     int    `json:"line"`
			Column   int    `json:"column"`
		} `json:"position"`
	}
	type message struct {
		OSV     *osvEntry `json:"osv"`
		Finding *struct {
			OSV          string  `json:"osv"`
			FixedVersion string  `json:"fixed_version"`
			Trace        []trace `json:"trace"`
		} `json:"finding"`
	}

	entries := map[string]*osvEntry{}
	// The same advisory is reported at module, package and symbol level; the call site
	// is what the reader wants, so the most specific trace wins.
	type hit struct {
		osv     string
		module  string
		version string
		fixed   string
		at      *trace
	}
	hits := map[string]*hit{}
	var order []string

	dec := json.NewDecoder(bytes.NewReader(data))
	for {
		var m message
		if err := dec.Decode(&m); err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		if m.OSV != nil {
			entries[m.OSV.ID] = m.OSV
		}
		if m.Finding == nil || len(m.Finding.Trace) == 0 {
			continue
		}
		top := m.Finding.Trace[0]
		key := m.Finding.OSV + "|" + top.Module
		h, ok := hits[key]
		if !ok {
			h = &hit{osv: m.Finding.OSV, module: top.Module, version: top.Version, fixed: m.Finding.FixedVersion}
			hits[key], order = h, append(order, key)
		}
		if h.fixed == "" {
			h.fixed = m.Finding.FixedVersion
		}
		if h.at == nil && top.Position != nil {
			at := top
			h.at = &at
		}
	}

	var out []*Finding
	for _, key := range order {
		h := hits[key]
		e, ok := entries[h.osv]
		if !ok {
			e = &osvEntry{ID: h.osv}
		}
		f := e.finding(Package{Ecosystem: "go", Name: h.module, Version: strings.TrimPrefix(h.version, "v")})
		f.Source = "govulncheck"
		if h.fixed != "" {
			f.Fixed = strings.TrimPrefix(h.fixed, "v")
		}
		if h.at != nil && h.at.Position != nil {
			f.Path, f.Line, f.Column = h.at.Position.Filename, h.at.Position.Line, h.at.Position.Column
			if h.at.Function != "" {
				f.Detail = prepend(f.Detail, "Reached from "+h.at.Function+".")
			}
		} else {
			f.Detail = prepend(f.Detail, "No call into the vulnerable code was found.")
			if f.Severity == Unknown || f.Severity.Rank() > Low.Rank() {
				f.Severity = Low
			}
		}
		out = append(out, f)
	}
	return out, nil
}

// ------------------------------------------------------------------ osv-scanner

func readOSVScanner(data []byte) ([]*Finding, error) {
	var doc struct {
		Results []struct {
			Source struct {
				Path string `json:"path"`
			} `json:"source"`
			Packages []struct {
				Package struct {
					Name      string `json:"name"`
					Version   string `json:"version"`
					Ecosystem string `json:"ecosystem"`
				} `json:"package"`
				Vulnerabilities []*osvEntry `json:"vulnerabilities"`
			} `json:"packages"`
		} `json:"results"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	var out []*Finding
	for _, r := range doc.Results {
		for _, p := range r.Packages {
			pkg := Package{Ecosystem: ourEcosystem(p.Package.Ecosystem), Name: p.Package.Name, Version: p.Package.Version}
			for _, e := range p.Vulnerabilities {
				f := e.finding(pkg)
				f.Source = "osv-scanner"
				f.Path = r.Source.Path
				out = append(out, f)
			}
		}
	}
	return out, nil
}

// ourEcosystem maps an OSV ecosystem name back onto this project's ids, so a finding
// lands on the island its package is on. OSV qualifies some names ("Debian:12"), which
// the part before the colon answers.
func ourEcosystem(name string) string {
	name, _, _ = strings.Cut(name, ":")
	for id, osv := range osvEcosystems {
		if strings.EqualFold(osv, name) {
			return id
		}
	}
	return strings.ToLower(name)
}

// ------------------------------------------------------------------ npm audit

// npm audit --json (npm 7 and later) reports one entry per affected package, with the
// advisories behind it in "via". A via entry that is a string names another package
// that pulls the problem in, which is a chain rather than an advisory.
func readNPMAudit(data []byte) ([]*Finding, error) {
	type via struct {
		Source   json.Number `json:"source"`
		Name     string      `json:"name"`
		Title    string      `json:"title"`
		URL      string      `json:"url"`
		Severity string      `json:"severity"`
		Range    string      `json:"range"`
		CWE      []string    `json:"cwe"`
	}
	var doc struct {
		Vulnerabilities map[string]struct {
			Name     string            `json:"name"`
			Severity string            `json:"severity"`
			Via      []json.RawMessage `json:"via"`
			Range    string            `json:"range"`
			Nodes    []string          `json:"nodes"`
			Fix      json.RawMessage   `json:"fixAvailable"`
		} `json:"vulnerabilities"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	var out []*Finding
	for _, name := range sortedKeys(doc.Vulnerabilities) {
		v := doc.Vulnerabilities[name]
		fixed := npmFix(v.Fix)
		indirect := []string{}
		advisories := 0
		for _, raw := range v.Via {
			var through string
			if json.Unmarshal(raw, &through) == nil {
				indirect = append(indirect, through)
				continue
			}
			var a via
			if json.Unmarshal(raw, &a) != nil || a.Title == "" {
				continue
			}
			advisories++
			out = append(out, &Finding{
				Kind:      KindVulnerability,
				Source:    "npm-audit",
				Ref:       advisoryRef(a.URL, a.Source.String()),
				Severity:  severity(first(a.Severity, v.Severity)),
				Title:     trim(a.Title, 200),
				Detail:    npmDetail(a.Range, a.CWE, nil),
				URL:       a.URL,
				Ecosystem: "npm",
				Package:   first(a.Name, name),
				Fixed:     fixed,
			})
		}
		if advisories > 0 || len(indirect) == 0 {
			continue
		}
		// Nothing is wrong with this package itself: it depends on something that is.
		out = append(out, &Finding{
			Kind:      KindVulnerability,
			Source:    "npm-audit",
			Ref:       "npm:" + name,
			Severity:  severity(v.Severity),
			Title:     "Depends on a vulnerable package",
			Detail:    npmDetail(v.Range, nil, indirect),
			Ecosystem: "npm",
			Package:   name,
			Fixed:     fixed,
		})
	}
	return out, nil
}

func npmDetail(affected string, cwe, through []string) string {
	var parts []string
	if affected != "" {
		parts = append(parts, "Affects "+affected+".")
	}
	if len(through) > 0 {
		sort.Strings(through)
		parts = append(parts, "Through "+strings.Join(through, ", ")+".")
	}
	if len(cwe) > 0 {
		parts = append(parts, strings.Join(cwe, ", ")+".")
	}
	return strings.Join(parts, " ")
}

// npmFix reads fixAvailable, which is false, true, or the version npm would install.
func npmFix(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var fix struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	if json.Unmarshal(raw, &fix) == nil {
		return fix.Version
	}
	return ""
}

// npm 6 wrote a flat advisory table instead.
func readNPMAuditV6(data []byte) ([]*Finding, error) {
	var doc struct {
		Advisories map[string]struct {
			ID                 json.Number `json:"id"`
			ModuleName         string      `json:"module_name"`
			Severity           string      `json:"severity"`
			Title              string      `json:"title"`
			Overview           string      `json:"overview"`
			URL                string      `json:"url"`
			CVES               []string    `json:"cves"`
			VulnerableVersions string      `json:"vulnerable_versions"`
			PatchedVersions    string      `json:"patched_versions"`
			Findings           []struct {
				Version string `json:"version"`
			} `json:"findings"`
		} `json:"advisories"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	var out []*Finding
	for _, key := range sortedKeys(doc.Advisories) {
		a := doc.Advisories[key]
		version := ""
		if len(a.Findings) > 0 {
			version = a.Findings[0].Version
		}
		ref := advisoryRef(a.URL, a.ID.String())
		if len(a.CVES) > 0 {
			ref = a.CVES[0]
		}
		out = append(out, &Finding{
			Kind:      KindVulnerability,
			Source:    "npm-audit",
			Ref:       ref,
			Severity:  severity(a.Severity),
			Title:     trim(a.Title, 200),
			Detail:    trim(a.Overview, 1200),
			URL:       a.URL,
			Ecosystem: "npm",
			Package:   a.ModuleName,
			Version:   version,
			Fixed:     strings.TrimPrefix(a.PatchedVersions, ">="),
		})
	}
	return out, nil
}

// advisoryRef names an advisory: its GHSA id when the URL carries one, else the
// numeric id npm gave it.
func advisoryRef(url, id string) string {
	if i := strings.LastIndex(url, "/GHSA-"); i >= 0 {
		return url[i+1:]
	}
	if id != "" && id != "0" {
		return "npm-advisory-" + id
	}
	return "npm-advisory"
}

// ------------------------------------------------------------------ trivy

func readTrivy(data []byte) ([]*Finding, error) {
	type cause struct {
		StartLine int `json:"StartLine"`
	}
	var doc struct {
		Results []struct {
			Target          string `json:"Target"`
			Type            string `json:"Type"`
			Class           string `json:"Class"`
			Vulnerabilities []struct {
				VulnerabilityID  string `json:"VulnerabilityID"`
				PkgName          string `json:"PkgName"`
				InstalledVersion string `json:"InstalledVersion"`
				FixedVersion     string `json:"FixedVersion"`
				Severity         string `json:"Severity"`
				Title            string `json:"Title"`
				Description      string `json:"Description"`
				PrimaryURL       string `json:"PrimaryURL"`
				CVSS             map[string]struct {
					V3Vector string `json:"V3Vector"`
				} `json:"CVSS"`
			} `json:"Vulnerabilities"`
			Misconfigurations []struct {
				ID            string `json:"ID"`
				Title         string `json:"Title"`
				Description   string `json:"Description"`
				Message       string `json:"Message"`
				Severity      string `json:"Severity"`
				PrimaryURL    string `json:"PrimaryURL"`
				CauseMetadata cause  `json:"CauseMetadata"`
			} `json:"Misconfigurations"`
			Secrets []struct {
				RuleID    string `json:"RuleID"`
				Title     string `json:"Title"`
				Severity  string `json:"Severity"`
				StartLine int    `json:"StartLine"`
			} `json:"Secrets"`
		} `json:"Results"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	var out []*Finding
	for _, r := range doc.Results {
		eco := trivyEcosystems[r.Type]
		for _, v := range r.Vulnerabilities {
			sev := severity(v.Severity)
			// Trivy carries the advisory's own CVSS vectors: they say more than the
			// one word its distribution chose.
			for _, source := range []string{"nvd", "ghsa", "redhat"} {
				if vec := v.CVSS[source].V3Vector; vec != "" {
					if score, ok := scoreCVSS(vec); ok {
						sev = severityOfScore(score)
						break
					}
				}
			}
			out = append(out, &Finding{
				Kind:      KindVulnerability,
				Source:    "trivy",
				Ref:       v.VulnerabilityID,
				Severity:  sev,
				Title:     trim(first(v.Title, v.VulnerabilityID), 200),
				Detail:    trim(v.Description, 1200),
				URL:       v.PrimaryURL,
				Path:      trivyPath(r.Class, r.Target),
				Ecosystem: eco,
				Package:   v.PkgName,
				Version:   v.InstalledVersion,
				Fixed:     v.FixedVersion,
			})
		}
		for _, m := range r.Misconfigurations {
			out = append(out, &Finding{
				Kind:     KindLint,
				Source:   "trivy",
				Ref:      m.ID,
				Severity: severity(m.Severity),
				Title:    trim(first(m.Title, m.ID), 200),
				Detail:   trim(first(m.Message, m.Description), 1200),
				URL:      m.PrimaryURL,
				Path:     r.Target,
				Line:     m.CauseMetadata.StartLine,
			})
		}
		for _, s := range r.Secrets {
			out = append(out, &Finding{
				Kind:     KindLint,
				Source:   "trivy",
				Ref:      s.RuleID,
				Severity: severity(s.Severity),
				Title:    trim(first(s.Title, s.RuleID), 200),
				Detail:   "A secret was found in this file. The value itself is not reported here.",
				Path:     r.Target,
				Line:     s.StartLine,
			})
		}
	}
	return out, nil
}

// trivyEcosystems maps Trivy's package types onto this project's ecosystem ids. A type
// that is not here (an OS package database) leaves the finding without an island, and
// it is shown on the file Trivy scanned instead.
var trivyEcosystems = map[string]string{
	"npm": "npm", "yarn": "npm", "pnpm": "npm", "node-pkg": "npm",
	"gomod": "go", "gobinary": "go",
	"pip": "pypi", "poetry": "pypi", "pipenv": "pypi", "python-pkg": "pypi",
	"cargo": "crates",
	"pom":   "maven", "gradle": "maven", "jar": "maven",
	"nuget": "nuget", "dotnet-core": "nuget",
}

// trivyPath keeps the target of a scan that has one: a lock file or a Dockerfile is a
// file in the repository, while an OS package database inside an image is not.
func trivyPath(class, target string) string {
	switch class {
	case "lang-pkgs", "config", "secret", "license-file":
		return target
	}
	return ""
}

// ------------------------------------------------------------------ golangci-lint

func readGolangCI(data []byte) ([]*Finding, error) {
	var doc struct {
		Issues []struct {
			FromLinter string `json:"FromLinter"`
			Text       string `json:"Text"`
			Severity   string `json:"Severity"`
			Pos        struct {
				Filename string `json:"Filename"`
				Line     int    `json:"Line"`
				Column   int    `json:"Column"`
			} `json:"Pos"`
		} `json:"Issues"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	var out []*Finding
	for _, i := range doc.Issues {
		out = append(out, &Finding{
			Kind:     KindLint,
			Source:   "golangci-lint",
			Ref:      i.FromLinter,
			Severity: lintSeverity(i.Severity, Medium),
			Title:    trim(i.Text, 200),
			Path:     i.Pos.Filename,
			Line:     i.Pos.Line,
			Column:   i.Pos.Column,
		})
	}
	return out, nil
}

// ------------------------------------------------------------------ eslint

func readESLint(data []byte) ([]*Finding, error) {
	var doc []struct {
		FilePath string `json:"filePath"`
		Messages []struct {
			RuleID   string `json:"ruleId"`
			Severity int    `json:"severity"`
			Message  string `json:"message"`
			Line     int    `json:"line"`
			Column   int    `json:"column"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	var out []*Finding
	for _, f := range doc {
		for _, m := range f.Messages {
			sev := Low
			if m.Severity >= 2 {
				sev = Medium
			}
			out = append(out, &Finding{
				Kind:     KindLint,
				Source:   "eslint",
				Ref:      first(m.RuleID, "eslint"),
				Severity: sev,
				Title:    trim(m.Message, 200),
				Path:     f.FilePath,
				Line:     m.Line,
				Column:   m.Column,
			})
		}
	}
	return out, nil
}

// ------------------------------------------------------------------ helpers

// lintSeverity keeps a linter's complaint below a vulnerability: "error" from a linter
// is not the same news as a critical advisory, and the streets would fill with bugs
// that only mean a missing comment.
func lintSeverity(s string, fallback Severity) Severity {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "error", "critical", "high":
		return Medium
	case "warning", "medium":
		return Low
	case "info", "low", "note":
		return Info
	}
	return fallback
}

func first(vs ...string) string {
	for _, v := range vs {
		if v != "" {
			return v
		}
	}
	return ""
}

func prepend(detail, note string) string {
	if detail == "" {
		return note
	}
	return note + "\n\n" + detail
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
