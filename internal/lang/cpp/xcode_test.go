package cpp

import (
	"strings"
	"testing"
	"time"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
)

// xcodeProject is a project file whose build settings spell header search paths in
// every form Xcode writes them: a list and a single string, $(inherited), $(SRCROOT)
// and $(PROJECT_DIR), escaped quotes around a path with a space, a relative entry, a
// recursive /**, a setting defined beside them, a conditional setting - and entries
// that must be dropped: an absolute path, one outside the repository, one naming a
// build product, a variable with an operator, a directory the repository lacks.
const xcodeProject = `// !$*UTF8*$!
{
	objects = {
/* Begin XCBuildConfiguration section */
		1A2B /* Debug */ = {
			isa = XCBuildConfiguration;
			buildSettings = {
				THIRD = "$(SRCROOT)/third";
				HEADER_SEARCH_PATHS = (
					"$(THIRD:dir)hdr",
					"$(inherited)",
					"$(SRCROOT)/Vendor/**",
					"\"$(PROJECT_DIR)/Lib Headers\"",
					/usr/include,
					"$(SRCROOT)/../outside",
					"$(BUILT_PRODUCTS_DIR)/include",
					"$(SRCROOT:dir)/x",
					"$(THIRD)/api",
					"$(SRCROOT)/missing",
					"$(UNKNOWN_ROOT)ios",
				);
				USER_HEADER_SEARCH_PATHS = "hdr \"quoted dir\"";
				"HEADER_SEARCH_PATHS[sdk=iphoneos*]" = "${SRCROOT}/ios";
			};
			name = Debug;
		};
		3C4D /* Release */ = {
			isa = XCBuildConfiguration;
			buildSettings = {
				THIRD = "$(SRCROOT)/elsewhere";
			};
			name = Release;
		};
/* End XCBuildConfiguration section */
	};
}
`

// Verifies: REQ-CPP-004, REQ-OBJC-015
func TestXcodeHeaderSearchPaths(t *testing.T) {
	root := langtest.Write(t, map[string]string{
		"App.xcodeproj/project.pbxproj": xcodeProject,
		"App/main.m":                    "",
		"Vendor/a/deep.h":               "",
		"Vendor/a/b/deep.h":             "",
		"Vendor/a/b/nested/only.h":      "",
		"Lib Headers/lh.h":              "",
		"third/api/t.h":                 "",
		"hdr/u.h":                       "",
		"quoted dir/q.h":                "",
		"ios/i.h":                       "",
		"missing.h":                     "",
		"Configs/Base.xcconfig":         "#include \"Shared.xcconfig\"\nHEADER_SEARCH_PATHS = $(inherited) $(SHARED)/inc // $(SRCROOT)/commented\n",
		"Configs/Shared.xcconfig":       "SHARED = $(PARENT)/shared\nPARENT = $(SRCROOT)\nSHARED = $(inherited)\n",
		"shared/inc/s.h":                "",
		"commented/c.h":                 "",
		"sub/Other.xcodeproj/project.pbxproj": `{ objects = { 1 = { buildSettings = { HEADER_SEARCH_PATHS = "$(SRCROOT)/subinc"; }; };` +
			`2 = {isa = PBXFileReference; name = "Referenced.xcconfig"; path = "../Configs/Referenced.xcconfig"; }; }; }`,
		"sub/src/x.m":                 "",
		"sub/Configs/Sub.xcconfig":    "HEADER_SEARCH_PATHS = subconfig\n",
		"Configs/Referenced.xcconfig": "#include \"Settings.xcconfig\"\nHEADER_SEARCH_PATHS = $(inherited) \"$(SRCROOT)/../referenced\"\n",
		"Configs/Settings.xcconfig":   "HEADER_SEARCH_PATHS = included\n",
		"sub/included/i2.h":           "",
		"included/i2.h":               "",
		"referenced/r.h":              "",
		"sub/subconfig/c2.h":          "",
		"elsewhere/api/t.h":           "",
		"sub/subinc/sub.h":            "",
	})
	r := newResolver(root, langtest.Files(t, root))
	for _, test := range []struct {
		file, include, want string
	}{
		{"App/main.m", "deep.h", "Vendor/a/deep.h"}, // recursive: the shallowest
		{"App/main.m", "b/deep.h", "Vendor/a/b/deep.h"},
		{"App/main.m", "only.h", "Vendor/a/b/nested/only.h"},
		{"App/main.m", "lh.h", "Lib Headers/lh.h"},
		{"App/main.m", "t.h", "third/api/t.h"},
		{"App/main.m", "u.h", "hdr/u.h"},
		{"App/main.m", "q.h", "quoted dir/q.h"},
		{"App/main.m", "i.h", "ios/i.h"},
		{"App/main.m", "s.h", "shared/inc/s.h"},
		{"sub/src/x.m", "sub.h", "sub/subinc/sub.h"},
		{"sub/src/x.m", "c2.h", "sub/subconfig/c2.h"},
		{"sub/src/x.m", "r.h", "referenced/r.h"}, // for the project naming it
		{"App/main.m", "r.h", ""},
		{"sub/src/x.m", "i2.h", "sub/included/i2.h"}, // through the file including it
		{"App/main.m", "i2.h", ""},
		{"sub/src/x.m", "lh.h", "Lib Headers/lh.h"}, // the root project's paths too
		{"App/main.m", "sub.h", ""},                 // not another project's
		{"App/main.m", "c.h", ""},                   // a comment
	} {
		got := r.xcode.find(test.file, test.include, r.files, r.byBase)
		if got != test.want {
			t.Errorf("%s includes <%s>: got %q, want %q", test.file, test.include, got, test.want)
		}
	}
	var directories []string
	for _, entry := range r.xcode.byProject[""] {
		directories = append(directories, entry.directory)
	}
	want := "Vendor Lib Headers third/api hdr quoted dir ios shared/inc"
	if got := strings.Join(directories, " "); got != want {
		t.Errorf("root project searches %q, want %q", got, want)
	}
	// Through the include resolution: after the includer's directory, before the
	// conventional ones.
	if got := r.Resolve("App/main.m", lang.RawImport{Module: "lh.h"}); got != (lang.Target{Local: "Lib Headers/lh.h"}) {
		t.Errorf("<lh.h>: %+v", got)
	}
}

// Broken project files and .xcconfig files - cut anywhere, include cycles,
// self-referencing settings - give no panic and end in bounded time.
//
// Verifies: REQ-OBJC-015
func TestXcodeBrokenSettings(t *testing.T) {
	files := map[string]string{
		"a.xcconfig": "#include \"b.xcconfig\"\nLOOP = $(LOOP)/x\nHEADER_SEARCH_PATHS = $(LOOP) \"unterminated\n",
		"b.xcconfig": "#include \"a.xcconfig\"\n#include \"../../etc/x.xcconfig\"\n",
		"inc/h.h":    "",
	}
	start := time.Now()
	for cut := 0; cut <= len(xcodeProject); cut += 7 {
		files["P.xcodeproj/project.pbxproj"] = xcodeProject[:cut]
		root := langtest.Write(t, files)
		readXcode(langtest.Files(t, root))
	}
	settings, paths := projectSettings("buildSettings = { A = (\"x\", ; B = \"$(A)\"; } buildSettings buildSettings = {")
	if len(paths) != 0 || len(settings) != 0 {
		t.Errorf("broken dictionary read as %v %v", settings, paths)
	}
	if got := expandXcode("$(A)", map[string]string{"A": "$(A)"}, 0); !strings.Contains(got, unexpanded) {
		t.Errorf("a self-referencing setting expanded to %q", got)
	}
	if elapsed := time.Since(start); elapsed > 20*time.Second {
		t.Errorf("took %v", elapsed)
	}
}
