param(
    [Parameter(Mandatory = $true)]
    [string]$CertificatePath,
    [switch]$Create
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$resolvedCertificatePath = [System.IO.Path]::GetFullPath($CertificatePath)

if ($Create) {
    $certificateDirectory = Split-Path -Parent $resolvedCertificatePath
    New-Item -Path $certificateDirectory -ItemType Directory -Force | Out-Null

    $certificate = New-SelfSignedCertificate `
        -Subject "CN=GorillaIT" `
        -CertStoreLocation "Cert:\CurrentUser\My" `
        -Type CodeSigningCert `
        -TextExtension @("2.5.29.37={text}1.3.6.1.5.5.7.3.3")

    Export-Certificate -Cert $certificate -FilePath $resolvedCertificatePath -Force | Out-Null
} elseif (-not (Test-Path -LiteralPath $resolvedCertificatePath)) {
    throw "MSIX test certificate not found: $resolvedCertificatePath"
}

$publicCertificate = [System.Security.Cryptography.X509Certificates.X509Certificate2]::new(
    $resolvedCertificatePath
)
if ($publicCertificate.Subject -ne "CN=GorillaIT") {
    throw "Unexpected MSIX test certificate subject: $($publicCertificate.Subject)"
}

Import-Certificate `
    -FilePath $resolvedCertificatePath `
    -CertStoreLocation "Cert:\LocalMachine\TrustedPeople" | Out-Null
Import-Certificate `
    -FilePath $resolvedCertificatePath `
    -CertStoreLocation "Cert:\LocalMachine\Root" | Out-Null

Write-Host "[INFO] Trusted MSIX test certificate: $($publicCertificate.Thumbprint)"
Write-Output $publicCertificate.Thumbprint
