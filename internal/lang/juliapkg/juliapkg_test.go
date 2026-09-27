package juliapkg

import "testing"

// A [compat] entry reads as Pkg reads it: a bare version is a caret range, ~ a
// tilde range, = an equality (a bound admitting what its parts leave open), an
// inequality or a hyphen range; comma-separated parts are a union.
//
// Verifies: REQ-JULIA-008
func TestCompatRanges(t *testing.T) {
	for spec, cases := range map[string]map[string]bool{
		"1.2":         {"1.2.0": true, "1.9.9": true, "2.0.0": false, "1.1.9": false},
		"0.2.3":       {"0.2.3": true, "0.2.9": true, "0.3.0": false},
		"0.0.3":       {"0.0.3": true, "0.0.4": false},
		"^0":          {"0.9.0": true, "1.0.0": false},
		"~1.2.3":      {"1.2.5": true, "1.3.0": false},
		"~1":          {"1.9.0": true, "2.0.0": false},
		"=1.2.3":      {"1.2.3": true, "1.2.4": false},
		"=1.2":        {"1.2.7": true, "1.3.0": false},
		">= 1.5":      {"1.5.0": true, "9.0.0": true, "1.4.9": false},
		"≥ 1.5":       {"1.5.0": true, "1.4.9": false},
		"< 2":         {"1.99.0": true, "2.0.0": false},
		"1.2 - 1.5":   {"1.5.9": true, "1.6.0": false, "1.1.0": false},
		"0.21, 1":     {"0.21.4": true, "1.3.0": true, "0.22.0": false, "2.0.0": false},
		"0.10.1 - 0":  {"0.99.0": true, "1.0.0": false},
		"1.2.0 - 1.5": {"1.2.0": true, "1.5.3": true},
	} {
		ranges, ok := CompatRanges(spec)
		if !ok {
			t.Errorf("%q: not read", spec)
			continue
		}
		for v, want := range cases {
			pv, _ := ParseVersion(v)
			got := false
			for _, r := range ranges {
				got = got || r.Contains(pv)
			}
			if got != want {
				t.Errorf("%q admits %s: %v, want %v", spec, v, got, want)
			}
		}
	}
	if _, ok := CompatRanges("latest"); ok {
		t.Error("latest read as a range")
	}
}

// The registry's range keys: "1" is every 1.x, "0.21.0" one release, "0 - 0.20.0"
// and the compressed "0.1-0.3" ranges, "*" unbounded.
//
// Verifies: REQ-SUP-055
func TestRegistryRange(t *testing.T) {
	for spec, cases := range map[string]map[string]bool{
		"1":          {"1.0.0": true, "1.9.3": true, "2.0.0": false, "0.9.0": false},
		"0.21.0":     {"0.21.0": true, "0.21.1": false},
		"0 - 0.20.0": {"0.19.5": true, "0.20.0": true, "0.20.1": false},
		"0.21.2 - 0": {"0.21.2": true, "0.99.0": true, "1.0.0": false, "0.21.1": false},
		"0.1-0.3":    {"0.3.9": true, "0.4.0": false},
		"1.2-*":      {"9.0.0": true, "1.1.0": false},
	} {
		r, ok := RegistryRange(spec)
		if !ok {
			t.Errorf("%q: not read", spec)
			continue
		}
		for v, want := range cases {
			pv, _ := ParseVersion(v)
			if got := r.Contains(pv); got != want {
				t.Errorf("%q contains %s: %v, want %v", spec, v, got, want)
			}
		}
	}
}

// Verifies: REQ-JULIA-008, REQ-SUP-055
func TestNewestAndExact(t *testing.T) {
	versions := []string{"0.21.4", "1.0.0", "1.4.1", "1.10.0", "2.0.0-rc1"}
	r, _ := CompatRanges("1")
	if got := Newest(versions, r); got != "1.10.0" {
		t.Errorf("newest 1.x: %s", got)
	}
	if got := Newest(versions, nil); got != "1.10.0" {
		t.Errorf("newest: %s", got)
	}
	r, _ = CompatRanges("3")
	if got := Newest(versions, r); got != "" {
		t.Errorf("none admitted: %s", got)
	}
	for spec, want := range map[string]string{"=1.2.3": "1.2.3", "= 1.2.3": "1.2.3", "=1.2": "", "1.2.3": "", "=1.2.3, =1.2.4": ""} {
		if got, _ := ExactCompat(spec); got != want {
			t.Errorf("ExactCompat(%q) = %q, want %q", spec, got, want)
		}
	}
	if !Stdlib("LinearAlgebra") || Stdlib("DataFrames") || RegistryDir("libpng_jll") != "L/libpng_jll" {
		t.Error("tables")
	}
}
