param(
    [string]$RepoRoot = "."
)

$ErrorActionPreference = "Stop"

$repo = Resolve-Path $RepoRoot
Set-Location $repo

$appRoot = Join-Path $repo "gorilla-ui/src/Gorilla.UI.App"
$templateRoot = Join-Path $appRoot "template"
$appProject = Join-Path $appRoot "Gorilla.UI.App.csproj"

if (-not (Test-Path $appProject)) {
    throw "WinUI app project not found at $appProject. Run scaffold-winui.ps1 first."
}

if (-not (Test-Path $templateRoot)) {
    throw "Template root not found: $templateRoot"
}

$map = @{
    "Models/UiOptionalInstallItem.cs" = "Models/UiOptionalInstallItem.cs"
    "Services/GorillaUiServices.cs"   = "Services/GorillaUiServices.cs"
    "Services/OperationTracker.cs"    = "Services/OperationTracker.cs"
    "ViewModels/HomeViewModel.cs"     = "ViewModels/HomeViewModel.cs"
    "Views/HomePage.xaml"             = "Views/HomePage.xaml"
    "Views/HomePage.xaml.cs"          = "Views/HomePage.xaml.cs"
}

foreach ($sourceRel in $map.Keys) {
    $targetRel = $map[$sourceRel]
    $source = Join-Path $templateRoot $sourceRel
    $target = Join-Path $appRoot $targetRel

    if (-not (Test-Path $source)) {
        throw "Template source missing: $source"
    }

    $targetDir = Split-Path -Parent $target
    New-Item -ItemType Directory -Path $targetDir -Force | Out-Null

    Copy-Item -Path $source -Destination $target -Force
    Write-Host "Applied template: $targetRel"
}

Write-Host "WinUI templates applied successfully."
