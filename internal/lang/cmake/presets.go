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
func readPresets(source []byte) *lang.Extraction {
	var p map[string]json.RawMessage
	extraction := &lang.Extraction{}
	if json.Unmarshal(source, &p) != nil {
		return extraction
	}
	lineOf := func(needle string) int { return max(lang.LineOf(source, needle, 0), 1) }
	var includes []string
	json.Unmarshal(p["include"], &includes)
	for _, include := range includes {
		extraction.Imports = append(extraction.Imports, lang.RawImport{
			Spec: `"include": "` + include + `"`, Module: presetPath(include), Name: kindFile, Line: lineOf(`"` + include + `"`),
		})
	}
	names := map[string]bool{}
	for _, kind := range presetKinds {
		var list []struct {
			Name          string `json:"name"`
			ToolchainFile string `json:"toolchainFile"`
		}
		json.Unmarshal(p[kind], &list)
		for _, preset := range list {
			names[preset.Name] = true
			if testCase := preset.ToolchainFile; testCase != "" && !strings.Contains(testCase, "$env{") && !strings.Contains(testCase, "$penv{") && !strings.HasPrefix(testCase, "/") {
				extraction.Imports = append(extraction.Imports, lang.RawImport{
					Spec: `"toolchainFile": "` + testCase + `"`, Module: presetPath(testCase), Name: kindFile, Line: lineOf(`"` + testCase + `"`),
				})
			}
		}
	}
	// Symbols in the order the file names them, with their lines.
	var set lang.SymbolSet
	for _, m := range presetName.FindAllSubmatchIndex(source, -1) {
		name := string(source[m[2]:m[3]])
		if names[name] {
			set.Add(name, "preset", bytes.Count(source[:m[0]], []byte("\n"))+1)
		}
	}
	extraction.Symbols = set.List()
	return extraction
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
