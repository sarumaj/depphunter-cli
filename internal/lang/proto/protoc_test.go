package proto

import (
	"reflect"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
)

// A protoc build without Buf: a Makefile gives protoc its roots through variables,
// the current directory and both flag spellings; a script in a service directory
// points above itself. Every import also exists under a decoy directory, so only the
// scripts' roots resolve it (a unique suffix would not). A Buf module falls back to
// the scripts' roots too. A binary Makefile and one with broken flags add nothing.
//
// Verifies: REQ-PROTO-004
func TestProtocImportRoots(t *testing.T) {
	msg := "syntax = \"proto3\";\nmessage M {}\n"
	root := langtest.Write(t, map[string]string{
		"Makefile": "PROTO_DIR := schemas\nTHIRD = deps\n" +
			"gen:\n\tprotoc -I$(PROTO_DIR) \\\n\t  --proto_path=$(THIRD) --proto_path $(CURDIR)/extra \\\n" +
			"\t  -I /usr/include -I$(GOPATH)/src --go_out=. $(PROTO_DIR)/app/v1/app.proto\n",
		"schemas/app/v1/app.proto": "syntax = \"proto3\";\nimport \"common/v1/types.proto\";\n" +
			"import \"ext/opts.proto\";\nimport \"lib/x.proto\";\nimport \"lib2/y.proto\";\n",
		"deps/common/v1/types.proto":       msg,
		"old/common/v1/types.proto":        msg,
		"extra/ext/opts.proto":             msg,
		"other/ext/opts.proto":             msg,
		"services/b/gen.sh":                "#!/bin/sh\ncd \"$(dirname \"$0\")\"\nprotoc -I=../../lib-protos:../../missing api.proto\n",
		"services/b/api.proto":             "syntax = \"proto3\";\nimport \"lib/x.proto\";\nimport \"common/v1/types.proto\";\n",
		"lib-protos/common/v1/types.proto": msg,
		"scripts/gen.sh":                   "protoc -I shared app.proto\n", // run from the root
		"shared/lib2/y.proto":              msg,
		"old/lib2/y.proto":                 msg,
		"lib-protos/lib/x.proto":           msg,
		"old/lib/x.proto":                  msg,
		"bufmod/buf.yaml":                  "version: v1\n",
		"bufmod/a.proto":                   "syntax = \"proto3\";\nimport \"common/v1/types.proto\";\n",
		"junk/Makefile":                    "\x00\xffprotoc -I\n--proto_path=\n-I ../../../outside -I$(UNSET)/x\n",
		"junk/build.sh":                    "protoc --proto_path",
	})
	res := langtest.Analyze(t, Plugin{}, root)
	local := func(p string) lang.Target { return lang.Target{Local: p} }
	langtest.CheckImports(t, res["schemas/app/v1/app.proto"], map[string]lang.Target{
		`import "common/v1/types.proto"`: local("deps/common/v1/types.proto"),
		`import "ext/opts.proto"`:        local("extra/ext/opts.proto"),
		// The service script's root, from any importer: it is the only one naming it.
		`import "lib/x.proto"`: local("lib-protos/lib/x.proto"),
		// A root the script's directory lacks is the repository root's.
		`import "lib2/y.proto"`: local("shared/lib2/y.proto"),
	})
	// The roots of the service's own script come before the Makefile's.
	langtest.CheckImports(t, res["services/b/api.proto"], map[string]lang.Target{
		`import "lib/x.proto"`:           local("lib-protos/lib/x.proto"),
		`import "common/v1/types.proto"`: local("lib-protos/common/v1/types.proto"),
	})
	langtest.CheckImports(t, res["bufmod/a.proto"], map[string]lang.Target{
		`import "common/v1/types.proto"`: local("deps/common/v1/types.proto"),
	})

	// Without the scripts, the decoys make every import ambiguous.
	bare := langtest.Write(t, map[string]string{
		"schemas/app/v1/app.proto":   "syntax = \"proto3\";\nimport \"common/v1/types.proto\";\n",
		"deps/common/v1/types.proto": msg,
		"old/common/v1/types.proto":  msg,
		"Makefile":                   "gen:\n\tgcc -Ideps -c x.c\n", // no protoc: not read
	})
	langtest.CheckImports(t, langtest.Analyze(t, Plugin{}, bare)["schemas/app/v1/app.proto"], map[string]lang.Target{
		`import "common/v1/types.proto"`: {Ecosystem: ecoBuf, Package: "common", Unresolved: true},
	})
}

// Verifies: REQ-PROTO-004
func TestProtocFlags(t *testing.T) {
	src := "ROOT ?= api\nexport INC=inc\n" +
		"protoc -Ia -I b -I=c --proto_path=d:e;f --proto_path \"g\" '-Ih' -I$(ROOT) -I${INC} -I$INC/sub -I$(PWD) -I`pwd`/k\n" +
		"protoc -I/abs -I$(HOME)/x -I C:\\x -IC:/y -I*.proto -I"
	want := []string{"a", "b", "c", "d", "e", "f", "g", "h", "api", "inc", "inc/sub", ".", "k"}
	if got := protocFlags(src); !reflect.DeepEqual(got, want) {
		t.Errorf("protocFlags:\n got %q\nwant %q", got, want)
	}
	for p, want := range map[string]bool{
		"Makefile": true, "sub/GNUmakefile": true, "rules.mk": true, "gen.sh": true, "x.PS1": true,
		"CMakeLists.txt": true, "cmake/proto.cmake": true, "Taskfile.yml": true, "justfile": true,
		"main.go": false, "Makefile.am": false, "build.gradle": false,
	} {
		if buildScript(p) != want {
			t.Errorf("buildScript(%s) = %v", p, !want)
		}
	}
}
