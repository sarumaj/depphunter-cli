package python

import (
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
