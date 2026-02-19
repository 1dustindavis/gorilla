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

            var mainWindow = WaitFor(() => app.GetMainWindow(automation), TimeSpan.FromSeconds(30));
            Assert.NotNull(mainWindow);
            Assert.Equal("Gorilla.UI.App", mainWindow!.Title);

            var heading = WaitFor(
                () => mainWindow!.FindFirstDescendant(cf => cf.ByText("Available Software")),
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

    private static void WriteFailureDetails(
        string artifactsDir,
        Exception ex,
        Application? app,
        UIA3Automation? automation
    )
    {
        var now = DateTime.UtcNow.ToString("yyyyMMdd-HHmmss");
        var errorFile = Path.Combine(artifactsDir, $"failure-{now}.txt");
        File.WriteAllText(errorFile, ex.ToString());

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
