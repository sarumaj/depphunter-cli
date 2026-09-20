@{
    RootModule        = 'Tools.psm1'
    ModuleVersion     = '1.0.0'
    RequiredModules   = @(
        'PSReadLine',
        @{ ModuleName = 'Pester'; ModuleVersion = '5.3.0' },
        @{ ModuleName = 'PSScriptAnalyzer'; RequiredVersion = '1.21.0' },
        'Az.Accounts'
    )
    NestedModules     = @('Private/Helpers.ps1')
    ScriptsToProcess  = 'init.ps1'
}
