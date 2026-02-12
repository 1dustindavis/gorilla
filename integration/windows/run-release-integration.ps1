param(
    [string]$WorkRoot = "$env:RUNNER_TEMP\gorilla-release-integration",
    [Parameter(Mandatory = $true)]
    [string]$GorillaExePath
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Require-Command {
    param([string]$Name)
    if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
        throw "Required command not found in PATH: $Name"
    }
}

function Get-FileSha256 {
    param([string]$Path)
    return (Get-FileHash -Path $Path -Algorithm SHA256).Hash.ToLowerInvariant()
}

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

function Resolve-PythonCommand {
    if (Get-Command python -ErrorAction SilentlyContinue) {
        return "python"
    }
    if (Get-Command py -ErrorAction SilentlyContinue) {
        return "py"
    }
    throw "python is required to host local integration fixtures"
}

$root = [System.IO.Path]::GetFullPath($WorkRoot)
$fixtureRoot = Join-Path $root "fixture"
$repoRoot = Join-Path $fixtureRoot "repo"
$packagesRoot = Join-Path $repoRoot "packages"
$catalogsRoot = Join-Path $repoRoot "catalogs"
$manifestsRoot = Join-Path $repoRoot "manifests"
$configRoot = Join-Path $fixtureRoot "configs"
$toolsRoot = Join-Path $fixtureRoot "tools"
$msiBuildRoot = Join-Path $fixtureRoot "msi"
$chocoRoot = Join-Path $fixtureRoot "choco"

$markerRoot = "C:\ProgramData\gorilla-it"
$exeMarker = Join-Path $markerRoot "exe.txt"
$msiMarker = Join-Path $markerRoot "msi.txt"
$nupkgMarker = Join-Path $markerRoot "nupkg.txt"
$ps1Marker = Join-Path $markerRoot "ps1.txt"

Write-Host "Preparing integration workspace at $root"
Remove-Item -LiteralPath $root -Recurse -Force -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Path $packagesRoot, $catalogsRoot, $manifestsRoot, $configRoot, $toolsRoot, $msiBuildRoot, $chocoRoot -Force | Out-Null

Write-Host "Cleaning old host markers"
Remove-Item -LiteralPath $markerRoot -Recurse -Force -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Path $markerRoot -Force | Out-Null

Require-Command -Name "go"
Require-Command -Name "choco"

Write-Host "Building exe installer fixtures"
$helperGo = Join-Path $toolsRoot "marker_helper.go"
@'
package main

import (
    "flag"
    "os"
    "path/filepath"
)

func main() {
    action := flag.String("action", "install", "install or uninstall")
    marker := flag.String("marker", "", "marker file path")
    version := flag.String("version", "", "marker content")
    flag.Parse()

    if *marker == "" {
        os.Exit(2)
    }

    if *action == "uninstall" {
        if err := os.Remove(*marker); err != nil && !os.IsNotExist(err) {
            os.Exit(1)
        }
        return
    }

    if err := os.MkdirAll(filepath.Dir(*marker), 0o755); err != nil {
        os.Exit(1)
    }
    if err := os.WriteFile(*marker, []byte(*version), 0o644); err != nil {
        os.Exit(1)
    }
}
'@ | Set-Content -LiteralPath $helperGo -NoNewline

$exeInstall = Join-Path $packagesRoot "exe/marker-installer.exe"
$exeUninstall = Join-Path $packagesRoot "exe/marker-uninstaller.exe"
New-Item -ItemType Directory -Path (Split-Path -Path $exeInstall -Parent) -Force | Out-Null
& go build -o $exeInstall $helperGo
Copy-Item -LiteralPath $exeInstall -Destination $exeUninstall -Force

Write-Host "Creating ps1 installer fixtures"
$ps1InstallV1 = Join-Path $packagesRoot "scripts/marker-install-v1.ps1"
$ps1InstallV2 = Join-Path $packagesRoot "scripts/marker-install-v2.ps1"
$ps1Uninstall = Join-Path $packagesRoot "scripts/marker-uninstall.ps1"
New-Item -ItemType Directory -Path (Split-Path -Path $ps1InstallV1 -Parent) -Force | Out-Null
@'
$marker = "C:\ProgramData\gorilla-it\ps1.txt"
New-Item -Path (Split-Path -Path $marker -Parent) -ItemType Directory -Force | Out-Null
Set-Content -LiteralPath $marker -Value "1.0.0" -NoNewline
'@ | Set-Content -LiteralPath $ps1InstallV1 -NoNewline
@'
$marker = "C:\ProgramData\gorilla-it\ps1.txt"
New-Item -Path (Split-Path -Path $marker -Parent) -ItemType Directory -Force | Out-Null
Set-Content -LiteralPath $marker -Value "2.0.0" -NoNewline
'@ | Set-Content -LiteralPath $ps1InstallV2 -NoNewline
@'
$marker = "C:\ProgramData\gorilla-it\ps1.txt"
if (Test-Path -LiteralPath $marker) {
    Remove-Item -LiteralPath $marker -Force
}
'@ | Set-Content -LiteralPath $ps1Uninstall -NoNewline

Write-Host "Creating nupkg fixtures"
$nupkgDir = Join-Path $chocoRoot "gorilla-it-nupkg"
$toolsDir = Join-Path $nupkgDir "tools"
New-Item -ItemType Directory -Path $toolsDir -Force | Out-Null

@'
$marker = "C:\ProgramData\gorilla-it\nupkg.txt"
New-Item -Path (Split-Path -Path $marker -Parent) -ItemType Directory -Force | Out-Null
Set-Content -LiteralPath $marker -Value "__VERSION__" -NoNewline
'@ | Set-Content -LiteralPath (Join-Path $toolsDir "chocolateyInstall.template.ps1") -NoNewline

@'
$marker = "C:\ProgramData\gorilla-it\nupkg.txt"
if (Test-Path -LiteralPath $marker) {
    Remove-Item -LiteralPath $marker -Force
}
'@ | Set-Content -LiteralPath (Join-Path $toolsDir "chocolateyUninstall.ps1") -NoNewline

function Build-Nupkg {
    param([string]$Version)

    $nuspecPath = Join-Path $nupkgDir "gorilla-it-nupkg.nuspec"
    @"
<?xml version="1.0"?>
<package>
  <metadata>
    <id>gorilla-it-nupkg</id>
    <version>$Version</version>
    <title>gorilla-it-nupkg</title>
    <authors>gorilla-it</authors>
    <description>Integration test package</description>
  </metadata>
</package>
"@ | Set-Content -LiteralPath $nuspecPath -NoNewline

    (Get-Content -LiteralPath (Join-Path $toolsDir "chocolateyInstall.template.ps1") -Raw).
        Replace("__VERSION__", $Version) |
        Set-Content -LiteralPath (Join-Path $toolsDir "chocolateyInstall.ps1") -NoNewline

    & choco pack $nuspecPath --outputdirectory (Join-Path $packagesRoot "nupkg") | Out-Host
}

New-Item -ItemType Directory -Path (Join-Path $packagesRoot "nupkg") -Force | Out-Null
Build-Nupkg -Version "1.0.0"
Build-Nupkg -Version "2.0.0"

$nupkgV1 = Join-Path $packagesRoot "nupkg/gorilla-it-nupkg.1.0.0.nupkg"
$nupkgV2 = Join-Path $packagesRoot "nupkg/gorilla-it-nupkg.2.0.0.nupkg"

Write-Host "Creating msi fixtures"
if (-not (Get-Command candle.exe -ErrorAction SilentlyContinue)) {
    Write-Host "Installing WiX toolset for MSI fixture build"
    & choco install wixtoolset -y --no-progress | Out-Host
}
Require-Command -Name "candle.exe"
Require-Command -Name "light.exe"

function Build-Msi {
    param(
        [string]$Version,
        [string]$OutputPath
    )

    $versionRoot = Join-Path $msiBuildRoot $Version
    New-Item -ItemType Directory -Path $versionRoot -Force | Out-Null

    $markerTxt = Join-Path $versionRoot "msi.txt"
    Set-Content -LiteralPath $markerTxt -Value $Version -NoNewline

    $wxs = Join-Path $versionRoot "fixture.wxs"
    @"
<?xml version="1.0" encoding="UTF-8"?>
<Wix xmlns="http://schemas.microsoft.com/wix/2006/wi">
  <Product Id="*" Name="gorilla-it-msi" Language="1033" Version="$Version" Manufacturer="gorilla-it" UpgradeCode="4BB76213-C480-4B66-BB00-5D66FC791F09">
    <Package InstallerVersion="200" Compressed="yes" InstallScope="perMachine" />
    <MajorUpgrade DowngradeErrorMessage="A newer version is already installed." />
    <MediaTemplate EmbedCab="yes" />
    <Feature Id="MainFeature" Title="MainFeature" Level="1">
      <ComponentRef Id="MarkerComponent" />
    </Feature>
    <Directory Id="TARGETDIR" Name="SourceDir">
      <Directory Id="CommonAppDataFolder">
        <Directory Id="GorillaItFolder" Name="gorilla-it">
          <Component Id="MarkerComponent" Guid="F7B17690-7238-4D73-8E0F-EA693E2B6E8B">
            <File Id="MarkerFile" Name="msi.txt" Source="$($markerTxt -replace "\\", "\\\\")" KeyPath="yes" />
          </Component>
        </Directory>
      </Directory>
    </Directory>
  </Product>
</Wix>
"@ | Set-Content -LiteralPath $wxs -NoNewline

    $wixobj = Join-Path $versionRoot "fixture.wixobj"
    & candle.exe -nologo -out $wixobj $wxs | Out-Host
    & light.exe -nologo -out $OutputPath $wixobj | Out-Host
}

$msiV1 = Join-Path $packagesRoot "msi/gorilla-it-msi-1.0.0.msi"
$msiV2 = Join-Path $packagesRoot "msi/gorilla-it-msi-2.0.0.msi"
New-Item -ItemType Directory -Path (Split-Path -Path $msiV1 -Parent) -Force | Out-Null
Build-Msi -Version "1.0.0" -OutputPath $msiV1
Build-Msi -Version "2.0.0" -OutputPath $msiV2

$hashExeInstall = Get-FileSha256 -Path $exeInstall
$hashExeUninstall = Get-FileSha256 -Path $exeUninstall
$hashPs1InstallV1 = Get-FileSha256 -Path $ps1InstallV1
$hashPs1InstallV2 = Get-FileSha256 -Path $ps1InstallV2
$hashPs1Uninstall = Get-FileSha256 -Path $ps1Uninstall
$hashNupkgV1 = Get-FileSha256 -Path $nupkgV1
$hashNupkgV2 = Get-FileSha256 -Path $nupkgV2
$hashMsiV1 = Get-FileSha256 -Path $msiV1
$hashMsiV2 = Get-FileSha256 -Path $msiV2

$checkScriptTemplate = '$path = "__PATH__"; $want = [version]"__TARGET__"; if (!(Test-Path -LiteralPath $path)) { exit 0 }; $raw = (Get-Content -LiteralPath $path -Raw).Trim(); $have = [version]$raw; if ($have -ge $want) { exit 1 }; exit 0'

function Build-CheckScript {
    param(
        [string]$Path,
        [string]$Target
    )
    return $checkScriptTemplate.Replace("__PATH__", $Path.Replace("\\", "\\\\")).Replace("__TARGET__", $Target)
}

$catalogPath = Join-Path $catalogsRoot "integration.yaml"
$catalogContent = @"
ExeV1:
  display_name: ExeV1
  check:
    script: |
$(Build-CheckScript -Path $exeMarker -Target "1.0.0" | ForEach-Object { "      $_" })
  installer:
    type: exe
    location: packages/exe/marker-installer.exe
    hash: $hashExeInstall
    arguments:
      - -action=install
      - -marker=$exeMarker
      - -version=1.0.0
  uninstaller:
    type: exe
    location: packages/exe/marker-uninstaller.exe
    hash: $hashExeUninstall
    arguments:
      - -action=uninstall
      - -marker=$exeMarker
  version: 1.0.0

ExeV2:
  display_name: ExeV2
  check:
    script: |
$(Build-CheckScript -Path $exeMarker -Target "2.0.0" | ForEach-Object { "      $_" })
  installer:
    type: exe
    location: packages/exe/marker-installer.exe
    hash: $hashExeInstall
    arguments:
      - -action=install
      - -marker=$exeMarker
      - -version=2.0.0
  uninstaller:
    type: exe
    location: packages/exe/marker-uninstaller.exe
    hash: $hashExeUninstall
    arguments:
      - -action=uninstall
      - -marker=$exeMarker
  version: 2.0.0

MsiV1:
  display_name: MsiV1
  check:
    script: |
$(Build-CheckScript -Path $msiMarker -Target "1.0.0" | ForEach-Object { "      $_" })
  installer:
    type: msi
    location: packages/msi/gorilla-it-msi-1.0.0.msi
    hash: $hashMsiV1
  uninstaller:
    type: msi
    location: packages/msi/gorilla-it-msi-1.0.0.msi
    hash: $hashMsiV1
  version: 1.0.0

MsiV2:
  display_name: MsiV2
  check:
    script: |
$(Build-CheckScript -Path $msiMarker -Target "2.0.0" | ForEach-Object { "      $_" })
  installer:
    type: msi
    location: packages/msi/gorilla-it-msi-2.0.0.msi
    hash: $hashMsiV2
  uninstaller:
    type: msi
    location: packages/msi/gorilla-it-msi-2.0.0.msi
    hash: $hashMsiV2
  version: 2.0.0

NupkgV1:
  display_name: NupkgV1
  check:
    script: |
$(Build-CheckScript -Path $nupkgMarker -Target "1.0.0" | ForEach-Object { "      $_" })
  installer:
    type: nupkg
    location: packages/nupkg/gorilla-it-nupkg.1.0.0.nupkg
    hash: $hashNupkgV1
    package_id: gorilla-it-nupkg
  uninstaller:
    type: nupkg
    location: packages/nupkg/gorilla-it-nupkg.1.0.0.nupkg
    hash: $hashNupkgV1
    package_id: gorilla-it-nupkg
  version: 1.0.0

NupkgV2:
  display_name: NupkgV2
  check:
    script: |
$(Build-CheckScript -Path $nupkgMarker -Target "2.0.0" | ForEach-Object { "      $_" })
  installer:
    type: nupkg
    location: packages/nupkg/gorilla-it-nupkg.2.0.0.nupkg
    hash: $hashNupkgV2
    package_id: gorilla-it-nupkg
  uninstaller:
    type: nupkg
    location: packages/nupkg/gorilla-it-nupkg.2.0.0.nupkg
    hash: $hashNupkgV2
    package_id: gorilla-it-nupkg
  version: 2.0.0

Ps1V1:
  display_name: Ps1V1
  check:
    script: |
$(Build-CheckScript -Path $ps1Marker -Target "1.0.0" | ForEach-Object { "      $_" })
  installer:
    type: ps1
    location: packages/scripts/marker-install-v1.ps1
    hash: $hashPs1InstallV1
  uninstaller:
    type: ps1
    location: packages/scripts/marker-uninstall.ps1
    hash: $hashPs1Uninstall
  version: 1.0.0

Ps1V2:
  display_name: Ps1V2
  check:
    script: |
$(Build-CheckScript -Path $ps1Marker -Target "2.0.0" | ForEach-Object { "      $_" })
  installer:
    type: ps1
    location: packages/scripts/marker-install-v2.ps1
    hash: $hashPs1InstallV2
  uninstaller:
    type: ps1
    location: packages/scripts/marker-uninstall.ps1
    hash: $hashPs1Uninstall
  version: 2.0.0
"@
$catalogContent | Set-Content -LiteralPath $catalogPath -NoNewline

@'
name: integration-install
managed_installs:
  - ExeV1
  - MsiV1
  - NupkgV1
  - Ps1V1
'@ | Set-Content -LiteralPath (Join-Path $manifestsRoot "integration-install.yaml") -NoNewline

@'
name: integration-update
managed_updates:
  - ExeV2
  - MsiV2
  - NupkgV2
  - Ps1V2
'@ | Set-Content -LiteralPath (Join-Path $manifestsRoot "integration-update.yaml") -NoNewline

@'
name: integration-uninstall
managed_uninstalls:
  - ExeV2
  - MsiV2
  - NupkgV2
  - Ps1V2
'@ | Set-Content -LiteralPath (Join-Path $manifestsRoot "integration-uninstall.yaml") -NoNewline

$pythonCmd = Resolve-PythonCommand
$serverPort = Get-Random -Minimum 18080 -Maximum 18999
$serverProc = Start-Process -FilePath $pythonCmd `
    -ArgumentList @("-m", "http.server", "$serverPort", "--bind", "127.0.0.1", "--directory", $repoRoot) `
    -PassThru -WindowStyle Hidden
Start-Sleep -Seconds 2

if ($serverProc.HasExited) {
    throw "Failed to start fixture HTTP server process"
}

$fileUrl = "http://127.0.0.1:$serverPort/"
Write-Host "Serving fixture repo from $fileUrl"

function Write-Config {
    param(
        [string]$ManifestName,
        [string]$Path
    )

    @"
url: $fileUrl
manifest: $ManifestName
catalogs:
  - integration
app_data_path: C:/ProgramData/gorilla-it/cache
"@ | Set-Content -LiteralPath $Path -NoNewline
}

$configInstall = Join-Path $configRoot "install.yaml"
$configUpdate = Join-Path $configRoot "update.yaml"
$configUninstall = Join-Path $configRoot "uninstall.yaml"
Write-Config -ManifestName "integration-install" -Path $configInstall
Write-Config -ManifestName "integration-update" -Path $configUpdate
Write-Config -ManifestName "integration-uninstall" -Path $configUninstall

# Gorilla writes reports to ProgramData\gorilla regardless of app_data_path.
New-Item -ItemType Directory -Path "C:\ProgramData\gorilla" -Force | Out-Null

$gorillaExePath = [System.IO.Path]::GetFullPath($GorillaExePath)
if (-not (Test-Path -LiteralPath $gorillaExePath)) {
    throw "gorilla.exe not found at path: $gorillaExePath"
}
Write-Host "Using gorilla.exe from: $gorillaExePath"

function Run-Gorilla {
    param([string]$ConfigPath)

    Write-Host "Running gorilla with config: $ConfigPath"
    & $gorillaExePath -config $ConfigPath -verbose
    if ($LASTEXITCODE -ne 0) {
        throw "gorilla run failed for $ConfigPath with exit code $LASTEXITCODE"
    }
}

try {
    Run-Gorilla -ConfigPath $configInstall
    Assert-Content -Path $exeMarker -Expected "1.0.0"
    Assert-Content -Path $msiMarker -Expected "1.0.0"
    Assert-Content -Path $nupkgMarker -Expected "1.0.0"
    Assert-Content -Path $ps1Marker -Expected "1.0.0"
    Write-Host "Install phase validated"

    Run-Gorilla -ConfigPath $configUpdate
    Assert-Content -Path $exeMarker -Expected "2.0.0"
    Assert-Content -Path $msiMarker -Expected "2.0.0"
    Assert-Content -Path $nupkgMarker -Expected "2.0.0"
    Assert-Content -Path $ps1Marker -Expected "2.0.0"
    Write-Host "Update phase validated"

    Run-Gorilla -ConfigPath $configUninstall
    Assert-Missing -Path $exeMarker
    Assert-Missing -Path $msiMarker
    Assert-Missing -Path $nupkgMarker
    Assert-Missing -Path $ps1Marker
    Write-Host "Uninstall phase validated"

    Write-Host "Gorilla released-binary integration run passed"
} finally {
    if ($serverProc -and -not $serverProc.HasExited) {
        Stop-Process -Id $serverProc.Id -Force -ErrorAction SilentlyContinue
    }
}
