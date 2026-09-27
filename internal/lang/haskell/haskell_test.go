package haskell

import (
	"reflect"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// A cabal project of two packages (shop-core: common stanza, custom-setup, a test
// suite, an .hs-boot file, a Happy parser, PackageImports; shop-app depending on
// it) with cabal.project constraints and source-repository-package stanzas,
// cabal.project.freeze and a build plan in dist-newstyle, literate modules outside
// any package (bird tracks, \begin{code}), and a stack project with hpack's
// package.yaml, git extra-deps and stack.yaml.lock.
//
// Verifies: REQ-HASKELL-001, REQ-HASKELL-002, REQ-HASKELL-003, REQ-HASKELL-004, REQ-HASKELL-005, REQ-HASKELL-006
// Verifies: REQ-HASKELL-007, REQ-HASKELL-008, REQ-HASKELL-009, REQ-HASKELL-010
func TestCabalAndStackProjects(t *testing.T) {
	res := langtest.Analyze(t, Plugin{}, "testdata/repo")
	base := lang.Target{Ecosystem: ecoStd, Package: "base"}
	conduit := lang.Target{Ecosystem: ecoHackage, Package: "conduit", Version: "6b98f070fea09a3bf0a5d0897a2e27e3aa91c8fe", Pinned: true, Origin: "https://github.com/snoyberg/conduit.git"}
	winConsole := lang.Target{Ecosystem: ecoHackage, Package: "win-console", Version: "v1.2", Floating: true, Origin: "https://github.com/acme/win-console"}
	containers := lang.Target{Ecosystem: ecoHackage, Package: "containers", Version: "0.6.7", Pinned: true}
	text := lang.Target{Ecosystem: ecoHackage, Package: "text", Version: "2.0.2", Pinned: true}
	mtl := lang.Target{Ecosystem: ecoHackage, Package: "mtl", Version: "2.3.1", Requested: ">=2.2 && <2.4", Pinned: true}
	aeson := lang.Target{Ecosystem: ecoHackage, Package: "aeson", Version: "2.2.3.0", Requested: "^>=2.2", Pinned: true}
	hashable := lang.Target{Ecosystem: ecoHackage, Package: "hashable", Version: "1.4.4.0", Pinned: true}
	deepseq := lang.Target{Ecosystem: ecoHackage, Package: "deepseq", Version: "1.4.8.1", Pinned: true}
	httpClient := lang.Target{Ecosystem: ecoHackage, Package: "http-client", Floating: true}
	hermes := lang.Target{Ecosystem: ecoHackage, Package: "hermes-json", Version: ">=0.6"}
	hspec := lang.Target{Ecosystem: ecoHackage, Package: "hspec", Floating: true}
	quickCheck := lang.Target{Ecosystem: ecoHackage, Package: "QuickCheck", Version: "2.14.3", Pinned: true}
	cabal := lang.Target{Ecosystem: ecoHackage, Package: "Cabal", Version: ">=3.10"}
	optparse := lang.Target{Ecosystem: ecoHackage, Package: "optparse-applicative", Floating: true}
	snapshotText := lang.Target{Ecosystem: ecoHackage, Package: "text", Version: "lts-22.43"}
	missiles := lang.Target{Ecosystem: ecoHackage, Package: "acme-missiles", Version: "0.3", Pinned: true}
	leftPad := lang.Target{Ecosystem: ecoHackage, Package: "left-pad", Version: "0123456789abcdef0123456789abcdef01234567", Pinned: true, Origin: "https://github.com/acme/left-pad-hs"}
	unlocked := lang.Target{Ecosystem: ecoHackage, Package: "unlocked-hs", Version: "main", Floating: true, Origin: "https://github.com/acme/unlocked-hs"}
	filepath := lang.Target{Ecosystem: ecoHackage, Package: "filepath", Version: ">= 1.4"}
	imports := map[string]map[string]lang.Target{
		"cabal.project": {
			"packages: shop-core":                    {Local: "shop-core/shop-core.cabal"},
			"packages: shop-app/":                    {Local: "shop-app/shop-app.cabal"},
			"source-repository-package: conduit":     conduit,
			"source-repository-package: win-console": winConsole,
		},
		"docs/Guide.lhs":    {"import Data.Maybe": base},
		"docs/Tutorial.lhs": {"import Shop.Types": {Local: "shop-core/src/Shop/Types.hs"}},
		"shop-app/app/Main.hs": {
			"import Shop.Cart":                {Local: "shop-core/src/Shop/Cart.hs"},
			"import Options.Applicative":      optparse,
			"import \"conduit\" Data.Conduit": conduit,
			"import System.Win32.Console":     {Ecosystem: ecoHackage, Package: "Win32", Unresolved: true},
			"import Data.Unknown.Thing":       {Ecosystem: ecoHackage, Package: "unknown", Unresolved: true},
			"import Numeric.Natural":          base,
		},
		"shop-app/shop-app.cabal": {
			"build-depends: base":                 base,
			"build-depends: shop-core":            {Local: "shop-core/shop-core.cabal"},
			"build-depends: optparse-applicative": optparse,
			"build-depends: conduit":              conduit,
			"build-depends: win-console":          winConsole,
		},
		"shop-core/Setup.hs": {"import Distribution.Simple": cabal},
		"shop-core/shop-core.cabal": {
			"setup-depends: base >=4 && <5":                  base,
			"setup-depends: Cabal >=3.10":                    cabal,
			"build-depends: base >=4.16 && <5":               base,
			"build-depends: containers":                      containers,
			"build-depends: aeson ^>=2.2":                    aeson,
			"build-depends: mtl >=2.2 && <2.4":               mtl,
			"build-depends: text":                            text,
			"build-depends: hashable":                        hashable,
			"build-depends: deepseq ==1.4.8.1":               deepseq,
			"build-depends: http-client":                     httpClient,
			"build-depends: hermes-json:{hermes-json} >=0.6": hermes,
			"build-depends: template-haskell":                {Ecosystem: ecoStd, Package: "template-haskell"},
			"build-depends: shop-core":                       {},
			"build-depends: hspec":                           hspec,
			"build-depends: QuickCheck == 2.14.3":            quickCheck,
		},
		"shop-core/src/Shop/Cart.hs": {
			"import {-# SOURCE #-} Shop.Types":                {Local: "shop-core/src/Shop/Types.hs-boot"},
			"import Shop.Types":                               {Local: "shop-core/src/Shop/Types.hs"},
			"import qualified Data.Map.Strict as M":           containers,
			"import Data.Text":                                text,
			"import qualified Data.Text as T":                 text,
			"import Control.Monad.State":                      mtl,
			"import Data.Aeson":                               aeson,
			"import Data.Hashable":                            hashable,
			"import Control.DeepSeq":                          deepseq,
			"import Network.HTTP.Client":                      httpClient,
			"import Data.Hermes":                              hermes,
			"import Data.List":                                base,
			"import qualified \"containers\" Data.Set as Set": containers,
			"import \"this\" Shop.Parser":                     {Local: "shop-core/src/Shop/Parser.y"},
			"import Paths_shop_core":                          {Local: "shop-core/shop-core.cabal"},
			"import Shop.Parser":                              {Local: "shop-core/src/Shop/Parser.y"},
			"import Main":                                     {},
		},
		"shop-core/src/Shop/Types.hs": {
			"import Data.Kind":    base,
			"import GHC.Generics": base,
		},
		"shop-core/src/Shop/Types.hs-boot": {},
		"shop-core/test/Spec.hs": {
			"import Test.Hspec":      hspec,
			"import Test.QuickCheck": quickCheck,
			"import Shop.Cart":       {Local: "shop-core/src/Shop/Cart.hs"},
			"import Control.Lens":    {Ecosystem: ecoHackage, Package: "lens", Unresolved: true},
		},
		"stackproj/app/Main.hs": {
			"import Lib":             {Local: "stackproj/src/Lib.hs"},
			"import System.FilePath": filepath,
			"import System.IO":       base,
		},
		"stackproj/package.yaml": {
			"dependencies: base >= 4.7 && < 5": base,
			"dependencies: text":               snapshotText,
			"dependencies: acme-missiles":      missiles,
			"dependencies: left-pad":           leftPad,
			"dependencies: unlocked-hs":        unlocked,
			"dependencies: stackproj":          {},
			"dependencies: filepath >= 1.4":    filepath,
		},
		"stackproj/src/Lib.hs": {
			"import Acme.Missiles":            missiles,
			"import qualified Data.Text as T": snapshotText,
			"import Data.LeftPad":             leftPad,
		},
		"stackproj/stack.yaml": {
			"packages: .":                   {Local: "stackproj/package.yaml"},
			"extra-deps: acme-missiles-0.3": missiles,
			"extra-deps: left-pad-hs":       leftPad,
			"extra-deps: unlocked-hs":       unlocked,
		},
	}
	if len(res) != len(imports) {
		var got []string
		for f := range res {
			got = append(got, f)
		}
		t.Errorf("analyzed %d files, want %d: %v", len(res), len(imports), got)
	}
	for file, want := range imports {
		langtest.CheckImports(t, res[file], want)
	}
	symbols := map[string]map[string]string{
		"cabal.project":        {},
		"docs/Guide.lhs":       {"Guide": "module", "greeting": "function"},
		"docs/Tutorial.lhs":    {"Tutorial": "module", "farewell": "function"},
		"shop-app/app/Main.hs": {"Main": "module", "main": "function"},
		"shop-core/shop-core.cabal": {
			"common deps":     "component",
			"library":         "component",
			"test-suite spec": "component",
		},
		"shop-core/src/Shop/Cart.hs": {
			"Shop.Cart": "module",
			"Cart":      "type",
			"add":       "function",
			"checkout":  "function",
		},
		"shop-core/src/Shop/Types.hs": {
			"Shop.Types":                "module",
			"Item":                      "type",
			"Price":                     "type",
			"Label":                     "type",
			":+:":                       "type",
			"Elem":                      "type family",
			"type instance Elem [e]":    "instance",
			"Priced":                    "class",
			"Priced.price":              "method",
			"Priced.discount":           "method",
			"Priced.surcharge":          "method",
			"instance Priced Item":      "instance",
			"instance Show (Priced' a)": "instance",
			"instance Eq Price":         "instance",
			"Free":                      "pattern",
			"label":                     "function",
			"<+>":                       "function",
			"total":                     "function",
			"foldl'":                    "function",
		},
		"shop-core/src/Shop/Types.hs-boot": {"Shop.Types": "module", "Item": "type"},
		"stackproj/src/Lib.hs":             {"Lib": "module", "someFunc": "function"},
	}
	for file, want := range symbols {
		langtest.CheckSymbols(t, res[file], want)
	}
	// Literate lines are the document's own.
	for _, s := range res["docs/Tutorial.lhs"].Symbols {
		if s.Name == "farewell" && s.Line != 8 {
			t.Errorf("farewell at line %d, want 8", s.Line)
		}
	}
}

// cabal's build plan answers --resolve-depth: a package's dependencies at the
// versions the plan chose, without GHC's own packages and the project's.
//
// Verifies: REQ-HASKELL-008
func TestPlanDependencies(t *testing.T) {
	r := newResolver("testdata/repo", langtest.Files(t, "testdata/repo"))
	for _, tt := range []struct {
		pkg  lang.Target
		want []lang.Target
	}{
		{lang.Target{Ecosystem: ecoHackage, Package: "aeson", Version: "2.2.3.0"}, []lang.Target{
			{Ecosystem: ecoHackage, Package: "conduit", Version: "6b98f070fea09a3bf0a5d0897a2e27e3aa91c8fe", Pinned: true, Origin: "https://github.com/snoyberg/conduit.git"},
			{Ecosystem: ecoHackage, Package: "containers", Version: "0.6.7", Pinned: true},
			{Ecosystem: ecoHackage, Package: "text", Version: "2.0.2", Pinned: true},
		}},
		{lang.Target{Ecosystem: ecoHackage, Package: "text", Version: "2.0.2"}, nil},
		// Another version than the plan's is not answered for.
		{lang.Target{Ecosystem: ecoHackage, Package: "aeson", Version: "2.1.0.0"}, nil},
		{lang.Target{Ecosystem: ecoStd, Package: "base", Version: "4.18.2.1"}, nil},
	} {
		if got := r.Dependencies(tt.pkg); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s %s: got %+v, want %+v", tt.pkg.Package, tt.pkg.Version, got, tt.want)
		}
	}
}

// What only looks like code - in nested comments, strings, character literals, CPP
// lines - is not read; primed names, promoted constructors, Template Haskell quotes
// and operators starting with dashes are not mistaken for literals or comments.
//
// Verifies: REQ-HASKELL-002, REQ-HASKELL-003, REQ-HASKELL-011
func TestLexerHidesNonCode(t *testing.T) {
	src := "{- outer {- inner -} import InComment -}\n" +
		"module M where\n" +
		"#include \"x.h\"\n" +
		"import Data.Maybe\n" +
		"s = \"import InString \\\n   \\gap\"\n" +
		"c = ['\\'', '\"', 'x']\n" +
		"f' x = x --> y -- import InLineComment\n" +
		"g = 'True : ''Maybe : []\n" +
		"import qualified Real.One as R hiding (x)\n" +
		"import safe Real.Two\n" +
		"x ∷ Int\n"
	tokens := lex([]byte(src))
	for _, tk := range tokens {
		if tk.k == tCon && (tk.s == "InComment" || tk.s == "InString" || tk.s == "InLineComment") {
			t.Errorf("%s read as code at line %d", tk.s, tk.line)
		}
	}
	ex, err := Plugin{}.Extract(&scan.File{Path: "M.hs"}, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	var specs []string
	for _, im := range ex.Imports {
		specs = append(specs, im.Spec)
	}
	if want := []string{"import Data.Maybe", "import qualified Real.One as R", "import safe Real.Two"}; !reflect.DeepEqual(specs, want) {
		t.Errorf("imports %q, want %q", specs, want)
	}
	var names []string
	for _, s := range ex.Symbols {
		names = append(names, s.Name+" "+s.Kind)
	}
	if want := []string{"M module", "s function", "c function", "f' function", "g function", "x function"}; !reflect.DeepEqual(names, want) {
		t.Errorf("symbols %q, want %q", names, want)
	}
}

// Build output (dist-newstyle, .stack-work) is not the project's code; package.yaml,
// stack.yaml and cabal.project get their own cache classes.
//
// Verifies: REQ-HASKELL-001
func TestClaims(t *testing.T) {
	for p, want := range map[string]bool{
		"src/A.hs": true, "doc/B.lhs": true, "src/A.hs-boot": true, "cbits/C.hsc": true,
		"shop.cabal": true, "cabal.project": true, "package.yaml": true, "stack.yaml": true,
		"cabal.project.freeze": false, "stack.yaml.lock": false, "config.yaml": false, "README.md": false,
		"dist-newstyle/build/x/A.hs": false, ".stack-work/dist/x/Paths_a.hs": false,
	} {
		if got := (Plugin{}).Claims(&scan.File{Path: p}); got != want {
			t.Errorf("%s: claimed %v, want %v", p, got, want)
		}
	}
	if (Plugin{}).Class(&scan.File{Path: "package.yaml"}) == (Plugin{}).Class(&scan.File{Path: "config.yaml"}) ||
		(Plugin{}).Class(&scan.File{Path: "package.yaml"}) == (Plugin{}).Class(&scan.File{Path: "stack.yaml"}) {
		t.Error("package.yaml and stack.yaml share a cache class with other YAML")
	}
}
