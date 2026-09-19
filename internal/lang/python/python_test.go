package python

import (
	"context"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// testdata/repo: a src-layout package declared in pyproject.toml (PEP 621, dependency
// groups and Poetry tables), pinned by uv.lock, plus a requirements file and scripts.
func analyse(t *testing.T) map[string]*lang.FileResult {
	t.Helper()
	const root = "testdata/repo"
	files, err := scan.Scan(context.Background(), root, scan.Options{})
	if err != nil {
		t.Fatal(err)
	}
	var claimed []*scan.File
	for _, f := range files {
		if (Plugin{}).Claims(f) {
			claimed = append(claimed, f)
		}
	}
	res, err := Plugin{}.Analyze(context.Background(), root, files, claimed)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestImportResolution(t *testing.T) {
	res := analyse(t)
	got := map[string]lang.Target{}
	for _, im := range res["src/app/main.py"].Imports {
		got[im.Spec] = im.Target
	}
	want := map[string]lang.Target{
		"from __future__ import annotations": {Ecosystem: "python-std", Package: "__future__"},
		"os":                                 {Ecosystem: "python-std", Package: "os"},
		"os.path":                            {Ecosystem: "python-std", Package: "os"},
		"json":                               {Ecosystem: "python-std", Package: "json"},
		"from typing import List":            {Ecosystem: "python-std", Package: "typing"},
		"requests":                           {Ecosystem: "pypi", Package: "requests", Version: ">=2.31"},
		"yaml":                               {Ecosystem: "pypi", Package: "PyYAML", Version: "6.0.1"},
		"from bs4 import BeautifulSoup":      {Ecosystem: "pypi", Package: "beautifulsoup4", Version: "^4.12"},
		"numpy":                              {Ecosystem: "pypi", Package: "numpy", Version: "1.26.4"},
		"certifi":                            {Ecosystem: "pypi", Package: "certifi", Version: "2024.2.2"},
		"pytest":                             {Ecosystem: "pypi", Package: "pytest", Version: ">=8"},
		"black":                              {Ecosystem: "pypi", Package: "black", Version: "24.1.0"},
		"notdeclared.sub":                    {Ecosystem: "pypi", Package: "notdeclared", Unresolved: true},
		"from . import utils":                {Local: "src/app/utils.py"},
		"from .models import User":           {Local: "src/app/models"},
		"from .models.user import User":      {Local: "src/app/models/user.py"},
		"from ..outside import x":            {},
		"from app.helpers import thing":      {Local: "src/app/helpers.py"},
		"from app.models import *":           {Local: "src/app/models"},
	}
	for spec, w := range want {
		g, ok := got[spec]
		if !ok {
			t.Errorf("%s: not captured", spec)
		} else if g != w {
			t.Errorf("%s: got %+v, want %+v", spec, g, w)
		}
	}
	if len(got) != len(want) {
		t.Errorf("got %d imports, want %d: %v", len(got), len(want), got)
	}
}

func TestScriptDirectoryImports(t *testing.T) {
	got := map[string]lang.Target{}
	for _, im := range analyse(t)["scripts/tool.py"].Imports {
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
	for _, s := range analyse(t)["src/app/main.py"].Symbols {
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
