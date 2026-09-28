package php

import (
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
)

// A package composer.lock holds at a branch carries the commit it was locked at, so
// the vulnerability database can be asked about that; a release carries none, its
// version being the question.
//
// Verifies: REQ-FND-026
func TestBranchPackagesCarryTheirCheckout(t *testing.T) {
	const sha = "0123456789abcdef0123456789abcdef01234567"
	root := langtest.Write(t, map[string]string{
		"composer.json": `{"require": {"acme/lib": "dev-main", "psr/log": "^3.0"}}`,
		"composer.lock": `{"packages": [
			{"name": "acme/lib", "version": "dev-main",
			 "source": {"type": "git", "url": "https://github.com/acme/lib.git", "reference": "` + sha + `"},
			 "autoload": {"psr-4": {"Acme\\Lib\\": "src/"}}},
			{"name": "psr/log", "version": "3.0.0",
			 "source": {"type": "git", "url": "https://github.com/php-fig/log.git", "reference": "` + sha + `"},
			 "autoload": {"psr-4": {"Psr\\Log\\": "src"}}}
		]}`,
		"index.php": "<?php\nuse Acme\\Lib\\Client;\nuse Psr\\Log\\LoggerInterface;\n",
	})
	results := langtest.Analyze(t, Plugin{}, root)
	langtest.CheckImports(t, results["index.php"], map[string]lang.Target{
		`use Acme\Lib\Client`:         {Ecosystem: "composer", Package: "acme/lib", Version: "dev-main", Pinned: true, Git: "https://github.com/acme/lib.git#" + sha},
		`use Psr\Log\LoggerInterface`: {Ecosystem: "composer", Package: "psr/log", Version: "3.0.0", Requested: "^3.0", Pinned: true},
	})
}
