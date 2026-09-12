param(
    [Parameter(Mandatory)][string]$GorillaExePath,
    [string]$OutputDirectory = "$PSScriptRoot\out",
    [int]$WindowWidth = 1280,
    [int]$WindowHeight = 800
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes
Add-Type -AssemblyName System.Drawing
Add-Type -AssemblyName System.Windows.Forms

Add-Type @'
using System;
using System.Runtime.InteropServices;

public static class ScreenshotNativeMethods
{
    [DllImport("user32.dll")]
    public static extern bool SetWindowPos(IntPtr hWnd, IntPtr hWndInsertAfter, int X, int Y, int cx, int cy, uint uFlags);

    [DllImport("user32.dll")]
    public static extern bool ShowWindow(IntPtr hWnd, int nCmdShow);

    [DllImport("user32.dll")]
    public static extern bool SetForegroundWindow(IntPtr hWnd);

    [DllImport("user32.dll")]
    public static extern bool SetCursorPos(int X, int Y);

    [DllImport("user32.dll")]
    public static extern void mouse_event(uint dwFlags, uint dx, uint dy, uint dwData, UIntPtr dwExtraInfo);

    [DllImport("user32.dll")]
    public static extern bool GetWindowRect(IntPtr hWnd, out RECT lpRect);

    [DllImport("user32.dll")]
    public static extern uint GetDpiForWindow(IntPtr hWnd);

    public const uint SWP_NOZORDER = 0x0004;
    public const uint SWP_NOACTIVATE = 0x0010;
    public const int SW_RESTORE = 9;
    public const uint MOUSEEVENTF_LEFTDOWN = 0x0002;
    public const uint MOUSEEVENTF_LEFTUP = 0x0004;

    [StructLayout(LayoutKind.Sequential)]
    public struct RECT
    {
        public int Left;
        public int Top;
        public int Right;
        public int Bottom;
    }
}
'@

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "../../..")).Path
$GorillaExePath = [System.IO.Path]::GetFullPath($GorillaExePath)
$OutputDirectory = [System.IO.Path]::GetFullPath($OutputDirectory)
$appExe = Join-Path $repoRoot "gorilla-ui\src\Gorilla.UI.App\bin\x64\Release\net8.0-windows10.0.19041.0\win-x64\Gorilla.UI.App.exe"
$fixtureSource = Join-Path $PSScriptRoot "fixture"
$workRoot = Join-Path $env:RUNNER_TEMP "gorilla-app-catalog-screenshots"
$fixtureRoot = Join-Path $workRoot "fixture"
$toolsRoot = Join-Path $workRoot "tools"
$serverSource = Join-Path $toolsRoot "fixture_server.go"
$serverExe = Join-Path $toolsRoot "fixture-server.exe"
$configPath = Join-Path $workRoot "config.yaml"
$cachePath = Join-Path $workRoot "ui-state\optional-installs-cache.json"
$appDataPath = "C:\ProgramData\gorilla-app-catalog-screenshots"
$serviceLogPath = Join-Path $appDataPath "gorilla.log"
$serviceName = "gorilla-app-catalog-screenshots"
$pipeName = "gorilla-app-catalog-screenshots"
$registryRoot = "HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall"
$seededRegistryPaths = @(
    (Join-Path $registryRoot "GorillaScreenshotSevenZip"),
    (Join-Path $registryRoot "GorillaScreenshotAudacity"),
    (Join-Path $registryRoot "GorillaScreenshotGit")
)

function Wait-ServiceState {
    param([Parameter(Mandatory)][string]$Expected, [int]$TimeoutSeconds = 20)

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        $service = Get-Service -Name $serviceName -ErrorAction SilentlyContinue
        if ($service -and $service.Status.ToString() -eq $Expected) { return }
        Start-Sleep -Milliseconds 250
    } while ((Get-Date) -lt $deadline)

    $actual = (Get-Service -Name $serviceName -ErrorAction SilentlyContinue)?.Status
    throw "Timed out waiting for service '$serviceName' state '$Expected'. Actual: $actual"
}

function Remove-ScreenshotService {
    $service = Get-Service -Name $serviceName -ErrorAction SilentlyContinue
    if (-not $service) { return }

    $cim = Get-CimInstance Win32_Service -Filter "Name='$serviceName'" -ErrorAction SilentlyContinue
    if ($cim -and $cim.ProcessId -gt 0) {
        Stop-Process -Id $cim.ProcessId -Force -ErrorAction SilentlyContinue
    }

    & sc.exe delete $serviceName | Out-Host
    $deadline = (Get-Date).AddSeconds(20)
    do {
        if (-not (Get-Service -Name $serviceName -ErrorAction SilentlyContinue)) { return }
        Start-Sleep -Milliseconds 250
    } while ((Get-Date) -lt $deadline)
    throw "Timed out removing screenshot service '$serviceName'"
}

function Set-RegistryFixture {
    param(
        [Parameter(Mandatory)][string]$Path,
        [Parameter(Mandatory)][string]$DisplayName,
        [Parameter(Mandatory)][string]$DisplayVersion
    )

    New-Item -Path $Path -Force | Out-Null
    Set-ItemProperty -Path $Path -Name DisplayName -Value $DisplayName
    Set-ItemProperty -Path $Path -Name DisplayVersion -Value $DisplayVersion
}

function Wait-FixtureServer {
    param(
        [Parameter(Mandatory)][System.Diagnostics.Process]$Process,
        [Parameter(Mandatory)][string]$ManifestUrl,
        [int]$TimeoutSeconds = 15
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        if ($Process.HasExited) {
            throw "Fixture HTTP server exited before becoming ready. Exit code: $($Process.ExitCode)"
        }
        try {
            $response = Invoke-WebRequest -Uri $ManifestUrl -UseBasicParsing -TimeoutSec 2
            if ($response.StatusCode -eq 200 -and $response.Content -match 'name:\s*app-catalog-screenshots') {
                Write-Host "Fixture HTTP server ready: $ManifestUrl"
                return
            }
        } catch {
            # Retry until the server is listening and the expected manifest is available.
        }
        Start-Sleep -Milliseconds 250
    } while ((Get-Date) -lt $deadline)

    throw "Timed out waiting for fixture HTTP server at $ManifestUrl"
}

function Wait-ForMainWindow {
    param([Parameter(Mandatory)][System.Diagnostics.Process]$Process, [int]$TimeoutSeconds = 30)

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        $Process.Refresh()
        if ($Process.HasExited) {
            throw "Gorilla UI exited before a main window was available. Exit code: $($Process.ExitCode)"
        }
        if ($Process.MainWindowHandle -ne [IntPtr]::Zero) {
            return $Process.MainWindowHandle
        }
        Start-Sleep -Milliseconds 250
    } while ((Get-Date) -lt $deadline)

    throw "Timed out waiting for Gorilla UI main window"
}

function Get-WindowSize {
    param([Parameter(Mandatory)][IntPtr]$Handle)

    [ScreenshotNativeMethods+RECT]$rect = New-Object ScreenshotNativeMethods+RECT
    if (-not [ScreenshotNativeMethods]::GetWindowRect($Handle, [ref]$rect)) {
        throw "Unable to read Gorilla UI window bounds"
    }
    return [pscustomobject]@{
        Width = $rect.Right - $rect.Left
        Height = $rect.Bottom - $rect.Top
    }
}

function Set-CanonicalWindow {
    param([Parameter(Mandatory)][IntPtr]$Handle)

    [ScreenshotNativeMethods]::ShowWindow($Handle, [ScreenshotNativeMethods]::SW_RESTORE) | Out-Null
    $workingArea = [System.Windows.Forms.Screen]::PrimaryScreen.WorkingArea
    if ($WindowWidth -gt $workingArea.Width -or $WindowHeight -gt $workingArea.Height) {
        throw "Requested $WindowWidth x $WindowHeight window does not fit in runner working area $($workingArea.Width) x $($workingArea.Height)"
    }

    $x = $workingArea.Left + [Math]::Floor(($workingArea.Width - $WindowWidth) / 2)
    $y = $workingArea.Top + [Math]::Floor(($workingArea.Height - $WindowHeight) / 2)
    $flags = [ScreenshotNativeMethods]::SWP_NOZORDER -bor [ScreenshotNativeMethods]::SWP_NOACTIVATE
    if (-not [ScreenshotNativeMethods]::SetWindowPos($Handle, [IntPtr]::Zero, $x, $y, $WindowWidth, $WindowHeight, $flags)) {
        throw "Unable to size Gorilla UI window"
    }
    [ScreenshotNativeMethods]::SetForegroundWindow($Handle) | Out-Null
    Start-Sleep -Milliseconds 500

    $actual = Get-WindowSize -Handle $Handle
    if ($actual.Width -ne $WindowWidth -or $actual.Height -ne $WindowHeight) {
        throw "Canonical Gorilla UI window size mismatch. Requested $WindowWidth x $WindowHeight; actual $($actual.Width) x $($actual.Height)."
    }
}

function Get-ElementById {
    param(
        [Parameter(Mandatory)][System.Windows.Automation.AutomationElement]$Root,
        [Parameter(Mandatory)][string]$AutomationId
    )

    $condition = New-Object System.Windows.Automation.PropertyCondition(
        [System.Windows.Automation.AutomationElement]::AutomationIdProperty,
        $AutomationId
    )
    return $Root.FindFirst([System.Windows.Automation.TreeScope]::Descendants, $condition)
}

function Wait-ForElementById {
    param(
        [Parameter(Mandatory)][System.Windows.Automation.AutomationElement]$Root,
        [Parameter(Mandatory)][string]$AutomationId,
        [int]$TimeoutSeconds = 20
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        $element = Get-ElementById -Root $Root -AutomationId $AutomationId
        if ($null -ne $element) { return $element }
        Start-Sleep -Milliseconds 200
    } while ((Get-Date) -lt $deadline)
    throw "Timed out waiting for UI element '$AutomationId'"
}

function Set-SearchText {
    param(
        [Parameter(Mandatory)][System.Windows.Automation.AutomationElement]$Root,
        [Parameter(Mandatory)][AllowEmptyString()][string]$Text
    )

    $search = Wait-ForElementById -Root $Root -AutomationId "CatalogSearchBox"
    $pattern = $search.GetCurrentPattern([System.Windows.Automation.ValuePattern]::Pattern)
    ([System.Windows.Automation.ValuePattern]$pattern).SetValue($Text)
    Start-Sleep -Milliseconds 600
}

function Click-Element {
    param([Parameter(Mandatory)][System.Windows.Automation.AutomationElement]$Element)

    $rect = $Element.Current.BoundingRectangle
    if ($rect.IsEmpty) { throw "Cannot click an element with an empty bounding rectangle" }
    $x = [int][Math]::Round($rect.Left + ($rect.Width / 2))
    $y = [int][Math]::Round($rect.Top + ($rect.Height / 2))
    [ScreenshotNativeMethods]::SetCursorPos($x, $y) | Out-Null
    [ScreenshotNativeMethods]::mouse_event([ScreenshotNativeMethods]::MOUSEEVENTF_LEFTDOWN, 0, 0, 0, [UIntPtr]::Zero)
    [ScreenshotNativeMethods]::mouse_event([ScreenshotNativeMethods]::MOUSEEVENTF_LEFTUP, 0, 0, 0, [UIntPtr]::Zero)
}

function Open-DetailsWithRetry {
    param(
        [Parameter(Mandatory)][System.Windows.Automation.AutomationElement]$Root,
        [Parameter(Mandatory)][string]$ItemName,
        [int]$TimeoutSeconds = 30
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        $item = Wait-ForElementById -Root $Root -AutomationId $ItemName -TimeoutSeconds 2
        try {
            $scrollPattern = $item.GetCurrentPattern([System.Windows.Automation.ScrollItemPattern]::Pattern)
            ([System.Windows.Automation.ScrollItemPattern]$scrollPattern).ScrollIntoView()
        } catch {
        }
        $item.SetFocus()
        Start-Sleep -Milliseconds 250

        $title = Wait-ForElementById -Root $item -AutomationId "CatalogDisplayName" -TimeoutSeconds 2
        Click-Element -Element $title

        $attemptDeadline = (Get-Date).AddSeconds(2)
        do {
            $details = Get-ElementById -Root $Root -AutomationId "AppDetailsRoot"
            if ($null -ne $details) { return $details }
            Start-Sleep -Milliseconds 200
        } while ((Get-Date) -lt $attemptDeadline -and (Get-Date) -lt $deadline)
    } while ((Get-Date) -lt $deadline)

    throw "Timed out after $TimeoutSeconds seconds opening details for '$ItemName'."
}

function Save-WindowScreenshot {
    param(
        [Parameter(Mandatory)][IntPtr]$Handle,
        [Parameter(Mandatory)][string]$Name
    )

    [ScreenshotNativeMethods+RECT]$rect = New-Object ScreenshotNativeMethods+RECT
    if (-not [ScreenshotNativeMethods]::GetWindowRect($Handle, [ref]$rect)) {
        throw "Unable to read window bounds for screenshot '$Name'"
    }

    $width = $rect.Right - $rect.Left
    $height = $rect.Bottom - $rect.Top
    if ($width -ne $WindowWidth -or $height -ne $WindowHeight) {
        throw "Refusing screenshot '$Name' because window is $width x $height instead of canonical $WindowWidth x $WindowHeight."
    }

    $bitmap = New-Object System.Drawing.Bitmap($width, $height)
    $graphics = [System.Drawing.Graphics]::FromImage($bitmap)
    try {
        [ScreenshotNativeMethods]::SetForegroundWindow($Handle) | Out-Null
        Start-Sleep -Milliseconds 250
        $graphics.CopyFromScreen($rect.Left, $rect.Top, 0, 0, $bitmap.Size)
        $path = Join-Path $OutputDirectory $Name
        $bitmap.Save($path, [System.Drawing.Imaging.ImageFormat]::Png)
        Write-Host "Captured $path"
    } finally {
        $graphics.Dispose()
        $bitmap.Dispose()
    }
}

function Write-AutomationTree {
    param(
        [Parameter(Mandatory)][System.Windows.Automation.AutomationElement]$Root,
        [Parameter(Mandatory)][string]$Path
    )

    $lines = [System.Collections.Generic.List[string]]::new()
    function Visit-Element {
        param([System.Windows.Automation.AutomationElement]$Element, [int]$Depth)
        if ($Depth -gt 8) { return }
        try {
            $lines.Add(("{0}{1} | id={2} | type={3}" -f ('  ' * $Depth), $Element.Current.Name, $Element.Current.AutomationId, $Element.Current.ControlType.ProgrammaticName))
            $children = $Element.FindAll([System.Windows.Automation.TreeScope]::Children, [System.Windows.Automation.Condition]::TrueCondition)
            foreach ($child in $children) { Visit-Element -Element $child -Depth ($Depth + 1) }
        } catch {
            $lines.Add(("{0}<automation element unavailable>" -f ('  ' * $Depth)))
        }
    }
    Visit-Element -Element $Root -Depth 0
    $lines | Set-Content -LiteralPath $Path
}

New-Item -ItemType Directory -Path $OutputDirectory -Force | Out-Null
Remove-Item -LiteralPath $workRoot -Recurse -Force -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Path $fixtureRoot, $toolsRoot -Force | Out-Null
Copy-Item -Path (Join-Path $fixtureSource "*") -Destination $fixtureRoot -Recurse -Force

if (-not (Test-Path -LiteralPath $GorillaExePath)) { throw "gorilla.exe not found: $GorillaExePath" }
if (-not (Test-Path -LiteralPath $appExe)) { throw "Gorilla UI executable not found: $appExe" }
if (-not (Get-Command go -ErrorAction SilentlyContinue)) { throw "go is required to build the local fixture HTTP server" }

$noopPath = Join-Path $fixtureRoot "packages\scripts\noop.ps1"
$catalogPath = Join-Path $fixtureRoot "catalogs\screenshots.yaml"
$noopHash = (Get-FileHash -LiteralPath $noopPath -Algorithm SHA256).Hash.ToLowerInvariant()
(Get-Content -LiteralPath $catalogPath -Raw).Replace("__NOOP_HASH__", $noopHash) |
    Set-Content -LiteralPath $catalogPath -NoNewline

# Match the localhost fixture-serving pattern used by the Windows release/UI integration harnesses.
@'
package main

import (
    "flag"
    "log"
    "net/http"
)

func main() {
    addr := flag.String("addr", "127.0.0.1:18080", "listen address")
    root := flag.String("root", ".", "directory to serve")
    flag.Parse()

    fs := http.FileServer(http.Dir(*root))
    if err := http.ListenAndServe(*addr, fs); err != nil {
        log.Fatal(err)
    }
}
'@ | Set-Content -LiteralPath $serverSource -NoNewline
& go build -o $serverExe $serverSource
if ($LASTEXITCODE -ne 0 -or -not (Test-Path -LiteralPath $serverExe)) {
    throw "Failed to build screenshot fixture HTTP server"
}

$uiProcess = $null
$serverProcess = $null
try {
    Remove-ScreenshotService
    Remove-Item -LiteralPath $appDataPath -Recurse -Force -ErrorAction SilentlyContinue
    foreach ($path in $seededRegistryPaths) {
        Remove-Item -Path $path -Recurse -Force -ErrorAction SilentlyContinue
    }

    Set-RegistryFixture -Path $seededRegistryPaths[0] -DisplayName "Gorilla Screenshot 7-Zip" -DisplayVersion "23.01"
    Set-RegistryFixture -Path $seededRegistryPaths[1] -DisplayName "Gorilla Screenshot Audacity" -DisplayVersion "3.7.3"
    Set-RegistryFixture -Path $seededRegistryPaths[2] -DisplayName "Gorilla Screenshot Git" -DisplayVersion "2.50.1"

    $serverPort = Get-Random -Minimum 20000 -Maximum 20999
    $fixtureUri = "http://127.0.0.1:$serverPort/"
    $serverProcess = Start-Process -FilePath $serverExe `
        -ArgumentList @("-addr", "127.0.0.1:$serverPort", "-root", $fixtureRoot) `
        -PassThru -WindowStyle Hidden `
        -RedirectStandardOutput (Join-Path $OutputDirectory "fixture-server.stdout.log") `
        -RedirectStandardError (Join-Path $OutputDirectory "fixture-server.stderr.log")
    Wait-FixtureServer -Process $serverProcess -ManifestUrl "${fixtureUri}manifests/screenshots.yaml"
    Write-Host "Serving screenshot fixture repository from $fixtureUri"

    New-Item -ItemType Directory -Path (Split-Path -Parent $configPath), (Split-Path -Parent $cachePath) -Force | Out-Null
    @"
url: $fixtureUri
manifest: screenshots
catalogs:
  - screenshots
app_data_path: C:/ProgramData/gorilla-app-catalog-screenshots
service_interval: 24h
debug: true
"@ | Set-Content -LiteralPath $configPath -NoNewline

    $env:GORILLA_UI_E2E_PIPE_NAME = $pipeName
    $env:GORILLA_UI_CACHE_PATH = $cachePath
    $env:GORILLA_UI_DEBUG = "1"
    $env:GORILLA_UI_LOG_PATH = Join-Path $OutputDirectory "ui-client.log"

    & $GorillaExePath -config $configPath -integration-test-service-identity $serviceName -serviceinstall | Out-Host
    if ($LASTEXITCODE -ne 0) { throw "Failed to install screenshot service" }
    & $GorillaExePath -config $configPath -integration-test-service-identity $serviceName -servicestart | Out-Host
    if ($LASTEXITCODE -ne 0) { throw "Failed to start screenshot service" }
    Wait-ServiceState -Expected "Running"

    $uiProcess = Start-Process -FilePath $appExe -PassThru
    $windowHandle = Wait-ForMainWindow -Process $uiProcess
    Set-CanonicalWindow -Handle $windowHandle

    $root = [System.Windows.Automation.AutomationElement]::FromHandle($windowHandle)
    [void](Wait-ForElementById -Root $root -AutomationId "HomeHeading")
    [void](Wait-ForElementById -Root $root -AutomationId "SevenZip")
    [void](Wait-ForElementById -Root $root -AutomationId "Audacity")

    Set-SearchText -Root $root -Text ""
    Save-WindowScreenshot -Handle $windowHandle -Name "catalog-default.png"

    Set-SearchText -Root $root -Text "media"
    [void](Wait-ForElementById -Root $root -AutomationId "VLC")
    Save-WindowScreenshot -Handle $windowHandle -Name "catalog-search.png"

    Set-SearchText -Root $root -Text "desktop"
    [void](Wait-ForElementById -Root $root -AutomationId "SevenZip")
    [void](Wait-ForElementById -Root $root -AutomationId "Audacity")
    Save-WindowScreenshot -Handle $windowHandle -Name "catalog-mixed-actions.png"

    Set-SearchText -Root $root -Text ""
    [void](Open-DetailsWithRetry -Root $root -ItemName "SevenZip")
    Start-Sleep -Milliseconds 500
    Save-WindowScreenshot -Handle $windowHandle -Name "catalog-detail.png"

    Write-AutomationTree -Root $root -Path (Join-Path $OutputDirectory "automation-tree.txt")

    [ScreenshotNativeMethods+RECT]$actualRect = New-Object ScreenshotNativeMethods+RECT
    if (-not [ScreenshotNativeMethods]::GetWindowRect($windowHandle, [ref]$actualRect)) {
        throw "Unable to read final Gorilla UI window bounds"
    }
    $actualWidth = $actualRect.Right - $actualRect.Left
    $actualHeight = $actualRect.Bottom - $actualRect.Top
    if ($actualWidth -ne $WindowWidth -or $actualHeight -ne $WindowHeight) {
        throw "Final Gorilla UI window size mismatch. Requested $WindowWidth x $WindowHeight; actual $actualWidth x $actualHeight."
    }

    $screen = [System.Windows.Forms.Screen]::PrimaryScreen
    [ordered]@{
        ref = $env:GITHUB_REF
        commit = $env:GITHUB_SHA
        capturedAtUtc = [DateTime]::UtcNow.ToString("o")
        requestedWindow = [ordered]@{ width = $WindowWidth; height = $WindowHeight }
        actualWindow = [ordered]@{ width = $actualWidth; height = $actualHeight }
        primaryScreen = [ordered]@{ width = $screen.Bounds.Width; height = $screen.Bounds.Height }
        workingArea = [ordered]@{ width = $screen.WorkingArea.Width; height = $screen.WorkingArea.Height }
        dpi = [ScreenshotNativeMethods]::GetDpiForWindow($windowHandle)
        theme = "runner-default-light"
        catalog = "realistic-open-source-fixture"
        fixtureTransport = "localhost-http"
    } | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath (Join-Path $OutputDirectory "manifest.json")

    if (Test-Path -LiteralPath $serviceLogPath) {
        Copy-Item -LiteralPath $serviceLogPath -Destination (Join-Path $OutputDirectory "gorilla.log") -Force
    }
} catch {
    $errorText = $_ | Out-String
    $errorText | Set-Content -LiteralPath (Join-Path $OutputDirectory "capture-failure.txt")
    if (Test-Path -LiteralPath $serviceLogPath) {
        Copy-Item -LiteralPath $serviceLogPath -Destination (Join-Path $OutputDirectory "gorilla.log") -Force -ErrorAction SilentlyContinue
    }
    throw
} finally {
    if ($uiProcess -and -not $uiProcess.HasExited) {
        Stop-Process -Id $uiProcess.Id -Force -ErrorAction SilentlyContinue
    }
    Remove-ScreenshotService
    if ($serverProcess -and -not $serverProcess.HasExited) {
        Stop-Process -Id $serverProcess.Id -Force -ErrorAction SilentlyContinue
    }
    foreach ($path in $seededRegistryPaths) {
        Remove-Item -Path $path -Recurse -Force -ErrorAction SilentlyContinue
    }
    Remove-Item Env:GORILLA_UI_E2E_PIPE_NAME -ErrorAction SilentlyContinue
    Remove-Item Env:GORILLA_UI_CACHE_PATH -ErrorAction SilentlyContinue
    Remove-Item Env:GORILLA_UI_DEBUG -ErrorAction SilentlyContinue
    Remove-Item Env:GORILLA_UI_LOG_PATH -ErrorAction SilentlyContinue
}
