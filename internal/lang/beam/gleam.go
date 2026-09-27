package beam

import "strings"

// gleamStdlib are gleam_stdlib's modules, past and present: gleam/<name>.
var gleamStdlib = setOf(`base bit_array bit_builder bit_string bool bytes_builder bytes_tree dict dynamic
float function int io iterator list map option order pair queue regex result set string
string_builder string_tree uri`)

// GleamPackage names the Hex package of a Gleam module path (gleam/erlang/process,
// lustre/element): the longest run of leading segments joined by "_" that the
// project knows (gleam_erlang, lustre); else, for gleam/<x>, gleam_stdlib when
// x is one of its modules, gleam_community_<y> for gleam/community/<y> and
// gleam_<x> otherwise; else the first segment. known reports whether the
// project declares or locks the name. The Gleam plugin and Erlang calls of
// compiled Gleam modules (gleam@list) share it.
//
// Implements: REQ-GLEAM-004, REQ-GLEAM-009
func GleamPackage(module string, known func(string) bool) (name string, ok bool) {
	segs := strings.Split(module, "/")
	for k := len(segs); k > 0; k-- {
		if name := strings.Join(segs[:k], "_"); known(name) {
			return name, true
		}
	}
	name = segs[0]
	if segs[0] == "gleam" && len(segs) > 1 {
		switch {
		case gleamStdlib[segs[1]]:
			name = "gleam_stdlib"
		case segs[1] == "community" && len(segs) > 2:
			name = "gleam_community_" + segs[2]
		default:
			name = "gleam_" + segs[1]
		}
	}
	return name, known(name)
}

// gleamModule is the Gleam module path of a source file: its path under the
// nearest src/, test/ or dev/ directory, without .gleam.
func gleamModule(file string) (string, bool) {
	rest, ok := strings.CutSuffix(file, ".gleam")
	if !ok {
		return "", false
	}
	segs := strings.Split(rest, "/")
	for i := len(segs) - 2; i >= 0; i-- {
		if s := segs[i]; s == "src" || s == "test" || s == "dev" {
			return strings.Join(segs[i+1:], "/"), true
		}
	}
	return "", false
}
