package scope

import (
	"reflect"
	"sort"
	"testing"
)

// Verifies: REQ-SUP-034
func TestMatchesTheWayGoprivateDoes(t *testing.T) {
	p := New([]string{"corp.example/*", "@acme/*", "com.acme.*"})
	for name, want := range map[string]bool{
		// A pattern matches leading path elements, so it covers what is under it.
		"corp.example/team/billing": true,
		"corp.example/lib":          true,
		"corp.example":              false, // one element short of the pattern
		"notcorp.example/team":      false,
		// A scope, a group: the same rule with no slashes to speak of.
		"@acme/widgets":       true,
		"@acmeish/widgets":    false,
		"com.acme.billing":    true,
		"com.acmeish.billing": false,
		// And the world stays public.
		"github.com/spf13/cobra": false,
		"react":                  false,
	} {
		if got := p.Match("go", name); got != want {
			t.Errorf("Match(%q) = %v, want %v", name, got, want)
		}
	}
}

// Verifies: REQ-SUP-035
func TestAPatternMayNameItsEcosystem(t *testing.T) {
	p := New([]string{"npm:@acme/*", "oci:harbor.corp/*"})
	if !p.Match("npm", "@acme/widgets") {
		t.Error("the npm pattern did not match an npm package")
	}
	// The same name in another ecosystem is somebody else's.
	if p.Match("pypi", "@acme/widgets") {
		t.Error("an npm-scoped pattern matched a PyPI package")
	}
	if !p.Match("oci", "harbor.corp/team/app") {
		t.Error("the oci pattern did not match an image")
	}
	// The PowerShell Gallery's id is psgallery; a gallery pattern keeps to it.
	p = New([]string{"psgallery:Acme.*"})
	if !p.Match("psgallery", "Acme.Tools") {
		t.Error("the psgallery pattern did not match a gallery module")
	}
	if p.Match("nuget", "Acme.Tools") {
		t.Error("a psgallery-scoped pattern matched a NuGet package")
	}
	// So is the island of C and C++ libraries no manifest declares.
	p = New([]string{"c-external:acme*"})
	if !p.Match("c-external", "acmecore") || p.Match("npm", "acmecore") {
		t.Error("a c-external-scoped pattern did not keep to that ecosystem")
	}
	p = New([]string{"vcpkg:acme*", "conan:corp-*"})
	if !p.Match("vcpkg", "acme-core") || !p.Match("conan", "corp-net") || p.Match("conan", "acme-core") {
		t.Error("vcpkg- and conan-scoped patterns did not keep to their ecosystems")
	}
	// Composer packages are vendor/name: the vendor is the leading element.
	p = New([]string{"composer:acme/*"})
	if !p.Match("composer", "acme/billing") || p.Match("npm", "acme/billing") || p.Match("composer", "acmex/billing") {
		t.Error("a composer-scoped pattern did not keep to its ecosystem and vendor")
	}
	p = New([]string{"rubygems:acme-*"})
	if !p.Match("rubygems", "acme-auth") || p.Match("npm", "acme-auth") || p.Match("rubygems", "rails") {
		t.Error("a rubygems-scoped pattern did not keep to its ecosystem")
	}
	p = New([]string{"swiftpm:github.com/acme/*"})
	if !p.Match("swiftpm", "github.com/acme/private-kit") || p.Match("swiftpm", "github.com/apple/swift-nio") || p.Match("npm", "github.com/acme/private-kit") {
		t.Error("a swiftpm-scoped pattern did not keep to its ecosystem and owner")
	}
	p = New([]string{"cran:acme*", "bioconductor:AcmeBio*"})
	if !p.Match("cran", "acmetools") || p.Match("pypi", "acmetools") || p.Match("cran", "dplyr") ||
		!p.Match("bioconductor", "AcmeBioSeq") || p.Match("cran", "AcmeBioSeq") {
		t.Error("cran- and bioconductor-scoped patterns did not keep to their ecosystems")
	}
	p = New([]string{"hackage:acme-*"})
	if !p.Match("hackage", "acme-json") || p.Match("npm", "acme-json") || p.Match("hackage", "aeson") {
		t.Error("a hackage-scoped pattern did not keep to its ecosystem")
	}
	p = New([]string{"terraform-module:acme/*", "terraform-provider:acme/*"})
	if !p.Match("terraform-module", "acme/vpc/aws") || !p.Match("terraform-provider", "acme/cloud") ||
		p.Match("npm", "acme/vpc") || p.Match("terraform-module", "terraform-aws-modules/vpc/aws") {
		t.Error("terraform-module- and terraform-provider-scoped patterns did not keep to their ecosystems")
	}
	p = New([]string{"buf:buf.build/acme/*"})
	if !p.Match("buf", "buf.build/acme/payments") || p.Match("npm", "buf.build/acme/payments") ||
		p.Match("buf", "buf.build/googleapis/googleapis") {
		t.Error("a buf-scoped pattern did not keep to its ecosystem")
	}
	p = New([]string{"hex:acme_*"})
	if !p.Match("hex", "acme_auth") || p.Match("pub", "acme_auth") || p.Match("hex", "plug") {
		t.Error("a hex-scoped pattern did not keep to its ecosystem")
	}
	p = New([]string{"pub:acme_*"})
	if !p.Match("pub", "acme_auth") || p.Match("npm", "acme_auth") || p.Match("pub", "http") {
		t.Error("a pub-scoped pattern did not keep to its ecosystem")
	}
	p = New([]string{"cocoapods:Acme*", "carthage:github.com/acme/*"})
	if !p.Match("cocoapods", "AcmeKit") || p.Match("npm", "AcmeKit") || p.Match("cocoapods", "AFNetworking") ||
		!p.Match("carthage", "github.com/acme/Net") || p.Match("swiftpm", "github.com/acme/Net") {
		t.Error("cocoapods- and carthage-scoped patterns did not keep to their ecosystems")
	}
	p = New([]string{"luarocks:acme-*", "wally:acme/*"})
	if !p.Match("luarocks", "acme-http") || p.Match("npm", "acme-http") || p.Match("luarocks", "penlight") ||
		!p.Match("wally", "acme/net") || p.Match("luarocks", "acme/net") {
		t.Error("luarocks- and wally-scoped patterns did not keep to their ecosystems")
	}
	p = New([]string{"cpan:Acme-*"})
	if !p.Match("cpan", "Acme-Widget") || p.Match("npm", "Acme-Widget") || p.Match("cpan", "Moose") {
		t.Error("a cpan-scoped pattern did not keep to its ecosystem")
	}
	p = New([]string{"opam:acme-*"})
	if !p.Match("opam", "acme-shop") || p.Match("npm", "acme-shop") || p.Match("opam", "lwt") {
		t.Error("an opam-scoped pattern did not keep to its ecosystem")
	}
	// The prefix is recognized whatever its case, and must then match as well.
	p = New([]string{"NPM:@acme/*"})
	if !p.Match("npm", "@acme/widgets") {
		t.Error("an upper-case ecosystem prefix was accepted but matched nothing")
	}
}

// Verifies: REQ-SUP-035
func TestAHostAndPortIsNotAnEcosystem(t *testing.T) {
	// "localhost:5000/*" is a registry, not the ecosystem "localhost". Cutting at the
	// colon regardless would have filed it under an ecosystem nothing ever names, and
	// the pattern would have matched nothing at all.
	p := New([]string{"localhost:5000/*"})
	if !p.Match("oci", "localhost:5000/app") {
		t.Error("a registry with a port was read as an ecosystem prefix")
	}
}

// Verifies: REQ-SUP-034
func TestOneEntryMayHoldSeveralPatterns(t *testing.T) {
	// One flag, one environment variable and one config entry all say the same thing,
	// because GOPRIVATE itself is a comma-separated list.
	p := New([]string{"corp.example/*, @acme/*", "", "  "})
	if !p.Match("go", "corp.example/x") || !p.Match("npm", "@acme/y") {
		t.Errorf("a comma-separated entry did not split: %v", p.Patterns())
	}
}

// Verifies: REQ-SUP-034
func TestNothingDeclaredMatchesNothing(t *testing.T) {
	for _, p := range []*Private{nil, New(nil), New([]string{"", ","})} {
		if !p.Empty() {
			t.Errorf("%v is not empty", p.Patterns())
		}
		if p.Match("go", "corp.example/x") {
			t.Error("an empty matcher claimed a package")
		}
	}
}

// Verifies: REQ-SUP-036
func TestReadsWhatTheMachineAlreadySaysAboutGo(t *testing.T) {
	env := map[string]string{
		"GOPRIVATE": "corp.example/*,other.example/x",
		"GONOPROXY": "corp.example/*", // usually a copy of GOPRIVATE; a repeat is harmless
		"GONOSUMDB": "none",           // means "nothing", not a module called none
	}
	got := FromGoEnv(func(k string) string { return env[k] })
	sort.Strings(got)
	want := []string{"go:corp.example/*", "go:corp.example/*", "go:other.example/x"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	// And they apply to Go and to nothing else, since that is what they were set for.
	p := New(got)
	if !p.Match("go", "corp.example/lib") {
		t.Error("GOPRIVATE did not reach the Go ecosystem")
	}
	if p.Match("npm", "corp.example/lib") {
		t.Error("GOPRIVATE reached beyond Go")
	}
}

// Julia packages are the julia ecosystem's; the standard library is not an
// ecosystem a pattern can name.
//
// Verifies: REQ-SUP-035
func TestAJuliaPatternKeepsToJulia(t *testing.T) {
	p := New([]string{"julia:Acme*"})
	if !p.Match("julia", "AcmeBilling") || p.Match("npm", "AcmeBilling") || p.Match("julia", "DataFrames") {
		t.Error("a julia-scoped pattern did not keep to its ecosystem")
	}
}

// Zig packages are named by their URL; a pattern scoped to zig keeps to them.
//
// Verifies: REQ-SUP-035
func TestAZigPatternKeepsToZig(t *testing.T) {
	p := New([]string{"zig:git.acme.example/*"})
	if !p.Match("zig", "git.acme.example/tools/zlog") || p.Match("npm", "git.acme.example/tools/zlog") || p.Match("zig", "github.com/ziglibs/known-folders") {
		t.Error("a zig-scoped pattern did not keep to its ecosystem")
	}
}

// A package on the Maven island is group:artifact (Java, Kotlin, Scala, Clojure and
// Bazel alike); a Maven pattern names its group, or one artifact.
//
// Verifies: REQ-SUP-035
func TestAMavenPatternMatchesGroupAndArtifact(t *testing.T) {
	p := New([]string{"maven:com.acme.*"})
	if !p.Match("maven", "com.acme.billing:api") || !p.Match("maven", "com.acme:billing") || p.Match("maven", "cheshire:cheshire") {
		t.Error("a Maven group pattern did not match group:artifact names")
	}
	if !New([]string{"acme"}).Match("maven", "acme:shared") {
		t.Error("a bare group did not match its artifacts")
	}
	// One artifact, as Java, Kotlin and Scala imports are named too.
	one := New([]string{"maven:com.acme:lib"})
	if !one.Match("maven", "com.acme:lib") || one.Match("maven", "com.acme:lib-extra") || one.Match("maven", "com.acme:other") {
		t.Error("an artifact pattern did not keep to its artifact")
	}
}

// Bazel modules are named by the registry, WORKSPACE downloads by their URL; a
// pattern scoped to either keeps to its island.
//
// Verifies: REQ-SUP-035
func TestABazelPatternKeepsToBazel(t *testing.T) {
	p := New([]string{"bazel:acme_*", "bazel-repo:git.acme.example/*"})
	if !p.Match("bazel", "acme_rules") || p.Match("npm", "acme_rules") || p.Match("bazel", "rules_go") ||
		!p.Match("bazel-repo", "git.acme.example/tools/zlib") || p.Match("bazel", "git.acme.example/tools/zlib") {
		t.Error("a bazel-scoped pattern did not keep to its ecosystem")
	}
}

// Flake inputs are named by their URL and nixpkgs packages by their attribute; a
// pattern scoped to either keeps to its island.
//
// Verifies: REQ-SUP-035
func TestANixPatternKeepsToNix(t *testing.T) {
	p := New([]string{"nix:git.acme.example/*", "nixpkgs:acme-*"})
	if !p.Match("nix", "git.acme.example/infra/flakes") || p.Match("zig", "git.acme.example/infra/flakes") ||
		!p.Match("nixpkgs", "acme-cli") || p.Match("nix", "acme-cli") || p.Match("nixpkgs", "hello") {
		t.Error("a nix-scoped pattern did not keep to its ecosystem")
	}
}

// Elm packages are named author/name; an elm-scoped pattern keeps to them.
//
// Verifies: REQ-SUP-035
func TestAnElmPatternKeepsToElm(t *testing.T) {
	p := New([]string{"elm:acme/*"})
	if !p.Match("elm", "acme/elm-widgets") || p.Match("npm", "acme/elm-widgets") || p.Match("elm", "elm/html") {
		t.Error("an elm-scoped pattern did not keep to its ecosystem")
	}
}

// PureScript registry packages are named as the registry names them; a
// purescript-scoped pattern keeps to them.
//
// Verifies: REQ-SUP-035
func TestAPureScriptPatternKeepsToPureScript(t *testing.T) {
	p := New([]string{"purescript:acme-*"})
	if !p.Match("purescript", "acme-widgets") || p.Match("npm", "acme-widgets") || p.Match("purescript", "halogen") {
		t.Error("a purescript-scoped pattern did not keep to its ecosystem")
	}
}

// Crystal shards are named as shard.yml names them; a shards-scoped pattern
// keeps to them.
//
// Verifies: REQ-SUP-035
func TestAShardsPatternKeepsToShards(t *testing.T) {
	p := New([]string{"shards:acme_*"})
	if !p.Match("shards", "acme_billing") || p.Match("rubygems", "acme_billing") || p.Match("shards", "kemal") {
		t.Error("a shards-scoped pattern did not keep to its ecosystem")
	}
}

// Paket's GitHub, git and HTTP dependencies are named by their URL; a
// paket-scoped pattern keeps to them.
//
// Verifies: REQ-SUP-035
func TestAPaketPatternKeepsToPaket(t *testing.T) {
	p := New([]string{"paket:github.com/acme/*"})
	if !p.Match("paket", "github.com/acme/shared") || p.Match("nuget", "github.com/acme/shared") || p.Match("paket", "github.com/fsharp/FAKE") {
		t.Error("a paket-scoped pattern did not keep to its ecosystem")
	}
}

// dub packages are named as their recipes name them; a dub-scoped pattern keeps
// to them.
//
// Verifies: REQ-SUP-035
func TestADubPatternKeepsToDub(t *testing.T) {
	p := New([]string{"dub:acme-*"})
	if !p.Match("dub", "acme-billing") || p.Match("npm", "acme-billing") || p.Match("dub", "vibe-d") {
		t.Error("a dub-scoped pattern did not keep to its ecosystem")
	}
}

// fpm packages are named by their dependency keys and modules no project
// provides by their module names; patterns scoped to fpm and fortran-external
// keep to them.
//
// Verifies: REQ-SUP-035
func TestAnFpmPatternKeepsToFpm(t *testing.T) {
	p := New([]string{"fpm:acme-*", "fortran-external:acme_*"})
	if !p.Match("fpm", "acme-solvers") || p.Match("dub", "acme-solvers") || p.Match("fpm", "stdlib") {
		t.Error("an fpm-scoped pattern did not keep to its ecosystem")
	}
	if !p.Match("fortran-external", "acme_mesh") || p.Match("fpm", "acme_mesh") {
		t.Error("a fortran-external-scoped pattern did not keep to its ecosystem")
	}
}
