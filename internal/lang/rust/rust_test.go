package rust

import (
	"reflect"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
)

// testdata/repo: a Cargo workspace with a workspace dependency, a renamed dependency,
// a path dependency, Cargo.lock pins and mod.rs / crate:: / super:: module paths.
func analyze(t *testing.T) map[string]*lang.FileResult {
	t.Helper()
	return langtest.Analyze(t, Plugin{}, "testdata/repo")
}

func TestResolution(t *testing.T) {
	res := analyze(t)
	langtest.CheckImports(t, res["app/src/main.rs"], map[string]lang.Target{
		"use std::collections::HashMap":  {Ecosystem: "rust-std", Package: "std"},
		"use crate::net":                 {Local: "app/src/net/mod.rs"},
		"use crate::net::server::Server": {Local: "app/src/net/server.rs"},
		"use serde::Deserialize":         {Ecosystem: "crates", Package: "serde", Version: "1.0.200", Requested: "1.0", Pinned: true},
		"use tokio::runtime":             {Ecosystem: "crates", Package: "tokio", Version: "1.37.0", Requested: "1", Pinned: true},
		"use core_lib::util":             {Local: "core-lib/src/util.rs"},
		"use json::Value":                {Ecosystem: "crates", Package: "serde_json", Version: "1.0.117", Requested: "1", Pinned: true},
		"use rand::Rng":                  {Ecosystem: "crates", Package: "rand", Unresolved: true},
		// Cargo.toml alone never pins: "1.0.86" means ^1.0.86 until Cargo.lock says otherwise.
		"use anyhow::Result": {Ecosystem: "crates", Package: "anyhow", Version: "1.0.86"},
		"extern crate alloc": {Ecosystem: "rust-std", Package: "alloc"},
		"mod net":            {Local: "app/src/net/mod.rs"},
		"use Mode":           {}, // a local enum, not a crate
		"mod config":         {Local: "app/src/config.rs"},
	})
	langtest.CheckImports(t, res["app/src/net/mod.rs"], map[string]lang.Target{
		"mod server":                  {Local: "app/src/net/server.rs"},
		"use super::config::Settings": {Local: "app/src/config.rs"},
	})
	langtest.CheckImports(t, res["app/src/net/server.rs"], map[string]lang.Target{
		"use crate::config": {Local: "app/src/config.rs"},
		"use super":         {Local: "app/src/net/mod.rs"},
	})
}

func TestSymbols(t *testing.T) {
	langtest.CheckSymbols(t, analyze(t)["app/src/main.rs"], map[string]string{"main": "func", "App": "struct", "App.run": "method"})
}

func TestExpandUse(t *testing.T) {
	got := expandUse("crate::a::{self, b::{c, d as e}, f::*}")
	want := []string{"crate::a", "crate::a::b::c", "crate::a::b::d", "crate::a::f"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestLockTree checks the crate graph Cargo.lock resolves: --resolve-depth walks it
// without asking crates.io anything.
func TestLockTree(t *testing.T) {
	r, err := (Plugin{}).Resolver("testdata/repo", langtest.Files(t, "testdata/repo"))
	if err != nil {
		t.Fatal(err)
	}
	tr, ok := r.(lang.Transitive)
	if !ok {
		t.Fatal("the resolver cannot answer for transitive dependencies")
	}
	got := tr.Dependencies(lang.Target{Ecosystem: "crates", Package: "serde"})
	if len(got) != 1 || got[0].Package != "serde_derive" || got[0].Version != "1.0.200" || !got[0].Pinned {
		t.Fatalf("serde depends on %+v, want serde_derive 1.0.200 pinned", got)
	}
	// "syn 2.0.60" names a version beside the crate; only the name is the edge.
	if got = tr.Dependencies(lang.Target{Ecosystem: "crates", Package: "serde_derive"}); len(got) != 1 || got[0].Package != "syn" {
		t.Errorf("serde_derive depends on %+v, want syn", got)
	}
}
