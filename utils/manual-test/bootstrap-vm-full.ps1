<#
.SYNOPSIS
Bootstrap Gorilla on a Windows VM and install full integration test prerequisites.

.DESCRIPTION
- Runs bootstrap-vm.ps1 to install gorilla.exe + config.
- Installs prerequisite tools needed by integration/windows/run-release-integration.ps1.
#>

[CmdletBinding()]
param(
    [string]$BaseUrl = "http://localhost:8080/",
    [string]$InstallPath = "$env:ProgramData\gorilla\bin",
    [string]$ConfigPath = "$env:ProgramData\gorilla\config.yaml",
    [string]$AppDataPath = "$env:ProgramData\gorilla",
    [string]$Manifest = "example_manifest",
    [string[]]$Catalogs = @("example_catalog"),
    [switch]$InstallService,
    [switch]$StartService,
    [switch]$NoPause
)

$ErrorActionPreference = "Stop"

function Write-Step {
    param([string]$Message)
    Write-Host "==> $Message" -ForegroundColor Cyan
}

function Test-IsAdministrator {
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal($identity)
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Ensure-PathEntry {
    param([string]$DirectoryPath)

    if (-not (Test-Path -LiteralPath $DirectoryPath)) {
        return
    }

    $resolvedDir = (Resolve-Path -LiteralPath $DirectoryPath).Path.TrimEnd("\")
    $machinePath = [Environment]::GetEnvironmentVariable("Path", "Machine")
    if (-not $machinePath) {
        $machinePath = ""
    }

    $machineEntries = @($machinePath -split ";" | Where-Object { $_ -ne "" })
    $normalizedMachineEntries = @($machineEntries | ForEach-Object { $_.TrimEnd("\").ToLowerInvariant() })
    $normalizedDir = $resolvedDir.ToLowerInvariant()

    if ($normalizedMachineEntries -notcontains $normalizedDir) {
        $newMachinePath = if ($machinePath) { "$machinePath;$resolvedDir" } else { $resolvedDir }
        [Environment]::SetEnvironmentVariable("Path", $newMachinePath, "Machine")
        Write-Step "Added $resolvedDir to machine PATH"
    }

    $processEntries = @($env:Path -split ";" | Where-Object { $_ -ne "" })
    $normalizedProcessEntries = @($processEntries | ForEach-Object { $_.TrimEnd("\").ToLowerInvariant() })
    if ($normalizedProcessEntries -notcontains $normalizedDir) {
        $env:Path = if ($env:Path) { "$env:Path;$resolvedDir" } else { $resolvedDir }
    }
}

function Ensure-Chocolatey {
    if (Get-Command choco.exe -ErrorAction SilentlyContinue) {
        Write-Step "Chocolatey already installed"
        return
    }

    Write-Step "Installing Chocolatey"
    Set-ExecutionPolicy Bypass -Scope Process -Force
    [System.Net.ServicePointManager]::SecurityProtocol = [System.Net.SecurityProtocolType]::Tls12
    Invoke-Expression ((New-Object System.Net.WebClient).DownloadString('https://community.chocolatey.org/install.ps1'))
    Ensure-PathEntry -DirectoryPath "$env:ProgramData\chocolatey\bin"

    if (-not (Get-Command choco.exe -ErrorAction SilentlyContinue)) {
        throw "choco.exe not found in PATH after Chocolatey install."
    }
}

function Install-ChocoPackageIfMissing {
    param(
        [string]$CommandName,
        [string]$PackageName
    )

    if (Get-Command $CommandName -ErrorAction SilentlyContinue) {
        Write-Step "$CommandName already available"
        return
    }

    Write-Step "Installing $PackageName via Chocolatey"
    & choco install $PackageName -y --no-progress | Out-Host
}

if (-not (Test-IsAdministrator)) {
    throw "Run this script from an elevated PowerShell session (Run as Administrator)."
}

$scriptRoot = Split-Path -Parent $PSCommandPath
$bootstrapScript = Join-Path $scriptRoot "bootstrap-vm.ps1"

if (-not (Test-Path -LiteralPath $bootstrapScript)) {
    throw "Missing bootstrap script: $bootstrapScript"
}

Write-Step "Running base VM bootstrap"
& $bootstrapScript `
    -BaseUrl $BaseUrl `
    -InstallPath $InstallPath `
    -ConfigPath $ConfigPath `
    -AppDataPath $AppDataPath `
    -Manifest $Manifest `
    -Catalogs $Catalogs `
    -InstallService:$InstallService `
    -StartService:$StartService `
    -NoPause

Ensure-Chocolatey
Install-ChocoPackageIfMissing -CommandName "go.exe" -PackageName "golang"
Install-ChocoPackageIfMissing -CommandName "candle.exe" -PackageName "wixtoolset"

Ensure-PathEntry -DirectoryPath "$env:ProgramFiles\Go\bin"
Ensure-PathEntry -DirectoryPath "${env:ProgramFiles(x86)}\WiX Toolset v3.11\bin"

if (-not (Get-Command go.exe -ErrorAction SilentlyContinue)) {
    throw "go.exe not found in PATH after installation."
}

if (-not (Get-Command candle.exe -ErrorAction SilentlyContinue)) {
    throw "candle.exe not found in PATH after installation."
}

if (-not (Get-Command light.exe -ErrorAction SilentlyContinue)) {
    throw "light.exe not found in PATH after installation."
}

Write-Step "Full bootstrap complete"
Write-Host "Prereqs ready for integration/windows/run-release-integration.ps1"

if (-not $NoPause) {
    Write-Host ""
    Write-Host "Press any key to continue..."
    [void]$Host.UI.RawUI.ReadKey("NoEcho,IncludeKeyDown")
}
