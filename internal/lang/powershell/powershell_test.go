package powershell

import (
	"testing"

	"github.com/sarumaj/depphunter-cli/internal/lang"
	"github.com/sarumaj/depphunter-cli/internal/lang/langtest"
)

// testdata/repo: a module with a manifest (RequiredModules, RootModule, NestedModules,
// ScriptsToProcess) and a script using every other way of pulling in code.
func analyze(t *testing.T) map[string]*lang.FileResult {
	t.Helper()
	return langtest.Analyze(t, Plugin{}, "testdata/repo")
}

// Verifies: REQ-PS-001, REQ-PS-002, REQ-PS-003, REQ-PS-004, REQ-PS-005, REQ-PS-007, REQ-PS-008, REQ-PS-010
func TestScript(t *testing.T) {
	langtest.CheckImports(t, analyze(t)["scripts/deploy.ps1"], map[string]lang.Target{
		"#Requires -Modules Az.Storage": {Ecosystem: "psgallery", Package: "Az.Storage"},
		// RequiredVersion in a #Requires table names one version.
		"#Requires -Modules Az.Resources":           {Ecosystem: "psgallery", Package: "Az.Resources", Version: "6.1.0", Pinned: true},
		"using module ../tools/Tools/Tools.psm1":    {Local: "tools/Tools/Tools.psm1"},
		"Import-Module Tools":                       {Local: "tools/Tools/Tools.psd1"},
		"Import-Module PSReadLine":                  {Ecosystem: "powershell", Package: "PSReadLine"},
		"Import-Module Pester":                      {Ecosystem: "psgallery", Package: "Pester", Version: "5.3.0"},
		"Import-Module $PSScriptRoot/lib/Util.psm1": {Local: "scripts/lib/Util.psm1"},
		"Import-Module SqlServer":                   {Ecosystem: "psgallery", Package: "SqlServer", Unresolved: true},
		"Import-Module PowerShellGet":               {Ecosystem: "powershell", Package: "PowerShellGet"},
		`. "$PSScriptRoot\common.ps1"`:              {Local: "scripts/common.ps1"},
		". ./helpers.ps1":                           {Local: "scripts/helpers.ps1"},
		`& "$PSScriptRoot/tasks/build.ps1"`:         {Local: "scripts/tasks/build.ps1"},
		". $env:HOME/profile.ps1":                   {},
	})
}

// Verifies: REQ-PS-006, REQ-PS-008, REQ-PS-010
func TestManifest(t *testing.T) {
	langtest.CheckImports(t, analyze(t)["tools/Tools/Tools.psd1"], map[string]lang.Target{
		"RequiredModules: PSReadLine": {Ecosystem: "powershell", Package: "PSReadLine"},
		// ModuleVersion is a minimum, RequiredVersion names one version.
		"RequiredModules: Pester":            {Ecosystem: "psgallery", Package: "Pester", Version: "5.3.0"},
		"RequiredModules: PSScriptAnalyzer":  {Ecosystem: "psgallery", Package: "PSScriptAnalyzer", Version: "1.21.0", Pinned: true},
		"RequiredModules: Az.Accounts":       {Ecosystem: "psgallery", Package: "Az.Accounts"},
		"RootModule: Tools.psm1":             {Local: "tools/Tools/Tools.psm1"},
		"NestedModules: Private/Helpers.ps1": {Local: "tools/Tools/Private/Helpers.ps1"},
		"ScriptsToProcess: init.ps1":         {Local: "tools/Tools/init.ps1"},
	})
}

func TestSymbols(t *testing.T) {
	langtest.CheckSymbols(t, analyze(t)["scripts/deploy.ps1"], map[string]string{"Deploy-App": "func", "Deployer": "class", "Deployer.Run": "method"})
}
