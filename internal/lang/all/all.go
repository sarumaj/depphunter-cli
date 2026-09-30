// Package all lists every language plugin the command runs, in the order it runs
// them.
package all

import (
	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/ada"
	"github.com/sarumaj/depphunter-cli/internal/lang/bazel"
	"github.com/sarumaj/depphunter-cli/internal/lang/beam"
	"github.com/sarumaj/depphunter-cli/internal/lang/ci"
	"github.com/sarumaj/depphunter-cli/internal/lang/clojure"
	"github.com/sarumaj/depphunter-cli/internal/lang/cmake"
	"github.com/sarumaj/depphunter-cli/internal/lang/commonlisp"
	"github.com/sarumaj/depphunter-cli/internal/lang/cpp"
	"github.com/sarumaj/depphunter-cli/internal/lang/crystal"
	"github.com/sarumaj/depphunter-cli/internal/lang/csharp"
	"github.com/sarumaj/depphunter-cli/internal/lang/cue"
	dlang "github.com/sarumaj/depphunter-cli/internal/lang/d"
	"github.com/sarumaj/depphunter-cli/internal/lang/dart"
	"github.com/sarumaj/depphunter-cli/internal/lang/dhall"
	"github.com/sarumaj/depphunter-cli/internal/lang/docker"
	"github.com/sarumaj/depphunter-cli/internal/lang/elm"
	"github.com/sarumaj/depphunter-cli/internal/lang/fortran"
	"github.com/sarumaj/depphunter-cli/internal/lang/fsharp"
	"github.com/sarumaj/depphunter-cli/internal/lang/gleam"
	"github.com/sarumaj/depphunter-cli/internal/lang/golang"
	"github.com/sarumaj/depphunter-cli/internal/lang/haskell"
	"github.com/sarumaj/depphunter-cli/internal/lang/haxe"
	"github.com/sarumaj/depphunter-cli/internal/lang/java"
	"github.com/sarumaj/depphunter-cli/internal/lang/javascript"
	"github.com/sarumaj/depphunter-cli/internal/lang/jsonnet"
	"github.com/sarumaj/depphunter-cli/internal/lang/julia"
	"github.com/sarumaj/depphunter-cli/internal/lang/kotlin"
	"github.com/sarumaj/depphunter-cli/internal/lang/lua"
	"github.com/sarumaj/depphunter-cli/internal/lang/markdown"
	"github.com/sarumaj/depphunter-cli/internal/lang/nim"
	"github.com/sarumaj/depphunter-cli/internal/lang/nix"
	"github.com/sarumaj/depphunter-cli/internal/lang/objc"
	"github.com/sarumaj/depphunter-cli/internal/lang/ocaml"
	"github.com/sarumaj/depphunter-cli/internal/lang/perl"
	"github.com/sarumaj/depphunter-cli/internal/lang/php"
	"github.com/sarumaj/depphunter-cli/internal/lang/powershell"
	"github.com/sarumaj/depphunter-cli/internal/lang/proto"
	"github.com/sarumaj/depphunter-cli/internal/lang/puppet"
	"github.com/sarumaj/depphunter-cli/internal/lang/purescript"
	"github.com/sarumaj/depphunter-cli/internal/lang/python"
	"github.com/sarumaj/depphunter-cli/internal/lang/r"
	"github.com/sarumaj/depphunter-cli/internal/lang/racket"
	"github.com/sarumaj/depphunter-cli/internal/lang/rego"
	"github.com/sarumaj/depphunter-cli/internal/lang/ruby"
	"github.com/sarumaj/depphunter-cli/internal/lang/rust"
	"github.com/sarumaj/depphunter-cli/internal/lang/scala"
	"github.com/sarumaj/depphunter-cli/internal/lang/shader"
	"github.com/sarumaj/depphunter-cli/internal/lang/shell"
	"github.com/sarumaj/depphunter-cli/internal/lang/solidity"
	"github.com/sarumaj/depphunter-cli/internal/lang/swift"
	"github.com/sarumaj/depphunter-cli/internal/lang/terraform"
	"github.com/sarumaj/depphunter-cli/internal/lang/zig"
)

// Options are the settings of the plugins that take any.
type Options struct {
	// Python is a Python interpreter's path (--python), for the Python plugin.
	Python string
	// Getenv reads the process environment, for the Python plugin's activated
	// VIRTUAL_ENV and PYTHONPATH.
	Getenv func(string) string
}

// Plugins returns every language plugin. The order is part of the result: the
// analysis runs the plugins one after another in it, so it decides the order of the
// graph's nodes and edges.
func Plugins(options Options) []lang.Plugin {
	return []lang.Plugin{
		golang.Plugin{},
		javascript.Plugin{},
		python.Plugin{Interpreter: options.Python, Getenv: options.Getenv},
		rust.Plugin{},
		java.Plugin{},
		kotlin.Plugin{},
		scala.Plugin{},
		csharp.Plugin{},
		fsharp.Plugin{},
		cpp.Plugin{Fetches: cmake.Plugin{}},
		cmake.Plugin{},
		php.Plugin{},
		ruby.Plugin{},
		swift.Plugin{},
		objc.Plugin{},
		dart.Plugin{},
		beam.Plugin{},
		r.Plugin{},
		haskell.Plugin{},
		lua.Plugin{},
		perl.Plugin{},
		ocaml.Plugin{},
		julia.Plugin{},
		zig.Plugin{},
		clojure.Plugin{},
		bazel.Plugin{},
		nix.Plugin{},
		gleam.Plugin{},
		elm.Plugin{},
		purescript.Plugin{},
		crystal.Plugin{},
		dlang.Plugin{},
		fortran.Plugin{},
		haxe.Plugin{},
		ada.Plugin{},
		racket.Plugin{},
		commonlisp.Plugin{},
		solidity.Plugin{},
		nim.Plugin{},
		jsonnet.Plugin{},
		cue.Plugin{},
		dhall.Plugin{},
		puppet.Plugin{},
		rego.Plugin{},
		shader.Plugin{},
		powershell.Plugin{},
		ci.Plugin{},
		docker.Plugin{},
		terraform.Plugin{},
		proto.Plugin{},
		shell.Plugin{},
		markdown.Plugin{},
	}
}
