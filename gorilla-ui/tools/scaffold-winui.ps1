param(
    [string]$RepoRoot = "."
)

$ErrorActionPreference = "Stop"

$repo = Resolve-Path $RepoRoot
Set-Location $repo

$uiRoot = Join-Path $repo "gorilla-ui"
$solutionPath = Join-Path $uiRoot "Gorilla.UI.sln"
$appDir = Join-Path $uiRoot "src/Gorilla.UI.App"
$appProject = Join-Path $appDir "Gorilla.UI.App.csproj"
$clientProject = Join-Path $uiRoot "src/Gorilla.UI.Client/Gorilla.UI.Client.csproj"

if (-not (Test-Path $solutionPath)) {
    throw "Solution file not found at $solutionPath"
}

$templateAvailable = dotnet new list | Select-String -Pattern "winui3"
if (-not $templateAvailable) {
    throw "WinUI 3 template not found. Install WinUI/Windows App SDK templates in Visual Studio first."
}

dotnet new winui3 -n Gorilla.UI.App -o $appDir --force

dotnet add $appProject reference $clientProject

dotnet sln $solutionPath add $appProject

Write-Host "WinUI app scaffolded and wired to Gorilla.UI.Client"
Write-Host "Project: $appProject"
Write-Host "Solution: $solutionPath"
