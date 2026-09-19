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
