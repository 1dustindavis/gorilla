using System.ComponentModel;
using System.Diagnostics;
using System.Runtime.InteropServices;
using System.Text;
using FlaUI.Core;
using FlaUI.Core.AutomationElements;
using FlaUI.UIA3;
using Xunit;

namespace Gorilla.UI.App.WindowsUiTests;

public sealed class InstalledProductBoundaryTests
{
    private const uint TokenQuery = 0x0008;
    private const int TokenElevation = 20;
    private const int ErrorInsufficientBuffer = 122;
    private const string AppProcessName = "Gorilla.UI.App";
    private const string NormalUserTrustLevel = "0x20000";

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
        var (installedExePath, expectedPackageFullName) = ResolveInstalledPackageProcess(appUserModelId);
        var requestCountBeforeLaunch = CountServiceRequests(serviceLogPath, "ListOptionalInstalls");
        var existingProcessIds = GetAppProcessIds();

        Application? application = null;
        Process? process = null;
        UIA3Automation? automation = null;
        try
        {
            LaunchAsNormalUser(installedExePath);
            process = WaitForNewAppProcess(existingProcessIds, TimeSpan.FromSeconds(30));

            var actualPackageFullName = GetProcessPackageFullName(process);
            Assert.Equal(expectedPackageFullName, actualPackageFullName);

            var isElevated = IsProcessElevated(process);
            Assert.False(
                isElevated,
                $"Packaged App Catalog process {process.Id} is elevated; expected SAFER normal-user execution."
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
                "Expected the non-elevated packaged UI to communicate with the installed service."
            );
            Assert.Null(window.FindFirstDescendant(cf => cf.ByAutomationId("CatalogLoadFailed")));
            Assert.Null(window.FindFirstDescendant(cf => cf.ByAutomationId("CatalogNoCachedData")));
            Assert.Null(window.FindFirstDescendant(cf => cf.ByAutomationId("CatalogDegradedWarningText")));
            Assert.Null(window.FindFirstDescendant(cf => cf.ByAutomationId("InfrastructureWarningText")));

            WriteBoundaryEvidence(
                appUserModelId,
                expectedPackageFullName,
                actualPackageFullName,
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

    private static (string ExePath, string PackageFullName) ResolveInstalledPackageProcess(string appUserModelId)
    {
        var application = Application.LaunchStoreApp(appUserModelId);
        using var process = Process.GetProcessById(application.ProcessId);
        try
        {
            var stopwatch = Stopwatch.StartNew();
            string? executablePath = null;
            while (stopwatch.Elapsed < TimeSpan.FromSeconds(30))
            {
                process.Refresh();
                if (process.HasExited)
                {
                    throw new InvalidOperationException("Packaged App Catalog exited while resolving its installed executable path.");
                }

                try
                {
                    executablePath = process.MainModule?.FileName;
                }
                catch (Win32Exception)
                {
                    // Process startup can briefly race module enumeration.
                }

                if (!string.IsNullOrWhiteSpace(executablePath))
                {
                    return (executablePath, GetProcessPackageFullName(process));
                }
                Thread.Sleep(100);
            }
            throw new TimeoutException("Timed out resolving the installed App Catalog executable path.");
        }
        finally
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
                // Fall through to hard process cleanup.
            }
            try
            {
                process.Refresh();
                if (!process.HasExited)
                {
                    process.Kill();
                    process.WaitForExit(5000);
                }
            }
            catch
            {
                // Best-effort discovery-process cleanup.
            }
        }
    }

    private static void LaunchAsNormalUser(string installedExePath)
    {
        var runAsPath = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.System), "runas.exe");
        if (!File.Exists(runAsPath))
        {
            throw new FileNotFoundException("Windows runas.exe was not found.", runAsPath);
        }

        var startInfo = new ProcessStartInfo
        {
            FileName = runAsPath,
            UseShellExecute = false,
            CreateNoWindow = true,
            RedirectStandardOutput = true,
            RedirectStandardError = true
        };
        startInfo.ArgumentList.Add($"/trustlevel:{NormalUserTrustLevel}");
        startInfo.ArgumentList.Add($"\"{installedExePath}\"");

        using var launcher = Process.Start(startInfo)
            ?? throw new InvalidOperationException("Unable to start the Windows SAFER normal-user launcher.");
        if (!launcher.WaitForExit(10000))
        {
            launcher.Kill();
            throw new TimeoutException("Windows SAFER normal-user launcher did not exit within 10 seconds.");
        }
        if (launcher.ExitCode != 0)
        {
            var stdout = launcher.StandardOutput.ReadToEnd();
            var stderr = launcher.StandardError.ReadToEnd();
            throw new InvalidOperationException(
                $"Windows SAFER normal-user launcher failed with exit code {launcher.ExitCode}. stdout='{stdout}' stderr='{stderr}'"
            );
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
        throw new TimeoutException($"Timed out after {timeout.TotalSeconds:n0}s waiting for normal-user {AppProcessName}.");
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
            if (!File.Exists(logPath))
            {
                return 0;
            }

            using var stream = new FileStream(
                logPath,
                FileMode.Open,
                FileAccess.Read,
                FileShare.ReadWrite | FileShare.Delete
            );
            using var reader = new StreamReader(stream, Encoding.UTF8, detectEncodingFromByteOrderMarks: true);
            var count = 0;
            while (reader.ReadLine() is { } line)
            {
                if (line.Contains($"named pipe request: {operation} ", StringComparison.Ordinal))
                {
                    count++;
                }
            }
            return count;
        }
        catch (IOException)
        {
            return 0;
        }
    }

    private static bool IsProcessElevated(Process process)
    {
        using var token = OpenProcessTokenForQuery(process);
        var elevation = 0;
        var elevationSize = Marshal.SizeOf<int>();
        if (!GetTokenInformation(token.DangerousGetHandle(), TokenElevation, ref elevation, elevationSize, out _))
        {
            throw new Win32Exception(Marshal.GetLastWin32Error(), "GetTokenInformation(TokenElevation) failed for the App Catalog process.");
        }
        return elevation != 0;
    }

    private static Microsoft.Win32.SafeHandles.SafeFileHandle OpenProcessTokenForQuery(Process process)
    {
        if (!OpenProcessToken(process.Handle, TokenQuery, out var tokenHandle))
        {
            throw new Win32Exception(Marshal.GetLastWin32Error(), "OpenProcessToken failed for the App Catalog process.");
        }
        return new Microsoft.Win32.SafeHandles.SafeFileHandle(tokenHandle, ownsHandle: true);
    }

    private static string GetProcessPackageFullName(Process process)
    {
        uint length = 0;
        var result = GetPackageFullName(process.Handle, ref length, null);
        if (result != ErrorInsufficientBuffer)
        {
            throw new Win32Exception(result, $"GetPackageFullName size query failed for App Catalog process {process.Id}.");
        }

        var buffer = new char[checked((int)length)];
        result = GetPackageFullName(process.Handle, ref length, buffer);
        if (result != 0)
        {
            throw new Win32Exception(result, $"GetPackageFullName failed for App Catalog process {process.Id}.");
        }
        return new string(buffer).TrimEnd('\0');
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
        string expectedPackageFullName,
        string actualPackageFullName,
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
                $"ExpectedPackageFullName: {expectedPackageFullName}",
                $"ProcessPackageFullName: {actualPackageFullName}",
                $"ProcessId: {processId}",
                $"TokenElevation: {isElevated}",
                $"LaunchPath: Windows SAFER normal-user level ({NormalUserTrustLevel}) against installed package executable",
                $"ListOptionalInstallsRequestsBefore: {serviceRequestsBefore}",
                $"ListOptionalInstallsRequestsAfter: {serviceRequestsAfter}",
                "ServiceCommunication: non-elevated packaged UI rendered catalog data and completed an explicit service-backed refresh"
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

    [DllImport("kernel32.dll", CharSet = CharSet.Unicode)]
    private static extern int GetPackageFullName(nint processHandle, ref uint packageFullNameLength, [Out] char[]? packageFullName);
}
