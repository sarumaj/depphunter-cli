package ada

import "strings"

// stdRoots are the roots of the predefined language environment (Ada.*,
// System.*, Interfaces.*) and of GNAT's own library (GNAT.*), which every
// compiler installation ships.
var stdRoots = map[string]bool{"ada": true, "system": true, "interfaces": true, "gnat": true}

// stdUnits are predefined library units outside those roots: Standard and the
// Ada 83 names the language keeps as renamings (Text_IO is Ada.Text_IO).
var stdUnits = map[string]bool{
	"standard": true, "text_io": true, "sequential_io": true, "direct_io": true, "io_exceptions": true,
	"unchecked_conversion": true, "unchecked_deallocation": true, "calendar": true, "machine_code": true,
}

// stdPackage is the ada-std package of a predefined unit: its first two
// segments (ada.containers for Ada.Containers.Vectors), in lower case.
func stdPackage(unit string) (string, bool) {
	segments := strings.Split(unit, ".")
	if len(segments) == 1 && stdUnits[unit] {
		return unit, true
	}
	if !stdRoots[segments[0]] {
		return "", false
	}
	if len(segments) > 2 {
		segments = segments[:2]
	}
	return strings.Join(segments, "."), true
}

// knownCrates maps unit prefixes (lower case) to the Alire crate that provides
// them, for units whose names do not spell their crate. The longest matching
// prefix wins.
var knownCrates = map[string]string{
	"aws":                     "aws",
	"soap":                    "aws",
	"templates_parser":        "templates_parser",
	"gnatcoll":                "gnatcoll",
	"gnatcoll.sql":            "gnatcoll_sql",
	"gnatcoll.sql.sqlite":     "gnatcoll_sqlite",
	"gnatcoll.sql.postgres":   "gnatcoll_postgres",
	"gnatcoll.iconv":          "gnatcoll_iconv",
	"gnatcoll.gmp":            "gnatcoll_gmp",
	"gnatcoll.python":         "gnatcoll_python3",
	"gnatcoll.readline":       "gnatcoll_readline",
	"gnatcoll.syslog":         "gnatcoll_syslog",
	"gnatcoll.coders.zlib":    "gnatcoll_zlib",
	"gnatcoll.coders.lzma":    "gnatcoll_lzma",
	"gnatcoll.omp":            "gnatcoll_omp",
	"gnatcoll.xref":           "gnatcoll_xref",
	"gnatcoll.projects":       "gnatcoll_projects",
	"aunit":                   "aunit",
	"sax":                     "xmlada",
	"dom":                     "xmlada",
	"schema":                  "xmlada",
	"unicode":                 "xmlada",
	"input_sources":           "xmlada",
	"glib":                    "gtkada",
	"gtk":                     "gtkada",
	"gdk":                     "gtkada",
	"pango":                   "gtkada",
	"cairo":                   "gtkada",
	"gtkada":                  "gtkada",
	"libadalang":              "libadalang",
	"langkit_support":         "langkit_support",
	"gpr":                     "libgpr",
	"gpr2":                    "libgpr2",
	"vss":                     "vss_text",
	"vss.json":                "vss_extra",
	"vss.xml":                 "vss_extra",
	"vss.http":                "vss_extra",
	"vss.command_line":        "vss_extra",
	"vss.regular_expressions": "vss_extra",
	"toml":                    "ada_toml",
	"json":                    "json",
	"spark":                   "sparklib",
	"sparklib":                "sparklib",
	"league":                  "matreshka_league",
	"matreshka":               "matreshka_league",
	"util":                    "utilada",
	"terminal_interface":      "ncursesada",
	"sdl":                     "sdlada",
	"gnoga":                   "gnoga",
	"spawn":                   "spawn",
	"markdown":                "markdown",
	"gnatdoc":                 "libgnatdoc",
	"lal_refactor":            "liblal_refactor",
	"trendy_test":             "trendy_test",
	"posix":                   "florist_blady",
	"semantic_versioning":     "semantic_versioning",
	"simple_logging":          "simple_logging",
	"clic":                    "clic",
	"aaa":                     "aaa",
	"uri":                     "uri_ada",
	"ansi":                    "ansiada",
	"zlib":                    "zlib_ada",
	"adasat":                  "adasat",
	"prettier_ada":            "prettier_ada",
	"gnatformat":              "libgnatformat",
	"hal":                     "hal",
	"cortex_m":                "cortex_m",
	"atomic":                  "atomic",
	"bbqueue":                 "bbqueue",
}

// knownProjects maps GNAT project names (lower case, without .gpr) to the
// crate that ships them, where the two differ.
var knownProjects = map[string]string{
	"gpr":              "libgpr",
	"gpr2":             "libgpr2",
	"gnatcoll_core":    "gnatcoll",
	"xml_ez_out":       "xmlezout",
	"lal_refactor":     "liblal_refactor",
	"libgnatdoc":       "libgnatdoc",
	"vss_text":         "vss_text",
	"gnatcoll_python":  "gnatcoll_python3",
	"sparklib_light":   "sparklib",
	"florist":          "florist_blady",
	"ncursesada":       "ncursesada",
	"utilada_core":     "utilada",
	"utilada_sys":      "utilada",
	"matreshka_league": "matreshka_league",
}

// knownCrate is the table's crate for a unit and the number of segments the
// matching prefix has.
func knownCrate(unit string) (string, int) {
	segments := strings.Split(unit, ".")
	for k := len(segments); k > 0; k-- {
		if c, ok := knownCrates[strings.Join(segments[:k], ".")]; ok {
			return c, k
		}
	}
	return "", 0
}
