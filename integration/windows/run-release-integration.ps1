param(
    [string]$WorkRoot = "$env:RUNNER_TEMP\gorilla-release-integration",
    [Parameter(Mandatory = $true)]
    [string]$GorillaExePath
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Assert-Content {
    param(
        [string]$Path,
        [string]$Expected
    )

    if (-not (Test-Path -LiteralPath $Path)) {
        throw "Expected file does not exist: $Path"
    }

    $actual = (Get-Content -LiteralPath $Path -Raw).Trim()
    if ($actual -ne $Expected) {
        throw "Unexpected content in $Path. expected='$Expected' actual='$actual'"
    }
}

function Assert-Missing {
    param([string]$Path)

    if (Test-Path -LiteralPath $Path) {
        throw "Expected file to be missing: $Path"
    }
}

function Assert-PreparedPath {
    param([string]$Path)

    if (-not (Test-Path -LiteralPath $Path)) {
        throw "Preparation output missing: $Path. Run prepare-release-integration.ps1 first."
    }
}

function Write-Config {
    param(
        [string]$ManifestName,
        [string]$Path,
        [string]$FileUrl
    )

    @"
url: $FileUrl
manifest: $ManifestName
catalogs:
  - integration
app_data_path: C:/ProgramData/gorilla-it/cache
"@ | Set-Content -LiteralPath $Path -NoNewline
}

function Run-Gorilla {
    param(
        [string]$ExePath,
        [string]$ConfigPath,
        [string]$Phase
    )

    Write-Host "::group::[TEST] $Phase"
    Write-Host "[RUN] gorilla -config $ConfigPath -verbose"
    & $ExePath -config $ConfigPath -verbose
    if ($LASTEXITCODE -ne 0) {
        Write-Host "::endgroup::"
        throw "gorilla run failed for $ConfigPath with exit code $LASTEXITCODE"
    }
    Write-Host "[PASS] gorilla exit code 0"
    Write-Host "::endgroup::"
}

$root = [System.IO.Path]::GetFullPath($WorkRoot)
$fixtureRoot = Join-Path $root "fixture"
$repoRoot = Join-Path $fixtureRoot "repo"
$configRoot = Join-Path $fixtureRoot "configs"
$toolsRoot = Join-Path $fixtureRoot "tools"
$serverExe = Join-Path $toolsRoot "fixture-server.exe"

Assert-PreparedPath -Path (Join-Path $repoRoot "catalogs/integration.yaml")
Assert-PreparedPath -Path (Join-Path $repoRoot "manifests/integration-install.yaml")
Assert-PreparedPath -Path (Join-Path $repoRoot "manifests/integration-update.yaml")
Assert-PreparedPath -Path (Join-Path $repoRoot "manifests/integration-uninstall.yaml")
Assert-PreparedPath -Path $serverExe

$markerRoot = "C:\ProgramData\gorilla-it"
$exeMarker = Join-Path $markerRoot "exe.txt"
$msiMarker = Join-Path $markerRoot "msi.txt"
$nupkgMarker = Join-Path $markerRoot "nupkg.txt"
$ps1Marker = Join-Path $markerRoot "ps1.txt"

Write-Host "::group::[TEST] Environment setup"
Remove-Item -LiteralPath $markerRoot -Recurse -Force -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Path $markerRoot -Force | Out-Null
New-Item -ItemType Directory -Path $configRoot -Force | Out-Null
New-Item -ItemType Directory -Path "C:\ProgramData\gorilla" -Force | Out-Null
Write-Host "[INFO] Cleaned marker directory: $markerRoot"
Write-Host "::endgroup::"

$gorillaExePath = [System.IO.Path]::GetFullPath($GorillaExePath)
if (-not (Test-Path -LiteralPath $gorillaExePath)) {
    throw "gorilla.exe not found at path: $gorillaExePath"
}
Write-Host "[INFO] Using gorilla.exe from: $gorillaExePath"

$serverPort = Get-Random -Minimum 18080 -Maximum 18999
$serverProc = Start-Process -FilePath $serverExe `
    -ArgumentList @("-addr", "127.0.0.1:$serverPort", "-root", $repoRoot) `
    -PassThru -WindowStyle Hidden
Start-Sleep -Seconds 2

if ($serverProc.HasExited) {
    throw "Failed to start fixture HTTP server process"
}

$fileUrl = "http://127.0.0.1:$serverPort/"
Write-Host "[INFO] Serving fixture repo from $fileUrl"

$configInstall = Join-Path $configRoot "install.yaml"
$configUpdate = Join-Path $configRoot "update.yaml"
$configUninstall = Join-Path $configRoot "uninstall.yaml"
Write-Config -ManifestName "integration-install" -Path $configInstall -FileUrl $fileUrl
Write-Config -ManifestName "integration-update" -Path $configUpdate -FileUrl $fileUrl
Write-Config -ManifestName "integration-uninstall" -Path $configUninstall -FileUrl $fileUrl

$results = @()
$currentPhase = "initialization"

try {
    $currentPhase = "install"
    Run-Gorilla -ExePath $gorillaExePath -ConfigPath $configInstall -Phase "Install"
    Assert-Content -Path $exeMarker -Expected "1.0.0"
    Assert-Content -Path $msiMarker -Expected "1.0.0"
    Assert-Content -Path $nupkgMarker -Expected "1.0.0"
    Assert-Content -Path $ps1Marker -Expected "1.0.0"
    $results += "[PASS] Install"

    $currentPhase = "update"
    Run-Gorilla -ExePath $gorillaExePath -ConfigPath $configUpdate -Phase "Update"
    Assert-Content -Path $exeMarker -Expected "2.0.0"
    Assert-Content -Path $msiMarker -Expected "2.0.0"
    Assert-Content -Path $nupkgMarker -Expected "2.0.0"
    Assert-Content -Path $ps1Marker -Expected "2.0.0"
    $results += "[PASS] Update"

    $currentPhase = "uninstall"
    Run-Gorilla -ExePath $gorillaExePath -ConfigPath $configUninstall -Phase "Uninstall"
    Assert-Missing -Path $exeMarker
    Assert-Missing -Path $msiMarker
    Assert-Missing -Path $nupkgMarker
    Assert-Missing -Path $ps1Marker
    $results += "[PASS] Uninstall"

    Write-Host "========== Integration Test Summary =========="
    $results | ForEach-Object { Write-Host $_ }
    Write-Host "[PASS] Gorilla released-binary integration run passed"

    if ($env:GITHUB_STEP_SUMMARY) {
        @"
## Windows Released-Binary Integration Results

- PASS: Install
- PASS: Update
- PASS: Uninstall

Overall: PASS
"@ | Add-Content -LiteralPath $env:GITHUB_STEP_SUMMARY
    }
} catch {
    $errorMessage = $_.Exception.Message
    Write-Host "========== Integration Test Summary =========="
    $results | ForEach-Object { Write-Host $_ }
    Write-Host "[FAIL] Phase: $currentPhase"
    Write-Host "[FAIL] $errorMessage"

    if ($env:GITHUB_STEP_SUMMARY) {
        @"
## Windows Released-Binary Integration Results

- FAIL: $currentPhase
- Error: $errorMessage

Overall: FAIL
"@ | Add-Content -LiteralPath $env:GITHUB_STEP_SUMMARY
    }
    throw
} finally {
    if ($serverProc -and -not $serverProc.HasExited) {
        Stop-Process -Id $serverProc.Id -Force -ErrorAction SilentlyContinue
    }
}
