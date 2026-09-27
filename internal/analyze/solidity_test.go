package analyze

import (
	"context"
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/graph"
	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/javascript"
	"github.com/sarumaj/depphunter-cli/internal/lang/solidity"
)

// A Hardhat project's Solidity imports @openzeppelin/contracts from
// node_modules, and its deployment script imports the same package's build
// output: one npm node, versioned by the one package-lock.json.
//
// Verifies: REQ-SOLIDITY-008
func TestOneNPMNodeAcrossSolidityAndJavaScript(t *testing.T) {
	root := t.TempDir()
	writeProject(t, root, map[string]string{
		"hardhat.config.js": "module.exports = { solidity: \"0.8.24\" };\n",
		"package.json":      `{"name": "token", "dependencies": {"@openzeppelin/contracts": "^5.0.0"}}`,
		"package-lock.json": `{"name": "token", "lockfileVersion": 3, "packages": {
  "": {"name": "token", "dependencies": {"@openzeppelin/contracts": "^5.0.0"}},
  "node_modules/@openzeppelin/contracts": {"version": "5.0.2"}}}`,
		"contracts/Token.sol": "pragma solidity ^0.8.24;\nimport {ERC20} from \"@openzeppelin/contracts/token/ERC20/ERC20.sol\";\ncontract Token is ERC20 {}\n",
		"scripts/deploy.js":   "const erc20 = require(\"@openzeppelin/contracts/build/contracts/ERC20.json\");\n",
	})
	g, _, err := Run(context.Background(), root, Options{Plugins: []lang.Plugin{javascript.Plugin{}, solidity.Plugin{}}})
	if err != nil {
		t.Fatal(err)
	}
	var npm []string
	for _, n := range g.Nodes {
		if n.Kind == graph.KindPackage && n.Parent == graph.EcosystemID("npm") {
			npm = append(npm, n.Name)
		}
	}
	if len(npm) != 1 || npm[0] != "@openzeppelin/contracts" {
		t.Fatalf("npm packages %v, want only @openzeppelin/contracts", npm)
	}
	id := graph.PackageID("npm", "@openzeppelin/contracts")
	from := map[string]bool{}
	for _, e := range g.Edges {
		if e.To == id {
			from[e.From] = true
		}
	}
	for _, f := range []string{"contracts/Token.sol", "scripts/deploy.js"} {
		if !from[graph.FileID(f)] {
			t.Errorf("%s has no edge to %s", f, id)
		}
	}
	if n := byID(g)[id]; n.Version != "5.0.2" || n.Unresolved {
		t.Errorf("version %q unresolved %v, want 5.0.2 resolved", n.Version, n.Unresolved)
	}
}
