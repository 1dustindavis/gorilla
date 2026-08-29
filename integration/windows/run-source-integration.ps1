param(
    [string]$WorkRoot = "$env:TEMP\gorilla-source-integration",
    [string]$GorillaExePath = "",
    [switch]$UsePrebuiltFixtures
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

New-Item -ItemType Directory -Path $WorkRoot -Force | Out-Null

$prebuiltCertificatePath = Join-Path $WorkRoot "fixture\tools\gorilla-it-sideload.cer"
if ($UsePrebuiltFixtures) {
    if (-not (Test-Path -LiteralPath $prebuiltCertificatePath)) {
        throw "Prebuilt fixtures were requested, but the fixture certificate was not found at $prebuiltCertificatePath"
    }

    Write-Host "Using explicitly supplied prebuilt release integration fixtures from $WorkRoot"
    & (Join-Path $PSScriptRoot "initialize-msix-test-certificate.ps1") `
        -CertificatePath $prebuiltCertificatePath | Out-Null
} else {
    Write-Host "Regenerating release integration fixtures from the current checkout in $WorkRoot"
    $certPath = Join-Path $WorkRoot "gorilla-it-sideload.cer"
    $thumbprint = & (Join-Path $PSScriptRoot "initialize-msix-test-certificate.ps1") `
        -CertificatePath $certPath `
        -Create

    & (Join-Path $PSScriptRoot "prepare-release-integration.ps1") `
        -WorkRoot $WorkRoot `
        -MsixCertThumbprint $thumbprint
}

& (Join-Path $PSScriptRoot "run-release-integration.ps1") `
    -WorkRoot $WorkRoot `
    -GorillaExePath $GorillaExePath
