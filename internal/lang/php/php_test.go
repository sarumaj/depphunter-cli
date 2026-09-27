package php

import (
	"reflect"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// testdata/repo: a Composer project with PSR-4, PSR-0, classmap and files autoload
// rules, a composer.lock whose packages carry their own autoload rules, a
// path-repository package (packages/tools) and templates including each other.
// testdata/installed: a project without a lock, whose vendor/composer/installed.json
// says what is installed.
func analyze(t *testing.T, root string) map[string]*lang.FileResult {
	t.Helper()
	return langtest.Analyze(t, Plugin{}, root)
}

func composer(name, version, requested string) lang.Target {
	return lang.Target{Ecosystem: "composer", Package: name, Version: version, Requested: requested, Pinned: true}
}

func std(ext string) lang.Target { return lang.Target{Ecosystem: "php-std", Package: ext} }

// Verifies: REQ-PHP-001, REQ-PHP-002, REQ-PHP-003, REQ-PHP-004, REQ-PHP-005, REQ-PHP-006, REQ-PHP-007, REQ-PHP-009, REQ-PHP-010
func TestResolution(t *testing.T) {
	res := analyze(t, "testdata/repo")
	langtest.CheckImports(t, res["src/Http/Controller/UserController.php"], map[string]lang.Target{
		"use App\\Models\\User":                           {Local: "src/Models/User.php"},
		"use App\\Models":                                 {Local: "src/Models"}, // a namespace, not a class
		"use App\\Services\\Mailer":                       {Local: "src/Services/Mailer.php"},
		"use App\\Services\\Billing\\Invoice":             {Local: "src/Services/Billing/Invoice.php"},
		"use Monolog\\Logger":                             composer("monolog/monolog", "3.5.0", "^3.0"),
		"use Psr\\Log\\LoggerInterface":                   composer("psr/log", "3.0.0", ""), // lock only: installed for monolog
		"use GuzzleHttp\\Client":                          composer("guzzlehttp/guzzle", "7.8.1", ""),
		"use Symfony\\Component\\HttpFoundation\\Request": composer("symfony/http-foundation", "v6.4.2", "^6.4"),
		// Neither installed nor declared: named by the namespace, unresolved.
		"use Symfony\\Component\\Console\\Command\\Command": {Ecosystem: "composer", Package: "symfony/console", Unresolved: true},
		"use Carbon\\Carbon":                               {Ecosystem: "composer", Package: "carbon/carbon", Unresolved: true},
		"use Acme\\Tools\\Formatter":                       {Local: "packages/tools/src/Formatter.php"}, // path repository
		"use Legacy_Mailer":                                {Local: "lib/Legacy/Mailer.php"},            // PSR-0
		"use Exception":                                    std("core"),
		"use function App\\Support\\format_money":          {Local: "src/Support/functions.php"},
		"use function GuzzleHttp\\Psr7\\str":               composer("guzzlehttp/guzzle", "7.8.1", ""),
		"use const App\\Support\\VERSION":                  {Local: "src/Support/functions.php"},
		"require_once __DIR__ . '/../../../bootstrap.php'": {Local: "bootstrap.php"},
		"include 'config/app.php'":                         {Local: "config/app.php"}, // the include path: the project root
		"require dirname(__DIR__, 2) . '/helpers.php'":     {Local: "src/helpers.php"},
		"extends BaseController":                           {Local: "src/Http/Controller/BaseController.php"},
		"implements Contracts\\Handles":                    {Local: "src/Http/Controller/Contracts/Handles.php"},
		"implements \\JsonSerializable":                    std("json"),
		"new \\DateTimeImmutable":                          std("date"),
		"\\Monolog\\Registry::":                            composer("monolog/monolog", "3.5.0", "^3.0"),
		"new \\Unknown\\Thing\\Widget":                     {Ecosystem: "composer", Package: "unknown/thing", Unresolved: true},
		"\\App\\Models\\User::":                            {Local: "src/Models/User.php"},
		"catch \\RuntimeException":                         std("spl"),
		"\\strlen()":                                       std("core"),
		"\\collect()":                                      {}, // a global helper nobody here defines
	})
	langtest.CheckImports(t, res["tests/UserTest.php"], map[string]lang.Target{
		// phpunit autoloads by classmap only: matched by its name.
		"use PHPUnit\\Framework\\TestCase": composer("phpunit/phpunit", "10.5.5", "^10.5"),
		"use Mockery":                      composer("mockery/mockery", "1.6.7", "^1.6"), // PSR-0 "Mockery"
		"use App\\Models\\User":            {Local: "src/Models/User.php"},
		"use UserSeeder":                   {Local: "database/seeds/UserSeeder.php"}, // classmap
	})
	langtest.CheckImports(t, res["src/Models/User.php"], map[string]lang.Target{
		"trait HasName": {Local: "src/Models/HasName.php"},
	})
	// A nested composer.json without a lock takes packages from the project above.
	langtest.CheckImports(t, res["packages/tools/src/Formatter.php"], map[string]lang.Target{
		"use Psr\\Log\\LoggerInterface": composer("psr/log", "3.0.0", ""),
	})
	langtest.CheckImports(t, res["templates/view.phtml"], map[string]lang.Target{
		"include __DIR__ . '/partials/header.phtml'": {Local: "templates/partials/header.phtml"},
	})
}

// Verifies: REQ-PHP-008
func TestInstalledJSON(t *testing.T) {
	res := analyze(t, "testdata/installed")
	langtest.CheckImports(t, res["src/Shop.php"], map[string]lang.Target{
		"use Monolog\\Logger":              composer("monolog/monolog", "3.6.0", "^3.0"),
		"use Psr\\Log\\NullLogger":         composer("psr/log", "3.0.1", ""),
		"use Laminas\\Diactoros\\Response": {Ecosystem: "composer", Package: "laminas/diactoros", Unresolved: true},
		// Not installed: psr has three packages here, the segments pick one.
		"use Psr\\Http\\Message\\ResponseInterface": {Ecosystem: "composer", Package: "psr/http-message", Version: "^2.0"},
		"use Psr\\Http\\Factory\\X":                 {Ecosystem: "composer", Package: "psr/http-factory", Version: "1.1.0", Pinned: true},
	})
	r := newResolver("testdata/installed", langtest.Files(t, "testdata/installed"))
	if !r.Installed(lang.Target{Ecosystem: "composer", Package: "monolog/monolog", Version: "3.6.0"}) {
		t.Error("monolog is known from installed.json")
	}
	got := r.Dependencies(lang.Target{Ecosystem: "composer", Package: "monolog/monolog", Version: "3.6.0"})
	if want := []lang.Target{composer("psr/log", "3.0.1", "")}; !reflect.DeepEqual(got, want) {
		t.Errorf("monolog depends on %+v, want %+v", got, want)
	}
}

// Verifies: REQ-PHP-011, REQ-SUP-009
func TestLockTree(t *testing.T) {
	r, err := (Plugin{}).Resolver("testdata/repo", langtest.Files(t, "testdata/repo"))
	if err != nil {
		t.Fatal(err)
	}
	tr, ok := r.(lang.Transitive)
	if !ok {
		t.Fatal("the resolver cannot answer for transitive dependencies")
	}
	for pkg, want := range map[string][]lang.Target{
		// php and ext-json are the platform, not packages.
		"guzzlehttp/guzzle":       {composer("psr/log", "3.0.0", "")},
		"symfony/http-foundation": {composer("symfony/polyfill-mbstring", "v1.28.0", "")},
		// Required but not in the lock: the constraint, floating.
		"phpunit/phpunit": {{Ecosystem: "composer", Package: "sebastian/diff", Version: "^5.0"}},
		"psr/log":         {},
	} {
		got := tr.Dependencies(lang.Target{Ecosystem: "composer", Package: pkg})
		if len(got) == 0 && len(want) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s depends on %+v, want %+v", pkg, got, want)
		}
	}
	if in, ok := r.(lang.Installed); !ok || in.Installed(composer("psr/log", "3.0.0", "")) {
		t.Error("a locked package is not answered for from installed.json")
	}
}

// Verifies: REQ-PHP-003
func TestSymbols(t *testing.T) {
	res := analyze(t, "testdata/repo")
	langtest.CheckSymbols(t, res["src/Http/Controller/UserController.php"], map[string]string{
		"App\\Http\\Controller": "namespace", "UserController": "class", "UserController.LIMIT": "const",
		"UserController.index": "method", "UserController.helper": "method", // nested() is local to a closure
	})
	langtest.CheckSymbols(t, res["src/Support/functions.php"], map[string]string{
		"App\\Support": "namespace", "VERSION": "const", "format_money": "func", "legacy": "func",
		"APP_DEBUG": "const",
	})
	langtest.CheckSymbols(t, res["src/Services/Billing/Invoice.php"], map[string]string{
		"App\\Services\\Billing": "namespace", "Invoice": "enum", "Invoice.label": "method",
	})
	langtest.CheckSymbols(t, res["src/Models/HasName.php"], map[string]string{
		"App\\Models": "namespace", "HasName": "trait", "HasName.name": "method",
	})
	langtest.CheckSymbols(t, res["src/Http/Controller/Contracts/Handles.php"], map[string]string{
		"App\\Http\\Controller\\Contracts": "namespace", "Handles": "interface", "Handles.handle": "method",
	})
}

// Braced namespaces: a name in code is relative to the block it is in.
//
// Verifies: REQ-PHP-002, REQ-PHP-003
func TestBracedNamespaces(t *testing.T) {
	src := []byte(`<?php
namespace A {
    class K extends Base {}
    $o = new class { public function anon() {} };
}
namespace B {
    use X\Y;
    class L extends Y\Z implements \Countable {}
}
`)
	ex, err := (Plugin{}).Extract(&scan.File{Path: "x.php"}, src)
	if err != nil {
		t.Fatal(err)
	}
	var got []lang.RawImport
	for _, im := range ex.Imports {
		got = append(got, lang.RawImport{Spec: im.Spec, Module: im.Module, Name: im.Name})
	}
	want := []lang.RawImport{
		{Spec: "extends Base", Module: `A\Base`, Name: kindLocal},
		{Spec: `use X\Y`, Module: `X\Y`, Name: kindClass},
		{Spec: `extends Y\Z`, Module: `X\Y\Z`, Name: kindClass},
		{Spec: `implements \Countable`, Module: "Countable", Name: kindClass},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
	names := map[string]bool{}
	for _, s := range ex.Symbols {
		names[s.Name] = true
	}
	if !names["K"] || !names["L"] || names[".anon"] || names["anon"] {
		t.Errorf("symbols %+v", ex.Symbols)
	}
}

// Verifies: REQ-PHP-002
func TestParseUse(t *testing.T) {
	for text, want := range map[string][]use{
		`use A\B;`:           {{kindClass, `A\B`, "B"}},
		`use \A\B as C;`:     {{kindClass, `A\B`, "C"}},
		`use A\B, C\D AS E;`: {{kindClass, `A\B`, "B"}, {kindClass, `C\D`, "E"}},
		"use A\\{B, C\\D as E, function f, const G,\n};": {{kindClass, `A\B`, "B"}, {kindClass, `A\C\D`, "E"}, {kindFunction, `A\f`, "f"}, {kindConst, `A\G`, "G"}},
		`use function A\f, A\g;`:                         {{kindFunction, `A\f`, "f"}, {kindFunction, `A\g`, "g"}},
		`use const A\B;`:                                 {{kindConst, `A\B`, "B"}},
		`use function A\{f, g};`:                         {{kindFunction, `A\f`, "f"}, {kindFunction, `A\g`, "g"}},
		"use A\\B; // why":                               {{kindClass, `A\B`, "B"}},
	} {
		if got := parseUse(text); !reflect.DeepEqual(got, want) {
			t.Errorf("%q: got %+v, want %+v", text, got, want)
		}
	}
}

// Verifies: REQ-PHP-004
func TestIncludePath(t *testing.T) {
	for text, want := range map[string]string{
		`require 'a.php'`:                                 "a.php",
		`require_once("lib/b.php")`:                       "lib/b.php",
		`include __DIR__ . '/c.php'`:                      "__DIR__/c.php",
		`include_once dirname(__FILE__) . '/d.php'`:       "__DIR__/d.php",
		`require dirname(__DIR__) . "/e.php"`:             "__DIR__/../e.php",
		`require DIRNAME(__DIR__, 2) . '/f.php'`:          "__DIR__/../../f.php",
		`require __DIR__ . DIRECTORY_SEPARATOR . 'g.php'`: "__DIR__/g.php",
		`require (__DIR__ . '/h.php')`:                    "__DIR__/h.php",
		`require $path`:                                   "",
		`require __DIR__ . "/$name.php"`:                  "",
		`require APP_ROOT . '/x.php'`:                     "",
		`require __DIR__`:                                 "",
		`require __DIR__ . '/a.' . 'php'`:                 "__DIR__/a.php",
	} {
		got, ok := includePath(text)
		if got != want || ok != (want != "") {
			t.Errorf("%s: got %q %v, want %q", text, got, ok, want)
		}
	}
}

// Verifies: REQ-PHP-009
func TestPinned(t *testing.T) {
	for c, want := range map[string]bool{
		"1.2.3": true, "v1.2.3": true, "1.2": true, "=1.2.3": true, "==1.2.3": true,
		"1.0.0-RC1": true, "1.2.3@beta": true,
		"dev-main#0123456789abcdef0123456789abcdef01234567": true,
		"^1.2": false, "~1.2": false, ">=1.0": false, "1.2.*": false, "*": false,
		"dev-main": false, "^1.0 || ^2.0": false, ">=1.0 <2.0": false, "dev-main as 1.0.x-dev": false,
		"": false,
	} {
		if got := pinned(c); got != want {
			t.Errorf("pinned(%q) = %v, want %v", c, got, want)
		}
	}
}

// Verifies: REQ-PHP-006
func TestBuiltin(t *testing.T) {
	for _, c := range []struct {
		fqn, kind, ext string
	}{
		{"ArrayObject", kindClass, "spl"}, {"pdo", kindClass, "pdo"}, {`Random\Randomizer`, kindClass, "random"},
		{"FFI", kindClass, "ffi"}, {"array_map", kindFunction, "standard"}, {"mb_strlen", kindFunction, "mbstring"},
		{"json_encode", kindFunction, "json"}, {"is_array", kindFunction, "standard"}, {"PHP_EOL", kindConst, "core"},
		{`MongoDB\Client`, kindClass, ""}, {"collect", kindFunction, ""}, {"get_option", kindFunction, ""},
		{"Carbon", kindClass, ""},
	} {
		ext, ok := builtin(c.fqn, c.kind)
		if ext != c.ext || ok != (c.ext != "") {
			t.Errorf("builtin(%s, %s) = %q %v, want %q", c.fqn, c.kind, ext, ok, c.ext)
		}
	}
}

// Verifies: REQ-PHP-010
func TestKebab(t *testing.T) {
	for in, want := range map[string]string{"HttpFoundation": "http-foundation", "PHPUnit": "phpunit", "Monolog": "monolog", "OAuth2": "oauth2", "PhpCsFixer": "php-cs-fixer"} {
		if got := kebab(in); got != want {
			t.Errorf("kebab(%s) = %s, want %s", in, got, want)
		}
	}
}

// Verifies: REQ-PHP-001
func TestClaims(t *testing.T) {
	for p, want := range map[string]bool{"a.php": true, "b.PHTML": true, "c.inc": true, "d.phps": false, "e.js": false} {
		if got := (Plugin{}).Claims(&scan.File{Path: p}); got != want {
			t.Errorf("Claims(%s) = %v", p, got)
		}
	}
	if (Plugin{}).Claims(&scan.File{Path: "x.php", Binary: true}) {
		t.Error("a binary file is not claimed")
	}
}
