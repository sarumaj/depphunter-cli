package elm

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/sarumaj/depphunter-cli/internal/lang"
)

// toolingElm is the compiler version elm-tooling.json pins (`tools.elm`) for a
// project in directory: the file in directory or the nearest directory above it in the
// repository, as elm-tooling looks it up. Its other tools (elm-format,
// elm-json, elm-test-rs) are programs run on the code, downloaded from their
// releases like the compiler, so they are not dependencies (a nimble
// requirement of nim and shard.yml's crystal: are not either). "" when no file
// pins an exact version.
//
// Implements: REQ-ELM-005
func toolingElm(root, directory string) string {
	if root == "" {
		return ""
	}
	for d := range lang.DirectoryAndAncestors(directory) {
		if source, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(d), "elm-tooling.json")); err == nil {
			var raw struct {
				Tools map[string]any `json:"tools"`
			}
			json.Unmarshal(source, &raw)
			v, _ := raw.Tools["elm"].(string)
			if _, ok := parseVersion(v); ok {
				return v
			}
			return "" // the nearest file decides
		}
	}
	return ""
}
