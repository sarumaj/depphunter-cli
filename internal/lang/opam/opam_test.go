package opam

import (
	"reflect"
	"testing"
)

// A description's depends formula: filters with version constraints and
// variables, alternatives in groups, `{= version}`, comments of both kinds, triple-
// quoted strings, sections, and pin-depends as a list of pairs.
//
// Verifies: REQ-OCAML-007
func TestRead(t *testing.T) {
	src := `opam-version: "2.0"
name: "shop" # the package
version: "1.0"
description: """A "quoted" description"""
(* depends: ["commented"] *)
depends: [
  "ocaml" {>= "4.14"}
  "dune" {>= "3.0" & build}
  "lwt" {>= "5.6" & < "6"}
  "yojson" {= "2.1.0"}
  "alcotest" {with-test}
  ("ssl" | "tls" {>= "0.17" | = "0.16"})
  "shop-core" {= version}
  "conf-libev" {os != "win32"}
]
url { src: "https://example.com/x.tgz" }
pin-depends: ["cmdliner.dev" "git+https://github.com/dbuenzli/cmdliner.git#main"]
`
	f := Read([]byte(src))
	if f.Name != "shop" || f.Version != "1.0" {
		t.Errorf("name %q version %q", f.Name, f.Version)
	}
	want := []Dep{
		{Name: "ocaml", Constraint: ">= 4.14", Line: 7},
		{Name: "dune", Constraint: ">= 3.0", Flags: []string{"build"}, Line: 8},
		{Name: "lwt", Constraint: ">= 5.6 & < 6", Line: 9},
		{Name: "yojson", Constraint: "= 2.1.0", Exact: "2.1.0", Line: 10},
		{Name: "alcotest", Flags: []string{"with-test"}, Line: 11},
		{Name: "ssl", Line: 12},
		{Name: "tls", Constraint: ">= 0.17 | = 0.16", Line: 12},
		{Name: "shop-core", Constraint: "= version", Line: 13},
		{Name: "conf-libev", Line: 14},
	}
	if !reflect.DeepEqual(f.Depends, want) {
		t.Errorf("depends\n got %+v\nwant %+v", f.Depends, want)
	}
	if pins := []Pin{{Name: "cmdliner", Version: "dev", URL: "git+https://github.com/dbuenzli/cmdliner.git#main", Line: 17}}; !reflect.DeepEqual(f.Pins, pins) {
		t.Errorf("pins %+v", f.Pins)
	}
	if !Compiler("ocaml-base-compiler") || !Compiler("base-unix") || Compiler("dune") || !ExactVersion("v0.16.0") || ExactVersion(">= 1") {
		t.Error("Compiler/ExactVersion")
	}
}
