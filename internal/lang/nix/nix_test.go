package nix

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// The fixture is a flake (flake.nix and a version 7 flake.lock) whose inputs use
// every reference form - github with a branch, gitlab with a tag, git+https with
// ref and rev, path:, a tarball with flake = false, an indirect registry name,
// an attribute-set reference, follows, FlakeHub, and an outputs argument no input
// declares. Around it: a callPackage'd package, a NixOS module tree, a
// development shell, a library, niv's nix/sources.json and npins' sources, and a
// file using both. examples/nolock is a flake without a lock.

var (
	nixpkgs = lang.Target{Ecosystem: ecoNix, Package: "github.com/nixos/nixpkgs", Version: "ad57eef", Requested: "nixos-24.05", Pinned: true, Git: "https://github.com/NixOS/nixpkgs#ad57eef4ef0659193044870c731987a6df5cf56b"}
	channel = lang.Target{Ecosystem: ecoNix, Package: "nixpkgs", Floating: true}
)

func pkg(attr string) lang.Target {
	return lang.Target{Ecosystem: ecoNixpkgs, Package: attr, Version: "ad57eef", Pinned: true}
}

// Verifies: REQ-NIX-004, REQ-NIX-005, REQ-NIX-006, REQ-NIX-009
func TestFlakeInputs(t *testing.T) {
	res := langtest.Analyze(t, Plugin{}, "testdata/repo")
	langtest.CheckImports(t, res["flake.nix"], map[string]lang.Target{
		"inputs.nixpkgs":      nixpkgs,
		"inputs.flake-utils":  {Ecosystem: ecoNix, Package: "github.com/numtide/flake-utils", Version: "b1d9ab7", Pinned: true, Git: "https://github.com/numtide/flake-utils#b1d9ab70662946ef0850d488da1c9019f3a9752a"},
		"inputs.home-manager": {Ecosystem: ecoNix, Package: "github.com/nix-community/home-manager", Version: "5d15142", Requested: "release-24.05", Pinned: true, Git: "https://github.com/nix-community/home-manager#5d151429e1e79107acf6d06dcc5ace4e642ec239"},
		"inputs.foo":          {Ecosystem: ecoNix, Package: "git.example.org/team/foo", Version: "0123456", Pinned: true, Git: "https://git.example.org/team/foo#0123456789abcdef0123456789abcdef01234567"},
		"inputs.bar":          {Local: "sub/flake.nix"},
		"inputs.baz":          {Ecosystem: ecoNix, Package: "example.org/downloads/baz", Version: "1.2", Pinned: true},
		"inputs.tagged":       {Ecosystem: ecoNix, Package: "gitlab.com/acme/tools", Version: "cafebab", Requested: "v1.4.0", Pinned: true, Git: "https://gitlab.com/acme/tools#cafebabecafebabecafebabecafebabecafebabe"},
		// flake:nixpkgs is the registry's alias, whatever the lock resolved it to.
		"inputs.registry": {Ecosystem: ecoNix, Package: "nixpkgs", Version: "bfb7a88", Requested: "nixos-unstable", Pinned: true, Git: "https://github.com/NixOS/nixpkgs#bfb7a882678e518398ce9a31a881538679f6f092"},
		"inputs.typed":    {Ecosystem: ecoNix, Package: "github.com/acme/typed", Version: "deadbee", Requested: "dev", Pinned: true, Git: "https://github.com/acme/typed#deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"},
		"inputs.same":     nixpkgs, // follows: the followed input, no node of its own
		"inputs.hub":      {Ecosystem: ecoNix, Package: "flakehub.com/f/NixOS/nixpkgs", Version: "9d29cd2", Requested: "0.1.*", Pinned: true},
		// An outputs argument without an input: the registry's "systems".
		"inputs.systems": {Ecosystem: ecoNix, Package: "systems", Version: "da67096", Pinned: true, Git: "https://github.com/nix-systems/default#da67096a3b9bf56a91d16901293e51ba5b49a27e"},
		"./overlays":     {Local: "overlays/default.nix"},
		"./pkgs/hello":   {Local: "pkgs/hello/default.nix"},
		"./shell.nix":    {Local: "shell.nix"},
		"./modules":      {Local: "modules/default.nix"},
		"./lib":          {Local: "lib/default.nix"},
	})
	// Without a lock, the reference alone: a branch floats, a commit or a content
	// hash pins, a tag neither.
	langtest.CheckImports(t, res["examples/nolock/flake.nix"], map[string]lang.Target{
		"inputs.nixpkgs": {Ecosystem: ecoNix, Package: "github.com/nixos/nixpkgs", Version: "nixos-24.05", Floating: true},
		"inputs.commit":  {Ecosystem: ecoNix, Package: "github.com/acme/commit", Version: "0123456", Pinned: true, Git: "https://github.com/acme/commit#0123456789abcdef0123456789abcdef01234567"},
		"inputs.tag":     {Ecosystem: ecoNix, Package: "github.com/acme/tag", Version: "v2.0.1"},
		"inputs.hashed":  {Ecosystem: ecoNix, Package: "example.org/hashed", Version: "sha256-AAAAAAAAAAAA", Pinned: true},
		"inputs.ssh":     {Ecosystem: ecoNix, Package: "github.com/acme/private", Version: "main", Floating: true},
		"inputs.local":   {Local: "sub/flake.nix"},
		"inputs.alias":   {Ecosystem: ecoNix, Package: "github.com/nixos/nixpkgs", Version: "nixos-24.05", Floating: true},
		"inputs.nested":  {}, // an input of an input: only a lock knows it
		"inputs.srht":    {Ecosystem: ecoNix, Package: "git.sr.ht/~user/repo", Floating: true},
		"inputs.utils":   {Ecosystem: ecoNix, Package: "utils", Floating: true},
		"./shell.nix":    {Local: "examples/nolock/shell.nix"},
	})
	// Its packages take the unlocked nixpkgs' branch: they float.
	floating := func(attr string) lang.Target {
		return lang.Target{Ecosystem: ecoNixpkgs, Package: attr, Version: "nixos-24.05", Floating: true}
	}
	langtest.CheckImports(t, res["examples/nolock/shell.nix"], map[string]lang.Target{
		"requests":                               floating("python3Packages.requests"),
		"pkgs.jq":                                floating("jq"),
		"pkgs.legacyPackages.x86_64-linux.hello": floating("hello"),
	})
}

// Verifies: REQ-NIX-005, REQ-FND-026
func TestFlakeLock(t *testing.T) {
	res := langtest.Analyze(t, Plugin{}, "testdata/repo")
	langtest.CheckImports(t, res["flake.lock"], map[string]lang.Target{
		"bar":          {Local: "sub/flake.nix"},
		"baz":          {Ecosystem: ecoNix, Package: "example.org/downloads/baz", Version: "1.2", Pinned: true},
		"flake-utils":  {Ecosystem: ecoNix, Package: "github.com/numtide/flake-utils", Version: "b1d9ab7", Pinned: true, Git: "https://github.com/numtide/flake-utils#b1d9ab70662946ef0850d488da1c9019f3a9752a"},
		"foo":          {Ecosystem: ecoNix, Package: "git.example.org/team/foo", Version: "0123456", Pinned: true, Git: "https://git.example.org/team/foo#0123456789abcdef0123456789abcdef01234567"},
		"home-manager": {Ecosystem: ecoNix, Package: "github.com/nix-community/home-manager", Version: "5d15142", Requested: "release-24.05", Pinned: true, Git: "https://github.com/nix-community/home-manager#5d151429e1e79107acf6d06dcc5ace4e642ec239"},
		"hub":          {Ecosystem: ecoNix, Package: "flakehub.com/f/NixOS/nixpkgs", Version: "9d29cd2", Requested: "0.1.*", Pinned: true},
		"nixpkgs":      nixpkgs,
		"registry":     {Ecosystem: ecoNix, Package: "nixpkgs", Version: "bfb7a88", Requested: "nixos-unstable", Pinned: true, Git: "https://github.com/NixOS/nixpkgs#bfb7a882678e518398ce9a31a881538679f6f092"},
		"systems":      {Ecosystem: ecoNix, Package: "github.com/nix-systems/default", Version: "da67096", Pinned: true, Git: "https://github.com/nix-systems/default#da67096a3b9bf56a91d16901293e51ba5b49a27e"},
		"systems_2":    {Ecosystem: ecoNix, Package: "systems", Version: "da67096", Pinned: true, Git: "https://github.com/nix-systems/default#da67096a3b9bf56a91d16901293e51ba5b49a27e"},
		"tagged":       {Ecosystem: ecoNix, Package: "gitlab.com/acme/tools", Version: "cafebab", Requested: "v1.4.0", Pinned: true, Git: "https://gitlab.com/acme/tools#cafebabecafebabecafebabecafebabecafebabe"},
		"typed":        {Ecosystem: ecoNix, Package: "github.com/acme/typed", Version: "deadbee", Requested: "dev", Pinned: true, Git: "https://github.com/acme/typed#deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"},
	})
	if got := res["flake.lock"].Imports[0].Line; got != 3 {
		t.Errorf("bar's line: got %d, want 3", got)
	}

	// --resolve-depth: each node's inputs, follows taken to the node they name.
	r := newResolver(langtest.Files(t, "testdata/repo"))
	for name, want := range map[string][]lang.Target{
		"flake-utils":  {{Ecosystem: ecoNix, Package: "github.com/nix-systems/default", Version: "da67096", Pinned: true, Git: "https://github.com/nix-systems/default#da67096a3b9bf56a91d16901293e51ba5b49a27e"}},
		"home-manager": {nixpkgs},
		"foo":          {nixpkgs},
		"nixpkgs":      nil,
	} {
		t0 := langtest.Imports(t, res["flake.lock"])[name]
		if got := r.Dependencies(t0); !reflect.DeepEqual(got, want) {
			t.Errorf("%s depends on %+v, want %+v", name, got, want)
		}
	}
}

// Verifies: REQ-NIX-002, REQ-NIX-007, REQ-NIX-009
func TestImportsAndPaths(t *testing.T) {
	res := langtest.Analyze(t, Plugin{}, "testdata/repo")
	langtest.CheckImports(t, res["default.nix"], map[string]lang.Target{
		"<nixpkgs>":    channel, // <nixpkgs/nixos> is the same channel
		"./pkgs/hello": {Local: "pkgs/hello/default.nix"},
		"./shell.nix":  {Local: "shell.nix"},
		"./lib":        {Local: "lib/default.nix"},
	})
	// A NixOS module: imports (a directory is its default.nix), systemPackages
	// under with pkgs, home.packages, a flake input through inputs, readFile.
	langtest.CheckImports(t, res["modules/default.nix"], map[string]lang.Target{
		"./services.nix":     {Local: "modules/services.nix"},
		"./users":            {Local: "modules/users/default.nix"},
		"../lib/options.nix": {Local: "lib/options.nix"},
		"vim":                pkg("vim"),
		"git":                pkg("git"),
		"pkgs.htop":          pkg("htop"),
		"inputs.foo":         {Ecosystem: ecoNix, Package: "git.example.org/team/foo", Version: "0123456", Pinned: true, Git: "https://git.example.org/team/foo#0123456789abcdef0123456789abcdef01234567"},
		"../lib/VERSION":     {Local: "lib/VERSION"},
	})
	// callPackage's formals are packages; a let-bound derivation and a function's
	// result are not. Paths in an indented string's ${} are read, the text and
	// ''${ } escapes are not.
	langtest.CheckImports(t, res["pkgs/hello/default.nix"], map[string]lang.Target{
		"./src":                      {Local: "pkgs/hello/src"},
		"./fix.patch":                {Local: "pkgs/hello/fix.patch"},
		"openssl":                    pkg("openssl"),
		"zlib":                       pkg("zlib"),
		"python3Packages.setuptools": pkg("python3Packages.setuptools"),
		"./src/hello.c":              {Local: "pkgs/hello/src/hello.c"},
	})
	langtest.CheckImports(t, res["shell.nix"], map[string]lang.Target{
		"<nixpkgs>":        channel,
		"pkgs.git":         pkg("git"),
		"pkgs.nodejs_20":   pkg("nodejs_20"),
		"openssl":          pkg("openssl"),
		"zlib":             pkg("zlib"),
		"pkgs.systemd":     pkg("systemd"), // lib.optionals cond [ ... ]
		"pkgs.cmake":       pkg("cmake"),
		"pkgs.python3":     pkg("python3"), // python3.withPackages (ps: [ ... ])
		"./scripts/env.sh": {Local: "scripts/env.sh"},
	})
	langtest.CheckImports(t, res["lib/default.nix"], map[string]lang.Target{"./VERSION": {Local: "lib/VERSION"}})
	langtest.CheckImports(t, res["overlays/default.nix"], map[string]lang.Target{})
}

// Verifies: REQ-NIX-008, REQ-NIX-006, REQ-NIX-010
func TestPins(t *testing.T) {
	res := langtest.Analyze(t, Plugin{}, "testdata/repo")
	hm := lang.Target{Ecosystem: ecoNix, Package: "github.com/nix-community/home-manager", Version: "a1b2c3d", Requested: "release-24.05", Pinned: true, Git: "https://github.com/nix-community/home-manager#a1b2c3d4e5f60718293a4b5c6d7e8f9012345678"}
	nivNixpkgs := lang.Target{Ecosystem: ecoNix, Package: "github.com/nixos/nixpkgs", Version: "fedcba9", Requested: "nixos-23.11", Pinned: true, Git: "https://github.com/NixOS/nixpkgs#fedcba9876543210fedcba9876543210fedcba98"}
	release := lang.Target{Ecosystem: ecoNix, Package: "nixpkgs", Version: "nixos-24.05.1234.abcdef0", Requested: "nixos-24.05", Pinned: true}
	langtest.CheckImports(t, res["nix/sources.json"], map[string]lang.Target{
		"home-manager": hm,
		"nixpkgs":      nivNixpkgs,
		"tool":         {Ecosystem: ecoNix, Package: "downloads.example.org/tool", Version: "2.0", Pinned: true},
	})
	langtest.CheckImports(t, res["npins/sources.json"], map[string]lang.Target{
		"nixpkgs": release,
		"lix":     {Ecosystem: ecoNix, Package: "git.lix.systems/lix-project/lix", Version: "9876543", Requested: "main", Pinned: true, Git: "https://git.lix.systems/lix-project/lix.git#9876543210abcdef9876543210abcdef98765432"},
		"treefmt": {Ecosystem: ecoNix, Package: "github.com/numtide/treefmt-nix", Version: "1111111", Requested: "v2.1.0", Pinned: true, Git: "https://github.com/numtide/treefmt-nix#1111111111222222222233333333334444444444"},
	})
	// sources.x and pins.x through the loaders' bindings, and the fetchers.
	langtest.CheckImports(t, res["legacy.nix"], map[string]lang.Target{
		"./nix/sources.nix":    {Local: "nix/sources.nix"},
		"sources.nixpkgs":      nivNixpkgs,
		"./npins":              {Local: "npins/default.nix"},
		"sources.home-manager": hm,
		"pins.nixpkgs":         release,
		"fetchTarball https://github.com/NixOS/nixpkgs/archive/0123456789abcdef0123456789abcdef01234567.tar.gz": {Ecosystem: ecoNix, Package: "github.com/nixos/nixpkgs", Version: "0123456", Pinned: true},
		"fetchGit https://github.com/acme/tool.git":                                                             {Ecosystem: ecoNix, Package: "github.com/acme/tool", Version: "main", Floating: true},
		"fetchTarball https://nixos.org/channels/nixos-24.05/nixexprs.tar.xz":                                   {Ecosystem: ecoNix, Package: "nixpkgs", Version: "nixos-24.05", Floating: true},
		"getFlake github:numtide/flake-utils/v1.0.0":                                                            {Ecosystem: ecoNix, Package: "github.com/numtide/flake-utils", Version: "v1.0.0"},
	})
}

// Verifies: REQ-NIX-003
func TestSymbols(t *testing.T) {
	res := langtest.Analyze(t, Plugin{}, "testdata/repo")
	langtest.CheckSymbols(t, res["flake.nix"], map[string]string{
		"packages": "output", "packages.default": "output", "packages.tool": "output",
		"devShells": "output", "devShells.default": "output",
		"nixosModules": "output", "nixosModules.default": "output",
		"overlays": "output", "overlays.default": "output",
		"nixosConfigurations": "output", "nixosConfigurations.box": "output",
		"lib": "output",
	})
	langtest.CheckSymbols(t, res["lib/default.nix"], map[string]string{
		"helper":        "function",
		"version":       "var",
		"mkThing":       "function",
		"strings.upper": "function",
		"quoted":        "attr",
	})
	langtest.CheckSymbols(t, res["modules/default.nix"], map[string]string{
		"imports": "attr", "environment.systemPackages": "attr", "home.packages": "attr",
		"services.foo": "attr", "users.motd": "attr",
	})
	langtest.CheckSymbols(t, res["overlays/default.nix"], map[string]string{"hello-wrapped": "attr"})
	langtest.CheckSymbols(t, res["pkgs/hello/default.nix"], map[string]string{"local": "var"})
	langtest.CheckSymbols(t, res["flake.lock"], map[string]string{})
}

// Inside nixpkgs, the package lists name nixpkgs' own packages: a pkgs/by-name
// package is its package.nix, anything else is dropped.
//
// Verifies: REQ-NIX-007
func TestInsideNixpkgs(t *testing.T) {
	root := t.TempDir()
	for p, src := range map[string]string{
		"pkgs/top-level/all-packages.nix":     "{ }\n",
		"pkgs/by-name/he/hello/package.nix":   "{ stdenv }: stdenv.mkDerivation { pname = \"hello\"; }\n",
		"pkgs/tools/misc/greet/default.nix":   "{ stdenv, hello, openssl }: stdenv.mkDerivation { buildInputs = [ hello openssl ]; }\n",
		"pkgs/development/libraries/x/foo.md": "",
	} {
		if err := os.MkdirAll(filepath.Join(root, filepath.Dir(p)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, p), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	res := langtest.Analyze(t, Plugin{}, root)
	langtest.CheckImports(t, res["pkgs/tools/misc/greet/default.nix"], map[string]lang.Target{
		"hello":   {Local: "pkgs/by-name/he/hello/package.nix"},
		"openssl": {},
	})
}

// Nix's result symlinks (nix build's output links) are not files of the project.
//
// Verifies: REQ-NIX-001
func TestResultLinksSkipped(t *testing.T) {
	root, out := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(out, "store.nix"), []byte("{ }"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "default.nix"), []byte("import ./result"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, link := range []string{"result", "result-dev"} {
		if err := os.Symlink(out, filepath.Join(root, link)); err != nil {
			t.Skip("symlinks unavailable:", err)
		}
	}
	res := langtest.Analyze(t, Plugin{}, root)
	if len(res) != 1 || res["default.nix"] == nil {
		t.Errorf("analyzed %v, want default.nix alone", keys(res))
	}
	langtest.CheckImports(t, res["default.nix"], map[string]lang.Target{"./result": {}})
}

func keys[V any](m map[string]V) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}

// Verifies: REQ-NIX-001
func TestClaims(t *testing.T) {
	for p, want := range map[string]string{
		"flake.nix": "flake", "x/flake.lock": "lock", "nix/sources.json": "niv", "npins/sources.json": "npins",
		"default.nix": "", "sources.json": "", "other/sources.json": "",
	} {
		f := &scan.File{Path: p}
		if got := (Plugin{}).Class(f); got != want {
			t.Errorf("%s: class %q, want %q", p, got, want)
		}
		if claims := (Plugin{}).Claims(f); claims != (want != "" || strings.HasSuffix(p, ".nix")) {
			t.Errorf("%s: claimed %v", p, claims)
		}
	}
}

// Verifies: REQ-NIX-004, REQ-NIX-006
func TestReferences(t *testing.T) {
	for s, want := range map[string]lang.Target{
		"github:NixOS/nixpkgs":                                    {Ecosystem: ecoNix, Package: "github.com/nixos/nixpkgs", Floating: true},
		"github:NixOS/nixpkgs?ref=nixos-24.05":                    {Ecosystem: ecoNix, Package: "github.com/nixos/nixpkgs", Version: "nixos-24.05", Floating: true},
		"github:o/r?rev=0123456789abcdef0123456789abcdef01234567": {Ecosystem: ecoNix, Package: "github.com/o/r", Version: "0123456", Pinned: true, Git: "https://github.com/o/r#0123456789abcdef0123456789abcdef01234567"},
		"github:o/r/v1.2.3":                                       {Ecosystem: ecoNix, Package: "github.com/o/r", Version: "v1.2.3"},
		"github:o/r?host=github.example.com":                      {Ecosystem: ecoNix, Package: "github.example.com/o/r", Floating: true},
		"gitlab:group%2Fsub/repo":                                 {Ecosystem: ecoNix, Package: "gitlab.com/group/sub/repo", Floating: true},
		"git+https://example.org/r.git?ref=v1.0":                  {Ecosystem: ecoNix, Package: "example.org/r", Version: "v1.0"},
		"git+https://example.org/r?ref=refs/tags/rel":             {Ecosystem: ecoNix, Package: "example.org/r", Version: "rel"},
		"hg+https://example.org/hg/r":                             {Ecosystem: ecoNix, Package: "example.org/hg/r", Floating: true},
		"https://github.com/o/r/archive/refs/heads/main.tar.gz":   {Ecosystem: ecoNix, Package: "github.com/o/r", Version: "main", Floating: true},
		"https://gitlab.com/o/r/-/archive/v2/r-v2.tar.gz":         {Ecosystem: ecoNix, Package: "gitlab.com/o/r", Version: "v2"},
		"https://api.github.com/repos/o/r/tarball/v3":             {Ecosystem: ecoNix, Package: "github.com/o/r", Version: "v3"},
		"https://channels.nixos.org/nixos-24.05/nixexprs.tar.xz":  {Ecosystem: ecoNix, Package: "nixpkgs", Version: "nixos-24.05", Floating: true},
		"tarball+https://example.org/x/latest":                    {Ecosystem: ecoNix, Package: "example.org/x/latest", Floating: true},
		"file+https://example.org/data.json":                      {Ecosystem: ecoNix, Package: "example.org/data.json", Floating: true},
		"nixpkgs":                                                 {Ecosystem: ecoNix, Package: "nixpkgs", Floating: true},
		"flake:nixpkgs/nixos-24.05":                               {Ecosystem: ecoNix, Package: "nixpkgs", Version: "nixos-24.05", Floating: true},
		"@nixpkgs@":                                               {},
		"path:./x":                                                {},
		"git+file:///home/me/src":                                 {},
	} {
		got, _ := parseRef(s).target()
		if got != want {
			t.Errorf("%s: got %+v, want %+v", s, got, want)
		}
	}
}

// Verifies: REQ-NIX-011
func TestLexer(t *testing.T) {
	src := "{ a = \"x ${./one.nix} \\${./no.nix} $${./no2.nix}\";\n" +
		"  b = ''\n    ''${./no3.nix} ''' ''$ ${./two.nix}\n  '';\n" +
		"  c = ./dir/${name}.nix; d = <nixpkgs/lib>; e = https://example.org/x?y=1;\n" +
		"  # import ./comment.nix\n  /* import ./block.nix */ f = a/b; g = 6 / 2; h = x: x; }\n"
	ex := extract([]byte(src), false)
	var specs []string
	for _, im := range ex.Imports {
		specs = append(specs, im.Spec)
	}
	want := []string{"./one.nix", "./two.nix", "<nixpkgs/lib>", "a/b"}
	if !reflect.DeepEqual(specs, want) {
		t.Errorf("imports %v, want %v", specs, want)
	}
	tokens := lex([]byte("''a ''' b ''$c ''\\n''"))
	var text string
	for _, tk := range tokens {
		if tk.kind == tStrText {
			text += tk.text
		}
	}
	if text != "a '' b $c n" {
		t.Errorf("indented string text %q", text)
	}
}

// Every prefix of every fixture file, and runs of each construct, extract without
// a panic and in time linear in their size.
//
// Verifies: REQ-NIX-011
func TestTruncated(t *testing.T) {
	var files []string
	filepath.Walk("testdata", func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			files = append(files, p)
		}
		return nil
	})
	for _, p := range files {
		src, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		f := &scan.File{Path: strings.TrimPrefix(filepath.ToSlash(p), "testdata/repo/")}
		for i := 0; i <= len(src); i++ {
			if _, err := (Plugin{}).Extract(f, src[:i]); err != nil {
				t.Fatal(err)
			}
			extract(src[:i], true)
		}
	}
	for _, unit := range []string{"{", "[", "(", "${", "\"${", "''${", "let ", "a.", "a.b.", "x: ", "{ a, ", "with a; ",
		"a/", "./a/${", "<a/", "https:", "rec {", "inherit (", "if a then ", "a ++ ", "import ./x ", "a = ",
		"}", "]", ")", "''", "\"", "#", "/*", "{ a = [ ", "assert a; ", "a or ", "-", "!"} {
		src := []byte(strings.Repeat(unit, 200_000/len(unit)+1))
		start := time.Now()
		extract(src, false)
		extract(src, true)
		if d := time.Since(start); d > 5*time.Second {
			t.Errorf("%q x %d: %v", unit, len(src)/len(unit), d)
		}
	}
}
