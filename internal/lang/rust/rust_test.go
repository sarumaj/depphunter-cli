package rust

import (
	"os"
	"path/filepath"
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

// A lock file holding one crate in two versions - syn 1 beside syn 2 is the usual
// case - pins each dependency to the version its own requirement means, and walks
// each version's own dependencies.
func TestLockWithTwoVersionsOfACrate(t *testing.T) {
	root := t.TempDir()
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("Cargo.toml", "[package]\nname = \"app\"\n\n[dependencies]\nsyn = \"2\"\nold = \"0.3\"\n")
	write("Cargo.lock", `version = 3

[[package]]
name = "app"
version = "0.1.0"
dependencies = ["old", "syn 2.0.48"]

[[package]]
name = "old"
version = "0.3.1"
dependencies = ["syn 1.0.109"]

[[package]]
name = "syn"
version = "2.0.48"
dependencies = ["unicode-ident"]

[[package]]
name = "syn"
version = "1.0.109"
dependencies = ["quote", "unicode-ident"]

[[package]]
name = "quote"
version = "1.0.35"

[[package]]
name = "unicode-ident"
version = "1.0.12"
`)
	os.MkdirAll(filepath.Join(root, "src"), 0o755)
	write("src/main.rs", "use syn::Item;\nuse old::Thing;\nfn main() {}\n")

	res := langtest.Analyze(t, Plugin{}, root)
	langtest.CheckImports(t, res["src/main.rs"], map[string]lang.Target{
		"use syn::Item":  {Ecosystem: "crates", Package: "syn", Version: "2.0.48", Requested: "2", Pinned: true},
		"use old::Thing": {Ecosystem: "crates", Package: "old", Version: "0.3.1", Requested: "0.3", Pinned: true},
	})

	r, err := (Plugin{}).Resolver(root, langtest.Files(t, root))
	if err != nil {
		t.Fatal(err)
	}
	tr := r.(lang.Transitive)
	got := tr.Dependencies(lang.Target{Ecosystem: "crates", Package: "old", Version: "0.3.1"})
	if len(got) != 1 || got[0].Package != "syn" || got[0].Version != "1.0.109" {
		t.Errorf("old depends on %+v, want syn 1.0.109", got)
	}
	if got := tr.Dependencies(lang.Target{Ecosystem: "crates", Package: "syn", Version: "2.0.48"}); len(got) != 1 {
		t.Errorf("syn 2 depends on %+v, want unicode-ident alone", got)
	}
	if got := tr.Dependencies(lang.Target{Ecosystem: "crates", Package: "syn", Version: "1.0.109"}); len(got) != 2 {
		t.Errorf("syn 1 depends on %+v, want quote and unicode-ident", got)
	}
}
