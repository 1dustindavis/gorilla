param(
    [string]$WorkRoot = "$env:RUNNER_TEMP\gorilla-ui-e2e",
    [string]$GorillaExePath = "",
    [int]$MaxAttempts = 1
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
$appDataPath = "C:\ProgramData\gorilla-ui-e2e"
$uiCachePath = Join-Path $root "ui-state\optional-installs-cache.json"
$resultsDirectory = Join-Path $root "ui-results"
$artifactsDirectory = Join-Path $root "ui-artifacts"

if (-not (Test-Path -LiteralPath $serverExe) -or -not (Test-Path -LiteralPath $catalogPath)) {
    Write-Host "[INFO] Reusing Windows integration fixture preparation for UI E2E"
    & $prepareScript -WorkRoot $root
    if ($LASTEXITCODE -ne 0) {
        throw "prepare-release-integration.ps1 failed with exit code $LASTEXITCODE"
    }
}

@'
name: ui-e2e
optional_installs:
  - Ps1V1
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

function Invoke-TestPhase {
    param(
        [Parameter(Mandatory)][string]$Phase,
        [Parameter(Mandatory)][string]$Filter,
        [Parameter(Mandatory)][int]$Attempt
    )

    & $uiTestScript `
        -ResultsDirectory $resultsDirectory `
        -ArtifactsDirectory $artifactsDirectory `
        -TestFilter $Filter `
        -ResultPrefix "windows-ui-$($Phase.ToLowerInvariant())-scenario-$Attempt" `
        -MaxAttempts 1 `
        -SkipBuild
}

for ($scenarioAttempt = 1; $scenarioAttempt -le $MaxAttempts; $scenarioAttempt++) {
    Write-Host "UI E2E scenario attempt $scenarioAttempt/$MaxAttempts"
    $serverProc = $null
    try {
        Remove-TestService
        Remove-Item -LiteralPath $appDataPath -Recurse -Force -ErrorAction SilentlyContinue
        Remove-Item -LiteralPath $markerPath -Force -ErrorAction SilentlyContinue
        Remove-Item -LiteralPath (Split-Path -Parent $uiCachePath) -Recurse -Force -ErrorAction SilentlyContinue
        New-Item -ItemType Directory -Path (Split-Path -Parent $configPath), $resultsDirectory, $artifactsDirectory -Force | Out-Null

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
service_name: $serviceName
service_pipe_name: $servicePipeName
service_interval: 24h
"@ | Set-Content -LiteralPath $configPath -NoNewline

        $env:GORILLA_UI_PIPE_NAME = $servicePipeName
        $env:GORILLA_UI_CACHE_PATH = $uiCachePath
        $env:GORILLA_UI_E2E_MARKER_PATH = $markerPath
        $env:GORILLA_UI_E2E_CACHE_PATH = $uiCachePath

        & $GorillaExePath -config $configPath -serviceinstall | Out-Host
        if ($LASTEXITCODE -ne 0) { throw "Failed to install source-built Gorilla service" }
        & $GorillaExePath -config $configPath -servicestart | Out-Host
        if ($LASTEXITCODE -ne 0) { throw "Failed to start source-built Gorilla service" }
        Wait-ServiceState -Expected "Running"

        Invoke-TestPhase -Phase "healthy" -Filter "E2EPhase=Healthy|FullyQualifiedName~AppLaunchSmokeTests" -Attempt $scenarioAttempt

        # The unavailable-service workflow deliberately terminates the real service process.
        # This avoids coupling E2E reliability to graceful SCM shutdown while still proving
        # that the UI recovers from the production service boundary disappearing.
        Stop-TestServiceProcess
        Invoke-TestPhase -Phase "service-unavailable" -Filter "E2EPhase=ServiceUnavailable" -Attempt $scenarioAttempt
        Write-Host "UI E2E scenario passed"
        return
    } catch {
        Write-Warning "UI E2E scenario attempt $scenarioAttempt failed: $_"
        if ($scenarioAttempt -ge $MaxAttempts) {
            throw
        }
        Start-Sleep -Seconds (15 * $scenarioAttempt)
    } finally {
        Remove-TestService
        if ($serverProc -and -not $serverProc.HasExited) {
            Stop-Process -Id $serverProc.Id -Force -ErrorAction SilentlyContinue
        }
    }
}
