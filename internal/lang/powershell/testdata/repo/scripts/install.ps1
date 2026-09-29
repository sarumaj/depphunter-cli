# Bootstraps the build tools.
Install-Module -Name PSScriptAnalyzer -RequiredVersion 1.22.0 -Scope CurrentUser -Force
Install-Module InvokeBuild, platyPS -MinimumVersion 5.10 -Repository CorpGallery
Install-PSResource -Name Pester -Version '[5.0,6.0)' -TrustRepository
Install-PSResource PSFramework -Version 1.12.346
Save-Module -Name Plaster -Path ./modules
Install-Module -Name $moduleName
