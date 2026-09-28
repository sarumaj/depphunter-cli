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

// Verifies: REQ-RS-002, REQ-RS-003, REQ-RS-004, REQ-RS-005, REQ-RS-006, REQ-RS-007, REQ-RS-008
func TestResolution(t *testing.T) {
	results := analyze(t)
	langtest.CheckImports(t, results["app/src/main.rs"], map[string]lang.Target{
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
	langtest.CheckImports(t, results["app/src/net/mod.rs"], map[string]lang.Target{
		"mod server":                  {Local: "app/src/net/server.rs"},
		"use super::config::Settings": {Local: "app/src/config.rs"},
	})
	langtest.CheckImports(t, results["app/src/net/server.rs"], map[string]lang.Target{
		"use crate::config": {Local: "app/src/config.rs"},
		"use super":         {Local: "app/src/net/mod.rs"},
	})
}

func TestSymbols(t *testing.T) {
	langtest.CheckSymbols(t, analyze(t)["app/src/main.rs"], map[string]string{"main": "func", "App": "struct", "App.run": "method"})
}

// Verifies: REQ-RS-001
func TestExpandUse(t *testing.T) {
	got := expandUse("crate::a::{self, b::{c, d as e}, f::*}")
	want := []string{"crate::a", "crate::a::b::c", "crate::a::b::d", "crate::a::f"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestLockTree checks the crate graph Cargo.lock resolves: --resolve-depth walks it
// without asking crates.io anything.
//
// Verifies: REQ-SUP-009
func TestLockTree(t *testing.T) {
	r, err := (Plugin{}).Resolver("testdata/repo", langtest.Files(t, "testdata/repo"))
	if err != nil {
		t.Fatal(err)
	}
	transitive, ok := r.(lang.Transitive)
	if !ok {
		t.Fatal("the resolver cannot answer for transitive dependencies")
	}
	got := transitive.Dependencies(lang.Target{Ecosystem: "crates", Package: "serde"})
	if len(got) != 1 || got[0].Package != "serde_derive" || got[0].Version != "1.0.200" || !got[0].Pinned {
		t.Fatalf("serde depends on %+v, want serde_derive 1.0.200 pinned", got)
	}
	// "syn 2.0.60" names a version beside the crate; only the name is the edge.
	if got = transitive.Dependencies(lang.Target{Ecosystem: "crates", Package: "serde_derive"}); len(got) != 1 || got[0].Package != "syn" {
		t.Errorf("serde_derive depends on %+v, want syn", got)
	}
}

// A lock file holding one crate in two versions - syn 1 beside syn 2 is the usual
// case - pins each dependency to the version its own requirement means, and walks
// each version's own dependencies.
//
// Verifies: REQ-RS-007, REQ-SUP-009
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

	results := langtest.Analyze(t, Plugin{}, root)
	langtest.CheckImports(t, results["src/main.rs"], map[string]lang.Target{
		"use syn::Item":  {Ecosystem: "crates", Package: "syn", Version: "2.0.48", Requested: "2", Pinned: true},
		"use old::Thing": {Ecosystem: "crates", Package: "old", Version: "0.3.1", Requested: "0.3", Pinned: true},
	})

	r, err := (Plugin{}).Resolver(root, langtest.Files(t, root))
	if err != nil {
		t.Fatal(err)
	}
	transitive := r.(lang.Transitive)
	got := transitive.Dependencies(lang.Target{Ecosystem: "crates", Package: "old", Version: "0.3.1"})
	if len(got) != 1 || got[0].Package != "syn" || got[0].Version != "1.0.109" {
		t.Errorf("old depends on %+v, want syn 1.0.109", got)
	}
	if got := transitive.Dependencies(lang.Target{Ecosystem: "crates", Package: "syn", Version: "2.0.48"}); len(got) != 1 {
		t.Errorf("syn 2 depends on %+v, want unicode-ident alone", got)
	}
	if got := transitive.Dependencies(lang.Target{Ecosystem: "crates", Package: "syn", Version: "1.0.109"}); len(got) != 2 {
		t.Errorf("syn 1 depends on %+v, want quote and unicode-ident", got)
	}
}

// A dependency from an alternative registry carries it - the name Cargo.toml gives,
// else the index URL Cargo.lock records - so that only that registry is asked
// about it; a crates.io dependency carries none.
//
// Verifies: REQ-RS-010
func TestAlternativeRegistryTravelsWithTheCrate(t *testing.T) {
	root := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("Cargo.toml", "[package]\nname = \"app\"\n\n[dependencies]\n"+
		"billing = { version = \"1\", registry = \"corp\" }\nledger = \"2\"\nserde = \"1\"\n")
	write("Cargo.lock", `version = 3

[[package]]
name = "billing"
version = "1.2.0"
source = "registry+https://crates.corp.example/index"
dependencies = ["serde"]

[[package]]
name = "ledger"
version = "2.0.1"
source = "sparse+https://crates.corp.example/index/"

[[package]]
name = "serde"
version = "1.0.200"
source = "registry+https://github.com/rust-lang/crates.io-index"
`)
	os.MkdirAll(filepath.Join(root, "src"), 0o755)
	write("src/main.rs", "use billing::Invoice;\nuse ledger::Entry;\nuse serde::Serialize;\nfn main() {}\n")

	results := langtest.Analyze(t, Plugin{}, root)
	langtest.CheckImports(t, results["src/main.rs"], map[string]lang.Target{
		"use billing::Invoice": {Ecosystem: "crates", Package: "billing", Version: "1.2.0", Requested: "1", Pinned: true, Registry: "corp"},
		"use ledger::Entry":    {Ecosystem: "crates", Package: "ledger", Version: "2.0.1", Requested: "2", Pinned: true, Registry: "https://crates.corp.example/index"},
		"use serde::Serialize": {Ecosystem: "crates", Package: "serde", Version: "1.0.200", Requested: "1", Pinned: true},
	})
	r, err := (Plugin{}).Resolver(root, langtest.Files(t, root))
	if err != nil {
		t.Fatal(err)
	}
	got := r.(lang.Transitive).Dependencies(lang.Target{Ecosystem: "crates", Package: "billing", Version: "1.2.0"})
	if len(got) != 1 || got[0].Package != "serde" || got[0].Registry != "" {
		t.Errorf("billing depends on %+v, want crates.io's serde", got)
	}
}
