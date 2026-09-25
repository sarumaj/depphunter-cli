package python

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
)

// testdata/repo: a src-layout package declared in pyproject.toml (PEP 621, dependency
// groups and Poetry tables), pinned by uv.lock, plus a requirements file and scripts.
func analyze(t *testing.T) map[string]*lang.FileResult {
	t.Helper()
	return langtest.Analyze(t, Plugin{}, "testdata/repo")
}

// Verifies: REQ-PY-001, REQ-PY-002, REQ-PY-003, REQ-PY-005, REQ-PY-006, REQ-PY-007, REQ-PY-009, REQ-PY-010, REQ-PY-013
func TestImportResolution(t *testing.T) {
	res := analyze(t)
	langtest.CheckImports(t, res["src/app/main.py"], map[string]lang.Target{
		"from __future__ import annotations": {Ecosystem: "python-std", Package: "__future__"},
		"os":                                 {Ecosystem: "python-std", Package: "os"},
		"os.path":                            {Ecosystem: "python-std", Package: "os"},
		"json":                               {Ecosystem: "python-std", Package: "json"},
		"from typing import List":            {Ecosystem: "python-std", Package: "typing"},
		"requests":                           {Ecosystem: "pypi", Package: "requests", Version: "2.31.0", Requested: ">=2.31", Pinned: true},
		"yaml":                               {Ecosystem: "pypi", Package: "PyYAML", Version: "6.0.1", Pinned: true},
		"from bs4 import BeautifulSoup":      {Ecosystem: "pypi", Package: "beautifulsoup4", Version: "^4.12"},
		"numpy":                              {Ecosystem: "pypi", Package: "numpy", Version: "1.26.4", Pinned: true},
		"certifi":                            {Ecosystem: "pypi", Package: "certifi", Version: "2024.2.2", Pinned: true},
		"pytest":                             {Ecosystem: "pypi", Package: "pytest", Version: ">=8"},
		"black":                              {Ecosystem: "pypi", Package: "black", Version: "24.1.0", Pinned: true},
		"notdeclared.sub":                    {Ecosystem: "pypi", Package: "notdeclared", Unresolved: true},
		"from . import utils":                {Local: "src/app/utils.py"},
		"from .models import User":           {Local: "src/app/models"},
		"from .models.user import User":      {Local: "src/app/models/user.py"},
		"from ..outside import x":            {},
		"from app.helpers import thing":      {Local: "src/app/helpers.py"},
		"from app.models import *":           {Local: "src/app/models"},
	})
}

// Verifies: REQ-PY-004
func TestScriptDirectoryImports(t *testing.T) {
	got := map[string]lang.Target{}
	for _, im := range analyze(t)["scripts/tool.py"].Imports {
		got[im.Spec] = im.Target
	}
	if got["sibling"] != (lang.Target{Local: "scripts/sibling.py"}) {
		t.Errorf("sibling: %+v", got["sibling"])
	}
	if got["logging"] != (lang.Target{Ecosystem: "python-std", Package: "logging"}) {
		t.Errorf("logging: %+v", got["logging"])
	}
}

// Verifies: REQ-LANG-023, REQ-LANG-024, REQ-PY-014
func TestSymbols(t *testing.T) {
	got := map[string]string{}
	for _, s := range analyze(t)["src/app/main.py"].Symbols {
		got[s.Name] = s.Kind
	}
	for name, kind := range map[string]string{
		"run": "func", "dec": "func", "Service": "class", "Service.start": "method",
		"Service.name": "method", "CONSTANT": "var",
	} {
		if got[name] != kind {
			t.Errorf("symbol %s: kind %q, want %q (all: %v)", name, got[name], kind, got)
		}
	}
}

// Verifies: REQ-PY-011, REQ-PY-012
func TestSetuptoolsManifests(t *testing.T) {
	pypi := func(pkg, version string) lang.Target {
		// "==" pins, everything else these manifests write is a lower bound.
		return lang.Target{Ecosystem: "pypi", Package: pkg, Version: version, Pinned: lang.Pinned(version)}
	}
	langtest.CheckImports(t, langtest.Analyze(t, Plugin{}, "testdata/setuptools")["pkg/app.py"], map[string]lang.Target{
		"requests": pypi("requests", ">=2.31"), // setup.cfg, trailing comment dropped
		"click":    pypi("click", ""),
		"pytest":   pypi("pytest", ">=8"),  // setup.cfg extras
		"rich":     pypi("rich", "13.7.0"), // setup.py
		"attr":     pypi("attrs", ""),
		"sphinx":   pypi("sphinx", ">=7"), // setup.py extras
		"ruff":     pypi("ruff", ""),
		"numpy":    {Ecosystem: "pypi", Package: "numpy", Unresolved: true},
	})
}

// TestLockTree checks the dependency edges a lock file records; uv writes them as a
// list of tables, poetry as a table, pdm as requirement strings.
//
// Verifies: REQ-SUP-009
func TestLockTree(t *testing.T) {
	r, err := (Plugin{}).Resolver("testdata/repo", langtest.Files(t, "testdata/repo"))
	if err != nil {
		t.Fatal(err)
	}
	tr, ok := r.(lang.Transitive)
	if !ok {
		t.Fatal("the resolver cannot answer for transitive dependencies")
	}
	got := map[string]lang.Target{}
	for _, d := range tr.Dependencies(lang.Target{Ecosystem: "pypi", Package: "requests"}) {
		got[d.Package] = d
	}
	if len(got) != 2 {
		t.Fatalf("requests depends on %+v, want certifi and urllib3", got)
	}
	if c := got["certifi"]; c.Version != "2024.2.2" || !c.Pinned {
		t.Errorf("certifi: %+v, want 2024.2.2 pinned", c)
	}
	if u := got["urllib3"]; u.Version != "2.2.1" {
		t.Errorf("urllib3: %+v, want 2.2.1", u)
	}
}

// Verifies: REQ-SUP-009
func TestLockDependencyShapes(t *testing.T) {
	for _, c := range []struct {
		name string
		in   any
		want []string
	}{
		{"poetry table", map[string]any{"certifi": ">=2017.4.17"}, []string{"certifi"}},
		{"uv tables", []any{map[string]any{"name": "certifi"}}, []string{"certifi"}},
		{"pdm strings", []any{"certifi>=2017.4.17; python_version >= '3'"}, []string{"certifi"}},
		{"nothing", nil, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := lockDependencies(c.in)
			if len(got) != len(c.want) {
				t.Fatalf("got %v, want %v", got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Errorf("got %v, want %v", got, c.want)
				}
			}
		})
	}
}

// A Pipenv project declares its distributions in the Pipfile alone: under packages
// and dev-packages, as a version string or as a table with a version key.
//
// Verifies: REQ-PY-008
func TestPipfile(t *testing.T) {
	root := t.TempDir()
	for name, content := range map[string]string{
		"Pipfile": "[packages]\nrequests = \"*\"\nflask = {version = \">=3.0\", extras = [\"async\"]}\n\n" +
			"[dev-packages]\npytest = \"==8.1.1\"\n",
		"app.py": "import requests\nimport flask\nimport pytest\nimport undeclared\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	langtest.CheckImports(t, langtest.Analyze(t, Plugin{}, root)["app.py"], map[string]lang.Target{
		"requests":   {Ecosystem: "pypi", Package: "requests"},
		"flask":      {Ecosystem: "pypi", Package: "flask", Version: ">=3.0"},
		"pytest":     {Ecosystem: "pypi", Package: "pytest", Version: "8.1.1", Pinned: true},
		"undeclared": {Ecosystem: "pypi", Package: "undeclared", Unresolved: true},
	})
}

// testdata/env: a project whose imports only an environment can resolve, and that
// environment - a virtual environment with a distribution installed from a local
// directory, one declared and installed editable, one from an index, and two from a
// Git repository sharing the google namespace.
const envInterpreter = "testdata/env/python/bin/python3.12"

// Verifies: REQ-PY-015
func TestInstalledPackagesNoIndexHas(t *testing.T) {
	res := langtest.Analyze(t, Plugin{Interpreter: envInterpreter}, "testdata/env/repo")
	langtest.CheckImports(t, res["app.py"], map[string]lang.Target{
		"acme":                 {Ecosystem: "pypi", Package: "acme-core", Version: "1.4.0", Origin: "file:///home/me/src/acme-core"},
		"acme.ledger":          {Ecosystem: "pypi", Package: "acme-core", Version: "1.4.0", Origin: "file:///home/me/src/acme-core"},
		"billing":              {Ecosystem: "pypi", Package: "acme-billing", Version: "0.3.0", Origin: "file:///home/me/src/acme-billing"},
		"six":                  {Ecosystem: "pypi", Package: "six", Unresolved: true},
		"google.cloud.storage": {Ecosystem: "pypi", Package: "google-cloud-storage", Version: "2.16.0", Origin: "git+https://git.corp.example/mirror/gcs.git"},
		"google":               {Ecosystem: "pypi", Package: "google", Unresolved: true},
	})

	// Without an environment, the same imports are what they always were.
	bare := langtest.Analyze(t, Plugin{}, "testdata/env/repo")
	langtest.CheckImports(t, bare["app.py"], map[string]lang.Target{
		"acme":                 {Ecosystem: "pypi", Package: "acme", Unresolved: true},
		"acme.ledger":          {Ecosystem: "pypi", Package: "acme", Unresolved: true},
		"billing":              {Ecosystem: "pypi", Package: "billing", Unresolved: true},
		"six":                  {Ecosystem: "pypi", Package: "six", Unresolved: true},
		"google.cloud.storage": {Ecosystem: "pypi", Package: "google", Unresolved: true},
		"google":               {Ecosystem: "pypi", Package: "google", Unresolved: true},
	})
}

// Verifies: REQ-PY-015
func TestInstalledDependencies(t *testing.T) {
	r, err := (Plugin{Interpreter: envInterpreter}).Resolver("testdata/env/repo", langtest.Files(t, "testdata/env/repo"))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]lang.Target{}
	for _, d := range r.(lang.Transitive).Dependencies(lang.Target{Ecosystem: "pypi", Package: "acme-core"}) {
		got[d.Package] = d
	}
	want := map[string]lang.Target{
		// Declared by the project, installed from a directory.
		"acme-billing": {Ecosystem: "pypi", Package: "acme-billing", Version: "0.3.0", Origin: "file:///home/me/src/acme-billing"},
		// Installed from an index: its installed version, and an index may be asked.
		"six": {Ecosystem: "pypi", Package: "six", Version: "1.16.0"},
		// Required, and not installed at all.
		"requests": {Ecosystem: "pypi", Package: "requests"},
	}
	if len(got) != len(want) {
		t.Fatalf("acme-core depends on %+v, want %+v (pytest is only for an extra)", got, want)
	}
	for name, w := range want {
		if got[name] != w {
			t.Errorf("%s: %+v, want %+v", name, got[name], w)
		}
	}
	// One installed from an index is left to the lock files and the indexes.
	if deps := r.(lang.Transitive).Dependencies(lang.Target{Ecosystem: "pypi", Package: "six"}); deps != nil {
		t.Errorf("six: %+v, want nothing from the environment", deps)
	}
	// ... and the report is told which answers came from the environment.
	in := r.(lang.Installed)
	if !in.Installed(lang.Target{Ecosystem: "pypi", Package: "acme-core"}) || in.Installed(lang.Target{Ecosystem: "pypi", Package: "six"}) {
		t.Error("the environment's answers are not told apart from the lock files'")
	}
}

// Verifies: REQ-PY-015
func TestFindEnvironment(t *testing.T) {
	root := t.TempDir()
	if findEnvironment(root, "", func(string) string { return "" }) != nil {
		t.Error("an environment was read with none given")
	}
	// An activated virtual environment.
	active, _ := filepath.Abs("testdata/env/python")
	env := findEnvironment(root, "", func(k string) string {
		if k == "VIRTUAL_ENV" {
			return active
		}
		return ""
	})
	if env.get("acme-core") == nil {
		t.Errorf("VIRTUAL_ENV was not read: %+v", env)
	}
	// The project's own .venv, which includes the system site-packages, and an
	// editable install through a path file.
	venv := filepath.Join(root, ".venv")
	base := filepath.Join(root, "base")
	src := filepath.Join(root, "widgets-src")
	for _, dir := range []string{
		filepath.Join(venv, "lib", "python3.11", "site-packages", "widgets-1.0.dist-info"),
		filepath.Join(base, "lib", "python3.11", "site-packages", "gadgets-2.0.dist-info"),
		filepath.Join(base, "bin"),
		filepath.Join(src, "widgets"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		filepath.Join(venv, "pyvenv.cfg"): "home = " + filepath.Join(base, "bin") + "\ninclude-system-site-packages = true\nversion_info = 3.11.9\n",
		filepath.Join(venv, "lib", "python3.11", "site-packages", "widgets-1.0.dist-info", "METADATA"):        "Name: widgets\nVersion: 1.0\n",
		filepath.Join(venv, "lib", "python3.11", "site-packages", "widgets-1.0.dist-info", "RECORD"):          "__editable__.widgets-1.0.pth,,\n",
		filepath.Join(venv, "lib", "python3.11", "site-packages", "widgets-1.0.dist-info", "direct_url.json"): `{"url": "file:///src/widgets", "dir_info": {"editable": true}}`,
		filepath.Join(venv, "lib", "python3.11", "site-packages", "__editable__.widgets-1.0.pth"):             src + "\n",
		filepath.Join(src, "widgets", "__init__.py"):                                                          "",
		filepath.Join(base, "lib", "python3.11", "site-packages", "gadgets-2.0.dist-info", "METADATA"):        "Name: gadgets\nVersion: 2.0\n",
		filepath.Join(base, "lib", "python3.11", "site-packages", "gadgets-2.0.dist-info", "top_level.txt"):   "gadgets\n",
		filepath.Join(base, "lib", "python3.11", "site-packages", "gadgets-2.0.dist-info", "direct_url.json"): `{"url": "https://files.corp.example/gadgets-2.0.whl", "archive_info": {}}`,
	}
	for name, content := range files {
		if err := os.WriteFile(name, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	env = findEnvironment(root, "", nil)
	if d := env.provider([]string{"widgets"}); d == nil || d.origin != "file:///src/widgets" {
		t.Errorf("the editable install through a path file: %+v", d)
	}
	if d := env.provider([]string{"gadgets", "sub"}); d == nil || d.version != "2.0" {
		t.Errorf("the base interpreter's site-packages: %+v", d)
	}
}
