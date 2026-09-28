package index

import (
	"encoding/xml"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang/edn"
	"github.com/sarumaj/depphunter-cli/internal/userconf"
)

// machineJVM reads the Maven repositories this machine's JVM build tools are
// configured with: Maven's settings (mirrors and the repositories of active
// profiles), the init scripts of Gradle's user home, the Clojure CLI's user deps.edn,
// Leiningen's user profile, sbt's repositories file and COURSIER_REPOSITORIES. They
// all serve the one Maven ecosystem, so what one tool replaces Central with replaces
// it for the others as well; the first replacement found is the one used, in the
// order they are read here.
//
// Implements: REQ-SUP-015, REQ-SUP-064
func machineJVM(m userconf.Machine, k sink) {
	var settings [][]byte
	for _, name := range m.MavenSettings() {
		if data, err := os.ReadFile(name); err == nil {
			settings = append(settings, data)
		}
	}
	mavenSettings(k, settings...)
	for _, name := range m.GradleInitScripts() {
		if data, err := os.ReadFile(name); err == nil {
			parseGradleRepositories(data, k)
		}
	}
	if directory := m.ClojureConfigDirectory(); directory != "" {
		if data, err := os.ReadFile(join(directory, "deps.edn")); err == nil {
			parseClojureRepositories("deps.edn", data, k)
		}
	}
	if name := m.LeinProfiles(); name != "" {
		if data, err := os.ReadFile(name); err == nil {
			parseLeinProfiles(data, k)
		}
	}
	if name := m.SbtRepositories(); name != "" {
		if data, err := os.ReadFile(name); err == nil {
			parseSbtRepositories(data, m.SbtOverrideBuildRepositories(), k)
		}
	}
	parseCoursierRepositories(m.Environment("COURSIER_REPOSITORIES"), k)
}

// mavenSettingsDoc is what is read of one Maven settings.xml.
type mavenSettingsDoc struct {
	Mirrors []struct {
		ID       string `xml:"id"`
		URL      string `xml:"url"`
		MirrorOf string `xml:"mirrorOf"`
	} `xml:"mirrors>mirror"`
	Profiles []struct {
		ID         string `xml:"id"`
		Activation struct {
			ActiveByDefault string `xml:"activeByDefault"`
		} `xml:"activation"`
		Repositories []struct {
			ID  string `xml:"id"`
			URL string `xml:"url"`
		} `xml:"repositories>repository"`
	} `xml:"profiles>profile"`
	ActiveProfiles []string `xml:"activeProfiles>activeProfile"`
}

// mavenSettings reads the Maven settings files, the user's first, as Maven merges
// them.
//
// A mirror of `*` (or `external:*`) stands in for every repository, Central and
// those a project declares; a mirror of `central` replaces Central only; a mirror
// of some other repository serves what that repository holds, beside Central. A
// mirror that says nothing is taken to mirror everything, as the most common setup
// does.
//
// The repositories of the active profiles are asked beside Central, as Maven adds
// them to every build; one with the id `central` replaces Central, and one a
// mirror names is served by that mirror. A profile is active when <activeProfiles>
// lists it; when none is, the profiles with <activeByDefault>true are. A profile
// activated by a property, the OS, the JDK or a file is not read: whether it is
// active depends on the build. <pluginRepositories> serve Maven's plugins, not the
// code, and are not read either.
//
// Implements: REQ-SUP-015, REQ-SUP-063
func mavenSettings(k sink, files ...[]byte) {
	var docs []mavenSettingsDoc
	for _, data := range files {
		var doc mavenSettingsDoc
		if xml.Unmarshal(data, &doc) == nil {
			docs = append(docs, doc)
		}
	}
	mirrored := map[string]bool{}
	for _, doc := range docs {
		for _, m := range doc.Mirrors {
			kind := Additive
			of := strings.Split(strings.ReplaceAll(m.MirrorOf, " ", ""), ",")
			switch {
			case strings.TrimSpace(m.MirrorOf) == "", slices.Contains(of, "*"), slices.ContainsFunc(of, func(s string) bool {
				return strings.HasPrefix(s, "external:")
			}):
				kind = ReplaceAll
			case slices.Contains(of, "central"):
				kind = Replace
			}
			for _, id := range of {
				mirrored[id] = true
			}
			k.put(Maven, Source{URL: strings.TrimSpace(m.URL), Kind: kind})
		}
	}

	defined := map[string]bool{}
	for _, doc := range docs {
		for _, p := range doc.Profiles {
			defined[strings.TrimSpace(p.ID)] = true
		}
	}
	active := map[string]bool{}
	for _, doc := range docs {
		for _, id := range doc.ActiveProfiles {
			if id = strings.TrimSpace(id); defined[id] {
				active[id] = true
			}
		}
	}
	byDefault := len(active) == 0
	for _, doc := range docs {
		for _, p := range doc.Profiles {
			on := active[strings.TrimSpace(p.ID)]
			if byDefault {
				on = strings.TrimSpace(p.Activation.ActiveByDefault) == "true"
			}
			if !on {
				continue
			}
			for _, r := range p.Repositories {
				u, id := strings.TrimSpace(r.URL), strings.TrimSpace(r.ID)
				switch {
				case MavenPublic(u), mirrored[id]:
				case id == "central":
					k.add(Maven, u, "")
				default:
					k.extra(Maven, u)
				}
			}
		}
	}
}

// parseLeinProfiles reads the :repositories of the :user profile in Leiningen's
// profiles.clj, which Leiningen merges into every project. The other profiles there
// are only active when a project or a command line names them.
//
// Implements: REQ-SUP-015
func parseLeinProfiles(data []byte, k sink) {
	for _, top := range edn.Read(data) {
		if user := top.Get("user"); user != nil {
			clojureRepositories(edn.Unquote(user.Get("repositories")), k)
		}
	}
}

var (
	// sbtResolvers starts a setting of an sbt build's resolvers.
	sbtResolvers = regexp.MustCompile(`\b(?:resolvers|externalResolvers)\s*(?:\+\+=|\+=|:=)`)
	// sbtAt is `"name" at "url"`.
	sbtAt = regexp.MustCompile(`"[^"]*"\s+at\s+"([^"]+)"`)
	// sbtURL is `Resolver.url("name", url("url"))`, with the Ivy patterns after it
	// when there are any.
	sbtURL = regexp.MustCompile(`Resolver\.url\(\s*"[^"]*"\s*,\s*(?:new\s+(?:java\.net\.)?URL|url)\(\s*"([^"]+)"\s*\)\s*\)(\s*\(\s*Resolver\.ivyStylePatterns)?`)
	// sbtMaven is `MavenRepository("name", "url")`.
	sbtMaven = regexp.MustCompile(`MavenRepository\(\s*"[^"]*"\s*,\s*"([^"]+)"`)
)

// parseSbtResolvers reads the Maven repositories an sbt build adds to its resolvers
// (`resolvers += "name" at "url"`, `resolvers ++= Seq(...)`, `Resolver.url(...)`
// and `MavenRepository(...)` written out literally). sbt asks them beside Maven
// Central. A `Resolver.url` with Ivy patterns is not a Maven layout and is not read,
// nor is `publishTo`, which is where the build is published, not what it resolves.
// project/*.sbt configures sbt's plugins, whose resolvers are not the code's.
//
// Implements: REQ-SUP-015, REQ-SUP-063
func parseSbtResolvers(data []byte, k sink) {
	source := string(data)
	for _, span := range sbtResolvers.FindAllStringIndex(source, -1) {
		setting := sbtSetting(source[span[1]:])
		var urls []string
		for _, m := range sbtAt.FindAllStringSubmatch(setting, -1) {
			urls = append(urls, m[1])
		}
		for _, m := range sbtURL.FindAllStringSubmatch(setting, -1) {
			if m[2] == "" {
				urls = append(urls, m[1])
			}
		}
		for _, m := range sbtMaven.FindAllStringSubmatch(setting, -1) {
			urls = append(urls, m[1])
		}
		for _, u := range urls {
			if u = strings.TrimSpace(u); strings.HasPrefix(u, "http") && !MavenPublic(u) {
				k.extra(Maven, u)
			}
		}
	}
}

// sbtSetting is the expression that starts source: up to the end of its line, or
// further while a bracket is open or the line ends in an operator, and never past
// the bracket or comma that closes the setting inside `.settings(...)`.
func sbtSetting(source string) string {
	depth := 0
	for i := 0; i < len(source); i++ {
		switch c := source[i]; c {
		case '"':
			for i++; i < len(source) && source[i] != '"'; i++ {
				if source[i] == '\\' {
					i++
				}
			}
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth--; depth < 0 {
				return source[:i]
			}
		case ',':
			if depth == 0 {
				return source[:i]
			}
		case '\n':
			if depth == 0 {
				if line := strings.TrimSpace(source[:i]); line != "" && !strings.HasSuffix(line, "+") && !strings.HasSuffix(line, "=") {
					return source[:i]
				}
			}
		}
	}
	return source
}

// parseSbtRepositories reads the [repositories] of sbt's repositories file: the
// predefined `maven-central`, and `name: url` entries (with Ivy patterns after a
// comma, read only when `mavenCompatible` says the layout is Maven's; `bootOnly`
// ones serve sbt's launcher only). Without -Dsbt.override.build.repos=true sbt's
// builds keep their own resolvers and Central, and each entry is asked beside
// them; with it the file's list is all that is asked, in its order, and Central
// only when the list names `maven-central`.
//
// Implements: REQ-SUP-015, REQ-SUP-063
func parseSbtRepositories(data []byte, override bool, k sink) {
	in := false
	var urls []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			in = line == "[repositories]"
			continue
		}
		if !in {
			continue
		}
		_, rest, ok := strings.Cut(line, ":")
		if !ok || strings.HasPrefix(rest, "//") {
			if line == "maven-central" {
				urls = append(urls, public[Maven])
			}
			continue
		}
		parts := strings.Split(rest, ",")
		u := strings.TrimSpace(parts[0])
		ivy, maven := false, false
		skip := !strings.HasPrefix(u, "http")
		for _, p := range parts[1:] {
			switch p = strings.TrimSpace(p); {
			case p == "mavenCompatible":
				maven = true
			case p == "bootOnly":
				skip = true
			case strings.Contains(p, "["):
				ivy = true
			}
		}
		if !skip && (!ivy || maven) {
			urls = append(urls, u)
		}
	}
	if override {
		k.off(Maven)
	}
	for _, u := range urls {
		switch {
		case override:
			k.put(Maven, Source{URL: u, Kind: Listed})
		case !MavenPublic(u):
			k.extra(Maven, u)
		}
	}
}

// coursierRepositories are the repository names COURSIER_REPOSITORIES may use for
// a Maven repository, other than `sonatype:<name>`.
var coursierRepositories = map[string]string{
	"jitpack": "https://jitpack.io",
	"google":  "https://maven.google.com",
	"clojars": Clojars,
}

// parseCoursierRepositories reads COURSIER_REPOSITORIES, the `|`-separated list the
// Coursier-based tools (coursier, Mill, Scala CLI, Bloop) resolve from instead of
// their defaults, in order: `central`, `sonatype:<name>`, `jitpack`, `google`,
// `clojars` and URLs. Local repositories (`ivy2Local`, `m2Local`) and Ivy ones
// (`ivy:`) are not read. Set, it replaces Maven Central: Central is asked only when
// the list names it.
//
// Implements: REQ-SUP-015, REQ-SUP-063
func parseCoursierRepositories(value string, k sink) {
	if strings.TrimSpace(value) == "" {
		return
	}
	k.off(Maven)
	for _, e := range strings.Split(value, "|") {
		e = strings.TrimSpace(e)
		u := ""
		switch {
		case e == "central":
			u = public[Maven]
		case strings.HasPrefix(e, "sonatype:"):
			u = "https://oss.sonatype.org/content/repositories/" + strings.TrimPrefix(e, "sonatype:")
		case strings.HasPrefix(e, "https://"), strings.HasPrefix(e, "http://"):
			u = e
		default:
			u = coursierRepositories[e]
		}
		if u != "" {
			k.put(Maven, Source{URL: u, Kind: Listed})
		}
	}
}
