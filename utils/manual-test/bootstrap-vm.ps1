<#
.SYNOPSIS
Bootstrap Gorilla on a Windows VM from prepared manual-test assets.

.DESCRIPTION
- Downloads gorilla.exe from a provided base URL.
- Ensures Chocolatey exists (needed for .nupkg flows).
- Writes config.yaml to ProgramData.
- Optionally installs/starts service mode.
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
    [switch]$SkipChocolateyInstall
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

function Test-CommandExists {
    param([string]$CommandName)
    return [bool](Get-Command $CommandName -ErrorAction SilentlyContinue)
}

function Ensure-Chocolatey {
    if (Test-CommandExists "choco") {
        Write-Step "Chocolatey already installed"
        return
    }

    if ($SkipChocolateyInstall) {
        throw "Chocolatey is not installed. Re-run without -SkipChocolateyInstall, or install it manually."
    }

    Write-Step "Installing Chocolatey"
    Set-ExecutionPolicy Bypass -Scope Process -Force
    [System.Net.ServicePointManager]::SecurityProtocol = [System.Net.ServicePointManager]::SecurityProtocol -bor 3072
    Invoke-Expression ((New-Object System.Net.WebClient).DownloadString("https://community.chocolatey.org/install.ps1"))
}

function Convert-ToYamlPath {
    param([string]$PathValue)
    return ($PathValue -replace "\\", "/")
}

function Write-GorillaConfig {
    param(
        [string]$ConfigFilePath,
        [string]$URLValue,
        [string]$ManifestValue,
        [string[]]$CatalogList,
        [string]$AppDataPathValue
    )

    $configDir = Split-Path -Parent $ConfigFilePath
    New-Item -ItemType Directory -Path $configDir -Force | Out-Null
    New-Item -ItemType Directory -Path $AppDataPathValue -Force | Out-Null

    $yamlCatalogs = ($CatalogList | ForEach-Object { "  - $_" }) -join "`r`n"
    $yamlAppDataPath = Convert-ToYamlPath -PathValue $AppDataPathValue

    $content = @"
url: $URLValue
url_packages: $URLValue
manifest: $ManifestValue
catalogs:
$yamlCatalogs
app_data_path: $yamlAppDataPath
# service_name: gorilla
# service_interval: 1h
# service_pipe_name: gorilla-service
"@

    Set-Content -Path $ConfigFilePath -Value $content -Encoding ASCII
}

if (-not (Test-IsAdministrator)) {
    throw "Run this script from an elevated PowerShell session (Run as Administrator)."
}

if (-not $BaseUrl.EndsWith("/")) {
    $BaseUrl = "$BaseUrl/"
}

Ensure-Chocolatey

New-Item -ItemType Directory -Path $InstallPath -Force | Out-Null
$binaryPath = Join-Path $InstallPath "gorilla.exe"
$binaryUrl = "$BaseUrl" + "gorilla.exe"

Write-Step "Downloading Gorilla binary from $binaryUrl"
Invoke-WebRequest -Uri $binaryUrl -OutFile $binaryPath

Write-Step "Writing config to $ConfigPath"
Write-GorillaConfig -ConfigFilePath $ConfigPath -URLValue $BaseUrl -ManifestValue $Manifest -CatalogList $Catalogs -AppDataPathValue $AppDataPath

if ($InstallService) {
    Write-Step "Installing Gorilla Windows service"
    & $binaryPath -c $ConfigPath -serviceinstall
}

if ($StartService) {
    Write-Step "Starting Gorilla Windows service"
    & $binaryPath -c $ConfigPath -servicestart
}

Write-Step "Bootstrap complete"
Write-Host "Binary: $binaryPath"
Write-Host "Config: $ConfigPath"
Write-Host ""
Write-Host "Manual test command:"
Write-Host "& `"$binaryPath`" -c `"$ConfigPath`" -C -v"
