package cmake

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// presetKinds are the preset lists of CMakePresets.json (version 6).
var presetKinds = []string{"configurePresets", "buildPresets", "testPresets", "packagePresets", "workflowPresets"}

var presetName = regexp.MustCompile(`"name"\s*:\s*"((?:[^"\\]|\\.)*)"`)

// readPresets reads CMakePresets.json or CMakeUserPresets.json: every preset is a
// symbol, the files it includes (version 4) and a configure preset's toolchainFile
// are imports. ${sourceDir} is the presets file's directory; a toolchain elsewhere
// ($env{VCPKG_ROOT}/...) is not the project's and is not recorded.
//
// Implements: REQ-CMAKE-009
func readPresets(src []byte) *lang.Extraction {
	var p map[string]json.RawMessage
	ex := &lang.Extraction{}
	if json.Unmarshal(src, &p) != nil {
		return ex
	}
	lineOf := func(needle string) int {
		if i := bytes.Index(src, []byte(needle)); i >= 0 {
			return bytes.Count(src[:i], []byte("\n")) + 1
		}
		return 1
	}
	var includes []string
	json.Unmarshal(p["include"], &includes)
	for _, inc := range includes {
		ex.Imports = append(ex.Imports, lang.RawImport{
			Spec: `"include": "` + inc + `"`, Module: presetPath(inc), Name: kindFile, Line: lineOf(`"` + inc + `"`),
		})
	}
	names := map[string]bool{}
	for _, kind := range presetKinds {
		var list []struct {
			Name          string `json:"name"`
			ToolchainFile string `json:"toolchainFile"`
		}
		json.Unmarshal(p[kind], &list)
		for _, pr := range list {
			names[pr.Name] = true
			if tc := pr.ToolchainFile; tc != "" && !strings.Contains(tc, "$env{") && !strings.Contains(tc, "$penv{") && !strings.HasPrefix(tc, "/") {
				ex.Imports = append(ex.Imports, lang.RawImport{
					Spec: `"toolchainFile": "` + tc + `"`, Module: presetPath(tc), Name: kindFile, Line: lineOf(`"` + tc + `"`),
				})
			}
		}
	}
	// Symbols in the order the file names them, with their lines.
	var set lang.SymbolSet
	for _, m := range presetName.FindAllSubmatchIndex(src, -1) {
		name := string(src[m[2]:m[3]])
		if names[name] {
			set.Add(name, "preset", bytes.Count(src[:m[0]], []byte("\n"))+1)
		}
	}
	ex.Symbols = set.List()
	return ex
}

// presetPath writes a presets file's path the way a CMake file would: relative to
// the file's own directory.
func presetPath(p string) string {
	for _, m := range []string{"${sourceDir}", "${fileDir}"} {
		if rest, ok := strings.CutPrefix(p, m); ok {
			return "${CMAKE_CURRENT_LIST_DIR}" + rest
		}
	}
	return "${CMAKE_CURRENT_LIST_DIR}/" + p
}
