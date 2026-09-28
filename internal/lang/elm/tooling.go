package elm

import (
	"encoding/json"
	"os"
	"path"
	"path/filepath"
)

// toolingElm is the compiler version elm-tooling.json pins (`tools.elm`) for a
// project in dir: the file in dir or the nearest directory above it in the
// repository, as elm-tooling looks it up. Its other tools (elm-format,
// elm-json, elm-test-rs) are programs run on the code, downloaded from their
// releases like the compiler, so they are not dependencies (a nimble
// requirement of nim and shard.yml's crystal: are not either). "" when no file
// pins an exact version.
//
// Implements: REQ-ELM-005
func toolingElm(root, dir string) string {
	if root == "" {
		return ""
	}
	for d := dir; ; d = path.Dir(d) {
		if src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(d), "elm-tooling.json")); err == nil {
			var raw struct {
				Tools map[string]any `json:"tools"`
			}
			json.Unmarshal(src, &raw)
			v, _ := raw.Tools["elm"].(string)
			if _, ok := parseVersion(v); ok {
				return v
			}
			return "" // the nearest file decides
		}
		if d == "." || d == "/" {
			return ""
		}
	}
}
