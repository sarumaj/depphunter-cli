package links

import "testing"

// Implements: REQ-MOD-014
func TestPage(t *testing.T) {
	for _, c := range []struct{ ecosystem, name, index, want string }{
		{"npm", "lodash", "", "https://www.npmjs.com/package/lodash"},
		{"npm", "@types/node", "https://registry.npmjs.org", "https://www.npmjs.com/package/@types/node"},
		{"pypi", "PyYAML", "", "https://pypi.org/project/PyYAML/"},
		{"go", "github.com/google/uuid", "", "https://pkg.go.dev/github.com/google/uuid"},
		{"go-std", "net/http", "", "https://pkg.go.dev/net/http"},
		{"crates", "serde", "", "https://crates.io/crates/serde"},
		{"maven", "com.google.guava:guava", "", "https://central.sonatype.com/artifact/com.google.guava/guava"},
		{"maven", "ring:ring", "https://repo.clojars.org", "https://clojars.org/ring"},
		{"maven", "metosin:reitit", "https://repo.clojars.org", "https://clojars.org/metosin/reitit"},
		{"maven", "not_declared", "", ""},
		{"nuget", "Newtonsoft.Json", "", "https://www.nuget.org/packages/Newtonsoft.Json"},
		{"oci", "alpine", "", "https://hub.docker.com/_/alpine"},
		{"oci", "library/node", "", "https://hub.docker.com/_/node"},
		{"oci", "docker/dockerfile", "", "https://hub.docker.com/r/docker/dockerfile"},
		{"oci", "ghcr.io/acme/app", "", ""},
		{"oci", "localhost/app", "", ""},
		{"composer", "monolog/monolog", "", "https://packagist.org/packages/monolog/monolog"},
		{"cocoapods", "Firebase/Core", "", "https://cocoapods.org/pods/Firebase"},
		{"cpan", "Path-Tiny", "", "https://metacpan.org/dist/Path-Tiny"},
		{"elm", "elm/core", "", "https://package.elm-lang.org/packages/elm/core/latest/"},
		{"purescript", "prelude", "", "https://pursuit.purescript.org/packages/purescript-prelude"},
		{"purescript", "github.com/acme/purescript-forked", "", ""},
		{"dub", "vibe-d:http", "", "https://code.dlang.org/packages/vibe-d"},
		{"puppet-forge", "puppetlabs-stdlib", "", "https://forge.puppet.com/modules/puppetlabs/stdlib"},
		{"puppet-forge", "github.com/puppetlabs/puppetlabs-apache", "", ""},
		{"wally", "roblox/roact", "", "https://wally.run/package/roblox/roact"},
		{"buf", "buf.build/grpc/go", "", "https://buf.build/grpc/go"},
		{"buf", "mystery", "", ""},
		{"terraform-module", "terraform-aws-modules/iam/aws//modules/iam-role", "", "https://registry.terraform.io/modules/terraform-aws-modules/iam/aws"},
		{"terraform-module", "app.terraform.io/acme/db/aws", "", ""},
		{"terraform-provider", "hashicorp/aws", "", "https://registry.terraform.io/providers/hashicorp/aws"},
		{"terraform-provider", "tf.corp.test/acme/acme", "", ""},
		{"luarocks", "lua-cjson", "", "https://luarocks.org/search?q=lua-cjson"},
		{"nixpkgs", "python3Packages.requests", "", "https://search.nixos.org/packages?query=python3Packages.requests"},
		{"python-std", "os.path", "", "https://docs.python.org/3/library/os.html"},
		{"node", "node:fs/promises", "", "https://nodejs.org/api/fs.html"},
		{"rust-std", "alloc", "", "https://doc.rust-lang.org/alloc/"},
		{"npm", "with space", "", ""},
		{"npm", "a?b", "", ""},
		{"pypi", "", "", ""},
		{"c-external", "boost", "", ""},
		{"gitlab-ci", "infra/pipelines", "", ""},
	} {
		if got := Page(c.ecosystem, c.name, c.index); got != c.want {
			t.Errorf("Page(%q, %q, %q) = %q, want %q", c.ecosystem, c.name, c.index, got, c.want)
		}
	}
}

// Implements: REQ-MOD-014
func TestRepository(t *testing.T) {
	for _, c := range []struct{ ecosystem, name, index, origin, git, want string }{
		{"nix", "nixpkgs", "", "", "https://github.com/NixOS/nixpkgs#bfb7a882", "https://github.com/NixOS/nixpkgs/tree/bfb7a882"},
		{"go", "x", "", "", "https://gitlab.com/group/sub/repo.git#abc", "https://gitlab.com/group/sub/repo/-/tree/abc"},
		{"go", "x", "", "", "https://codeberg.org/a/b#abc", "https://codeberg.org/a/b/src/commit/abc"},
		{"go", "x", "", "", "https://bitbucket.org/a/b#abc", "https://bitbucket.org/a/b/src/abc"},
		{"go", "x", "", "", "https://git.acme.corp/a/b.git#abc", "https://git.acme.corp/a/b"},
		{"go", "x", "", "", "https://git.acme.corp/a/b.git", "https://git.acme.corp/a/b"},
		{"cocoapods", "Lottie", "", "https://github.com/airbnb/lottie-ios.git", "", "https://github.com/airbnb/lottie-ios"},
		{"pypi", "x", "", "git+https://github.com/acme/x.git@v1.2#egg=x", "", "https://github.com/acme/x"},
		{"pypi", "x", "", "git+ssh://git@github.com/acme/x.git", "", "https://github.com/acme/x"},
		{"jsonnet-bundler", "git.acme.internal/ops/libs", "", "git@git.acme.internal:ops/libs.git", "", "https://git.acme.internal/ops/libs"},
		{"terraform-module", "github.com/acme/infra-modules//app", "", "git::git@github.com:acme/infra-modules.git//app", "", "https://github.com/acme/infra-modules"},
		{"bazel", "googletest", "", "https://github.com/google/googletest/archive/refs/tags/v1.15.0.tar.gz", "", "https://github.com/google/googletest"},
		{"pypi", "x", "", "https://files.example.com/pkgs/x-1.0.tar.gz", "", ""},
		{"pypi", "x", "", "./vendor/x", "", ""},
		{"pypi", "x", "", "/opt/src/x", "", ""},
		{"pypi", "x", "", "file:///opt/src/x", "", ""},
		{"swiftpm", "github.com/apple/swift-nio", "", "", "", "https://github.com/apple/swift-nio"},
		{"go", "github.com/spf13/cobra/v2", "", "", "", "https://github.com/spf13/cobra"},
		{"jsonnet-bundler", "github.com/grafana/jsonnet-libs/ksonnet-util", "", "", "", "https://github.com/grafana/jsonnet-libs"},
		{"zig", "codeberg.org/natecraddock/zf", "", "", "", "https://codeberg.org/natecraddock/zf"},
		{"go", "golang.org/x/sys", "", "", "", ""},
		{"go", "github.com/acme", "", "", "", ""},
		{"actions", "actions/checkout", "https://api.github.com", "", "", "https://github.com/actions/checkout"},
		{"actions", "octo-org/shared/.github/workflows/ci.yml", "https://ghe.acme.corp/api/v3", "", "", "https://ghe.acme.corp/octo-org/shared"},
		{"actions", "lonely", "", "", "", ""},
		{"elm", "elm/core", "", "", "", "https://github.com/elm/core"},
		{"npm", "lodash", "", "", "", ""},
	} {
		if got := Repository(c.ecosystem, c.name, c.index, c.origin, c.git); got != c.want {
			t.Errorf("Repository(%q, %q, %q, %q, %q) = %q, want %q", c.ecosystem, c.name, c.index, c.origin, c.git, got, c.want)
		}
	}
}
