package userconf

import (
	"path/filepath"
	"reflect"
	"testing"
)

// Maven's settings are the user's, then the installation's (MAVEN_HOME over M2_HOME).
//
// Verifies: REQ-SUP-064
func TestMavenSettings(t *testing.T) {
	home, mvn, m2 := t.TempDir(), t.TempDir(), t.TempDir()
	user := filepath.Join(home, ".m2", "settings.xml")
	for _, tc := range []struct {
		vars map[string]string
		want []string
	}{
		{nil, []string{user}},
		{map[string]string{"M2_HOME": m2}, []string{user, filepath.Join(m2, "conf", "settings.xml")}},
		{map[string]string{"M2_HOME": m2, "MAVEN_HOME": mvn}, []string{user, filepath.Join(mvn, "conf", "settings.xml")}},
	} {
		if got := machine(t, home, "linux", tc.vars).MavenSettings(); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%v: %v, want %v", tc.vars, got, tc.want)
		}
	}
}

// sbt's properties come from JAVA_OPTS, then SBT_OPTS; they move the global
// directory, the repositories file and switch the override on.
//
// Verifies: REQ-SUP-064, REQ-AUTH-020
func TestSbtLocations(t *testing.T) {
	home, base := t.TempDir(), t.TempDir()
	m := machine(t, home, "linux", nil)
	if got, want := m.SbtRepositories(), filepath.Join(home, ".sbt", "repositories"); got != want {
		t.Errorf("repositories: %s, want %s", got, want)
	}
	if m.SbtOverrideBuildRepos() {
		t.Error("override without the property")
	}
	want := []string{filepath.Join(home, ".sbt", ".credentials"), filepath.Join(home, ".ivy2", ".credentials")}
	if got := m.SbtCredentials(); !reflect.DeepEqual(got, want) {
		t.Errorf("credentials: %v, want %v", got, want)
	}

	m = machine(t, home, "linux", map[string]string{
		"JAVA_OPTS":       "-Dsbt.override.build.repos=true -Dsbt.global.base=/elsewhere",
		"SBT_OPTS":        `-Xmx2g "-Dsbt.global.base=` + base + `" -Dsbt.override.build.repos=false`,
		"SBT_CREDENTIALS": "/ci/credentials",
	})
	if got, want := m.SbtRepositories(), filepath.Join(base, "repositories"); got != want {
		t.Errorf("repositories: %s, want %s", got, want)
	}
	if m.SbtOverrideBuildRepos() {
		t.Error("SBT_OPTS did not win over JAVA_OPTS")
	}
	want = []string{"/ci/credentials", filepath.Join(base, ".credentials"), filepath.Join(home, ".ivy2", ".credentials")}
	if got := m.SbtCredentials(); !reflect.DeepEqual(got, want) {
		t.Errorf("credentials: %v, want %v", got, want)
	}
	m = machine(t, home, "linux", map[string]string{"SBT_OPTS": "-J-Dsbt.override.build.repos=true -Dsbt.repository.config=/r"})
	if !m.SbtOverrideBuildRepos() || m.SbtRepositories() != "/r" {
		t.Errorf("override %v, repositories %s", m.SbtOverrideBuildRepos(), m.SbtRepositories())
	}
}

// Coursier's configuration directory is the platform's, and COURSIER_CREDENTIALS
// is inline credentials or a file.
//
// Verifies: REQ-AUTH-020
func TestCoursierLocations(t *testing.T) {
	home, dir, appData := t.TempDir(), t.TempDir(), t.TempDir()
	for _, tc := range []struct {
		goos string
		vars map[string]string
		want string
	}{
		{"linux", nil, filepath.Join(home, ".config", "coursier")},
		{"linux", map[string]string{"XDG_CONFIG_HOME": dir}, filepath.Join(dir, "coursier")},
		{"linux", map[string]string{"COURSIER_CONFIG_DIR": dir}, dir},
		{"darwin", nil, filepath.Join(home, "Library", "Application Support", "Coursier")},
		{"windows", map[string]string{"APPDATA": appData}, filepath.Join(appData, "Coursier", "config")},
		{"windows", nil, filepath.Join(home, ".config", "coursier")},
	} {
		m := machine(t, home, tc.goos, tc.vars)
		if got := m.CoursierConfigDir(); got != tc.want {
			t.Errorf("%s %v: %s, want %s", tc.goos, tc.vars, got, tc.want)
		}
		if inline, file := m.CoursierCredentials(); inline != "" || file != filepath.Join(tc.want, "credentials.properties") {
			t.Errorf("%s %v: credentials %q %q", tc.goos, tc.vars, inline, file)
		}
	}
	abs := filepath.Join(dir, "c.properties")
	for value, want := range map[string][2]string{
		"host(realm) u:p":                   {"host(realm) u:p", ""},
		abs:                                 {"", abs},
		"file:///etc/coursier/c.properties": {"", "/etc/coursier/c.properties"},
	} {
		inline, file := machine(t, home, "linux", map[string]string{"COURSIER_CREDENTIALS": value}).CoursierCredentials()
		if inline != want[0] || file != want[1] {
			t.Errorf("%q: %q %q, want %q", value, inline, file, want)
		}
	}
}

// COURSIER_CREDENTIALS names a file by a file: URL the way the JVM reads one: on
// Windows a drive letter loses the slash before it and a host is a UNC server;
// elsewhere a host other than localhost names no local file. The platform is
// pinned both ways: through Platform and New, as depphunter runs, and on the
// machine directly.
//
// Verifies: REQ-AUTH-021
func TestCoursierCredentialsFileURL(t *testing.T) {
	saved := Platform
	t.Cleanup(func() { Platform = saved })
	for _, tc := range []struct {
		goos, value, want string
	}{
		{"windows", "file:///C:/x", `C:\x`},
		{"windows", "file:///c:/Users/me/.config/coursier/credentials.properties", `c:\Users\me\.config\coursier\credentials.properties`},
		{"windows", "file:/C:/x", `C:\x`},
		{"windows", "file://C:/x", `C:\x`},
		{"windows", "file:C:/x", `C:\x`},
		{"windows", "FILE:///D:/My%20Files/c.properties", `D:\My Files\c.properties`},
		{"windows", "file://localhost/C:/x", `C:\x`},
		{"windows", "file://server/share/c.properties", `\\server\share\c.properties`},
		{"windows", `C:\x\c.properties`, `C:\x\c.properties`},
		{"windows", `\\server\share\c.properties`, `\\server\share\c.properties`},
		{"linux", "file:///home/me/c.properties", "/home/me/c.properties"},
		{"linux", "file:/home/me/c.properties", "/home/me/c.properties"},
		{"linux", "file://localhost/home/me/My%20c.properties", "/home/me/My c.properties"},
		{"linux", "file://server/share/c.properties", ""},
		{"darwin", "file:///Users/me/c.properties", "/Users/me/c.properties"},
	} {
		Platform = tc.goos
		env := map[string]string{"COURSIER_CREDENTIALS": tc.value}
		for _, m := range []Machine{New(t.TempDir(), func(k string) string { return env[k] }), machine(t, t.TempDir(), tc.goos, env)} {
			if m.GOOS != tc.goos {
				t.Fatalf("GOOS %s, want %s", m.GOOS, tc.goos)
			}
			if inline, file := m.CoursierCredentials(); inline != "" || file != tc.want {
				t.Errorf("%s %q: inline %q, file %q, want %q", tc.goos, tc.value, inline, file, tc.want)
			}
		}
	}
	// Anything else is inline credentials.
	Platform = "linux"
	if inline, file := machine(t, t.TempDir(), "linux", map[string]string{"COURSIER_CREDENTIALS": "host(realm) u:p"}).CoursierCredentials(); inline != "host(realm) u:p" || file != "" {
		t.Errorf("inline: %q %q", inline, file)
	}
}

// Gradle's init scripts are init.gradle(.kts) in its user home, then init.d's
// scripts by name.
//
// Verifies: REQ-SUP-064
func TestGradleLocations(t *testing.T) {
	home, gradle := t.TempDir(), t.TempDir()
	if got, want := machine(t, home, "linux", nil).GradleUserHome(), filepath.Join(home, ".gradle"); got != want {
		t.Errorf("user home: %s, want %s", got, want)
	}
	for _, name := range []string{"init.gradle", "init.d/b.gradle", "init.d/a.gradle.kts", "init.d/c.txt", "gradle.properties"} {
		touch(t, filepath.Join(gradle, filepath.FromSlash(name)))
	}
	want := []string{filepath.Join(gradle, "init.gradle"), filepath.Join(gradle, "init.d", "a.gradle.kts"), filepath.Join(gradle, "init.d", "b.gradle")}
	if got := machine(t, home, "linux", map[string]string{"GRADLE_USER_HOME": gradle}).GradleInitScripts(); !reflect.DeepEqual(got, want) {
		t.Errorf("init scripts: %v, want %v", got, want)
	}
	if got := machine(t, "", "linux", nil).GradleInitScripts(); got != nil {
		t.Errorf("no home: %v", got)
	}
}

// The Clojure CLI's configuration is CLJ_CONFIG, else $XDG_CONFIG_HOME/clojure, else
// ~/.clojure; Leiningen's profiles are in LEIN_HOME, else ~/.lein.
//
// Verifies: REQ-SUP-064
func TestClojureLocations(t *testing.T) {
	home, dir := t.TempDir(), t.TempDir()
	for _, tc := range []struct {
		vars       map[string]string
		deps, lein string
	}{
		{nil, filepath.Join(home, ".clojure"), filepath.Join(home, ".lein", "profiles.clj")},
		{map[string]string{"XDG_CONFIG_HOME": dir}, filepath.Join(dir, "clojure"), filepath.Join(home, ".lein", "profiles.clj")},
		{map[string]string{"XDG_CONFIG_HOME": home, "CLJ_CONFIG": dir, "LEIN_HOME": dir}, dir, filepath.Join(dir, "profiles.clj")},
	} {
		m := machine(t, home, "linux", tc.vars)
		if got := m.ClojureConfigDir(); got != tc.deps {
			t.Errorf("%v: %s, want %s", tc.vars, got, tc.deps)
		}
		if got := m.LeinProfiles(); got != tc.lein {
			t.Errorf("%v: %s, want %s", tc.vars, got, tc.lein)
		}
	}
}
