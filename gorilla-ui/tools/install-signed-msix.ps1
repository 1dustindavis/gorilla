param(
    [string]$OutputDir = "",
    [string]$CertFileName = "Gorilla.UI.App.cer",
    [string]$MsixFileName = "Gorilla.UI.App.signed.msix",
    [ValidateSet("LocalMachine", "CurrentUser")]
    [string]$StoreScope = "LocalMachine"
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($OutputDir)) {
    $repoRoot = Split-Path (Split-Path $PSScriptRoot -Parent) -Parent
    $OutputDir = Join-Path $repoRoot "build"
}

function Write-Section {
    param([string]$Message)
    Write-Host ""
    Write-Host "==> $Message"
}

$certPath = Join-Path $OutputDir $CertFileName
$msixPath = Join-Path $OutputDir $MsixFileName
$logPath = Join-Path $OutputDir "win-install.log"

if (-not (Test-Path $certPath)) {
    throw "Certificate file not found: $certPath"
}

if (-not (Test-Path $msixPath)) {
    throw "MSIX file not found: $msixPath"
}

if (Test-Path $logPath) {
    Remove-Item $logPath -Force
}

$store = if ($StoreScope -eq "CurrentUser") {
    "Cert:\CurrentUser\TrustedPeople"
} else {
    "Cert:\LocalMachine\TrustedPeople"
}

Write-Section "Import certificate into TrustedPeople ($StoreScope)"
Import-Certificate -FilePath $certPath -CertStoreLocation $store *>&1 | Tee-Object -FilePath $logPath

Write-Section "Verify MSIX signature"
Get-AuthenticodeSignature $msixPath | Format-List Status,StatusMessage,SignerCertificate *>&1 | Tee-Object -FilePath $logPath -Append

Write-Section "Install package"
Add-AppxPackage -Path $msixPath *>&1 | Tee-Object -FilePath $logPath -Append

Write-Host ""
Write-Host "Done."
Write-Host "Installed package: $msixPath"
Write-Host "Install log: $logPath"
