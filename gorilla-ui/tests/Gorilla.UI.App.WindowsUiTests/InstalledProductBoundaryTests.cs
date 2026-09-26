using System.ComponentModel;
using System.Diagnostics;
using System.Runtime.InteropServices;
using Xunit;

namespace Gorilla.UI.App.WindowsUiTests;

public sealed class InstalledProductBoundaryTests
{
    private const uint TokenQuery = 0x0008;
    private const int TokenElevation = 20;

    [Fact]
    [Trait("E2EPhase", "Healthy")]
    public void PackagedUiRunsNonElevatedAndRefreshesFromInstalledService()
    {
        var appUserModelId = Environment.GetEnvironmentVariable("GORILLA_UI_APP_ID");
        if (string.IsNullOrWhiteSpace(appUserModelId))
        {
            // This assertion is specifically about the produced, installed MSIX.
            // Source-built E2E runs exercise the same UI behavior through their
            // existing tests but do not establish the installed-product boundary.
            return;
        }

        using var session = GorillaAppSession.Launch();
        try
        {
            var processId = session.ProcessId;
            using var process = Process.GetProcessById(processId);
            var isElevated = IsProcessElevated(process);
            Assert.False(
                isElevated,
                $"Packaged App Catalog process {processId} is elevated; expected the installed UI to run non-elevated."
            );

            var shell = new CatalogShellDriver(session);
            var home = new HomePageDriver(session);

            shell.Refresh();
            shell.WaitForRefreshComplete(TimeSpan.FromSeconds(30));
            _ = home.WaitForItem("Ps1V1");
            Assert.False(shell.HasLoadFailedState());
            Assert.False(shell.HasNoCachedDataState());
            Assert.True(string.IsNullOrWhiteSpace(shell.DegradedWarningText), shell.DegradedWarningText);
            Assert.True(string.IsNullOrWhiteSpace(shell.InfrastructureWarningText), shell.InfrastructureWarningText);

            WriteBoundaryEvidence(appUserModelId, processId, isElevated, shell.FreshnessText);
            session.CaptureCheckpoint("installed-product-non-elevated-service-refresh", includeAutomationTree: true);
        }
        catch (Exception ex)
        {
            session.CaptureFailure(ex, nameof(PackagedUiRunsNonElevatedAndRefreshesFromInstalledService));
            throw;
        }
    }

    private static bool IsProcessElevated(Process process)
    {
        if (!OpenProcessToken(process.Handle, TokenQuery, out var tokenHandle))
        {
            throw new Win32Exception(Marshal.GetLastWin32Error(), "OpenProcessToken failed for the App Catalog process.");
        }

        try
        {
            var elevation = 0;
            var elevationSize = Marshal.SizeOf<int>();
            if (!GetTokenInformation(
                    tokenHandle,
                    TokenElevation,
                    ref elevation,
                    elevationSize,
                    out _
                ))
            {
                throw new Win32Exception(Marshal.GetLastWin32Error(), "GetTokenInformation(TokenElevation) failed for the App Catalog process.");
            }

            return elevation != 0;
        }
        finally
        {
            _ = CloseHandle(tokenHandle);
        }
    }

    private static void WriteBoundaryEvidence(string appUserModelId, int processId, bool isElevated, string freshnessText)
    {
        var artifactsDirectory = Environment.GetEnvironmentVariable("WINDOWS_UI_TEST_ARTIFACTS_DIR");
        if (string.IsNullOrWhiteSpace(artifactsDirectory))
        {
            return;
        }

        Directory.CreateDirectory(artifactsDirectory);
        File.WriteAllLines(
            Path.Combine(artifactsDirectory, "installed-product-boundary.txt"),
            [
                $"AppUserModelId: {appUserModelId}",
                $"ProcessId: {processId}",
                $"TokenElevation: {isElevated}",
                "ServiceCommunication: explicit App Catalog refresh completed without degraded/infrastructure warning",
                $"FreshnessText: {freshnessText}"
            ]
        );
    }

    [DllImport("advapi32.dll", SetLastError = true)]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool OpenProcessToken(nint processHandle, uint desiredAccess, out nint tokenHandle);

    [DllImport("advapi32.dll", SetLastError = true)]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool GetTokenInformation(
        nint tokenHandle,
        int tokenInformationClass,
        ref int tokenInformation,
        int tokenInformationLength,
        out int returnLength
    );

    [DllImport("kernel32.dll", SetLastError = true)]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool CloseHandle(nint handle);
}
