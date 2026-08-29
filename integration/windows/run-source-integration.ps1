param(
    [string]$WorkRoot = "$env:TEMP\gorilla-source-integration",
    [string]$GorillaExePath = "",
    [switch]$UsePrebuiltFixtures
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Test-PrebuiltFixtureBundle {
    param([Parameter(Mandatory)][string]$Root)

    $requiredPaths = @(
        "fixture\tools\gorilla-it-sideload.cer",
        "fixture\tools\fixture-server.exe",
        "fixture\configs\install.yaml",
        "fixture\configs\update.yaml",
        "fixture\configs\uninstall.yaml",
        "fixture\repo\catalogs\integration.yaml",
        "fixture\repo\manifests\integration-install.yaml",
        "fixture\repo\manifests\integration-update.yaml",
        "fixture\repo\manifests\integration-uninstall.yaml",
        "fixture\repo\packages\exe\marker-installer.exe",
        "fixture\repo\packages\exe\marker-uninstaller.exe",
        "fixture\repo\packages\scripts\marker-install-v1.ps1",
        "fixture\repo\packages\scripts\marker-install-v2.ps1",
        "fixture\repo\packages\scripts\marker-uninstall.ps1",
        "fixture\repo\packages\nupkg\gorilla-it-nupkg.1.0.0.nupkg",
        "fixture\repo\packages\nupkg\gorilla-it-nupkg.2.0.0.nupkg",
        "fixture\repo\packages\msi\gorilla-it-msi-1.0.0.msi",
        "fixture\repo\packages\msi\gorilla-it-msi-2.0.0.msi",
        "fixture\repo\packages\msix\gorilla-it-msix-1.0.0.msix",
        "fixture\repo\packages\msix\gorilla-it-msix-2.0.0.msix",
        "fixture\repo\packages\msix\gorilla-it-msix-nouninstaller-1.0.0.msix"
    )

    $missing = @()
    foreach ($relativePath in $requiredPaths) {
        $path = Join-Path $Root $relativePath
        if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {
            $missing += $relativePath
        }
    }

    if ($missing.Count -gt 0) {
        Write-Warning "Prebuilt fixture bundle is incomplete. Missing: $($missing -join ', ')"
        return $false
    }

    return $true
}

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "../..")).Path
if ([string]::IsNullOrWhiteSpace($GorillaExePath)) {
    $GorillaExePath = Join-Path $repoRoot "build\gorilla.exe"
}

if (-not (Test-Path -LiteralPath $GorillaExePath)) {
    throw "Gorilla executable not found: $GorillaExePath. Run 'make build' first."
}

New-Item -ItemType Directory -Path $WorkRoot -Force | Out-Null

$useValidPrebuiltFixtures = $UsePrebuiltFixtures -and (Test-PrebuiltFixtureBundle -Root $WorkRoot)
$prebuiltCertificatePath = Join-Path $WorkRoot "fixture\tools\gorilla-it-sideload.cer"
if ($useValidPrebuiltFixtures) {
    Write-Host "Using explicitly supplied prebuilt release integration fixtures from $WorkRoot"
    & (Join-Path $PSScriptRoot "initialize-msix-test-certificate.ps1") `
        -CertificatePath $prebuiltCertificatePath | Out-Null
} else {
    if ($UsePrebuiltFixtures) {
        Write-Host "Falling back to locally generated release integration fixtures"
    } else {
        Write-Host "Regenerating release integration fixtures from the current checkout in $WorkRoot"
    }

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
