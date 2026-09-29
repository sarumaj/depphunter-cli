package javascript

import (
	"slices"
	"strings"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
)

// The commits of the testdata/gitlock fixtures: pub on GitHub, corp on a company
// host, and tr on GitLab, which pub depends on.
const (
	gitPub  = "3b8f1e6a9d2c4b7e0f5a8c1d6e9b2f4a7c0d3e5f"
	gitCorp = "9e4d2b7a1c8f3e6d0b5a9c2e7f1d4b8a3c6e0f92"
	gitTr   = "6c1a9f4e2d7b0c5a8e3f6d1b9c4a7e2f5d0b8c3a"
)

// gitLocks are the fixtures under testdata/gitlock: one project written in each
// lock format, with the same install. The project declares pub
// (github:acme-oss/pub), corp (git+ssh from git.corp.example) and dep, a
// registry package; pub depends on dep and on tr (gitlab:acme-oss/tr). The
// classic yarn.lock writes pub as a codeload.github.com tarball, dep's tarball
// URL with its SHA-1 after a "#", and tr's URL with a token in it; the Berry
// lock adds branchy, a git dependency on a branch with no commit.
var gitLocks = []string{"npm-v3", "npm-v1", "yarn-classic", "berry", "pnpm-v9", "pnpm-v6", "bun"}

// TestGitDependenciesPinnedByCommit checks that a git dependency of every lock
// format is pinned to the commit the lock names, with the repository it came
// from as its origin, whether the project imports it or another package depends
// on it; that a registry package is not taken for one; and that no credential
// of the lock's URL is kept.
//
// Verifies: REQ-JS-019, REQ-JS-007, REQ-JS-008, REQ-JS-009, REQ-JS-016, REQ-SUP-009
func TestGitDependenciesPinnedByCommit(t *testing.T) {
	for _, directory := range gitLocks {
		t.Run(directory, func(t *testing.T) {
			root := "testdata/gitlock/" + directory
			imports := langtest.Imports(t, langtest.Analyze(t, Plugin{}, root)["index.js"])
			check := func(what string, got lang.Target, version, repository string) {
				t.Helper()
				if got.Version != version || !got.Pinned || lang.RepositoryName(got.Origin) != repository {
					t.Errorf("%s: %+v, want version %q from %q, pinned", what, got, version, repository)
				}
				if strings.Contains(got.Origin, "token") {
					t.Errorf("%s: origin %q keeps a credential", what, got.Origin)
				}
			}
			check("pub", imports["pub"], gitPub, "github.com/acme-oss/pub")
			check("corp", imports["corp"], gitCorp, "git.corp.example/team/corp")
			check("dep", imports["dep"], "1.0.0", "")
			if got := imports["pub"].Requested; got != "github:acme-oss/pub" {
				t.Errorf("pub requested %q, want package.json's range", got)
			}

			r, err := (Plugin{}).Resolver(root, langtest.Files(t, root))
			if err != nil {
				t.Fatal(err)
			}
			got := map[string]lang.Target{}
			for _, d := range r.(lang.Transitive).Dependencies(imports["pub"]) {
				got[d.Package] = d
			}
			if len(got) != 2 {
				t.Errorf("pub depends on %+v, want dep and tr", got)
			}
			check("pub's tr", got["tr"], gitTr, "gitlab.com/acme-oss/tr")
			check("pub's dep", got["dep"], "1.0.0", "")
		})
	}
}

// A git dependency whose lock entry names a branch and no commit (Berry's
// "#head=main") is not pinned: the project's import keeps package.json's range,
// the repository is still its origin, and the lock file is noted. A lock whose
// git dependencies all have commits is not.
//
// Verifies: REQ-JS-019, REQ-TRC-017
func TestGitDependencyWithoutACommit(t *testing.T) {
	root := "testdata/gitlock/berry"
	imports := langtest.Imports(t, langtest.Analyze(t, Plugin{}, root)["index.js"])
	want := lang.Target{Ecosystem: "npm", Package: "branchy", Version: "https://github.com/acme-oss/branchy.git#head=main",
		Origin: "https://github.com/acme-oss/branchy.git"}
	if got := imports["branchy"]; got != want {
		t.Errorf("branchy: %+v, want %+v", got, want)
	}
	r := newResolver(langtest.Files(t, root))
	if got := langtest.Notes(r); !slices.Equal(got, []string{"yarn.lock git-unpinned"}) {
		t.Errorf("notes %q", got)
	}
	if !strings.Contains(r.Notes()[0].Message, "git dependency branchy (") {
		t.Errorf("the note does not name branchy: %q", r.Notes()[0].Message)
	}
	for _, directory := range gitLocks {
		if directory == "berry" {
			continue
		}
		if got := langtest.Notes(newResolver(langtest.Files(t, "testdata/gitlock/"+directory))); len(got) != 0 {
			t.Errorf("%s: notes %q", directory, got)
		}
	}
}

// gitSource reads every way a lock file writes a git dependency, and nothing
// else: a registry tarball whose URL ends in its SHA-1 is not one.
//
// Verifies: REQ-JS-019
func TestGitSource(t *testing.T) {
	const sha = "0123456789abcdef0123456789abcdef01234567"
	for _, c := range []struct {
		resolved, repository, commit string
		ok                           bool
	}{
		{"git+ssh://git@github.com/o/r.git#" + sha, "ssh://git@github.com/o/r.git", sha, true},
		{"git+https://github.com/o/r.git#" + strings.ToUpper(sha), "https://github.com/o/r.git", sha, true},
		{"git://github.com/o/r.git#" + sha, "git://github.com/o/r.git", sha, true},
		{"github:o/r#" + sha, "https://github.com/o/r", sha, true},
		{"gitlab:o/r", "https://gitlab.com/o/r", "", true},
		{"bitbucket:o/r#main", "https://bitbucket.org/o/r", "", true},
		{"https://codeload.github.com/o/r/tar.gz/" + sha, "https://github.com/o/r", sha, true},
		{"https://github.com/o/r.git#commit=" + sha, "https://github.com/o/r.git", sha, true},
		{"https://github.com/o/r.git#commit=" + sha + "&workspace=x", "https://github.com/o/r.git", sha, true},
		{"https://github.com/o/r.git#head=main", "https://github.com/o/r.git", "", true},
		{"https://github.com/o/r.git#tag=v1.0.0", "https://github.com/o/r.git", "", true},
		{"ssh://git@git.corp.example/team/corp.git#commit=" + sha, "ssh://git@git.corp.example/team/corp.git", sha, true},
		{"git@git.corp.example:team/corp.git#" + sha, "git@git.corp.example:team/corp.git", sha, true},
		// Bun writes an scp-style repository behind "ssh://".
		{"git+ssh://git@github.com:o/r.git#" + sha, "ssh://git@github.com/o/r.git", sha, true},
		{"git+https://oauth2:secret@gitlab.com/o/r.git#" + sha, "https://gitlab.com/o/r.git", sha, true},
		{"git+https://secret@github.com/o/r.git#" + sha, "https://github.com/o/r.git", sha, true},
		{"git+ssh://git:secret@host:2222/o/r.git#" + sha, "ssh://git@host:2222/o/r.git", sha, true},
		// Bun's abbreviated commit, and npm's semver range of tags, are no commit.
		{"github:o/r#1eea5ba", "https://github.com/o/r", "", true},
		{"git+https://github.com/o/r.git#semver:^1.0.0", "https://github.com/o/r.git", "", true},
		{"https://registry.yarnpkg.com/dep/-/dep-1.0.0.tgz#" + sha, "", "", false},
		{"https://registry.npmjs.org/dep/-/dep-1.0.0.tgz", "", "", false},
		{"https://codeload.github.com/o/r/zip/" + sha, "", "", false},
		{"https://example.com/archive.tgz", "", "", false},
		{"1.0.0", "", "", false},
		{"npm:real@1.0.0", "", "", false},
		{"file:../x", "", "", false},
		{"", "", "", false},
	} {
		repository, commit, ok := gitSource(c.resolved)
		if repository != c.repository || commit != c.commit || ok != c.ok {
			t.Errorf("gitSource(%q) = %q, %q, %v; want %q, %q, %v", c.resolved, repository, commit, ok, c.repository, c.commit, c.ok)
		}
	}
}
