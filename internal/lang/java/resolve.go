package java

import (
	"encoding/xml"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

var jdkPrefixes = []string{
	"java.", "jdk.", "sun.", "com.sun.", "org.w3c.dom", "org.xml.sax", "org.ietf.jgss", "org.omg.",
	// javax is split: these ship with the JDK, others (servlet, persistence, …) do not.
	"javax.swing", "javax.crypto", "javax.net", "javax.xml", "javax.sql", "javax.naming",
	"javax.management", "javax.imageio", "javax.sound", "javax.print", "javax.script",
	"javax.security", "javax.tools", "javax.lang.model", "javax.annotation.processing",
	"javax.accessibility", "javax.rmi", "javax.transaction.xa",
}

// knownGroups maps packages of popular libraries to their groupId where the two share
// too little for group() to connect them. Used only when that group is declared.
var knownGroups = map[string]string{
	"com.google.common":     "com.google.guava",
	"com.google.thirdparty": "com.google.guava",
	"org.junit":             "junit", // JUnit 4; JUnit 5 is org.junit.jupiter by prefix
	"junit":                 "junit",
	"lombok":                "org.projectlombok",
	"org.mockito":           "org.mockito",
	"reactor":               "io.projectreactor",
	"io.reactivex":          "io.reactivex.rxjava3",
}

type resolver struct {
	byName map[string][]string // "User.java" -> project paths
	dirs   []string            // directories holding Java files, sorted
	groups map[string]string   // Maven groupId -> version ("" when artifacts differ)
	alias  map[string]string   // artifactId or last groupId segment -> groupId
	own    []string            // groupIds of the project itself
}

func newResolver(all []*scan.File) *resolver {
	r := &resolver{byName: map[string][]string{}, groups: map[string]string{}, alias: map[string]string{}}
	dirs := map[string]bool{}
	for _, f := range all {
		base := path.Base(f.Path)
		switch {
		case strings.HasSuffix(base, ".java"):
			r.byName[base] = append(r.byName[base], f.Path)
			dirs[path.Dir(f.Path)] = true
		case base == "pom.xml":
			r.readPOM(f.Abs)
		case base == "build.gradle" || base == "build.gradle.kts":
			r.readGradle(f.Abs)
		case base == "libs.versions.toml":
			r.readCatalog(f.Abs)
		}
	}
	for d := range dirs {
		r.dirs = append(r.dirs, d)
	}
	sort.Strings(r.dirs)
	return r
}

func (r *resolver) addGroup(group, artifact, version string) {
	if group == "" {
		return
	}
	for _, a := range []string{artifact, group[strings.LastIndex(group, ".")+1:]} {
		if a != "" {
			r.alias[a] = group
		}
	}
	if old, ok := r.groups[group]; ok && old != version {
		version = "" // several artifacts of one group at different versions
	}
	r.groups[group] = version
}

func (r *resolver) Resolve(file string, imp lang.RawImport) lang.Target {
	spec := imp.Module
	wildcard := strings.HasSuffix(spec, ".*")
	spec = strings.TrimSuffix(spec, ".*")
	segments := strings.Split(spec, ".")
	for _, p := range jdkPrefixes {
		if strings.HasPrefix(spec+".", p) || strings.HasPrefix(spec, p) {
			return lang.Target{Ecosystem: ecoJDK, Package: strings.Join(segments[:min(2, len(segments))], ".")}
		}
	}
	// The longest prefix naming a project class (static imports add member names).
	for n := len(segments); n >= 1; n-- {
		rel := strings.Join(segments[:n], "/") + ".java"
		for _, p := range r.byName[segments[n-1]+".java"] {
			if p == rel || strings.HasSuffix(p, "/"+rel) {
				return lang.Target{Local: p}
			}
		}
	}
	if wildcard {
		rel := strings.Join(segments, "/")
		for _, d := range r.dirs {
			if d == rel || strings.HasSuffix(d, "/"+rel) {
				return lang.Target{Local: d}
			}
		}
	}
	for _, g := range r.own {
		if spec == g || strings.HasPrefix(spec, g+".") {
			return lang.Target{} // the project's own (e.g. generated) code we cannot see
		}
	}
	if g, ok := r.group(spec); ok {
		v := r.groups[g]
		return lang.Target{Ecosystem: ecoMaven, Package: g, Version: v, Pinned: pinnedMaven(v)}
	}
	return lang.Target{Ecosystem: ecoMaven, Package: strings.Join(segments[:max(1, min(3, len(segments)-1))], "."), Unresolved: true}
}

// pinnedMaven reports whether a Maven version names one artifact. A plain version
// does, whatever its shape (Spring writes 1.2.3.RELEASE); a range, the LATEST and
// RELEASE keywords, an unexpanded property and a snapshot - republished under the
// same name - do not.
func pinnedMaven(v string) bool {
	v = strings.TrimSpace(v)
	switch {
	case v == "", strings.Contains(v, "${"):
		return false
	case strings.HasPrefix(v, "["), strings.HasPrefix(v, "("):
		return lang.Pinned(v) // "[1.2.3]" is one version, "[1.0,2.0)" is not
	case strings.EqualFold(v, "LATEST"), strings.EqualFold(v, "RELEASE"):
		return false
	case strings.HasSuffix(strings.ToUpper(v), "-SNAPSHOT"):
		return false
	}
	return true
}

// group finds the declared groupId an import belongs to: the longest groupId that is a
// package prefix, else the one sharing the most leading segments (at least three, or
// all of a shorter groupId) - com.fasterxml.jackson.databind comes from the group
// com.fasterxml.jackson.core - else a group whose artifactId or last segment names
// the import's first package segment (okhttp3.* from com.squareup.okhttp3).
func (r *resolver) group(spec string) (string, bool) {
	best, bestScore := "", 0
	segments := strings.Split(spec, ".")
	for g := range r.groups {
		gs := strings.Split(g, ".")
		score := 0
		if spec == g || strings.HasPrefix(spec, g+".") {
			score = 100 + len(gs)
		} else {
			common := 0
			for common < len(gs) && common < len(segments) && gs[common] == segments[common] {
				common++
			}
			if common >= min(3, len(gs)) {
				score = common
			}
		}
		if score > bestScore || (score == bestScore && score > 0 && g < best) {
			best, bestScore = g, score
		}
	}
	if bestScore > 0 {
		return best, true
	}
	for pkg, g := range knownGroups {
		if _, declared := r.groups[g]; declared && (spec == pkg || strings.HasPrefix(spec, pkg+".")) {
			return g, true
		}
	}
	for _, s := range segments[:min(2, len(segments))] { // okhttp3.*, org.junit.*
		if g, ok := r.alias[s]; ok {
			return g, true
		}
	}
	return "", false
}

// ---------------------------------------------------------------- manifests

type pomDep struct {
	GroupID    string `xml:"groupId"`
	ArtifactID string `xml:"artifactId"`
	Version    string `xml:"version"`
}

var property = regexp.MustCompile(`\$\{([^}]+)\}`)

func (r *resolver) readPOM(abs string) {
	data, err := os.ReadFile(abs)
	if err != nil {
		return
	}
	var pom struct {
		GroupID string `xml:"groupId"`
		Version string `xml:"version"`
		Parent  struct {
			GroupID string `xml:"groupId"`
		} `xml:"parent"`
		Properties struct {
			Items []struct {
				XMLName xml.Name
				Value   string `xml:",chardata"`
			} `xml:",any"`
		} `xml:"properties"`
		Deps    []pomDep `xml:"dependencies>dependency"`
		Managed []pomDep `xml:"dependencyManagement>dependencies>dependency"`
	}
	if xml.Unmarshal(data, &pom) != nil {
		return
	}
	group := pom.GroupID
	if group == "" {
		group = pom.Parent.GroupID
	}
	if group != "" {
		r.own = append(r.own, group)
	}
	props := map[string]string{"project.version": pom.Version, "project.groupId": group}
	for _, p := range pom.Properties.Items {
		props[p.XMLName.Local] = strings.TrimSpace(p.Value)
	}
	expand := func(s string) string {
		return property.ReplaceAllStringFunc(s, func(m string) string {
			if v, ok := props[m[2:len(m)-1]]; ok {
				return v
			}
			return m
		})
	}
	managed := map[string]string{}
	for _, d := range pom.Managed {
		managed[expand(d.GroupID)] = expand(d.Version)
	}
	for _, d := range pom.Deps {
		g, v := expand(d.GroupID), expand(d.Version)
		if v == "" {
			v = managed[g]
		}
		if g != group {
			r.addGroup(g, expand(d.ArtifactID), v)
		}
	}
}

var (
	gradleDep   = regexp.MustCompile(`["']([\w.\-]+):([\w.\-]+)(?::([\w.\-$]+))?["']`)
	gradleGroup = regexp.MustCompile(`(?m)^\s*group\s*=\s*["']([\w.\-]+)["']`)
)

func (r *resolver) readGradle(abs string) {
	data, err := os.ReadFile(abs)
	if err != nil {
		return
	}
	if m := gradleGroup.FindSubmatch(data); m != nil {
		r.own = append(r.own, string(m[1]))
	}
	for _, m := range gradleDep.FindAllSubmatch(data, -1) {
		r.addGroup(string(m[1]), string(m[2]), string(m[3]))
	}
}

// readCatalog reads a Gradle version catalog: [libraries] entries as "g:a:v" or
// { module = "g:a", version = "1" | version.ref = "name" }.
func (r *resolver) readCatalog(abs string) {
	var cat struct {
		Versions  map[string]any
		Libraries map[string]any
	}
	if _, err := toml.DecodeFile(abs, &cat); err != nil {
		return
	}
	for _, lib := range cat.Libraries {
		switch v := lib.(type) {
		case string:
			parts := strings.Split(v, ":")
			if len(parts) >= 2 {
				ver := ""
				if len(parts) > 2 {
					ver = parts[2]
				}
				r.addGroup(parts[0], parts[1], ver)
			}
		case map[string]any:
			group, _ := v["group"].(string)
			artifact, _ := v["name"].(string)
			if mod, ok := v["module"].(string); ok {
				group, artifact, _ = strings.Cut(mod, ":")
			}
			ver := ""
			switch vv := v["version"].(type) {
			case string:
				ver = vv
			case map[string]any:
				if ref, ok := vv["ref"].(string); ok {
					ver, _ = cat.Versions[ref].(string)
				}
			}
			r.addGroup(group, artifact, ver)
		}
	}
}
