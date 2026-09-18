param(
    [Parameter(Mandatory)]
    [ValidateSet("Light", "Dark", "HighContrast")]
    [string]$Mode
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

Add-Type @'
using System;
using System.ComponentModel;
using System.Runtime.InteropServices;

public static class GorillaPresentationEnvironment
{
    private const uint SPI_GETHIGHCONTRAST = 0x0042;
    private const uint SPI_SETHIGHCONTRAST = 0x0043;
    private const uint HCF_HIGHCONTRASTON = 0x00000001;
    private const uint SPIF_UPDATEINIFILE = 0x0001;
    private const uint SPIF_SENDCHANGE = 0x0002;
    private const uint WM_SETTINGCHANGE = 0x001A;
    private const uint SMTO_ABORTIFHUNG = 0x0002;
    private static readonly IntPtr HWND_BROADCAST = new IntPtr(0xffff);

    [StructLayout(LayoutKind.Sequential, CharSet = CharSet.Unicode)]
    private struct HIGHCONTRAST_GET
    {
        public uint cbSize;
        public uint dwFlags;
        public IntPtr lpszDefaultScheme;
    }

    [StructLayout(LayoutKind.Sequential, CharSet = CharSet.Unicode)]
    private struct HIGHCONTRAST_SET
    {
        public uint cbSize;
        public uint dwFlags;
        [MarshalAs(UnmanagedType.LPWStr)]
        public string lpszDefaultScheme;
    }

    [DllImport("user32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
    private static extern bool SystemParametersInfo(
        uint uiAction, uint uiParam, ref HIGHCONTRAST_GET pvParam, uint fWinIni);

    [DllImport("user32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
    private static extern bool SystemParametersInfo(
        uint uiAction, uint uiParam, ref HIGHCONTRAST_SET pvParam, uint fWinIni);

    [DllImport("user32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
    private static extern IntPtr SendMessageTimeout(
        IntPtr hWnd, uint Msg, UIntPtr wParam, string lParam,
        uint fuFlags, uint uTimeout, out UIntPtr lpdwResult);

    public static bool IsHighContrastEnabled()
    {
        var hc = new HIGHCONTRAST_GET();
        hc.cbSize = (uint)Marshal.SizeOf<HIGHCONTRAST_GET>();
        if (!SystemParametersInfo(SPI_GETHIGHCONTRAST, hc.cbSize, ref hc, 0))
            throw new Win32Exception(Marshal.GetLastWin32Error());
        return (hc.dwFlags & HCF_HIGHCONTRASTON) != 0;
    }

    public static void SetHighContrast(bool enabled)
    {
        var hc = new HIGHCONTRAST_SET {
            cbSize = (uint)Marshal.SizeOf<HIGHCONTRAST_SET>(),
            dwFlags = enabled ? HCF_HIGHCONTRASTON : 0,
            lpszDefaultScheme = enabled ? "High Contrast Black" : null
        };
        if (!SystemParametersInfo(
            SPI_SETHIGHCONTRAST,
            hc.cbSize,
            ref hc,
            SPIF_UPDATEINIFILE | SPIF_SENDCHANGE))
        {
            throw new Win32Exception(Marshal.GetLastWin32Error());
        }
    }

    public static void BroadcastAppThemeChange()
    {
        UIntPtr result;
        SendMessageTimeout(
            HWND_BROADCAST,
            WM_SETTINGCHANGE,
            UIntPtr.Zero,
            "ImmersiveColorSet",
            SMTO_ABORTIFHUNG,
            2000,
            out result);
    }
}
'@

$themePath = "HKCU:\Software\Microsoft\Windows\CurrentVersion\Themes\Personalize"
New-Item -Path $themePath -Force | Out-Null

switch ($Mode) {
    "Light" {
        if ([GorillaPresentationEnvironment]::IsHighContrastEnabled()) {
            [GorillaPresentationEnvironment]::SetHighContrast($false)
        }
        Set-ItemProperty -Path $themePath -Name AppsUseLightTheme -Type DWord -Value 1
        [GorillaPresentationEnvironment]::BroadcastAppThemeChange()
    }
    "Dark" {
        if ([GorillaPresentationEnvironment]::IsHighContrastEnabled()) {
            throw "Cannot establish dark app mode while Windows high contrast is enabled."
        }
        Set-ItemProperty -Path $themePath -Name AppsUseLightTheme -Type DWord -Value 0
        [GorillaPresentationEnvironment]::BroadcastAppThemeChange()
    }
    "HighContrast" {
        Set-ItemProperty -Path $themePath -Name AppsUseLightTheme -Type DWord -Value 1
        [GorillaPresentationEnvironment]::BroadcastAppThemeChange()
        [GorillaPresentationEnvironment]::SetHighContrast($true)
    }
}

Start-Sleep -Seconds 2

$appsUseLightTheme = Get-ItemPropertyValue -Path $themePath -Name AppsUseLightTheme
$highContrast = [GorillaPresentationEnvironment]::IsHighContrastEnabled()

switch ($Mode) {
    "Light" {
        if ($appsUseLightTheme -ne 1 -or $highContrast) {
            throw "Failed to establish light presentation environment. AppsUseLightTheme=$appsUseLightTheme HighContrast=$highContrast"
        }
    }
    "Dark" {
        if ($appsUseLightTheme -ne 0 -or $highContrast) {
            throw "Failed to establish dark presentation environment. AppsUseLightTheme=$appsUseLightTheme HighContrast=$highContrast"
        }
    }
    "HighContrast" {
        if (-not $highContrast) {
            throw "Failed to establish Windows high-contrast presentation environment. AppsUseLightTheme=$appsUseLightTheme HighContrast=$highContrast"
        }
    }
}

Write-Host "Presentation environment: mode=$Mode AppsUseLightTheme=$appsUseLightTheme HighContrast=$highContrast"
