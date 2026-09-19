package rust

import (
	"context"
	"reflect"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/scan"
)

// testdata/repo: a Cargo workspace with a workspace dependency, a renamed dependency,
// a path dependency, Cargo.lock pins and mod.rs / crate:: / super:: module paths.
func analyse(t *testing.T) map[string]*lang.FileResult {
	t.Helper()
	const root = "testdata/repo"
	files, err := scan.Scan(context.Background(), root, scan.Options{})
	if err != nil {
		t.Fatal(err)
	}
	res, err := lang.Analyze(context.Background(), Plugin{}, root, files)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func targets(t *testing.T, res *lang.FileResult) map[string]lang.Target {
	t.Helper()
	if res == nil {
		t.Fatal("file not analysed")
	}
	out := map[string]lang.Target{}
	for _, im := range res.Imports {
		out[im.Spec] = im.Target
	}
	return out
}

func check(t *testing.T, got, want map[string]lang.Target) {
	t.Helper()
	for spec, w := range want {
		if g, ok := got[spec]; !ok {
			t.Errorf("%s: not captured (got %v)", spec, got)
		} else if g != w {
			t.Errorf("%s: got %+v, want %+v", spec, g, w)
		}
	}
	if len(got) != len(want) {
		t.Errorf("got %d imports, want %d: %v", len(got), len(want), got)
	}
}

func TestResolution(t *testing.T) {
	res := analyse(t)
	check(t, targets(t, res["app/src/main.rs"]), map[string]lang.Target{
		"use std::collections::HashMap":  {Ecosystem: "rust-std", Package: "std"},
		"use crate::net":                 {Local: "app/src/net/mod.rs"},
		"use crate::net::server::Server": {Local: "app/src/net/server.rs"},
		"use serde::Deserialize":         {Ecosystem: "crates", Package: "serde", Version: "1.0.200"},
		"use tokio::runtime":             {Ecosystem: "crates", Package: "tokio", Version: "1.37.0"},
		"use core_lib::util":             {Local: "core-lib/src/util.rs"},
		"use json::Value":                {Ecosystem: "crates", Package: "serde_json", Version: "1.0.117"},
		"use rand::Rng":                  {Ecosystem: "crates", Package: "rand", Unresolved: true},
		"extern crate alloc":             {Ecosystem: "rust-std", Package: "alloc"},
		"mod net":                        {Local: "app/src/net/mod.rs"},
		"use Mode":                       {}, // a local enum, not a crate
		"mod config":                     {Local: "app/src/config.rs"},
	})
	check(t, targets(t, res["app/src/net/mod.rs"]), map[string]lang.Target{
		"mod server":                  {Local: "app/src/net/server.rs"},
		"use super::config::Settings": {Local: "app/src/config.rs"},
	})
	check(t, targets(t, res["app/src/net/server.rs"]), map[string]lang.Target{
		"use crate::config": {Local: "app/src/config.rs"},
		"use super":         {Local: "app/src/net/mod.rs"},
	})
}

func TestSymbols(t *testing.T) {
	got := map[string]string{}
	for _, s := range analyse(t)["app/src/main.rs"].Symbols {
		got[s.Name] = s.Kind
	}
	want := map[string]string{"main": "func", "App": "struct", "App.run": "method"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestExpandUse(t *testing.T) {
	got := expandUse("crate::a::{self, b::{c, d as e}, f::*}")
	want := []string{"crate::a", "crate::a::b::c", "crate::a::b::d", "crate::a::f"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
