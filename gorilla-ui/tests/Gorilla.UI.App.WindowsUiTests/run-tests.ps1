param(
    [string]$ResultsDirectory = "$PSScriptRoot\TestResults",
    [string]$ArtifactsDirectory = "$PSScriptRoot\artifacts",
    [int]$MaxAttempts = 1,
    [switch]$SkipBuild
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Invoke-DotNet {
    param([Parameter(Mandatory)][string[]]$Arguments)

    & dotnet @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "dotnet $($Arguments -join ' ') failed with exit code $LASTEXITCODE"
    }
}

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "../../..")).Path
$appProject = Join-Path $repoRoot "gorilla-ui\src\Gorilla.UI.App\Gorilla.UI.App.csproj"
$testProject = Join-Path $PSScriptRoot "Gorilla.UI.App.WindowsUiTests.csproj"
$appExe = Join-Path $repoRoot "gorilla-ui\src\Gorilla.UI.App\bin\x64\Release\net8.0-windows10.0.19041.0\win-x64\Gorilla.UI.App.exe"

New-Item -ItemType Directory -Path $ResultsDirectory -Force | Out-Null
New-Item -ItemType Directory -Path $ArtifactsDirectory -Force | Out-Null

if (-not $SkipBuild) {
    Invoke-DotNet @("restore", $appProject)
    Invoke-DotNet @("restore", $testProject)
    Invoke-DotNet @(
        "build", $appProject,
        "-c", "Release",
        "-p:Platform=x64",
        "-p:WindowsPackageType=None",
        "-p:WindowsAppSDKSelfContained=true",
        "-p:PublishReadyToRun=false",
        "-p:PublishTrimmed=false",
        "--no-restore"
    )
    Invoke-DotNet @("build", $testProject, "-c", "Release", "--no-restore")
}

if (-not (Test-Path -LiteralPath $appExe)) {
    if ($SkipBuild) {
        throw "Expected Release x64 Gorilla.UI.App.exe was not found at $appExe. Run the Windows UI build before test execution."
    }
    throw "Expected Release x64 Gorilla.UI.App.exe was not found after build: $appExe"
}

Write-Host "Using Windows UI executable: $appExe"
$env:GORILLA_UI_APP_EXE = $appExe
$env:WINDOWS_UI_TEST_RESULTS_DIR = $ResultsDirectory
$env:WINDOWS_UI_TEST_ARTIFACTS_DIR = $ArtifactsDirectory

for ($attempt = 1; $attempt -le $MaxAttempts; $attempt++) {
    Write-Host "Windows UI test attempt $attempt/$MaxAttempts"
    & dotnet test $testProject `
        -c Release `
        --no-build `
        --logger "trx;LogFileName=windows-ui-test-attempt-$attempt.trx" `
        --results-directory $ResultsDirectory

    if ($LASTEXITCODE -eq 0) {
        exit 0
    }

    if ($attempt -lt $MaxAttempts) {
        Start-Sleep -Seconds (15 * $attempt)
    }
}

throw "Windows UI tests failed after $MaxAttempts attempt(s)"
