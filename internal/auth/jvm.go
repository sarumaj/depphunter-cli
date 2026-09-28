package auth

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang/edn"
	"github.com/sarumaj/depphunter-cli/internal/userconf"
)

// readMaven reads the <servers> of Maven's settings files (the user's, then the
// installation's), matched to their repositories there and to those the Clojure
// CLI's user deps.edn names, whose tools.deps looks the same servers up by
// repository name.
//
// Implements: REQ-AUTH-003, REQ-AUTH-020
func (c *Store) readMaven(m userconf.Machine) {
	var files [][]byte
	for _, name := range m.MavenSettings() {
		if data, err := os.ReadFile(name); err == nil {
			files = append(files, data)
		}
	}
	repos := map[string]string{}
	if dir := m.ClojureConfigDir(); dir != "" {
		if data, err := os.ReadFile(filepath.Join(dir, "deps.edn")); err == nil {
			for _, top := range edn.Read(data) {
				r := top.Get("mvn/repos")
				if r == nil || r.Kind != edn.Map {
					continue
				}
				for i := 1; i < len(r.Kids); i += 2 {
					name, v := r.Kids[i-1], r.Kids[i]
					if v.Kind == edn.Map {
						v = v.Get("url")
					}
					if name.Kind == edn.String && v != nil && v.Kind == edn.String {
						repos[name.Text] = v.Text
					}
				}
			}
		}
	}
	if len(files) > 0 {
		c.readMavenSettings(files, repos)
	}
}

// readJVM reads the credentials of sbt, Coursier and Gradle.
//
// Implements: REQ-AUTH-021
func (c *Store) readJVM(m userconf.Machine) {
	for _, name := range m.SbtCredentials() {
		if data, err := os.ReadFile(name); err == nil {
			c.readSbtCredentials(data)
		}
	}
	inline, file := m.CoursierCredentials()
	if file != "" {
		if data, err := os.ReadFile(file); err == nil {
			c.readCoursierProperties(data)
		}
	}
	c.readCoursierInline(inline)
	c.readGradle(m)
}

// properties reads a Java properties file's `key=value` (or `key: value`) lines;
// escapes and continued lines are not.
func properties(data []byte) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line[0] == '#' || line[0] == '!' {
			continue
		}
		i := strings.IndexAny(line, "=:")
		if i <= 0 {
			continue
		}
		out[strings.TrimSpace(line[:i])] = strings.TrimSpace(line[i+1:])
	}
	return out
}

// credentialHost is the host a credential file names, "" for anything that is not a
// bare host name (an address with a scheme, a path or user information).
func credentialHost(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" || strings.ContainsAny(host, "/@ ") {
		return ""
	}
	return host
}

// readSbtCredentials reads an sbt credentials file (`realm=`, `host=`, `user=`,
// `password=`) - ~/.sbt/.credentials, ~/.ivy2/.credentials or SBT_CREDENTIALS -
// which a build adds with `credentials += Credentials(Path.userHome / ...)`. The
// credential goes to its host whatever realm the server answers with.
//
// Implements: REQ-AUTH-021
func (c *Store) readSbtCredentials(data []byte) {
	p := properties(data)
	if host := credentialHost(p["host"]); host != "" && p["user"] != "" && p["password"] != "" {
		c.basic[host] = p["user"] + ":" + p["password"]
	}
}

// readCoursierProperties reads Coursier's credentials.properties:
// `<name>.host`, `<name>.username` and `<name>.password` per entry.
//
// Implements: REQ-AUTH-021
func (c *Store) readCoursierProperties(data []byte) {
	entries := map[string]map[string]string{}
	for key, value := range properties(data) {
		for _, field := range []string{"host", "username", "password"} {
			if name, ok := strings.CutSuffix(key, "."+field); ok && name != "" {
				if entries[name] == nil {
					entries[name] = map[string]string{}
				}
				entries[name][field] = value
			}
		}
	}
	for _, e := range entries {
		if host := credentialHost(e["host"]); host != "" && e["username"] != "" {
			c.basic[host] = e["username"] + ":" + e["password"]
		}
	}
}

// coursierInline is one line of inline Coursier credentials:
// `host(realm) user:password`, the realm optional.
var coursierInline = regexp.MustCompile(`^(\S+?)(?:\([^)]*\))?\s+([^:\s]+):(.*)$`)

// readCoursierInline reads credentials COURSIER_CREDENTIALS holds itself, one
// `host(realm) user:password` per line.
//
// Implements: REQ-AUTH-021
func (c *Store) readCoursierInline(value string) {
	for _, line := range strings.Split(value, "\n") {
		m := coursierInline.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		if host := credentialHost(m[1]); host != "" {
			c.basic[host] = m[2] + ":" + strings.TrimSpace(m[3])
		}
	}
}

var (
	// gradleMaven is a maven { ... } block without nested braces.
	gradleMaven = regexp.MustCompile(`\bmaven\s*\{([^{}]*)\}`)
	// gradleName is the block's `name = "x"`, `name "x"` or `name("x")`.
	gradleName = regexp.MustCompile(`\bname\s*(?:=\s*|\(\s*)?["']([^"']+)["']`)
	// gradleURL is the block's url, as parseGradleRepos in internal/index reads it.
	gradleURL = regexp.MustCompile(`\b(?:url|setUrl)\s*(?:=\s*|\(\s*)?(?:uri\(\s*)?["']([^"']+)["']`)
	// gradlePassword is `credentials(PasswordCredentials)` (`::class` in Kotlin).
	gradlePassword = regexp.MustCompile(`\bcredentials\s*\(\s*PasswordCredentials\b`)
)

// readGradle reads the credentials Gradle's convention gives a repository of the
// init scripts in its user home: a `maven { name = "corp"; url = ...;
// credentials(PasswordCredentials) }` takes `corpUsername` and `corpPassword` from
// gradle.properties in Gradle's user home, else from ORG_GRADLE_PROJECT_corpUsername
// and ORG_GRADLE_PROJECT_corpPassword. A repository a project's own build script
// declares is not matched: the repository would be choosing where the password goes.
//
// Implements: REQ-AUTH-021
func (c *Store) readGradle(m userconf.Machine) {
	scripts := m.GradleInitScripts()
	if len(scripts) == 0 {
		return
	}
	props := map[string]string{}
	if home := m.GradleUserHome(); home != "" {
		if data, err := os.ReadFile(filepath.Join(home, "gradle.properties")); err == nil {
			props = properties(data)
		}
	}
	property := func(name string) string {
		if v := props[name]; v != "" {
			return v
		}
		return m.Env("ORG_GRADLE_PROJECT_" + name)
	}
	for _, name := range scripts {
		data, err := os.ReadFile(name)
		if err != nil {
			continue
		}
		for _, block := range gradleMaven.FindAllStringSubmatch(string(data), -1) {
			repo, u := gradleName.FindStringSubmatch(block[1]), gradleURL.FindStringSubmatch(block[1])
			if repo == nil || u == nil || !gradlePassword.MatchString(block[1]) {
				continue
			}
			user, pass := property(repo[1]+"Username"), property(repo[1]+"Password")
			if user == "" || pass == "" {
				continue
			}
			if host := c.hostOf(u[1]); host != "" {
				c.basic[host] = user + ":" + pass
			}
		}
	}
}
