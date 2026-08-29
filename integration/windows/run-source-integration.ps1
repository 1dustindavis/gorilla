param(
    [string]$WorkRoot = "$env:TEMP\gorilla-source-integration",
    [string]$GorillaExePath = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "../..")).Path
if ([string]::IsNullOrWhiteSpace($GorillaExePath)) {
    $GorillaExePath = Join-Path $repoRoot "build\gorilla.exe"
}

if (-not (Test-Path -LiteralPath $GorillaExePath)) {
    throw "Gorilla executable not found: $GorillaExePath. Run 'make build' first."
}

$certPath = Join-Path $WorkRoot "gorilla-it-sideload.cer"
New-Item -ItemType Directory -Path $WorkRoot -Force | Out-Null
$thumbprint = & (Join-Path $PSScriptRoot "initialize-msix-test-certificate.ps1") `
    -CertificatePath $certPath `
    -Create

& (Join-Path $PSScriptRoot "prepare-release-integration.ps1") `
    -WorkRoot $WorkRoot `
    -MsixCertThumbprint $thumbprint

& (Join-Path $PSScriptRoot "run-release-integration.ps1") `
    -WorkRoot $WorkRoot `
    -GorillaExePath $GorillaExePath
