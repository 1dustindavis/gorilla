# Installing Chocolatey with Gorilla

Gorilla uses Chocolatey to install and remove `nupkg` items. Chocolatey itself can be bootstrapped as a PowerShell catalog item before any dependent packages:

```yaml
Chocolatey:
  display_name: Chocolatey
  check:
    file:
      - path: C:\ProgramData\chocolatey\bin\choco.exe
  installer:
    type: ps1
    location: packages/chocolatey/install.ps1
    hash: <sha256>
  version: 1.0
```

Download the current installation script from [Chocolatey's installation documentation](https://chocolatey.org/install), store it in your repository, and replace `<sha256>` with the hash of that exact file. Other `nupkg` items can then declare `Chocolatey` as a dependency.

Chocolatey extensions can be managed the same way as ordinary `nupkg` items. Use a stable file, version, or hash status check and declare `Chocolatey` as a dependency.
