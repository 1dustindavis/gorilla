param()

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$validator = Join-Path $PSScriptRoot "validate-package-branding.ps1"
$canonical = Join-Path $PSScriptRoot "..\src\Gorilla.UI.App\Package.appxmanifest"

& $validator -ManifestPath $canonical

$tempRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("gorilla-branding-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $tempRoot -Force | Out-Null

try {
    $customized = Join-Path $tempRoot "customized.appxmanifest"
    [xml]$manifest = Get-Content -LiteralPath $canonical -Raw
    $ns = New-Object System.Xml.XmlNamespaceManager($manifest.NameTable)
    $ns.AddNamespace("f", "http://schemas.microsoft.com/appx/manifest/foundation/windows10")
    $ns.AddNamespace("uap", "http://schemas.microsoft.com/appx/manifest/uap/windows10")

    $manifest.SelectSingleNode("/f:Package/f:Properties/f:DisplayName", $ns).InnerText = "Example Software"
    $visual = $manifest.SelectSingleNode("/f:Package/f:Applications/f:Application/uap:VisualElements", $ns)
    $visual.SetAttribute("DisplayName", "Example Software")
    $visual.SetAttribute("Description", "Example Software")
    $manifest.Save($customized)

    & $validator -ManifestPath $customized

    $protectedMutation = Join-Path $tempRoot "protected-mutation.appxmanifest"
    [xml]$mutated = Get-Content -LiteralPath $customized -Raw
    $mutatedNs = New-Object System.Xml.XmlNamespaceManager($mutated.NameTable)
    $mutatedNs.AddNamespace("desktop6", "http://schemas.microsoft.com/appx/manifest/desktop/windows10/6")
    $service = $mutated.SelectSingleNode("//desktop6:Service", $mutatedNs)
    $service.SetAttribute("Name", "not-gorilla")
    $mutated.Save($protectedMutation)

    $rejected = $false
    try {
        & $validator -ManifestPath $protectedMutation
    } catch {
        $rejected = $true
    }
    if (-not $rejected) {
        throw "Package branding boundary validation accepted a protected service identity change."
    }

    Write-Host "Package branding boundary tests passed"
} finally {
    Remove-Item -LiteralPath $tempRoot -Recurse -Force -ErrorAction SilentlyContinue
}
