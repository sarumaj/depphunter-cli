@{
    PSDependOptions = @{ Target = 'CurrentUser' }
    # Newest release.
    psake = 'latest'
    'BuildHelpers' = '2.0.16'
    Pester = @{
        Version    = '[5.0,6.0)'
        Parameters = @{ Repository = 'CorpGallery'; SkipPublisherCheck = $true }
    }
    'PSGalleryModule::PSDeploy' = ''
    'RamblingCookieMonster/PSStackExchange' = 'master'
    Tool = @{ DependencyType = 'Git'; Name = 'https://git.example/tool.git' }
    Renamed = @{ Name = 'Configuration'; Version = '1.5.1' }
}
