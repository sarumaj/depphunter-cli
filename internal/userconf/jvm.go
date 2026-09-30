package userconf

import (
	"cmp"
	"net/url"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

// ---------------------------------------------------------------- Maven

// MavenSettings lists Maven's settings files, the user's first since it dominates
// where the two disagree: ~/.m2/settings.xml, then conf/settings.xml of the Maven
// installation `MAVEN_HOME` (else `M2_HOME`) names. A `-s` on Maven's command line or
// in `MAVEN_ARGS` is not followed.
//
// Implements: REQ-SUP-064
func (m Machine) MavenSettings() []string {
	installation := cmp.Or(m.Environment("MAVEN_HOME"), m.Environment("M2_HOME"))
	return present(m.home(".m2", "settings.xml"), join(installation, "conf", "settings.xml"))
}

// ---------------------------------------------------------------- sbt

// SbtProperty is a Java system property sbt is started with: the last
// `-D<name>=<value>` of `JAVA_OPTS`, then of `SBT_OPTS`, which sbt's launcher
// script passes after it. A project's `.sbtopts` is the repository's and is not
// read, nor sbt's installation-wide `sbtopts`.
func (m Machine) SbtProperty(name string) (string, bool) {
	value, found := "", false
	for _, v := range []string{"JAVA_OPTS", "SBT_OPTS"} {
		for _, f := range strings.Fields(m.Environment(v)) {
			f = strings.TrimPrefix(strings.Trim(f, `"'`), "-J")
			if rest, ok := strings.CutPrefix(f, "-D"+name+"="); ok {
				value, found = strings.Trim(rest, `"'`), true
			} else if f == "-D"+name {
				value, found = "", true
			}
		}
	}
	return value, found
}

// sbtGlobalBase is sbt's global directory: `-Dsbt.global.base`, else ~/.sbt.
func (m Machine) sbtGlobalBase() string {
	directory, _ := m.SbtProperty("sbt.global.base")
	return cmp.Or(directory, m.home(".sbt"))
}

// SbtRepositories is the repositories file sbt's launcher reads:
// `-Dsbt.repository.config`, else `repositories` in sbt's global directory.
//
// Implements: REQ-SUP-064
func (m Machine) SbtRepositories() string {
	file, _ := m.SbtProperty("sbt.repository.config")
	return cmp.Or(file, join(m.sbtGlobalBase(), "repositories"))
}

// SbtOverrideBuildRepositories reports `-Dsbt.override.build.repos=true`, with which the
// repositories file replaces every resolver a build declares.
func (m Machine) SbtOverrideBuildRepositories() bool {
	v, ok := m.SbtProperty("sbt.override.build.repos")
	return ok && (v == "" || strings.EqualFold(v, "true"))
}

// SbtCredentials lists the credential files an sbt build conventionally reads:
// `SBT_CREDENTIALS`, `.credentials` in sbt's global directory and
// ~/.ivy2/.credentials.
//
// Implements: REQ-AUTH-020
func (m Machine) SbtCredentials() []string {
	var out []string
	for _, f := range []string{m.Environment("SBT_CREDENTIALS"), join(m.sbtGlobalBase(), ".credentials"), m.home(".ivy2", ".credentials")} {
		if f != "" && !slices.Contains(out, f) {
			out = append(out, f)
		}
	}
	return out
}

// ---------------------------------------------------------------- Coursier

// CoursierConfigDirectory is Coursier's configuration directory: `COURSIER_CONFIG_DIR`;
// `%APPDATA%\Coursier\config` on Windows; `~/Library/Application Support/Coursier`
// on macOS; else `$XDG_CONFIG_HOME/coursier` or ~/.config/coursier.
//
// Implements: REQ-AUTH-020
func (m Machine) CoursierConfigDirectory() string {
	return cmp.Or(m.Environment("COURSIER_CONFIG_DIR"), m.byPlatform(m.under("APPDATA", "Coursier", "config"),
		m.applicationSupport("Coursier"), m.absoluteXDGConfigHome("coursier")))
}

// CoursierCredentials is where Coursier's credentials are: `COURSIER_CREDENTIALS`
// holds them inline, or names a properties file when it is an absolute path or a
// `file:` URL; unset, credentials.properties in Coursier's configuration directory.
//
// Implements: REQ-AUTH-020
func (m Machine) CoursierCredentials() (inline, file string) {
	v := strings.TrimSpace(m.Environment("COURSIER_CREDENTIALS"))
	switch {
	case v == "":
		return "", join(m.CoursierConfigDirectory(), "credentials.properties")
	case len(v) >= 5 && strings.EqualFold(v[:5], "file:"):
		return "", fileURLPath(v, m.GOOS)
	case filepath.IsAbs(v), strings.HasPrefix(v, "/"), m.GOOS == "windows" && windowsAbsolute(v):
		return "", v
	}
	return v, ""
}

// fileURLPath is the local path a file: URL names on the platform goos, as the JVM
// reads one (new File(URI)): percent escapes decoded; file:///C:/x, file:/C:/x and
// the loose file://C:/x are C:\x on Windows, and file://server/share/x is the UNC
// path \\server\share\x there. Elsewhere only a URL without a host (or with
// localhost) names a local file. "" when it names none.
//
// Implements: REQ-AUTH-021
func fileURLPath(v, goos string) string {
	u, err := url.Parse(strings.TrimSpace(v))
	if err != nil || !strings.EqualFold(u.Scheme, "file") {
		return ""
	}
	p := u.Path
	if u.Opaque != "" { // file:C:/x
		if p, err = url.PathUnescape(u.Opaque); err != nil {
			return ""
		}
	}
	host := u.Host
	if strings.EqualFold(host, "localhost") {
		host = ""
	}
	if goos != "windows" {
		if host != "" {
			return ""
		}
		return p
	}
	switch {
	case len(host) == 2 && host[1] == ':' && driveLetter(host[0]):
		p = host + p // file://C:/x: the drive was taken for a host
	case host != "":
		return `\\` + host + strings.ReplaceAll(p, "/", `\`)
	case len(p) >= 3 && p[0] == '/' && driveLetter(p[1]) && p[2] == ':':
		p = p[1:]
	}
	if p == "" {
		return ""
	}
	return strings.ReplaceAll(p, "/", `\`)
}

// windowsAbsolute reports whether a path is absolute on Windows whichever platform reads
// it: a drive letter with a separator (C:\x, C:/x) or a UNC path.
func windowsAbsolute(p string) bool {
	if strings.HasPrefix(p, `\\`) || strings.HasPrefix(p, "//") {
		return true
	}
	return len(p) >= 3 && driveLetter(p[0]) && p[1] == ':' && (p[2] == '\\' || p[2] == '/')
}

func driveLetter(c byte) bool { return 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' }

// ---------------------------------------------------------------- Gradle

// GradleUserHome is `GRADLE_USER_HOME`, else ~/.gradle. `-Dgradle.user.home` in
// `GRADLE_OPTS` is not followed.
//
// Implements: REQ-SUP-064
func (m Machine) GradleUserHome() string {
	return cmp.Or(m.Environment("GRADLE_USER_HOME"), m.home(".gradle"))
}

// GradleInitScripts lists the init scripts Gradle runs from its user home, in its
// order: init.gradle or init.gradle.kts, then init.d's `*.gradle` and
// `*.gradle.kts` by name. The installation's init.d is not read.
//
// Implements: REQ-SUP-064
func (m Machine) GradleInitScripts() []string {
	directory := m.GradleUserHome()
	if directory == "" {
		return nil
	}
	var out []string
	for _, name := range []string{"init.gradle", "init.gradle.kts"} {
		if p := filepath.Join(directory, name); isFile(p) {
			out = append(out, p)
		}
	}
	var scripts []string
	for _, pattern := range []string{"*.gradle", "*.gradle.kts"} {
		found, _ := filepath.Glob(filepath.Join(directory, "init.d", pattern))
		scripts = append(scripts, found...)
	}
	sort.Strings(scripts)
	return append(out, scripts...)
}

// ---------------------------------------------------------------- Clojure

// ClojureConfigDirectory is the Clojure CLI's user configuration directory: `CLJ_CONFIG`;
// else `$XDG_CONFIG_HOME/clojure` when XDG_CONFIG_HOME is set; else ~/.clojure.
//
// Implements: REQ-SUP-064
func (m Machine) ClojureConfigDirectory() string {
	return cmp.Or(m.Environment("CLJ_CONFIG"), m.under("XDG_CONFIG_HOME", "clojure"), m.home(".clojure"))
}

// LeinProfiles is Leiningen's user profiles file: profiles.clj in `LEIN_HOME`, else
// in ~/.lein.
//
// Implements: REQ-SUP-064
func (m Machine) LeinProfiles() string {
	return cmp.Or(m.under("LEIN_HOME", "profiles.clj"), m.home(".lein", "profiles.clj"))
}
