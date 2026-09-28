package userconf

import (
	"net/url"
	"path/filepath"
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
	var out []string
	if user := join(m.Home, ".m2", "settings.xml"); user != "" {
		out = append(out, user)
	}
	for _, v := range []string{"MAVEN_HOME", "M2_HOME"} {
		if dir := m.Env(v); dir != "" {
			return append(out, filepath.Join(dir, "conf", "settings.xml"))
		}
	}
	return out
}

// ---------------------------------------------------------------- sbt

// SbtProperty is a Java system property sbt is started with: the last
// `-D<name>=<value>` of `JAVA_OPTS`, then of `SBT_OPTS`, which sbt's launcher
// script passes after it. A project's `.sbtopts` is the repository's and is not
// read, nor sbt's installation-wide `sbtopts`.
func (m Machine) SbtProperty(name string) (string, bool) {
	value, found := "", false
	for _, v := range []string{"JAVA_OPTS", "SBT_OPTS"} {
		for _, f := range strings.Fields(m.Env(v)) {
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
	if dir, ok := m.SbtProperty("sbt.global.base"); ok && dir != "" {
		return dir
	}
	return join(m.Home, ".sbt")
}

// SbtRepositories is the repositories file sbt's launcher reads:
// `-Dsbt.repository.config`, else `repositories` in sbt's global directory.
//
// Implements: REQ-SUP-064
func (m Machine) SbtRepositories() string {
	if file, ok := m.SbtProperty("sbt.repository.config"); ok && file != "" {
		return file
	}
	return join(m.sbtGlobalBase(), "repositories")
}

// SbtOverrideBuildRepos reports `-Dsbt.override.build.repos=true`, with which the
// repositories file replaces every resolver a build declares.
func (m Machine) SbtOverrideBuildRepos() bool {
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
	for _, f := range []string{m.Env("SBT_CREDENTIALS"), join(m.sbtGlobalBase(), ".credentials"), join(m.Home, ".ivy2", ".credentials")} {
		if f != "" && !contains(out, f) {
			out = append(out, f)
		}
	}
	return out
}

// ---------------------------------------------------------------- Coursier

// CoursierConfigDir is Coursier's configuration directory: `COURSIER_CONFIG_DIR`;
// `%APPDATA%\Coursier\config` on Windows; `~/Library/Application Support/Coursier`
// on macOS; else `$XDG_CONFIG_HOME/coursier` or ~/.config/coursier.
//
// Implements: REQ-AUTH-020
func (m Machine) CoursierConfigDir() string {
	if dir := m.Env("COURSIER_CONFIG_DIR"); dir != "" {
		return dir
	}
	switch m.GOOS {
	case "windows":
		if appData := m.Env("APPDATA"); appData != "" {
			return filepath.Join(appData, "Coursier", "config")
		}
	case "darwin", "ios":
		return join(m.Home, "Library", "Application Support", "Coursier")
	}
	if xdg := m.Env("XDG_CONFIG_HOME"); filepath.IsAbs(xdg) {
		return filepath.Join(xdg, "coursier")
	}
	return join(m.Home, ".config", "coursier")
}

// CoursierCredentials is where Coursier's credentials are: `COURSIER_CREDENTIALS`
// holds them inline, or names a properties file when it is an absolute path or a
// `file:` URL; unset, credentials.properties in Coursier's configuration directory.
//
// Implements: REQ-AUTH-020
func (m Machine) CoursierCredentials() (inline, file string) {
	v := strings.TrimSpace(m.Env("COURSIER_CREDENTIALS"))
	switch {
	case v == "":
		return "", join(m.CoursierConfigDir(), "credentials.properties")
	case len(v) >= 5 && strings.EqualFold(v[:5], "file:"):
		return "", fileURLPath(v, m.GOOS)
	case filepath.IsAbs(v), strings.HasPrefix(v, "/"), m.GOOS == "windows" && windowsAbs(v):
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

// windowsAbs reports whether a path is absolute on Windows whichever platform reads
// it: a drive letter with a separator (C:\x, C:/x) or a UNC path.
func windowsAbs(p string) bool {
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
	if dir := m.Env("GRADLE_USER_HOME"); dir != "" {
		return dir
	}
	return join(m.Home, ".gradle")
}

// GradleInitScripts lists the init scripts Gradle runs from its user home, in its
// order: init.gradle or init.gradle.kts, then init.d's `*.gradle` and
// `*.gradle.kts` by name. The installation's init.d is not read.
//
// Implements: REQ-SUP-064
func (m Machine) GradleInitScripts() []string {
	dir := m.GradleUserHome()
	if dir == "" {
		return nil
	}
	var out []string
	for _, name := range []string{"init.gradle", "init.gradle.kts"} {
		if p := filepath.Join(dir, name); isFile(p) {
			out = append(out, p)
		}
	}
	var scripts []string
	for _, pattern := range []string{"*.gradle", "*.gradle.kts"} {
		found, _ := filepath.Glob(filepath.Join(dir, "init.d", pattern))
		scripts = append(scripts, found...)
	}
	sort.Strings(scripts)
	return append(out, scripts...)
}

// ---------------------------------------------------------------- Clojure

// ClojureConfigDir is the Clojure CLI's user configuration directory: `CLJ_CONFIG`;
// else `$XDG_CONFIG_HOME/clojure` when XDG_CONFIG_HOME is set; else ~/.clojure.
//
// Implements: REQ-SUP-064
func (m Machine) ClojureConfigDir() string {
	if dir := m.Env("CLJ_CONFIG"); dir != "" {
		return dir
	}
	if xdg := m.Env("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "clojure")
	}
	return join(m.Home, ".clojure")
}

// LeinProfiles is Leiningen's user profiles file: profiles.clj in `LEIN_HOME`, else
// in ~/.lein.
//
// Implements: REQ-SUP-064
func (m Machine) LeinProfiles() string {
	if dir := m.Env("LEIN_HOME"); dir != "" {
		return filepath.Join(dir, "profiles.clj")
	}
	return join(m.Home, ".lein", "profiles.clj")
}
