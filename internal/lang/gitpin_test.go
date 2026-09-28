package lang

import "testing"

// GitPin reads a commit from every place the plugins record one, only when it is
// whole, and says whether it may leave the machine: only a repository on a public
// forge, or a package of an ecosystem whose plugins would have named a private one
// in its origin, is asked about.
//
// Verifies: REQ-FND-026
func TestGitPin(t *testing.T) {
	const sha = "0123456789abcdef0123456789abcdef01234567"
	sha256 := sha + sha[:24]
	for _, c := range []struct {
		name                     string
		eco, pkg, ver, origin, g string
		commit, repo             string
		public                   bool
	}{
		{"swiftpm revision", "swiftpm", "github.com/apple/swift-nio", sha, "", "", sha, "github.com/apple/swift-nio", true},
		{"SHA-256 commit", "zig", "codeberg.org/o/r", sha256, "", "", sha256, "codeberg.org/o/r", true},
		{"upper case is spelled as git spells it", "carthage", "github.com/o/r", "0123456789ABCDEF0123456789ABCDEF01234567", "", "", sha, "github.com/o/r", true},
		{"carthage on a company host", "carthage", "git.example.com/ios/Kit", sha, "", "", sha, "git.example.com/ios/Kit", false},
		{"mix git dep on GitHub (origin)", "hex", "phoenix", sha, "https://github.com/phoenixframework/phoenix.git", "", sha, "github.com/phoenixframework/phoenix", true},
		{"jsonnet-bundler on a private host", "jsonnet-bundler", "git.acme.internal/ops/libs", sha, "git@git.acme.internal:ops/libs.git", "", sha, "git.acme.internal/ops/libs", false},
		{"terraform module origin without a scheme", "terraform-module", "github.com/acme/tf-dns", sha, "github.com/acme/tf-dns", "", sha, "github.com/acme/tf-dns", true},
		{"shard named by its short name", "shards", "markd", sha, "", "", sha, "", true},
		{"shard version with its commit", "shards", "spectator", "0.12.1+git.commit." + sha, "", "", sha, "", true},
		{"alire crate with a private origin", "alire", "fancy", sha, "https://git.acme.dev/fancy.git", "", sha, "git.acme.dev/fancy", false},
		{"no repository, and an ecosystem that does not say", "gitlab-ci", "infra/pipelines", sha, "", "", sha, "", false},
		{"a GitHub Action could be on GitHub Enterprise", "actions", "actions/cache", sha, "", "", sha, "", false},
		{"npm github: specifier", "npm", "left-pad", "github:stevemao/left-pad#" + sha, "", "", sha, "github.com/stevemao/left-pad", true},
		{"npm git URL", "npm", "x", "git+ssh://git@gitlab.com/o/x.git#" + sha, "", "", sha, "gitlab.com/o/x", true},
		{"npm owner/repo", "npm", "x", "o/x#" + sha, "", "", sha, "github.com/o/x", true},
		{"npm names no repository by its package name", "npm", "github.com/o/x", sha, "", "", sha, "", false},
		{"composer branch at a commit names no repository", "composer", "acme/lib", "dev-main#" + sha, "", "", sha, "", false},
		{"the git field wins", "rubygems", "devise", "4.9.3", "https://github.com/heartcombo/devise.git", "https://github.com/heartcombo/devise.git#" + sha, sha, "github.com/heartcombo/devise", true},
		{"a cargo git crate is no less public for having no origin", "crates", "regex", "1.10.0", "", "https://github.com/rust-lang/regex#" + sha, sha, "github.com/rust-lang/regex", true},
		{"nix checkout on a company host", "nix", "git.example.org/team/foo", sha[:7], "", "https://git.example.org/team/foo#" + sha, sha, "git.example.org/team/foo", false},
		{"a path origin is no repository", "shards", "local", sha, "path:../local", "", sha, "", false},
		{"Bun's shortened commit", "npm", "forge-std", "github:foundry-rs/forge-std#1eea5ba", "", "", "", "", false},
		{"Nix's display form", "nix", "github.com/o/r", "0123456", "", "", "", "", false},
		{"a tag", "swiftpm", "github.com/o/r", "1.2.3", "", "", "", "", false},
		{"a shortened git field", "crates", "regex", "1.10.0", "", "https://github.com/rust-lang/regex#0123456", "", "", false},
	} {
		commit, repo, public := GitPin(c.eco, c.pkg, c.ver, c.origin, c.g)
		if commit != c.commit || repo != c.repo || public != c.public {
			t.Errorf("%s: GitPin = %q, %q, %v; want %q, %q, %v", c.name, commit, repo, public, c.commit, c.repo, c.public)
		}
	}
}

// Verifies: REQ-FND-026
func TestPublicForge(t *testing.T) {
	for repo, want := range map[string]bool{
		"https://github.com/o/r.git":     true,
		"git@gitlab.com:g/sub/r.git":     true,
		"bitbucket.org/o/r":              true,
		"https://git.sr.ht/~o/r":         true,
		"https://codeberg.org/o/r":       true,
		"https://github.example.com/o/r": false,
		"git@git.acme.internal:ops/lib":  false,
		"":                               false,
	} {
		if got := PublicForge(repo); got != want {
			t.Errorf("PublicForge(%q) = %v, want %v", repo, got, want)
		}
	}
}
