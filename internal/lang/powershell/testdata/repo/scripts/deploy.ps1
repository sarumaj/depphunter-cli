#Requires -Modules Az.Storage, @{ModuleName='Az.Resources'; RequiredVersion='6.1.0'}
using module ../tools/Tools/Tools.psm1

Import-Module Tools
Import-Module -Name PSReadLine, Pester -Force
Import-Module "$PSScriptRoot/lib/Util.psm1"
ipmo SqlServer
$psGet = Import-Module PowerShellGet -PassThru
. "$PSScriptRoot\common.ps1"
. ./helpers.ps1
& "$PSScriptRoot/tasks/build.ps1" -Force
. $env:HOME/profile.ps1
Write-Host "Import-Module NotReal"

function Deploy-App { }

class Deployer : Base {
    [void] Run() { }
}
