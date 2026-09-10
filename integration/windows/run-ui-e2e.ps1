param(
    [string]$WorkRoot = "$env:RUNNER_TEMP\gorilla-ui-e2e",
    [string]$GorillaExePath = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "../..")).Path
$uiTestScript = Join-Path $repoRoot "gorilla-ui\tests\Gorilla.UI.App.WindowsUiTests\run-tests.ps1"
$prepareScript = Join-Path $PSScriptRoot "prepare-release-integration.ps1"
if ([string]::IsNullOrWhiteSpace($GorillaExePath)) {
    $GorillaExePath = Join-Path $repoRoot "build\gorilla.exe"
}
$GorillaExePath = [System.IO.Path]::GetFullPath($GorillaExePath)
if (-not (Test-Path -LiteralPath $GorillaExePath)) {
    throw "Source-built gorilla.exe not found: $GorillaExePath"
}

$root = [System.IO.Path]::GetFullPath($WorkRoot)
$fixtureRoot = Join-Path $root "fixture"
$repoFixtureRoot = Join-Path $fixtureRoot "repo"
$serverExe = Join-Path $fixtureRoot "tools\fixture-server.exe"
$catalogPath = Join-Path $repoFixtureRoot "catalogs\integration.yaml"
$manifestPath = Join-Path $repoFixtureRoot "manifests\ui-e2e.yaml"
$configPath = Join-Path $fixtureRoot "configs\ui-e2e.yaml"
$serviceName = "gorilla-ui-e2e"
$servicePipeName = "gorilla-ui-e2e"
$markerPath = "C:\ProgramData\gorilla-it\ps1.txt"
$failureMarkerPath = "C:\ProgramData\gorilla-it\ps1-failure.txt"
$appDataPath = "C:\ProgramData\gorilla-ui-e2e"
$serviceLogPath = Join-Path $appDataPath "gorilla.log"
$serviceManifestPath = Join-Path $appDataPath "service-manifest.yaml"
$externalInstallScriptPath = Join-Path $repoFixtureRoot "packages\scripts\marker-install-v1.ps1"
$uiCachePath = Join-Path $root "ui-state\optional-installs-cache.json"
$evidenceRoot = Join-Path $root "ui-evidence"

Remove-Item -LiteralPath $evidenceRoot -Recurse -Force -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Path $evidenceRoot -Force | Out-Null

if (-not (Test-Path -LiteralPath $serverExe) -or -not (Test-Path -LiteralPath $catalogPath)) {
    Write-Host "[INFO] Reusing Windows integration fixture preparation for UI E2E"
    & $prepareScript -WorkRoot $root
    if ($LASTEXITCODE -ne 0) {
        throw "prepare-release-integration.ps1 failed with exit code $LASTEXITCODE"
    }
}

$failureScriptPath = Join-Path $repoFixtureRoot "packages\scripts\intentional-failure.ps1"
@'
Write-Error "Intentional App Catalog E2E installer failure"
exit 7
'@ | Set-Content -LiteralPath $failureScriptPath -NoNewline
$failureScriptHash = (Get-FileHash -LiteralPath $failureScriptPath -Algorithm SHA256).Hash.ToLowerInvariant()

$catalogRaw = Get-Content -LiteralPath $catalogPath -Raw
if ($catalogRaw -notmatch '(?m)^Ps1Failure:') {
    @"

Ps1Failure:
  display_name: Ps1Failure
  check:
    file:
      - path: '$failureMarkerPath'
  installer:
    type: ps1
    location: packages/scripts/intentional-failure.ps1
    hash: $failureScriptHash
  version: 1.0.0
"@ | Add-Content -LiteralPath $catalogPath -NoNewline
}

@'
name: ui-e2e
optional_installs:
  - Ps1V1
  - Ps1Failure
'@ | Set-Content -LiteralPath $manifestPath -NoNewline

function Wait-ServiceState {
    param(
        [Parameter(Mandatory)][string]$Expected,
        [int]$TimeoutSeconds = 20
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        $service = Get-Service -Name $serviceName -ErrorAction SilentlyContinue
        if ($service -and $service.Status.ToString() -eq $Expected) {
            return
        }
        Start-Sleep -Milliseconds 250
    } while ((Get-Date) -lt $deadline)

    $actual = (Get-Service -Name $serviceName -ErrorAction SilentlyContinue)?.Status
    throw "Timed out waiting for service '$serviceName' state '$Expected'. Actual: $actual"
}

function Stop-TestServiceProcess {
    $service = Get-CimInstance Win32_Service -Filter "Name='$serviceName'" -ErrorAction SilentlyContinue
    if (-not $service) {
        return
    }
    if ($service.ProcessId -gt 0) {
        Stop-Process -Id $service.ProcessId -Force -ErrorAction Stop
    }
    Wait-ServiceState -Expected "Stopped"
}

function Remove-TestService {
    $existing = Get-Service -Name $serviceName -ErrorAction SilentlyContinue
    if (-not $existing) {
        return
    }

    try {
        Stop-TestServiceProcess
    } catch {
        Write-Warning "Unable to terminate leftover E2E service process: $_"
    }

    & sc.exe delete $serviceName | Out-Host
    $deadline = (Get-Date).AddSeconds(20)
    do {
        if (-not (Get-Service -Name $serviceName -ErrorAction SilentlyContinue)) {
            return
        }
        Start-Sleep -Milliseconds 250
    } while ((Get-Date) -lt $deadline)
    throw "Timed out removing E2E service '$serviceName'"
}

function Copy-PhaseServiceEvidence {
    param([Parameter(Mandatory)][string]$PhaseDirectory)

    try {
        if (Test-Path -LiteralPath $serviceLogPath) {
            Copy-Item -LiteralPath $serviceLogPath -Destination (Join-Path $PhaseDirectory "gorilla.log") -Force
        }
        if (Test-Path -LiteralPath $serviceManifestPath) {
            Copy-Item -LiteralPath $serviceManifestPath -Destination (Join-Path $PhaseDirectory "service-manifest.yaml") -Force
        }
    } catch {
        Write-Warning "Unable to copy Gorilla service evidence: $_"
    }

    try {
        $service = Get-CimInstance Win32_Service -Filter "Name='$serviceName'" -ErrorAction SilentlyContinue
        if ($service) {
            @(
                "Name: $($service.Name)",
                "State: $($service.State)",
                "Status: $($service.Status)",
                "ProcessId: $($service.ProcessId)",
                "PathName: $($service.PathName)"
            ) | Set-Content -LiteralPath (Join-Path $PhaseDirectory "service-info.txt")
        }
    } catch {
        Write-Warning "Unable to capture Gorilla service process information: $_"
    }
}

function Invoke-TestPhase {
    param(
        [Parameter(Mandatory)][string]$Phase,
        [Parameter(Mandatory)][string]$Filter
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
            -SkipBuild
    } finally {
        Copy-PhaseServiceEvidence -PhaseDirectory $phaseDirectory
        Remove-Item Env:GORILLA_UI_LOG_PATH -ErrorAction SilentlyContinue
    }
}

function Get-ManagedRunStartCount {
    if (-not (Test-Path -LiteralPath $serviceLogPath)) {
        return 0
    }
    return @(
        Select-String -LiteralPath $serviceLogPath -SimpleMatch "Retrieving manifest:" -ErrorAction SilentlyContinue
    ).Count
}

function Wait-ManagedRunStartCount {
    param(
        [Parameter(Mandatory)][int]$MinimumCount,
        [int]$TimeoutSeconds = 30
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        $count = Get-ManagedRunStartCount
        if ($count -ge $MinimumCount) {
            return
        }
        Start-Sleep -Milliseconds 250
    } while ((Get-Date) -lt $deadline)

    throw "Timed out waiting for managed run start count $MinimumCount. Actual: $(Get-ManagedRunStartCount)"
}

function Get-ManagedRunCompletionCount {
    if (-not (Test-Path -LiteralPath $serviceLogPath)) {
        return 0
    }
    return @(
        Select-String -LiteralPath $serviceLogPath -SimpleMatch "Done!" -ErrorAction SilentlyContinue
    ).Count
}

function Wait-ManagedRunCompletionCount {
    param(
        [Parameter(Mandatory)][int]$MinimumCount,
        [int]$TimeoutSeconds = 30
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        $count = Get-ManagedRunCompletionCount
        if ($count -ge $MinimumCount) {
            return
        }
        Start-Sleep -Milliseconds 250
    } while ((Get-Date) -lt $deadline)

    throw "Timed out waiting for managed run completion count $MinimumCount. Actual: $(Get-ManagedRunCompletionCount)"
}

function Assert-OneTimeRemovalSurvivesScheduledRun {
    if (Test-Path -LiteralPath $markerPath) {
        throw "Expected Ps1V1 to be absent after App Catalog removal before lifecycle validation"
    }
    if (-not (Test-Path -LiteralPath $serviceManifestPath)) {
        throw "Expected service-managed manifest after App Catalog mutation: $serviceManifestPath"
    }

    $persistentManifest = Get-Content -LiteralPath $serviceManifestPath -Raw
    if ($persistentManifest -match '(?m)^\s*uninstalls\s*:') {
        throw "App Catalog removal persisted an uninstall policy: $serviceManifestPath"
    }
    if ($persistentManifest -match '(?m)^\s*-\s*Ps1V1\s*$') {
        throw "Ps1V1 remained in persistent service-managed selections after removal: $serviceManifestPath"
    }

    # Restart with a short interval so we can observe an actual ticker-triggered
    # service run using only persisted policy. Wait for the immediate startup run
    # to finish before simulating the out-of-band reinstall.
    Stop-TestServiceProcess
    (Get-Content -LiteralPath $configPath -Raw).Replace("service_interval: 24h", "service_interval: 2s") |
        Set-Content -LiteralPath $configPath -NoNewline

    $runsBeforeRestart = Get-ManagedRunCompletionCount
    & $GorillaExePath -config $configPath -integration-test-service-identity $serviceName -servicestart | Out-Host
    if ($LASTEXITCODE -ne 0) { throw "Failed to restart source-built Gorilla service for scheduled-run validation" }
    Wait-ServiceState -Expected "Running"
    Wait-ManagedRunCompletionCount -MinimumCount ($runsBeforeRestart + 1)

    if (-not (Test-Path -LiteralPath $externalInstallScriptPath)) {
        throw "External install fixture not found: $externalInstallScriptPath"
    }
    & $externalInstallScriptPath
    if (-not (Test-Path -LiteralPath $markerPath)) {
        throw "Out-of-band Ps1V1 reinstall did not create marker: $markerPath"
    }

    # Establish the run-start baseline only after the external reinstall has
    # completed. Requiring a later start proves that a managed run begins after
    # the reinstall rather than allowing an already-completed ticker run to
    # satisfy the assertion.
    $startsAfterReinstall = Get-ManagedRunStartCount
    Wait-ManagedRunStartCount -MinimumCount ($startsAfterReinstall + 1)

    # Once that post-reinstall run has definitely started, take a fresh
    # completion baseline. If it already finished before this read, the next
    # completion will come from an even later scheduled run; either way the
    # completion we wait for necessarily occurs after the external reinstall.
    $completionsAfterObservedStart = Get-ManagedRunCompletionCount
    Wait-ManagedRunCompletionCount -MinimumCount ($completionsAfterObservedStart + 1)

    if (-not (Test-Path -LiteralPath $markerPath)) {
        throw "Scheduled Gorilla run re-enforced a stale App Catalog uninstall after external reinstall"
    }

    $phaseDirectory = Join-Path $evidenceRoot "one-time-removal"
    New-Item -ItemType Directory -Path $phaseDirectory -Force | Out-Null
    @(
        "Persistent manifest after remove:",
        $persistentManifest.TrimEnd(),
        "",
        "Managed runs before service restart: $runsBeforeRestart",
        "Verified startup completion: $($runsBeforeRestart + 1)",
        "Run starts observed immediately after external reinstall: $startsAfterReinstall",
        "Run starts after ordering assertion: $(Get-ManagedRunStartCount)",
        "Completions after observed post-reinstall start: $(Get-ManagedRunCompletionCount)",
        "Marker present after scheduled run: $(Test-Path -LiteralPath $markerPath)"
    ) | Set-Content -LiteralPath (Join-Path $phaseDirectory "lifecycle.txt")
    Copy-PhaseServiceEvidence -PhaseDirectory $phaseDirectory
}

$serverProc = $null
try {
    Remove-TestService
    Remove-Item -LiteralPath $appDataPath -Recurse -Force -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath $markerPath -Force -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath $failureMarkerPath -Force -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath (Split-Path -Parent $uiCachePath) -Recurse -Force -ErrorAction SilentlyContinue
    New-Item -ItemType Directory -Path (Split-Path -Parent $configPath), $evidenceRoot -Force | Out-Null

    $serverPort = Get-Random -Minimum 19000 -Maximum 19999
    $serverProc = Start-Process -FilePath $serverExe `
        -ArgumentList @("-addr", "127.0.0.1:$serverPort", "-root", $repoFixtureRoot) `
        -PassThru -WindowStyle Hidden
    Start-Sleep -Seconds 1
    if ($serverProc.HasExited) {
        throw "Fixture HTTP server exited during startup"
    }

    $fileUrl = "http://127.0.0.1:$serverPort/"
    @"
url: $fileUrl
manifest: ui-e2e
catalogs:
  - integration
app_data_path: C:/ProgramData/gorilla-ui-e2e
service_interval: 24h
debug: true
"@ | Set-Content -LiteralPath $configPath -NoNewline

    $env:GORILLA_UI_E2E_PIPE_NAME = $servicePipeName
    $env:GORILLA_UI_CACHE_PATH = $uiCachePath
    $env:GORILLA_UI_E2E_MARKER_PATH = $markerPath
    $env:GORILLA_UI_E2E_CACHE_PATH = $uiCachePath

    & $GorillaExePath -config $configPath -integration-test-service-identity $serviceName -serviceinstall | Out-Host
    if ($LASTEXITCODE -ne 0) { throw "Failed to install source-built Gorilla service" }
    & $GorillaExePath -config $configPath -integration-test-service-identity $serviceName -servicestart | Out-Host
    if ($LASTEXITCODE -ne 0) { throw "Failed to start source-built Gorilla service" }
    Wait-ServiceState -Expected "Running"

    Invoke-TestPhase -Phase "healthy" -Filter "E2EPhase=Healthy|FullyQualifiedName~AppLaunchSmokeTests"
    Assert-OneTimeRemovalSurvivesScheduledRun

    # The unavailable-service workflow deliberately terminates the real service process.
    # This avoids coupling E2E reliability to graceful SCM shutdown while still proving
    # that the UI recovers from the production service boundary disappearing.
    Stop-TestServiceProcess
    Invoke-TestPhase -Phase "service-unavailable" -Filter "E2EPhase=ServiceUnavailable"
    Write-Host "UI E2E scenario passed"
} catch {
    $originalError = $_
    try {
        $originalError | Out-String | Set-Content -LiteralPath (Join-Path $evidenceRoot "harness-failure.txt")
    } catch {
        Write-Warning "Unable to capture UI E2E harness failure evidence: $_"
    }
    Write-Warning "UI E2E scenario failed: $originalError"
    throw $originalError
} finally {
    Remove-TestService
    if ($serverProc -and -not $serverProc.HasExited) {
        Stop-Process -Id $serverProc.Id -Force -ErrorAction SilentlyContinue
    }
}
