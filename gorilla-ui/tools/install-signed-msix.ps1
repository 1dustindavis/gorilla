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

function Get-MsixPackageIdentityName {
    param([string]$PackagePath)

    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $archive = [System.IO.Compression.ZipFile]::OpenRead($PackagePath)
    try {
        $entry = $archive.GetEntry("AppxManifest.xml")
        if ($null -eq $entry) {
            throw "AppxManifest.xml not found in package: $PackagePath"
        }

        $stream = $entry.Open()
        try {
            $reader = New-Object System.IO.StreamReader($stream)
            try {
                $xml = [xml]$reader.ReadToEnd()
            }
            finally {
                $reader.Dispose()
            }
        }
        finally {
            $stream.Dispose()
        }
    }
    finally {
        $archive.Dispose()
    }

    $identity = $xml.SelectSingleNode("/*[local-name()='Package']/*[local-name()='Identity']")
    if ($null -eq $identity -or $null -eq $identity.Attributes["Name"]) {
        throw "Could not resolve package identity name from AppxManifest.xml"
    }

    return $identity.Attributes["Name"].Value
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
try {
    Add-AppxPackage -Path $msixPath -ForceUpdateFromAnyVersion -ErrorAction Stop *>&1 | Tee-Object -FilePath $logPath -Append
}
catch {
    $_ *>&1 | Tee-Object -FilePath $logPath -Append

    if ($_.Exception.Message -notmatch "0x80073CFB") {
        throw
    }

    $packageName = Get-MsixPackageIdentityName -PackagePath $msixPath

    Write-Section "Existing package detected ($packageName). Removing and retrying"

    $installed = Get-AppxPackage -AllUsers | Where-Object Name -eq $packageName
    if ($null -ne $installed) {
        foreach ($pkg in $installed) {
            "Removing installed package: $($pkg.PackageFullName)" | Tee-Object -FilePath $logPath -Append
            Remove-AppxPackage -Package $pkg.PackageFullName -AllUsers -ErrorAction Continue *>&1 | Tee-Object -FilePath $logPath -Append
        }
    } else {
        "No installed package entries found for identity $packageName." | Tee-Object -FilePath $logPath -Append
    }

    $provisioned = Get-AppxProvisionedPackage -Online | Where-Object DisplayName -eq $packageName
    if ($null -ne $provisioned) {
        foreach ($prov in $provisioned) {
            "Removing provisioned package: $($prov.PackageName)" | Tee-Object -FilePath $logPath -Append
            Remove-AppxProvisionedPackage -Online -PackageName $prov.PackageName -ErrorAction Continue *>&1 | Tee-Object -FilePath $logPath -Append
        }
    } else {
        "No provisioned package entries found for identity $packageName." | Tee-Object -FilePath $logPath -Append
    }

    Write-Section "Retry install"
    Add-AppxPackage -Path $msixPath -ForceUpdateFromAnyVersion -ErrorAction Stop *>&1 | Tee-Object -FilePath $logPath -Append
}

Write-Host ""
Write-Host "Done."
Write-Host "Installed package: $msixPath"
Write-Host "Install log: $logPath"
