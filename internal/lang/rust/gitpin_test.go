package rust

import (
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
)

// A crate Cargo.lock says came from a git repository carries that checkout, so the
// vulnerability database can be asked about its commit; a registry crate and a git
// source without a whole commit carry none.
//
// Verifies: REQ-FND-026
func TestGitCratesCarryTheirCheckout(t *testing.T) {
	const sha = "9f9f9f9f9f9f9f9f9f9f9f9f9f9f9f9f9f9f9f9f"
	root := langtest.Write(t, map[string]string{
		"Cargo.toml": "[package]\nname = \"app\"\n\n[dependencies]\n" +
			"regex = { git = \"https://github.com/rust-lang/regex\", branch = \"main\" }\nserde = \"1\"\nodd = { git = \"https://example.com/odd\" }\n",
		"Cargo.lock": `version = 3

[[package]]
name = "regex"
version = "1.10.0"
source = "git+https://github.com/rust-lang/regex?branch=main#` + sha + `"
dependencies = ["serde"]

[[package]]
name = "serde"
version = "1.0.200"
source = "registry+https://github.com/rust-lang/crates.io-index"

[[package]]
name = "odd"
version = "0.1.0"
source = "git+https://example.com/odd#abc1234"
`,
		"src/main.rs": "use regex::Regex;\nuse serde::Serialize;\nuse odd::Thing;\nfn main() {}\n",
	})
	results := langtest.Analyze(t, Plugin{}, root)
	langtest.CheckImports(t, results["src/main.rs"], map[string]lang.Target{
		"use regex::Regex":     {Ecosystem: "crates", Package: "regex", Version: "1.10.0", Pinned: true, Git: "https://github.com/rust-lang/regex#" + sha},
		"use serde::Serialize": {Ecosystem: "crates", Package: "serde", Version: "1.0.200", Requested: "1", Pinned: true},
		"use odd::Thing":       {Ecosystem: "crates", Package: "odd", Version: "0.1.0", Pinned: true},
	})
}
