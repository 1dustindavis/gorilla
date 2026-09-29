param(
    [string]$WorkRoot = "$env:RUNNER_TEMP\gorilla-release-integration",
    [Parameter(Mandatory = $true)]
    [string]$GorillaExePath,
    [ValidateLength(1, 10)]
    [ValidatePattern('^[A-Za-z][A-Za-z0-9]*$')]
    [string]$FixtureNamespace = "Release"
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

function Assert-AppxInstalled {
    param(
        [string]$Name,
        [string]$ExpectedVersion
    )

    $pkg = Get-AppxProvisionedPackage -Online | Where-Object { $_.DisplayName -eq $Name }
    if (-not $pkg) {
        throw "Expected AppX package '$Name' to be installed but it was not found"
    }
    $actual = [version]$pkg.Version
    $expected = [version]$ExpectedVersion
    if ($actual -lt $expected) {
        throw "AppX package '$Name' version mismatch: expected>=$ExpectedVersion actual=$($pkg.Version)"
    }
}

function Assert-AppxMissing {
    param([string]$Name)

    $pkg = Get-AppxProvisionedPackage -Online | Where-Object { $_.DisplayName -eq $Name }
    if ($pkg) {
        throw "Expected AppX package '$Name' to be uninstalled but it is still registered (version $($pkg.Version))"
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

function Wait-ForFile {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Path,
        [int]$TimeoutSeconds = 30
    )

    $deadline = [DateTime]::UtcNow.AddSeconds($TimeoutSeconds)
    while ([DateTime]::UtcNow -lt $deadline) {
        if (Test-Path -LiteralPath $Path -PathType Leaf) {
            return
        }
        Start-Sleep -Milliseconds 250
    }

    throw "Timed out waiting for file: $Path"
}

$root = [System.IO.Path]::GetFullPath($WorkRoot)
$fixtureRoot = Join-Path $root "fixture"
$repoRoot = Join-Path $fixtureRoot "repo"
$configRoot = Join-Path $fixtureRoot "configs"
$toolsRoot = Join-Path $fixtureRoot "tools"
$serverExe = Join-Path $toolsRoot "fixture-server.exe"
$prepareScriptPath = Join-Path (Split-Path -Parent $PSCommandPath) "prepare-release-integration.ps1"
$preparedPaths = @(
    (Join-Path $repoRoot "catalogs/integration.yaml"),
    (Join-Path $repoRoot "manifests/integration-install.yaml"),
    (Join-Path $repoRoot "manifests/integration-update.yaml"),
    (Join-Path $repoRoot "manifests/integration-uninstall.yaml"),
    $serverExe
)

$missingPreparedPaths = @($preparedPaths | Where-Object { -not (Test-Path -LiteralPath $_) })
if ($missingPreparedPaths.Count -gt 0) {
    if (-not (Test-Path -LiteralPath $prepareScriptPath)) {
        throw "Preparation output missing and prepare script not found: $prepareScriptPath"
    }

    Write-Host "[INFO] Missing integration preparation output; running prepare-release-integration.ps1"
    & $prepareScriptPath -WorkRoot $root -FixtureNamespace $FixtureNamespace
    if ($LASTEXITCODE -ne 0) {
        throw "prepare-release-integration.ps1 failed with exit code $LASTEXITCODE"
    }
}

$missingPreparedPaths = @($preparedPaths | Where-Object { -not (Test-Path -LiteralPath $_) })
if ($missingPreparedPaths.Count -gt 0) {
    throw "Preparation output missing after running prepare-release-integration.ps1: $($missingPreparedPaths -join ', ')"
}

$markerRoot = "C:\ProgramData\gorilla-it"
$exeMarker = Join-Path $markerRoot "exe.txt"
$msiMarker = Join-Path $markerRoot "msi.txt"
$nupkgMarker = Join-Path $markerRoot "nupkg.txt"
$ps1Marker = Join-Path $markerRoot "ps1.txt"
$targetedMarker = Join-Path $markerRoot "targeted-requested.txt"
$unrelatedMarker = Join-Path $markerRoot "targeted-unrelated.txt"
$msixPackageName = "GorillaIntegrationTest$FixtureNamespace"
$msixNoUninstallerPackageName = "GorillaIntegrationTest${FixtureNamespace}NoUninstaller"

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
$configTargeted = Join-Path $configRoot "targeted-service.yaml"
Write-Config -ManifestName "integration-install" -Path $configInstall -FileUrl $fileUrl
Write-Config -ManifestName "integration-update" -Path $configUpdate -FileUrl $fileUrl
Write-Config -ManifestName "integration-uninstall" -Path $configUninstall -FileUrl $fileUrl
Write-Config -ManifestName "integration-targeted-service" -Path $configTargeted -FileUrl $fileUrl
Add-Content -LiteralPath $configTargeted -Value "`nservice_interval: 24h" -NoNewline

$results = @()
$currentPhase = "initialization"
$targetedServiceInstalled = $false

try {
    $currentPhase = "install"
    Run-Gorilla -ExePath $gorillaExePath -ConfigPath $configInstall -Phase "Install"
    Assert-Content -Path $exeMarker -Expected "1.0.0"
    Assert-Content -Path $msiMarker -Expected "1.0.0"
    Assert-Content -Path $nupkgMarker -Expected "1.0.0"
    Assert-Content -Path $ps1Marker -Expected "1.0.0"
    Assert-AppxInstalled -Name $msixPackageName -ExpectedVersion "1.0.0"
    Assert-AppxInstalled -Name $msixNoUninstallerPackageName -ExpectedVersion "1.0.0"
    $results += "[PASS] Install"

    $currentPhase = "update"
    Run-Gorilla -ExePath $gorillaExePath -ConfigPath $configUpdate -Phase "Update"
    Assert-Content -Path $exeMarker -Expected "2.0.0"
    Assert-Content -Path $msiMarker -Expected "2.0.0"
    Assert-Content -Path $nupkgMarker -Expected "2.0.0"
    Assert-Content -Path $ps1Marker -Expected "2.0.0"
    Assert-AppxInstalled -Name $msixPackageName -ExpectedVersion "2.0.0"
    Assert-AppxInstalled -Name $msixNoUninstallerPackageName -ExpectedVersion "1.0.0"
    $results += "[PASS] Update"

    $currentPhase = "uninstall"
    Run-Gorilla -ExePath $gorillaExePath -ConfigPath $configUninstall -Phase "Uninstall"
    Assert-Missing -Path $exeMarker
    Assert-Missing -Path $msiMarker
    Assert-Missing -Path $nupkgMarker
    Assert-Missing -Path $ps1Marker
    Assert-AppxMissing -Name $msixPackageName
    Assert-AppxMissing -Name $msixNoUninstallerPackageName
    $results += "[PASS] Uninstall"

    $currentPhase = "targeted App Catalog execution"
    Write-Host "::group::[TEST] Targeted App Catalog execution"

    $exeInstallerPath = Join-Path $repoRoot "packages/exe/marker-installer.exe"
    $exeUninstallerPath = Join-Path $repoRoot "packages/exe/marker-uninstaller.exe"
    $exeInstallerHash = (Get-FileHash -LiteralPath $exeInstallerPath -Algorithm SHA256).Hash.ToLowerInvariant()
    $exeUninstallerHash = (Get-FileHash -LiteralPath $exeUninstallerPath -Algorithm SHA256).Hash.ToLowerInvariant()
    $catalogPath = Join-Path $repoRoot "catalogs/integration.yaml"
    @"

TargetedOptional:
  display_name: TargetedOptional
  check:
    file:
      - path: '$targetedMarker'
  installer:
    type: exe
    location: packages/exe/marker-installer.exe
    hash: $exeInstallerHash
    arguments:
      - -action=install
      - -marker=$targetedMarker
      - -version=requested
  uninstaller:
    type: exe
    location: packages/exe/marker-uninstaller.exe
    hash: $exeUninstallerHash
    arguments:
      - -action=uninstall
      - -marker=$targetedMarker
  version: 1.0.0

UnrelatedManaged:
  display_name: UnrelatedManaged
  check:
    file:
      - path: '$unrelatedMarker'
  installer:
    type: exe
    location: packages/exe/marker-installer.exe
    hash: $exeInstallerHash
    arguments:
      - -action=install
      - -marker=$unrelatedMarker
      - -version=unrelated
  uninstaller:
    type: exe
    location: packages/exe/marker-uninstaller.exe
    hash: $exeUninstallerHash
    arguments:
      - -action=uninstall
      - -marker=$unrelatedMarker
  version: 1.0.0
"@ | Add-Content -LiteralPath $catalogPath -NoNewline

    $targetedManifestPath = Join-Path $repoRoot "manifests/integration-targeted-service.yaml"
    @'
name: integration-targeted-service
optional_installs:
  - TargetedOptional
'@ | Set-Content -LiteralPath $targetedManifestPath -NoNewline

    & $gorillaExePath -config $configTargeted -serviceinstall | Out-Host
    if ($LASTEXITCODE -ne 0) {
        throw "failed to install targeted integration service"
    }
    $targetedServiceInstalled = $true

    & $gorillaExePath -config $configTargeted -servicestart | Out-Host
    if ($LASTEXITCODE -ne 0) {
        throw "failed to start targeted integration service"
    }

    $deadline = [DateTime]::UtcNow.AddSeconds(30)
    $serviceReady = $false
    while ([DateTime]::UtcNow -lt $deadline) {
        & $gorillaExePath -config $configTargeted -servicecmd ListOptionalInstalls *> $null
        if ($LASTEXITCODE -eq 0) {
            $serviceReady = $true
            break
        }
        Start-Sleep -Milliseconds 250
    }
    if (-not $serviceReady) {
        throw "targeted integration service did not become ready"
    }

    Assert-Missing -Path $targetedMarker
    Assert-Missing -Path $unrelatedMarker

    # Add unrelated managed work only after startup convergence has finished. A
    # targeted InstallItem must load this effective context for catalog/policy
    # resolution without executing UnrelatedManaged. The pre-change full-run
    # callback would execute it here, making this a real service regression test.
    @'
name: integration-targeted-service
managed_installs:
  - UnrelatedManaged
optional_installs:
  - TargetedOptional
'@ | Set-Content -LiteralPath $targetedManifestPath -NoNewline

    & $gorillaExePath -config $configTargeted -servicecmd InstallItem:TargetedOptional | Out-Host
    if ($LASTEXITCODE -ne 0) {
        throw "targeted InstallItem command failed"
    }

    Wait-ForFile -Path $targetedMarker -TimeoutSeconds 30
    Start-Sleep -Seconds 1
    Assert-Content -Path $targetedMarker -Expected "requested"
    Assert-Missing -Path $unrelatedMarker

    & $gorillaExePath -config $configTargeted -servicestop | Out-Host
    if ($LASTEXITCODE -ne 0) {
        throw "failed to stop targeted integration service"
    }
    & $gorillaExePath -config $configTargeted -serviceremove | Out-Host
    if ($LASTEXITCODE -ne 0) {
        throw "failed to remove targeted integration service"
    }
    $targetedServiceInstalled = $false
    $results += "[PASS] Targeted App Catalog execution"
    Write-Host "[PASS] Requested optional item executed without unrelated managed convergence"
    Write-Host "::endgroup::"

    # This harness owns the marker root: it removes any prior contents and
    # recreates the directory during environment setup. Remove it after a
    # successful run so the next validation layer sees a clean machine. Keep
    # it on failure so the workflow can upload Gorilla's logs as diagnostics.
    Remove-Item -LiteralPath $markerRoot -Recurse -Force -ErrorAction Stop
    Assert-Missing -Path $markerRoot
    Write-Host "[INFO] Removed integration data directory: $markerRoot"

    Write-Host "========== Integration Test Summary =========="
    $results | ForEach-Object { Write-Host $_ }
    Write-Host "[PASS] Gorilla released-binary integration run passed"

    if ($env:GITHUB_STEP_SUMMARY) {
        @"
## Windows Released-Binary Integration Results

- PASS: Install (exe, msi, nupkg, ps1, msix)
- PASS: Update (exe, msi, nupkg, ps1, msix)
- PASS: Uninstall (exe, msi, nupkg, ps1, msix)
- PASS: Targeted App Catalog execution (requested item only; unrelated managed install remained absent)

Overall: PASS
"@ | Add-Content -LiteralPath $env:GITHUB_STEP_SUMMARY
    }
} catch {
    $errorMessage = $_.Exception.Message
    $annotationMessage = ("Phase '$currentPhase': $errorMessage").Replace("%", "%25").Replace("`r", "%0D").Replace("`n", "%0A")
    Write-Host "========== Integration Test Summary =========="
    $results | ForEach-Object { Write-Host $_ }
    Write-Host "[FAIL] Phase: $currentPhase"
    Write-Host "[FAIL] $errorMessage"
    Write-Host "::error title=Windows integration failed::$annotationMessage"

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
    if ($targetedServiceInstalled) {
        & $gorillaExePath -config $configTargeted -servicestop *> $null
        & $gorillaExePath -config $configTargeted -serviceremove *> $null
    }
    if ($serverProc -and -not $serverProc.HasExited) {
        Stop-Process -Id $serverProc.Id -Force -ErrorAction SilentlyContinue
    }
}
