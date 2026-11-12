param(
    [string]$ManifestPath = (Join-Path $PSScriptRoot "..\src\Gorilla.UI.App\Package.appxmanifest")
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$resolvedPath = (Resolve-Path -LiteralPath $ManifestPath).Path
[xml]$manifest = Get-Content -LiteralPath $resolvedPath -Raw

$ns = New-Object System.Xml.XmlNamespaceManager($manifest.NameTable)
$ns.AddNamespace("f", "http://schemas.microsoft.com/appx/manifest/foundation/windows10")
$ns.AddNamespace("uap3", "http://schemas.microsoft.com/appx/manifest/uap/windows10/3")
$ns.AddNamespace("desktop", "http://schemas.microsoft.com/appx/manifest/desktop/windows10")
$ns.AddNamespace("desktop6", "http://schemas.microsoft.com/appx/manifest/desktop/windows10/6")

function Assert-Equal {
    param([string]$Label, [AllowNull()][string]$Actual, [string]$Expected)
    if ($Actual -cne $Expected) { throw "$Label changed outside the supported branding surface. Expected '$Expected'; got '$Actual'." }
}

function Assert-SetEqual {
    param([string]$Label, [string[]]$Actual, [string[]]$Expected)
    $actualSorted = @($Actual | Sort-Object)
    $expectedSorted = @($Expected | Sort-Object)
    if (($actualSorted -join "`n") -cne ($expectedSorted -join "`n")) {
        throw "$Label changed outside the supported branding surface. Expected [$($expectedSorted -join ', ')]; got [$($actualSorted -join ', ')]."
    }
}

$identity = $manifest.SelectSingleNode("/f:Package/f:Identity", $ns)
if ($null -eq $identity) { throw "Package Identity is missing." }
Assert-Equal "Identity.Name" $identity.GetAttribute("Name") "133b2116-358b-42fb-8bc8-35009cc5d5af"
Assert-Equal "Identity.Publisher" $identity.GetAttribute("Publisher") "CN=Dustin Davis, O=Dustin Davis, L=Walnut Creek, S=ca, C=US"

$publisherDisplayName = $manifest.SelectSingleNode("/f:Package/f:Properties/f:PublisherDisplayName", $ns)
if ($null -eq $publisherDisplayName) { throw "PublisherDisplayName is missing." }
Assert-Equal "PublisherDisplayName" $publisherDisplayName.InnerText "Gorilla UI"

$application = $manifest.SelectSingleNode("/f:Package/f:Applications/f:Application", $ns)
if ($null -eq $application) { throw "Application is missing." }
Assert-Equal "Application.Id" $application.GetAttribute("Id") "App"
Assert-Equal "Application.Executable" $application.GetAttribute("Executable") '$targetnametoken$.exe'

$aliasExtension = $manifest.SelectSingleNode("//uap3:Extension[@Category='windows.appExecutionAlias']", $ns)
if ($null -eq $aliasExtension) { throw "windows.appExecutionAlias extension is missing." }
Assert-Equal "ExecutionAlias.Executable" $aliasExtension.GetAttribute("Executable") "gorilla.exe"
$alias = $aliasExtension.SelectSingleNode("uap3:AppExecutionAlias/desktop:ExecutionAlias", $ns)
if ($null -eq $alias) { throw "Execution alias is missing." }
Assert-Equal "ExecutionAlias.Alias" $alias.GetAttribute("Alias") "gorilla.exe"

$serviceExtension = $manifest.SelectSingleNode("//desktop6:Extension[@Category='windows.service']", $ns)
if ($null -eq $serviceExtension) { throw "windows.service extension is missing." }
Assert-Equal "Service.Executable" $serviceExtension.GetAttribute("Executable") "gorilla.exe"
$service = $serviceExtension.SelectSingleNode("desktop6:Service", $ns)
if ($null -eq $service) { throw "Packaged service is missing." }
Assert-Equal "Service.Name" $service.GetAttribute("Name") "gorilla"
Assert-Equal "Service.StartupType" $service.GetAttribute("StartupType") "auto"
Assert-Equal "Service.StartAccount" $service.GetAttribute("StartAccount") "localSystem"
Assert-Equal "Service.Arguments" $service.GetAttribute("Arguments") "-service"

$capabilities = @($manifest.SelectNodes("/f:Package/f:Capabilities/*", $ns) | ForEach-Object { "$($_.NamespaceURI)|$($_.GetAttribute('Name'))" })
Assert-SetEqual "Capabilities" $capabilities @(
    "http://schemas.microsoft.com/appx/manifest/foundation/windows10/restrictedcapabilities|runFullTrust",
    "http://schemas.microsoft.com/appx/manifest/foundation/windows10/restrictedcapabilities|packagedServices",
    "http://schemas.microsoft.com/appx/manifest/foundation/windows10/restrictedcapabilities|localSystemServices"
)

$deviceFamilies = @($manifest.SelectNodes("/f:Package/f:Dependencies/f:TargetDeviceFamily", $ns) | ForEach-Object { "$($_.GetAttribute('Name'))|$($_.GetAttribute('MinVersion'))|$($_.GetAttribute('MaxVersionTested'))" })
Assert-SetEqual "TargetDeviceFamily dependencies" $deviceFamilies @(
    "Windows.Universal|10.0.19041.0|10.0.19041.0",
    "Windows.Desktop|10.0.19041.0|10.0.19041.0"
)

Write-Host "Package branding boundary validation passed: $resolvedPath"
