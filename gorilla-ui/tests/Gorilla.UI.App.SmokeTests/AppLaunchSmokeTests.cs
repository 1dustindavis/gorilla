using System.Diagnostics;
using FlaUI.Core;
using FlaUI.Core.AutomationElements;
using FlaUI.Core.Capturing;
using FlaUI.UIA3;
using Xunit;

[assembly: CollectionBehavior(DisableTestParallelization = true)]

namespace Gorilla.UI.App.SmokeTests;

public sealed class AppLaunchSmokeTests
{
    [Fact]
    public void AppLaunchesAndShowsHomeHeading()
    {
        var appExePath = ResolveAppExePath();
        var artifactsDir = ResolveArtifactsDirectory();
        Directory.CreateDirectory(artifactsDir);

        Application? app = null;
        UIA3Automation? automation = null;

        try
        {
            app = Application.Launch(appExePath);
            automation = new UIA3Automation();

            var mainWindow = WaitForMainWindow(app, automation, TimeSpan.FromSeconds(30));
            Assert.Equal("Gorilla.UI.App", mainWindow.Title);

            var heading = WaitFor(
                () => mainWindow.FindFirstDescendant(cf => cf.ByText("Available Software")),
                TimeSpan.FromSeconds(30)
            );
            Assert.NotNull(heading);

            var itemsList = WaitFor(
                () => mainWindow.FindFirstDescendant(cf => cf.ByAutomationId("ItemsList")),
                TimeSpan.FromSeconds(30)
            );
            Assert.NotNull(itemsList);
        }
        catch (Exception ex)
        {
            WriteFailureDetails(artifactsDir, ex, app, automation);
            throw;
        }
        finally
        {
            automation?.Dispose();
            CloseOrKill(app);
        }
    }

    [Fact]
    public void AppLaunchesAndRemainsRunningAfterStartup()
    {
        var appExePath = ResolveAppExePath();
        var artifactsDir = ResolveArtifactsDirectory();
        Directory.CreateDirectory(artifactsDir);

        Application? app = null;
        UIA3Automation? automation = null;

        try
        {
            app = Application.Launch(appExePath);
            automation = new UIA3Automation();

            _ = WaitForMainWindow(app, automation, TimeSpan.FromSeconds(30));
            AssertAppStillRunning(app, TimeSpan.FromSeconds(2));
        }
        catch (Exception ex)
        {
            WriteFailureDetails(artifactsDir, ex, app, automation);
            throw;
        }
        finally
        {
            automation?.Dispose();
            CloseOrKill(app);
        }
    }

    private static string ResolveAppExePath()
    {
        var appExePath = Environment.GetEnvironmentVariable("GORILLA_UI_APP_EXE");
        if (string.IsNullOrWhiteSpace(appExePath))
        {
            throw new InvalidOperationException("GORILLA_UI_APP_EXE must be set to the built Gorilla.UI.App.exe path.");
        }

        if (!File.Exists(appExePath))
        {
            throw new FileNotFoundException($"GORILLA_UI_APP_EXE path does not exist: {appExePath}", appExePath);
        }

        return appExePath;
    }

    private static string ResolveArtifactsDirectory()
    {
        var dir = Environment.GetEnvironmentVariable("SMOKE_ARTIFACTS_DIR");
        if (!string.IsNullOrWhiteSpace(dir))
        {
            return dir;
        }

        return Path.Combine(Path.GetTempPath(), "gorilla-ui-smoke-artifacts");
    }

    private static T? WaitFor<T>(Func<T?> probe, TimeSpan timeout)
        where T : class
    {
        var sw = Stopwatch.StartNew();
        while (sw.Elapsed < timeout)
        {
            var value = probe();
            if (value is not null)
            {
                return value;
            }

            Thread.Sleep(250);
        }

        return null;
    }

    private static Window WaitForMainWindow(Application app, UIA3Automation automation, TimeSpan timeout)
    {
        var sw = Stopwatch.StartNew();
        while (sw.Elapsed < timeout)
        {
            if (AppHasExitedOrUnavailable(app))
            {
                throw BuildExitedEarlyException(app);
            }

            try
            {
                var mainWindow = app.GetMainWindow(automation, TimeSpan.FromMilliseconds(250));
                if (mainWindow is not null)
                {
                    return mainWindow;
                }
            }
            catch
            {
                if (AppHasExitedOrUnavailable(app))
                {
                    throw BuildExitedEarlyException(app);
                }
            }

            Thread.Sleep(250);
        }

        throw new TimeoutException($"Timed out waiting {timeout.TotalSeconds:n0}s for Gorilla.UI.App main window.");
    }

    private static bool AppHasExitedOrUnavailable(Application app)
    {
        try
        {
            return app.HasExited;
        }
        catch
        {
            return true;
        }
    }

    private static InvalidOperationException BuildExitedEarlyException(Application app)
    {
        try
        {
            var process = Process.GetProcessById(app.ProcessId);
            return new InvalidOperationException(
                $"Gorilla.UI.App exited before a main window was available. ExitCode={process.ExitCode}."
            );
        }
        catch
        {
            return new InvalidOperationException("Gorilla.UI.App exited before a main window was available.");
        }
    }

    private static void AssertAppStillRunning(Application app, TimeSpan duration)
    {
        var sw = Stopwatch.StartNew();
        while (sw.Elapsed < duration)
        {
            if (AppHasExitedOrUnavailable(app))
            {
                throw BuildExitedEarlyException(app);
            }

            Thread.Sleep(100);
        }
    }

    private static void WriteFailureDetails(
        string artifactsDir,
        Exception ex,
        Application? app,
        UIA3Automation? automation
    )
    {
        var now = DateTime.UtcNow.ToString("yyyyMMdd-HHmmss");
        var errorFile = Path.Combine(artifactsDir, $"failure-{now}.txt");
        var details = ex.ToString();
        if (app is not null)
        {
            try
            {
                details += $"{Environment.NewLine}ProcessId: {app.ProcessId}";
                details += $"{Environment.NewLine}HasExited: {app.HasExited}";
                if (app.HasExited)
                {
                    details += $"{Environment.NewLine}ExitCode: {Process.GetProcessById(app.ProcessId).ExitCode}";
                }
            }
            catch
            {
                // Best-effort diagnostics only.
            }
        }

        File.WriteAllText(errorFile, details);

        if (app is null || automation is null)
        {
            return;
        }

        try
        {
            var window = app.GetMainWindow(automation);
            if (window is null)
            {
                return;
            }

            var screenshotFile = Path.Combine(artifactsDir, $"failure-{now}.png");
            using var image = window.Capture();
            image.Save(screenshotFile);
        }
        catch
        {
            // Failure diagnostics should never mask the root test failure.
        }
    }

    private static void CloseOrKill(Application? app)
    {
        if (app is null)
        {
            return;
        }

        try
        {
            if (!app.HasExited)
            {
                app.Close();
            }
        }
        catch
        {
            // Ignore close failures and try hard kill below.
        }

        try
        {
            if (!app.HasExited)
            {
                app.Kill();
            }
        }
        catch
        {
            // No-op on cleanup failures.
        }
    }
}
