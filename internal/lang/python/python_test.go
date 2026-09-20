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

func TestImportResolution(t *testing.T) {
	res := analyze(t)
	langtest.CheckImports(t, res["src/app/main.py"], map[string]lang.Target{
		"from __future__ import annotations": {Ecosystem: "python-std", Package: "__future__"},
		"os":                                 {Ecosystem: "python-std", Package: "os"},
		"os.path":                            {Ecosystem: "python-std", Package: "os"},
		"json":                               {Ecosystem: "python-std", Package: "json"},
		"from typing import List":            {Ecosystem: "python-std", Package: "typing"},
		"requests":                           {Ecosystem: "pypi", Package: "requests", Version: ">=2.31"},
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
