param(
    [Parameter(Mandatory = $true)]
    [string]$GorillaMsixPath,
    [string]$GorillaReleaseExePath = "",
    [string]$WorkRoot = "$env:RUNNER_TEMP\gorilla-installed-product"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "../..")).Path
$prepareScript = Join-Path $PSScriptRoot "prepare-release-integration.ps1"
$uiTestScript = Join-Path $repoRoot "gorilla-ui\tests\Gorilla.UI.App.WindowsUiTests\run-tests.ps1"
$packageIdentityName = "133b2116-358b-42fb-8bc8-35009cc5d5af"
$serviceName = "gorilla"
$root = [System.IO.Path]::GetFullPath($WorkRoot)
$msixPath = [System.IO.Path]::GetFullPath($GorillaMsixPath)
if (-not (Test-Path -LiteralPath $msixPath)) {
    throw "Produced Gorilla MSIX not found: $msixPath"
}

$fixtureRoot = Join-Path $root "fixture"
$repoFixtureRoot = Join-Path $fixtureRoot "repo"
$serverExe = Join-Path $fixtureRoot "tools\fixture-server.exe"
$manifestPath = Join-Path $repoFixtureRoot "manifests\ui-e2e.yaml"
$configPath = "C:\ProgramData\gorilla\config.yaml"
$configDirectory = Split-Path -Parent $configPath
$appDataPath = "C:\ProgramData\gorilla-it"
$markerPath = Join-Path $appDataPath "ps1.txt"
$serviceLogPath = Join-Path $appDataPath "gorilla.log"
$evidenceRoot = Join-Path $root "installed-product-evidence"
$installedByHarness = $false
$configCreatedByHarness = $false
$configDirectoryCreatedByHarness = $false
$appDataCreatedByHarness = $false

function Wait-ServiceState {
    param(
        [Parameter(Mandatory = $true)][string]$Expected,
        [int]$TimeoutSeconds = 30
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        $service = Get-Service -Name $serviceName -ErrorAction SilentlyContinue
        if ($Expected -eq "Absent" -and -not $service) {
            return
        }
        if ($service -and $service.Status.ToString() -eq $Expected) {
            return
        }
        Start-Sleep -Milliseconds 250
    } while ((Get-Date) -lt $deadline)

    $actual = (Get-Service -Name $serviceName -ErrorAction SilentlyContinue)?.Status
    throw "Timed out waiting for service '$serviceName' state '$Expected'. Actual: $actual"
}

function Remove-TestPackage {
    if (-not $script:installedByHarness) {
        return
    }

    $packages = @(Get-AppxPackage -Name $packageIdentityName -ErrorAction SilentlyContinue)
    foreach ($package in $packages) {
        Write-Host "[INFO] Removing test-installed Gorilla package $($package.PackageFullName)"
        Remove-AppxPackage -Package $package.PackageFullName -ErrorAction Stop
    }
    Wait-ServiceState -Expected "Absent"
    $script:installedByHarness = $false
}

function Assert-CleanMachine {
    if (Get-AppxPackage -Name $packageIdentityName -ErrorAction SilentlyContinue) {
        throw "Installed-product validation requires a disposable machine with no existing Gorilla MSIX installation."
    }
    if (Get-Service -Name $serviceName -ErrorAction SilentlyContinue) {
        throw "Installed-product validation requires a disposable machine with no existing '$serviceName' service."
    }
    if (Test-Path -LiteralPath $configPath) {
        throw "Installed-product validation will not overwrite existing Gorilla configuration: $configPath"
    }
    if (Test-Path -LiteralPath $appDataPath) {
        throw "Installed-product validation will not overwrite existing Gorilla integration data: $appDataPath"
    }
}

function Copy-InstalledProductEvidence {
    param([Parameter(Mandatory = $true)][string]$PhaseDirectory)

    New-Item -ItemType Directory -Path $PhaseDirectory -Force | Out-Null
    try {
        if (Test-Path -LiteralPath $serviceLogPath) {
            Copy-Item -LiteralPath $serviceLogPath -Destination (Join-Path $PhaseDirectory "gorilla.log") -Force
        }
    } catch {
        Write-Warning "Unable to copy Gorilla service log: $_"
    }

    try {
        $service = Get-CimInstance Win32_Service -Filter "Name='$serviceName'" -ErrorAction SilentlyContinue
        if ($service) {
            $service | Format-List * | Out-String | Set-Content -LiteralPath (Join-Path $PhaseDirectory "service-info.txt")
        }
    } catch {
        Write-Warning "Unable to capture service information: $_"
    }

    try {
        Get-AppxPackage -Name $packageIdentityName -ErrorAction SilentlyContinue |
            Format-List * | Out-String | Set-Content -LiteralPath (Join-Path $PhaseDirectory "package-info.txt")
    } catch {
        Write-Warning "Unable to capture package information: $_"
    }
}

function Invoke-TestPhase {
    param(
        [Parameter(Mandatory = $true)][string]$Phase,
        [Parameter(Mandatory = $true)][string]$Filter,
        [Parameter(Mandatory = $true)][string]$AppUserModelId
    )

    $phaseDirectory = Join-Path $evidenceRoot $Phase
    New-Item -ItemType Directory -Path $phaseDirectory -Force | Out-Null
    $env:GORILLA_UI_DEBUG = "1"
    $env:GORILLA_UI_LOG_PATH = Join-Path $phaseDirectory "ui-client.log"
    try {
        & $uiTestScript `
            -ResultsDirectory $phaseDirectory `
            -ArtifactsDirectory $phaseDirectory `
            -TestFilter $Filter `
            -ResultPrefix "tests" `
            -AppUserModelId $AppUserModelId `
            -SkipBuild
        if ($LASTEXITCODE -ne 0) {
            throw "Installed-package FlaUI phase '$Phase' failed with exit code $LASTEXITCODE"
        }
    } finally {
        Copy-InstalledProductEvidence -PhaseDirectory $phaseDirectory
        Remove-Item Env:GORILLA_UI_LOG_PATH -ErrorAction SilentlyContinue
    }
}

$serverProc = $null
$primaryError = $null
$cleanupError = $null
try {
    Assert-CleanMachine
    Remove-Item -LiteralPath $root -Recurse -Force -ErrorAction SilentlyContinue
    New-Item -ItemType Directory -Path $root, $evidenceRoot -Force | Out-Null

    Write-Host "[INFO] Reusing Windows integration fixture preparation for installed-product validation"
    & $prepareScript -WorkRoot $root
    if ($LASTEXITCODE -ne 0) {
        throw "prepare-release-integration.ps1 failed with exit code $LASTEXITCODE"
    }

    @'
name: ui-e2e
optional_installs:
  - Ps1V1
'@ | Set-Content -LiteralPath $manifestPath -NoNewline

    $serverPort = Get-Random -Minimum 19000 -Maximum 19999
    $serverProc = Start-Process -FilePath $serverExe `
        -ArgumentList @("-addr", "127.0.0.1:$serverPort", "-root", $repoFixtureRoot) `
        -PassThru -WindowStyle Hidden
    Start-Sleep -Seconds 1
    if ($serverProc.HasExited) {
        throw "Fixture HTTP server exited during startup"
    }

    if (-not (Test-Path -LiteralPath $configDirectory)) {
        New-Item -ItemType Directory -Path $configDirectory -Force | Out-Null
        $configDirectoryCreatedByHarness = $true
    }
    New-Item -ItemType Directory -Path $appDataPath -Force | Out-Null
    $appDataCreatedByHarness = $true

    $fileUrl = "http://127.0.0.1:$serverPort/"
    @"
url: $fileUrl
manifest: ui-e2e
catalogs:
  - integration
app_data_path: C:/ProgramData/gorilla-it
service_interval: 24h
debug: true
"@ | Set-Content -LiteralPath $configPath -NoNewline
    $configCreatedByHarness = $true

    Write-Host "[INFO] Installing produced Gorilla MSIX: $msixPath"
    Add-AppxPackage -Path $msixPath -ErrorAction Stop
    $installedByHarness = $true
    $installedPackage = Get-AppxPackage -Name $packageIdentityName -ErrorAction Stop
    if ($null -eq $installedPackage) {
        throw "Gorilla MSIX installation did not register package identity '$packageIdentityName'"
    }

    $installedGorilla = Join-Path $installedPackage.InstallLocation "gorilla.exe"
    if (-not (Test-Path -LiteralPath $installedGorilla)) {
        throw "Installed Gorilla package does not contain gorilla.exe at $installedGorilla"
    }

    if (-not [string]::IsNullOrWhiteSpace($GorillaReleaseExePath)) {
        $releaseExe = [System.IO.Path]::GetFullPath($GorillaReleaseExePath)
        if (-not (Test-Path -LiteralPath $releaseExe)) {
            throw "Produced standalone gorilla.exe not found: $releaseExe"
        }
        $packagedHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $installedGorilla).Hash
        $releaseHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $releaseExe).Hash
        if ($packagedHash -cne $releaseHash) {
            throw "The gorilla.exe inside the produced MSIX does not match the produced standalone gorilla.exe"
        }
    }

    $service = Get-CimInstance Win32_Service -Filter "Name='$serviceName'" -ErrorAction Stop
    if ($null -eq $service) {
        throw "Installing the Gorilla MSIX did not register the '$serviceName' service"
    }
    if ($service.StartName -ne "LocalSystem") {
        throw "Gorilla service account is '$($service.StartName)'; expected LocalSystem"
    }
    if ($service.StartMode -ne "Auto") {
        throw "Gorilla service startup mode is '$($service.StartMode)'; expected Auto"
    }
    if ($service.PathName -notlike "*$installedGorilla*") {
        throw "Gorilla service is not registered to the packaged executable. PathName: $($service.PathName)"
    }

    if ((Get-Service -Name $serviceName).Status -ne "Running") {
        Start-Service -Name $serviceName
    }
    Wait-ServiceState -Expected "Running"

    & $installedGorilla -config $configPath -servicecmd ListOptionalInstalls | Out-Host
    if ($LASTEXITCODE -ne 0) {
        throw "Packaged gorilla.exe could not communicate with the installed Gorilla service over the real named pipe"
    }

    $appUserModelId = "$($installedPackage.PackageFamilyName)!App"
    $uiCachePath = Join-Path $env:LOCALAPPDATA "Packages\$($installedPackage.PackageFamilyName)\LocalCache\Local\Gorilla\ui\optional-installs-cache.json"
    $env:GORILLA_UI_E2E_MARKER_PATH = $markerPath
    $env:GORILLA_UI_E2E_CACHE_PATH = $uiCachePath

    Invoke-TestPhase -Phase "healthy" -Filter "E2EPhase=Healthy|FullyQualifiedName~AppLaunchSmokeTests" -AppUserModelId $appUserModelId

    Stop-Service -Name $serviceName -Force -ErrorAction Stop
    Wait-ServiceState -Expected "Stopped"
    Invoke-TestPhase -Phase "service-unavailable" -Filter "E2EPhase=ServiceUnavailable" -AppUserModelId $appUserModelId
} catch {
    $primaryError = $_
    try {
        New-Item -ItemType Directory -Path $evidenceRoot -Force | Out-Null
        $primaryError | Out-String | Set-Content -LiteralPath (Join-Path $evidenceRoot "harness-failure.txt")
        Copy-InstalledProductEvidence -PhaseDirectory $evidenceRoot
    } catch {
        Write-Warning "Unable to capture installed-product failure evidence: $_"
    }
} finally {
    try {
        Remove-TestPackage
    } catch {
        $cleanupError = $_
        try {
            New-Item -ItemType Directory -Path $evidenceRoot -Force | Out-Null
            $cleanupError | Out-String | Set-Content -LiteralPath (Join-Path $evidenceRoot "cleanup-failure.txt")
            Copy-InstalledProductEvidence -PhaseDirectory $evidenceRoot
        } catch {
            Write-Warning "Unable to capture installed-product cleanup failure evidence: $_"
        }
    }
    if ($serverProc -and -not $serverProc.HasExited) {
        Stop-Process -Id $serverProc.Id -Force -ErrorAction SilentlyContinue
    }
    if ($appDataCreatedByHarness) {
        Remove-Item -LiteralPath $appDataPath -Recurse -Force -ErrorAction SilentlyContinue
    }
    if ($configCreatedByHarness) {
        Remove-Item -LiteralPath $configPath -Force -ErrorAction SilentlyContinue
    }
    if ($configDirectoryCreatedByHarness -and (Test-Path -LiteralPath $configDirectory) -and -not (Get-ChildItem -LiteralPath $configDirectory -Force)) {
        Remove-Item -LiteralPath $configDirectory -Force -ErrorAction SilentlyContinue
    }
}

if ($null -ne $primaryError) {
    if ($null -ne $cleanupError) {
        Write-Warning "Installed-product cleanup also failed: $cleanupError"
    }
    throw $primaryError
}
if ($null -ne $cleanupError) {
    throw $cleanupError
}

Write-Host "Installed-product integration passed"
