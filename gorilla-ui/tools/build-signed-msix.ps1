param(
    [string]$RepoRoot = ".",
    [string]$OutputDir = "",
    [string]$CertSubject = "CN=GorillaUI",
    [string]$CertPassword = "gorilla-devtest",
    [string]$Platform = "x64"
)

$ErrorActionPreference = "Stop"

function Write-Section {
    param([string]$Message)
    Write-Host ""
    Write-Host "==> $Message"
}

$repo = Resolve-Path $RepoRoot
$uiProject = Join-Path $repo "gorilla-ui/src/Gorilla.UI.App/Gorilla.UI.App.csproj"
$appPackagesDir = Join-Path $repo "gorilla-ui/src/Gorilla.UI.App/AppPackages"

if ([string]::IsNullOrWhiteSpace($OutputDir)) {
    $OutputDir = Join-Path $repo "build"
}

if (-not (Test-Path $uiProject)) {
    throw "UI project not found at $uiProject"
}

New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null

$logPath = Join-Path $OutputDir "win-build.log"
$certPath = Join-Path $OutputDir "Gorilla.UI.App.cer"
$pfxPath = Join-Path $OutputDir "Gorilla.UI.App.pfx"
$signedMsixPath = Join-Path $OutputDir "Gorilla.UI.App.signed.msix"

if (Test-Path $logPath) {
    Remove-Item $logPath -Force
}

Write-Section "Git pull"
git -C $repo pull *>&1 | Tee-Object -FilePath $logPath

Write-Section "Create and export signing certificate"
$pwd = ConvertTo-SecureString $CertPassword -AsPlainText -Force
$cert = New-SelfSignedCertificate `
    -Type Custom `
    -Subject $CertSubject `
    -KeyUsage DigitalSignature `
    -FriendlyName "Gorilla UI Dev Test" `
    -CertStoreLocation "Cert:\CurrentUser\My" `
    -TextExtension @("2.5.29.37={text}1.3.6.1.5.5.7.3.3")

Export-Certificate -Cert $cert -FilePath $certPath -Force *>&1 | Tee-Object -FilePath $logPath -Append
Export-PfxCertificate -Cert $cert -FilePath $pfxPath -Password $pwd -Force *>&1 | Tee-Object -FilePath $logPath -Append

Write-Section "Build signed MSIX"
dotnet build $uiProject `
    -p:Platform=$Platform `
    -p:GenerateAppxPackageOnBuild=true `
    -p:PackageCertificateKeyFile="$pfxPath" `
    -p:PackageCertificatePassword=$CertPassword *>&1 | Tee-Object -FilePath $logPath -Append

Write-Section "Copy latest MSIX"
$msix = Get-ChildItem $appPackagesDir -Recurse -Filter *.msix | Sort-Object LastWriteTime -Descending | Select-Object -First 1
if ($null -eq $msix) {
    throw "No MSIX file found under $appPackagesDir"
}

Copy-Item $msix.FullName $signedMsixPath -Force

Write-Section "Verify signature"
$signature = Get-AuthenticodeSignature $signedMsixPath
$signature | Format-List Status,StatusMessage,SignerCertificate *>&1 | Tee-Object -FilePath $logPath -Append

if ($null -eq $signature.SignerCertificate) {
    throw "MSIX is not signed. See $logPath for details."
}

Write-Host ""
Write-Host "Done."
Write-Host "Signed MSIX: $signedMsixPath"
Write-Host "Certificate: $certPath"
Write-Host "Build log: $logPath"
