package d

import "strings"

// stdPackage is the package of D's runtime or standard library a module belongs
// to, named by its first two segments (std.algorithm, core.stdc, etc.c) or,
// for object, by itself; "" when the module is not one of them. druntime ships
// core.* and object, Phobos std.* and etc.c.*, and the compilers their
// intrinsics (ldc.*, gcc.*, dmd has none importable).
//
// Implements: REQ-DLANG-007
func stdPackage(module string) string {
	segs := strings.Split(module, ".")
	switch segs[0] {
	case "object":
		if len(segs) == 1 {
			return "object"
		}
	case "std", "core", "etc", "ldc", "gcc":
		if len(segs) == 1 {
			return segs[0]
		}
		return segs[0] + "." + segs[1]
	}
	return ""
}

// known maps module prefixes to the dub packages that publish them, most
// specific prefix first when looked up (longest match). Several candidates are
// alternatives: the first one the project declares wins, else the first. vibe.d
// was split into vibe-core, vibe-http, vibe-inet, vibe-stream and
// vibe-serialization, which vibe-d's sub-packages depend on, so a project
// declaring vibe-d gets vibe-d for all of them.
//
// Implements: REQ-DLANG-007
var known = map[string][]string{
	"vibe":              {"vibe-d"},
	"vibe.core":         {"vibe-core", "vibe-d"},
	"vibe.container":    {"vibe-container", "vibe-core", "vibe-d"},
	"vibe.http":         {"vibe-http", "vibe-d"},
	"vibe.web":          {"vibe-http", "vibe-d"},
	"vibe.inet":         {"vibe-inet", "vibe-d"},
	"vibe.stream":       {"vibe-stream", "vibe-d"},
	"vibe.data":         {"vibe-serialization", "vibe-d"},
	"vibe.internal":     {"vibe-core", "vibe-d"},
	"eventcore":         {"eventcore"},
	"diet":              {"diet-ng"},
	"taggedalgebraic":   {"taggedalgebraic"},
	"stdx.allocator":    {"stdx-allocator"},
	"mir":               {"mir-algorithm"},
	"mir.algebraic":     {"mir-core", "mir-algorithm"},
	"mir.bitop":         {"mir-core"},
	"mir.checkedint":    {"mir-core"},
	"mir.complex":       {"mir-core"},
	"mir.conv":          {"mir-core"},
	"mir.enums":         {"mir-core"},
	"mir.functional":    {"mir-core"},
	"mir.internal":      {"mir-core", "mir-algorithm"},
	"mir.math.common":   {"mir-core"},
	"mir.math.constant": {"mir-core"},
	"mir.math.ieee":     {"mir-core"},
	"mir.primitives":    {"mir-core"},
	"mir.qualifier":     {"mir-core"},
	"mir.reflection":    {"mir-core"},
	"mir.utility":       {"mir-core"},
	"mir.random":        {"mir-random"},
	"mir.ion":           {"mir-ion"},
	"mir.ser":           {"mir-ion"},
	"mir.deser":         {"mir-ion"},
	"mir.csv":           {"mir-ion"},
	"mir.yaml":          {"mir-ion"},
	"mir.blas":          {"mir-blas"},
	"mir.lapack":        {"mir-lapack"},
	"mir.optim":         {"mir-optim"},
	"mir.linux":         {"mir-linux-kernel"},
	"dyaml":             {"dyaml"},
	"tinyendian":        {"tinyendian"},
	"silly":             {"silly"},
	"unit_threaded":     {"unit-threaded"},
	"fluent.asserts":    {"fluent-asserts"},
	"dshould":           {"dshould"},
	"dunit":             {"dunit"},
	"requests":          {"requests"},
	"arsd":              {"arsd-official"},
	"ae":                {"ae"},
	"asdf":              {"asdf"},
	"msgpack":           {"msgpack-d"},
	"sdlang":            {"sdlang-d"},
	"dxml":              {"dxml"},
	"dpq2":              {"dpq2"},
	"ddbc":              {"ddbc"},
	"hibernated":        {"hibernated"},
	"libasync":          {"libasync"},
	"memutils":          {"memutils"},
	"botan":             {"botan"},
	"sumtype":           {"sumtype"},
	"deimos.openssl":    {"openssl"},
	"deimos":            {"openssl"},
	"openssl":           {"openssl"},
	"colorize":          {"colorize"},
	"scriptlike":        {"scriptlike"},
	"darg":              {"darg"},
	"gfm":               {"gfm"},
	"dlangui":           {"dlangui"},
	"dlib":              {"dlib"},
	"dparse":            {"libdparse"},
	"dsymbol":           {"dsymbol"},
	"dfmt":              {"dfmt"},
	"containers":        {"emsi_containers"},
	"dub":               {"dub"},
	"painlessjson":      {"painlessjson"},
	"mustache":          {"mustache-d"},
	"cachetools":        {"cachetools"},
	"hunt":              {"hunt"},
	"handy_httpd":       {"handy-httpd"},
	"serverino":         {"serverino"},
	"jsonizer":          {"jsonizer"},
	"x11":               {"x11"},
	"gtk":               {"gtk-d"},
	"gdk":               {"gtk-d"},
	"gio":               {"gtk-d"},
	"glib":              {"gtk-d"},
	"gobject":           {"gtk-d"},
	"cairo":             {"gtk-d"},
	"pango":             {"gtk-d"},
	"bindbc.loader":     {"bindbc-loader"},
	"bindbc.sdl":        {"bindbc-sdl"},
	"bindbc.opengl":     {"bindbc-opengl"},
	"bindbc.glfw":       {"bindbc-glfw"},
	"derelict.util":     {"derelict-util"},
	"derelict.sdl2":     {"derelict-sdl2"},
	"derelict.opengl":   {"derelict-gl3"},
	"derelict.opengl3":  {"derelict-gl3"},
	"derelict.glfw3":    {"derelict-glfw3"},
	"derelict.freetype": {"derelict-ft"},
	"automem":           {"automem"},
	"concepts":          {"concepts"},
	"dcompute":          {"dcompute"},
	"intel_intrinsics":  {"intel-intrinsics"},
	"inteli":            {"intel-intrinsics"},
	"lighttp":           {"lighttp"},
	"luad":              {"luad"},
	"pyd":               {"pyd"},
	"d2sqlite3":         {"d2sqlite3"},
	"mysql":             {"mysql-native"},
	"std_data_json":     {"std_data_json"},
	"stdx.data.json":    {"std_data_json"},
	"undead":            {"undead"},
	"argsd":             {"argsd"},
	"commandr":          {"commandr"},
	"jcli":              {"jcli"},
	"test_allocator":    {"test_allocator"},
	"bolts":             {"bolts"},
	"optional":          {"optional"},
	"dxt":               {"dxt"},
	"dpp":               {"dpp"},
	"libclang":          {"libclang"},
	"expected":          {"expected"},
	"aurorafw":          {"aurorafw"},
	"geod24":            {"geod24-bitblob"},
	"agora":             {"agora"},
	"localimport":       {"localimport"},
	"ocean":             {"ocean"},
	"turtle":            {"turtle"},
}

// table returns the candidates of the longest known prefix of a module and
// how many segments it matched. Without dub's own resolution this table and
// the declared names are all that attribute a module no fetched package has.
//
// Implements: REQ-DLANG-011
func table(module string) ([]string, int) {
	segs := strings.Split(module, ".")
	for k := len(segs); k >= 1; k-- {
		if c, ok := known[strings.Join(segs[:k], ".")]; ok {
			return c, k
		}
	}
	// bindbc.<x> and derelict.<x> are published as bindbc-<x> and derelict-<x>.
	if len(segs) >= 2 && (segs[0] == "bindbc" || segs[0] == "derelict") {
		return []string{segs[0] + "-" + segs[1]}, 2
	}
	return nil, 0
}

// fold makes package and module spellings comparable: dub names use - where
// module names use _, and case does not matter; a -d suffix (msgpack-d,
// sdlang-d) and a d- prefix are often dropped from module names.
func fold(s string) string {
	s = strings.ToLower(strings.ReplaceAll(s, "-", "_"))
	return strings.ReplaceAll(s, ".", "_")
}

// spellings are the ways a declared package's name may be written as a
// module's leading segments.
func spellings(name string) []string {
	f := fold(name)
	out := []string{f}
	if t, ok := strings.CutSuffix(f, "_d"); ok && t != "" {
		out = append(out, t)
	}
	if t, ok := strings.CutPrefix(f, "d_"); ok && t != "" {
		out = append(out, t)
	}
	if t, ok := strings.CutPrefix(f, "lib"); ok && t != "" {
		out = append(out, t)
	}
	return out
}
