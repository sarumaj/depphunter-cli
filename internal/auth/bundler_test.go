package auth

import (
	"path/filepath"
	"sort"
	"testing"
)

// bundlerEnvironment serves m as the environment and lists its names as the process
// environment, which is where readBundler finds the names of BUNDLE_ variables.
func bundlerEnvironment(t *testing.T, m map[string]string) func(string) string {
	t.Helper()
	saved := environ
	t.Cleanup(func() { environ = saved })
	environ = func() []string {
		var out []string
		for k, v := range m {
			out = append(out, k+"="+v)
		}
		sort.Strings(out)
		return out
	}
	return environment(m)
}

// A setting name decodes to the host it was written for; Bundler's own dotted
// settings are not hosts, and a value is the user:password Bundler sends.
//
// Verifies: REQ-AUTH-018
func TestBundlerKeyDecoding(t *testing.T) {
	for encoded, want := range map[string]string{
		"GEMS__CONTRIBSYS__COM":      "gems.contribsys.com",
		"GEM__FURY__IO":              "gem.fury.io",
		"MY___GEMS__EXAMPLE__COM":    "my-gems.example.com",
		"A_____B__COM":               "a-.b.com", // "-" then "." is five
		"RUBYGEMS__PKG__GITHUB__COM": "rubygems.pkg.github.com",
	} {
		if got := bundlerHost(encoded); got != want {
			t.Errorf("bundlerHost(%q) = %q, want %q", encoded, got, want)
		}
	}
	for host, setting := range map[string]bool{
		"build.nokogiri": true, "local.rack": true, "gem.test": true, "gem.mit": true, "github.https": true,
		"gem.fury.io": false, "github.com": false, "gems.example.com": false, "local.corp.test": false,
	} {
		if got := bundlerSetting(host); got != setting {
			t.Errorf("bundlerSetting(%q) = %v", host, got)
		}
	}
	for value, want := range map[string]string{
		"user:password":     "user:password",
		"token":             "token:",
		"deploy:p%40ss%3Aw": "deploy:p@ss:w",
		"user:pa:ss":        "user:pa:ss",
		":nouser":           "",
		"":                  "",
	} {
		if got := bundlerPair(value); got != want {
			t.Errorf("bundlerPair(%q) = %q, want %q", value, got, want)
		}
	}
}

// ~/.bundle/config and BUNDLE_<HOST> variables file their credentials under their
// hosts, the environment over the file; settings that only look like hosts, and
// values that are mirrors, file nothing; each credential goes to its host alone.
//
// Verifies: REQ-AUTH-018, REQ-AUTH-011
func TestBundlerCredentials(t *testing.T) {
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".bundle", "config"), `---
BUNDLE_GEMS__CONTRIBSYS__COM: "cfg-user:cfg-pass"
BUNDLE_GEM__FURY__IO: "fury-token"
BUNDLE_GITHUB__COM: "octo:ghp_x"
BUNDLE_HTTPS://RUBYGEMS__PKG__GITHUB__COM/ACME/: "acme:ghp_pkg"
BUNDLE_HTTPS://GEMS__PORT__TEST:8443/PRIVATE/: "port:secret"
BUNDLE_MIRROR__HTTPS://RUBYGEMS__ORG/: "https://mirror.corp.test"
BUNDLE_BUILD__NOKOGIRI: "--use-system-libraries"
BUNDLE_GEM__TEST: "rspec"
BUNDLE_PATH: "vendor/bundle"
BUNDLE_JOBS: 4
`)
	c := Read(home, bundlerEnvironment(t, map[string]string{
		"BUNDLE_GEMS__CONTRIBSYS__COM":   "env-user:env-pass",
		"BUNDLE_ENTERPRISE___GEMS__CORP": "ci:%2Bsecret",
		"BUNDLE_WITHOUT":                 "development:test",
		"BUNDLE_LOCAL__RACK":             "/src/rack",
		"BUNDLE_MIRROR__RUBYGEMS__ORG":   "https://mirror.corp.test",
		"BUNDLE_GITHUB__HTTPS":           "true",
		"UNRELATED__EXAMPLE__COM":        "x:y",
	}))
	for raw, want := range map[string]string{
		"https://gems.contribsys.com/info/sidekiq-pro":  basicHeader("env-user:env-pass"),
		"https://gem.fury.io/acme/info/lib":             basicHeader("fury-token:"),
		"https://github.com/acme/lib.git/info/refs":     basicHeader("octo:ghp_x"),
		"https://rubygems.pkg.github.com/acme/info/lib": basicHeader("acme:ghp_pkg"),
		"https://gems.port.test:8443/private/info/lib":  basicHeader("port:secret"),
		"https://enterprise-gems.corp/info/lib":         basicHeader("ci:+secret"),
		"https://gems.port.test/private/info/lib":       "",
		"https://evil.contribsys.com.attacker.test/":    "",
		"https://sub.gems.contribsys.com/info/lib":      "",
		"https://mirror.corp.test/info/rack":            "",
		"https://build.nokogiri/":                       "",
		"https://gem.test/":                             "",
		"https://local.rack/":                           "",
		"https://unrelated.example.com/":                "",
	} {
		if got := authorization(t, c, raw); got != want {
			t.Errorf("%s: %q, want %q", raw, got, want)
		}
	}
	if len(c.basic) != 6 {
		t.Errorf("filed %d credentials, want 6: %v", len(c.basic), c.basic)
	}
	// A nil environment reads no variable, even one the process has.
	c = Read(home, nil)
	if got := authorization(t, c, "https://gems.contribsys.com/"); got != basicHeader("cfg-user:cfg-pass") {
		t.Errorf("config alone: %q", got)
	}
	if got := authorization(t, c, "https://enterprise-gems.corp/"); got != "" {
		t.Errorf("a variable was read through a nil env: %q", got)
	}
}

// A host key is sent in preference to a source URL key on the same host.
//
// Verifies: REQ-AUTH-018
func TestBundlerHostKeyOverURLKey(t *testing.T) {
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".bundle", "config"), `---
BUNDLE_HTTPS://GEMS__CORP__TEST/A/: "url-a:x"
BUNDLE_HTTPS://GEMS__CORP__TEST/B/: "url-b:x"
BUNDLE_HTTPS://ONLY__URL__TEST/B/: "url-b:x"
BUNDLE_HTTPS://ONLY__URL__TEST/A/: "url-a:x"
BUNDLE_GEMS__CORP__TEST: "host:x"
`)
	c := Read(home, bundlerEnvironment(t, nil))
	if got := authorization(t, c, "https://gems.corp.test/a/info/x"); got != basicHeader("host:x") {
		t.Errorf("gems.corp.test: %q", got)
	}
	if got := authorization(t, c, "https://only.url.test/b/info/x"); got != basicHeader("url-a:x") {
		t.Errorf("only.url.test: %q, want the first URL key in order", got)
	}
}

// BUNDLE_USER_CONFIG names the file, else BUNDLE_USER_HOME the directory holding
// config; ~/.bundle/config is then not read.
//
// Verifies: REQ-AUTH-018
func TestBundlerUserConfigLocation(t *testing.T) {
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".bundle", "config"), "BUNDLE_HOME__CORP__TEST: \"home:x\"\n")
	other := filepath.Join(t.TempDir(), "bundler.yml")
	writeFile(t, other, "BUNDLE_USERCONFIG__CORP__TEST: \"uc:x\"\n")
	directory := t.TempDir()
	writeFile(t, filepath.Join(directory, "config"), "BUNDLE_USERHOME__CORP__TEST: \"uh:x\"\n")

	for _, testCase := range []struct {
		environment map[string]string
		host        string
	}{
		{nil, "home.corp.test"},
		{map[string]string{"BUNDLE_USER_CONFIG": other, "BUNDLE_USER_HOME": directory}, "userconfig.corp.test"},
		{map[string]string{"BUNDLE_USER_HOME": directory}, "userhome.corp.test"},
	} {
		c := Read(home, bundlerEnvironment(t, testCase.environment))
		if len(c.basic) != 1 || c.basic[testCase.host] == "" {
			t.Errorf("%v: %v, want %s only", testCase.environment, c.basic, testCase.host)
		}
	}
}

// The application's .bundle/config - beside the Gemfile, or where BUNDLE_APP_CONFIG
// points - belongs to the repository and is never read.
//
// Verifies: REQ-AUTH-019, REQ-AUTH-012
func TestBundlerAppConfigIsNotRead(t *testing.T) {
	home := t.TempDir()
	repository := filepath.Join(home, "src", "app")
	writeFile(t, filepath.Join(repository, "Gemfile"), "source \"https://gems.corp.test\"\ngem \"lib\"\n")
	writeFile(t, filepath.Join(repository, ".bundle", "config"), "BUNDLE_GEMS__CORP__TEST: \"repo:leak\"\n")
	writeFile(t, filepath.Join(repository, "ci-bundle", "config"), "BUNDLE_GEMS__CORP__TEST: \"repo:leak2\"\n")
	t.Chdir(repository)
	for _, app := range []string{filepath.Join(repository, ".bundle"), filepath.Join(repository, "ci-bundle"), ".bundle"} {
		c := Read(home, bundlerEnvironment(t, map[string]string{"BUNDLE_APP_CONFIG": app}))
		if got := authorization(t, c, "https://gems.corp.test/info/lib"); got != "" {
			t.Errorf("BUNDLE_APP_CONFIG=%s: the repository's credential was taken: %q", app, got)
		}
	}
}

// Over plain http a Bundler credential goes only to a host this machine's own
// configuration names with an http:// address: a source URL key or a mirror.
//
// Verifies: REQ-AUTH-011, REQ-AUTH-018
func TestBundlerPlainHTTPOnlyWhenConfigured(t *testing.T) {
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".bundle", "config"), `---
BUNDLE_HTTP://LEGACY__CORP__TEST/: "legacy:x"
BUNDLE_MIRROR__HTTPS://RUBYGEMS__ORG/: "http://mirror.corp.test/gems"
BUNDLE_MIRROR__CORP__TEST: "mirror:x"
`)
	c := Read(home, bundlerEnvironment(t, map[string]string{
		"BUNDLE_SECURE__CORP__TEST":    "secure:x",
		"BUNDLE_MIRROR__RUBYGEMS__ORG": "http://envmirror.corp.test",
		"BUNDLE_ENVMIRROR__CORP__TEST": "envmirror:x",
	}))
	for raw, want := range map[string]string{
		"http://legacy.corp.test/info/rack":      basicHeader("legacy:x"),
		"http://mirror.corp.test/gems/info/rack": basicHeader("mirror:x"),
		"http://envmirror.corp.test/info/rack":   basicHeader("envmirror:x"),
		"http://secure.corp.test/info/rack":      "",
		"https://secure.corp.test/info/rack":     basicHeader("secure:x"),
	} {
		if got := authorization(t, c, raw); got != want {
			t.Errorf("%s: %q, want %q", raw, got, want)
		}
	}
}

// Bundler's credential replaces a netrc entry for the same host, and a credential
// in an index URL this machine configured replaces Bundler's, as Bundler keeps the
// user information a source URL already has.
//
// Verifies: REQ-AUTH-018
func TestBundlerPrecedenceWithNetrcAndURL(t *testing.T) {
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".netrc"), "machine gems.corp.test login netrc password x\nmachine other.corp.test login netrc password y\n")
	writeFile(t, filepath.Join(home, ".bundle", "config"), "BUNDLE_GEMS__CORP__TEST: \"bundle:x\"\n")
	c := Read(home, bundlerEnvironment(t, nil))
	if got := authorization(t, c, "https://gems.corp.test/"); got != basicHeader("bundle:x") {
		t.Errorf("gems.corp.test: %q", got)
	}
	if got := authorization(t, c, "https://other.corp.test/"); got != basicHeader("netrc:y") {
		t.Errorf("other.corp.test: %q", got)
	}
	c.FromURL("https://url:z@gems.corp.test/", true)
	if got := authorization(t, c, "https://gems.corp.test/"); got != basicHeader("url:z") {
		t.Errorf("after a machine URL credential: %q", got)
	}
}
