package auth

import (
	"os"
	"path/filepath"
	"testing"
)

// aboveTheCheckout lays out base/outer/checkout/project with checkout holding a
// .git, and returns outer (this machine's), checkout (the repository's) and project
// (the analyzed directory).
func aboveTheCheckout(t *testing.T) (outer, checkout, project string) {
	t.Helper()
	outer = filepath.Join(t.TempDir(), "outer")
	checkout = filepath.Join(outer, "checkout")
	project = filepath.Join(checkout, "project")
	for _, directory := range []string{filepath.Join(checkout, ".git"), project} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return outer, checkout, project
}

// The .yarnrc.yml of a directory above the analyzed checkout is this machine's: it
// is merged with the home's key by key, the closer winning, and its credentials are
// sent. One inside the checkout is the repository's and gives nothing.
//
// Verifies: REQ-AUTH-024, REQ-SUP-079
func TestYarnFilesAboveTheCheckout(t *testing.T) {
	outer, checkout, project := aboveTheCheckout(t)
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".yarnrc.yml"), "npmScopes:\n  acme:\n    npmRegistryServer: https://acme.corp\n    npmAuthToken: home\n  team:\n    npmRegistryServer: https://team.corp\n")
	writeFile(t, filepath.Join(outer, ".yarnrc.yml"), "npmScopes:\n  acme:\n    npmAuthToken: \"${OUTER_TOKEN}\"\n  team:\n    npmAuthToken: team\n")
	writeFile(t, filepath.Join(checkout, ".yarnrc.yml"), "npmScopes:\n  acme:\n    npmAuthToken: repository\n  evil:\n    npmRegistryServer: https://evil.example\n    npmAuthToken: repository\n")
	c := ReadFor(home, project, bundlerEnvironment(t, map[string]string{"OUTER_TOKEN": "outer"}))
	for u, want := range map[string]string{
		"https://acme.corp/@acme%2fx/1.0.0":    bearer("outer"),
		"https://team.corp/@team%2fx/1.0.0":    bearer("team"),
		"https://evil.example/@evil%2fx/1.0.0": "",
	} {
		if got := authorization(t, c, u); got != want {
			t.Errorf("%s: %q, want %q", u, got, want)
		}
	}
	// Without the analyzed directory only the home's file is read.
	if got := authorization(t, Read(home, bundlerEnvironment(t, nil)), "https://acme.corp/@acme%2fx/1.0.0"); got != bearer("home") {
		t.Errorf("home alone: %q", got)
	}
}

// A Yarn credential without npmAlwaysAuth serves the scoped packages only, a
// scope's own credential winning for its scope; what npm's configuration holds for
// the same registry wins over all of Yarn's, the scoped packages' included.
//
// Verifies: REQ-AUTH-024
func TestYarnAlwaysAuthAndTheNpmrc(t *testing.T) {
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".npmrc"), "//npm.corp/npm/:_authToken=npm\n//held.corp/:_authToken=npm\n")
	writeFile(t, filepath.Join(home, ".yarnrc.yml"), `npmRegistryServer: https://yarn.corp/npm
npmAuthToken: top
npmScopes:
  acme:
    npmRegistryServer: https://yarn.corp/npm
    npmAuthToken: acme
npmRegistries:
  https://npm.corp/npm:
    npmAuthToken: yarn
    npmAlwaysAuth: true
  https://held.corp:
    npmAuthToken: yarn
`)
	c := Read(home, bundlerEnvironment(t, nil))
	for u, want := range map[string]string{
		"https://yarn.corp/npm/@acme%2fui/1.0.0": bearer("acme"),
		"https://yarn.corp/npm/@other%2fx/1.0.0": bearer("top"),
		"https://yarn.corp/npm/react/1.0.0":      "",
		"https://yarn.corp/npm/@acmex/1.0.0":     bearer("top"),
		"https://npm.corp/npm/react/1.0.0":       bearer("npm"),
		"https://npm.corp/npm/@acme%2fui/1.0.0":  bearer("npm"),
		"https://held.corp/@acme%2fui/1.0.0":     bearer("npm"),
	} {
		if got := authorization(t, c, u); got != want {
			t.Errorf("%s: %q, want %q", u, got, want)
		}
	}
}

// The nuget.config of a directory above the analyzed checkout is this machine's: a
// credential it keeps for a source it defines is sent. One inside the checkout is
// the repository's and gives nothing.
//
// Verifies: REQ-AUTH-004, REQ-SUP-079
func TestNuGetFilesAboveTheCheckout(t *testing.T) {
	outer, checkout, project := aboveTheCheckout(t)
	source := func(key, feed, user, password string) string {
		return `<configuration><packageSources><add key="` + key + `" value="` + feed + `"/></packageSources>` +
			`<packageSourceCredentials><` + key + `><add key="Username" value="` + user + `"/><add key="ClearTextPassword" value="` + password + `"/></` + key + `></packageSourceCredentials></configuration>`
	}
	writeFile(t, filepath.Join(outer, "NuGet.Config"), source("corp", "https://nuget.corp/v3/index.json", "ci", "outer"))
	writeFile(t, filepath.Join(checkout, "NuGet.Config"), source("repo", "https://repo.example/v3/index.json", "ci", "repository"))
	c := ReadFor(t.TempDir(), project, bundlerEnvironment(t, nil))
	if got := authorization(t, c, "https://nuget.corp/v3/index.json"); got != basicHeader("ci:outer") {
		t.Errorf("above the checkout: %q", got)
	}
	if got := authorization(t, c, "https://repo.example/v3/index.json"); got != "" {
		t.Errorf("inside the checkout: %q", got)
	}
}
