using System.ComponentModel;
using System.Diagnostics;
using System.Globalization;
using System.Reflection;
using System.Runtime.InteropServices;
using FlaUI.Core;
using FlaUI.Core.AutomationElements;
using FlaUI.UIA3;
using Xunit;

namespace Gorilla.UI.App.WindowsUiTests;

public sealed class InstalledProductBoundaryTests
{
    private const uint TokenQuery = 0x0008;
    private const int TokenElevation = 20;
    private const string AppProcessName = "Gorilla.UI.App";

    [Fact]
    [Trait("E2EPhase", "Healthy")]
    public void PackagedUiRunsNonElevatedAndRefreshesFromInstalledService()
    {
        var appUserModelId = Environment.GetEnvironmentVariable("GORILLA_UI_APP_ID");
        if (string.IsNullOrWhiteSpace(appUserModelId))
        {
            // This assertion is specifically about the produced, installed MSIX.
            return;
        }

        var slowMarkerPath = RequiredPath("GORILLA_UI_E2E_SLOW_MARKER_PATH");
        var serviceLogPath = Path.Combine(Path.GetDirectoryName(slowMarkerPath)!, "gorilla.log");
        var requestCountBeforeLaunch = CountServiceRequests(serviceLogPath, "ListOptionalInstalls");
        var existingProcessIds = GetAppProcessIds();

        Application? application = null;
        Process? process = null;
        UIA3Automation? automation = null;
        try
        {
            LaunchPackagedAppThroughUserShell(appUserModelId);
            process = WaitForNewAppProcess(existingProcessIds, TimeSpan.FromSeconds(30));
            var isElevated = IsProcessElevated(process);
            Assert.False(
                isElevated,
                $"Packaged App Catalog process {process.Id} is elevated; expected a normal shell-launched UI to run non-elevated."
            );

            application = Application.Attach(process.Id);
            automation = new UIA3Automation();
            var window = WaitForMainWindow(application, automation, process, TimeSpan.FromSeconds(30));

            _ = WaitForElement(window, "Ps1V1", TimeSpan.FromSeconds(30));

            var refresh = WaitForElement(window, "CatalogRefreshButton", TimeSpan.FromSeconds(30)).AsButton();
            var requestCountBeforeRefresh = CountServiceRequests(serviceLogPath, "ListOptionalInstalls");
            refresh.Invoke();
            WaitUntil(
                () => CountServiceRequests(serviceLogPath, "ListOptionalInstalls") > requestCountBeforeRefresh,
                process,
                TimeSpan.FromSeconds(30)
            );
            WaitUntil(() => refresh.IsEnabled, process, TimeSpan.FromSeconds(30));

            var requestCountAfter = CountServiceRequests(serviceLogPath, "ListOptionalInstalls");
            Assert.True(
                requestCountAfter > requestCountBeforeLaunch,
                "Expected the shell-launched packaged UI to communicate with the installed service."
            );
            Assert.Null(window.FindFirstDescendant(cf => cf.ByAutomationId("CatalogLoadFailed")));
            Assert.Null(window.FindFirstDescendant(cf => cf.ByAutomationId("CatalogNoCachedData")));
            Assert.Null(window.FindFirstDescendant(cf => cf.ByAutomationId("CatalogDegradedWarningText")));
            Assert.Null(window.FindFirstDescendant(cf => cf.ByAutomationId("InfrastructureWarningText")));

            WriteBoundaryEvidence(
                appUserModelId,
                process.Id,
                isElevated,
                requestCountBeforeLaunch,
                requestCountAfter
            );
        }
        finally
        {
            automation?.Dispose();
            if (application is not null)
            {
                try
                {
                    if (!application.HasExited)
                    {
                        application.Close();
                    }
                }
                catch
                {
                    // Fall through to process cleanup.
                }
            }
            if (process is not null)
            {
                try
                {
                    if (!process.HasExited)
                    {
                        process.Kill();
                    }
                }
                catch
                {
                    // Best-effort test cleanup.
                }
                process.Dispose();
            }
        }
    }

    private static void LaunchPackagedAppThroughUserShell(string appUserModelId)
    {
        var shellType = Type.GetTypeFromProgID("Shell.Application")
            ?? throw new InvalidOperationException("Windows Shell.Application COM activation is unavailable.");
        var shell = Activator.CreateInstance(shellType)
            ?? throw new InvalidOperationException("Unable to create Windows Shell.Application.");
        try
        {
            shellType.InvokeMember(
                "ShellExecute",
                BindingFlags.InvokeMethod,
                binder: null,
                target: shell,
                args: ["explorer.exe", $"shell:AppsFolder\\{appUserModelId}", "", "open", 1],
                culture: CultureInfo.InvariantCulture
            );
        }
        finally
        {
            if (Marshal.IsComObject(shell))
            {
                _ = Marshal.FinalReleaseComObject(shell);
            }
        }
    }

    private static HashSet<int> GetAppProcessIds()
    {
        var ids = new HashSet<int>();
        foreach (var process in Process.GetProcessesByName(AppProcessName))
        {
            using (process)
            {
                ids.Add(process.Id);
            }
        }
        return ids;
    }

    private static Process WaitForNewAppProcess(HashSet<int> existingProcessIds, TimeSpan timeout)
    {
        var stopwatch = Stopwatch.StartNew();
        while (stopwatch.Elapsed < timeout)
        {
            foreach (var candidate in Process.GetProcessesByName(AppProcessName))
            {
                if (!existingProcessIds.Contains(candidate.Id))
                {
                    return candidate;
                }
                candidate.Dispose();
            }
            Thread.Sleep(250);
        }
        throw new TimeoutException($"Timed out after {timeout.TotalSeconds:n0}s waiting for shell-launched {AppProcessName}.");
    }

    private static Window WaitForMainWindow(
        Application application,
        UIA3Automation automation,
        Process process,
        TimeSpan timeout
    )
    {
        var stopwatch = Stopwatch.StartNew();
        while (stopwatch.Elapsed < timeout)
        {
            ThrowIfExited(process);
            var window = application.GetMainWindow(automation, TimeSpan.FromMilliseconds(250));
            if (window is not null)
            {
                return window;
            }
            Thread.Sleep(250);
        }
        throw new TimeoutException($"Timed out after {timeout.TotalSeconds:n0}s waiting for the packaged App Catalog window.");
    }

    private static AutomationElement WaitForElement(Window window, string automationId, TimeSpan timeout)
    {
        var stopwatch = Stopwatch.StartNew();
        while (stopwatch.Elapsed < timeout)
        {
            var element = window.FindFirstDescendant(cf => cf.ByAutomationId(automationId));
            if (element is not null)
            {
                return element;
            }
            Thread.Sleep(250);
        }
        throw new TimeoutException($"Timed out after {timeout.TotalSeconds:n0}s waiting for automation id '{automationId}'.");
    }

    private static void WaitUntil(Func<bool> condition, Process process, TimeSpan timeout)
    {
        var stopwatch = Stopwatch.StartNew();
        while (stopwatch.Elapsed < timeout)
        {
            ThrowIfExited(process);
            if (condition())
            {
                return;
            }
            Thread.Sleep(250);
        }
        throw new TimeoutException($"Timed out after {timeout.TotalSeconds:n0}s waiting for installed-product state.");
    }

    private static void ThrowIfExited(Process process)
    {
        process.Refresh();
        if (process.HasExited)
        {
            throw new InvalidOperationException($"Packaged App Catalog exited unexpectedly with code {process.ExitCode}.");
        }
    }

    private static int CountServiceRequests(string logPath, string operation)
    {
        try
        {
            return File.Exists(logPath)
                ? File.ReadLines(logPath).Count(line =>
                    line.Contains($"named pipe request: {operation} ", StringComparison.Ordinal))
                : 0;
        }
        catch (IOException)
        {
            return 0;
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
            if (!GetTokenInformation(tokenHandle, TokenElevation, ref elevation, elevationSize, out _))
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

    private static string RequiredPath(string variableName)
    {
        var value = Environment.GetEnvironmentVariable(variableName);
        if (string.IsNullOrWhiteSpace(value))
        {
            throw new InvalidOperationException($"{variableName} must be set by the installed-product harness.");
        }
        return value;
    }

    private static void WriteBoundaryEvidence(
        string appUserModelId,
        int processId,
        bool isElevated,
        int serviceRequestsBefore,
        int serviceRequestsAfter
    )
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
                "LaunchPath: Windows interactive shell (Shell.Application -> explorer.exe -> AppsFolder)",
                $"ListOptionalInstallsRequestsBefore: {serviceRequestsBefore}",
                $"ListOptionalInstallsRequestsAfter: {serviceRequestsAfter}",
                "ServiceCommunication: packaged UI rendered catalog data and completed an explicit service-backed refresh"
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
