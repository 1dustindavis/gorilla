using System.Diagnostics;
using FlaUI.Core;
using FlaUI.Core.AutomationElements;
using FlaUI.Core.Capturing;
using FlaUI.UIA3;

namespace Gorilla.UI.App.WindowsUiTests;

internal sealed class GorillaAppSession : IDisposable
{
    private readonly string _artifactsDirectory;
    private readonly Application _application;
    private readonly UIA3Automation _automation;

    private GorillaAppSession(Application application, UIA3Automation automation, string artifactsDirectory)
    {
        _application = application;
        _automation = automation;
        _artifactsDirectory = artifactsDirectory;
    }

    public Window MainWindow { get; private set; } = null!;

    public static GorillaAppSession Launch()
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

        var artifactsDirectory = Environment.GetEnvironmentVariable("WINDOWS_UI_TEST_ARTIFACTS_DIR");
        if (string.IsNullOrWhiteSpace(artifactsDirectory))
        {
            artifactsDirectory = Path.Combine(Path.GetTempPath(), "gorilla-ui-windows-ui-test-artifacts");
        }
        Directory.CreateDirectory(artifactsDirectory);

        var application = Application.Launch(appExePath);
        var automation = new UIA3Automation();
        var session = new GorillaAppSession(application, automation, artifactsDirectory);
        try
        {
            session.MainWindow = session.WaitForMainWindow(TimeSpan.FromSeconds(30));
            return session;
        }
        catch
        {
            session.Dispose();
            throw;
        }
    }

    public T WaitFor<T>(Func<T?> probe, TimeSpan? timeout = null) where T : class
    {
        var limit = timeout ?? TimeSpan.FromSeconds(30);
        var sw = Stopwatch.StartNew();
        Exception? lastError = null;
        while (sw.Elapsed < limit)
        {
            ThrowIfExited();
            try
            {
                var value = probe();
                if (value is not null)
                {
                    return value;
                }
            }
            catch (Exception ex)
            {
                lastError = ex;
            }
            Thread.Sleep(250);
        }

        throw new TimeoutException($"Timed out after {limit.TotalSeconds:n0}s waiting for UI state.", lastError);
    }

    public void WaitUntil(Func<bool> condition, TimeSpan? timeout = null)
    {
        _ = WaitFor<object>(() => condition() ? new object() : null, timeout);
    }

    public void AssertStillRunning(TimeSpan duration)
    {
        var sw = Stopwatch.StartNew();
        while (sw.Elapsed < duration)
        {
            ThrowIfExited();
            Thread.Sleep(100);
        }
    }

    public void CaptureFailure(Exception exception, string testName)
    {
        try
        {
            var safeName = string.Concat(testName.Select(c => Path.GetInvalidFileNameChars().Contains(c) ? '_' : c));
            var stamp = DateTime.UtcNow.ToString("yyyyMMdd-HHmmss");
            var prefix = Path.Combine(_artifactsDirectory, $"{safeName}-{stamp}");
            var details = exception.ToString();
            try
            {
                details += $"{Environment.NewLine}ProcessId: {_application.ProcessId}";
                details += $"{Environment.NewLine}HasExited: {_application.HasExited}";
            }
            catch
            {
                // Best-effort diagnostics only.
            }
            File.WriteAllText(prefix + ".txt", details);

            try
            {
                using var image = MainWindow.Capture();
                image.Save(prefix + ".png");
            }
            catch
            {
                // Diagnostics must never mask the original failure.
            }
        }
        catch
        {
            // Diagnostics must never mask the original failure.
        }
    }

    private Window WaitForMainWindow(TimeSpan timeout)
    {
        return WaitFor(() =>
        {
            try
            {
                return _application.GetMainWindow(_automation, TimeSpan.FromMilliseconds(250));
            }
            catch
            {
                ThrowIfExited();
                return null;
            }
        }, timeout);
    }

    private void ThrowIfExited()
    {
        try
        {
            if (!_application.HasExited)
            {
                return;
            }
        }
        catch
        {
            throw new InvalidOperationException("Gorilla.UI.App process is unavailable.");
        }

        throw new InvalidOperationException("Gorilla.UI.App exited unexpectedly.");
    }

    public void Dispose()
    {
        try
        {
            if (!_application.HasExited)
            {
                _application.Close();
            }
        }
        catch
        {
            // Fall through to hard kill.
        }
        try
        {
            if (!_application.HasExited)
            {
                _application.Kill();
            }
        }
        catch
        {
            // Best-effort cleanup.
        }
        _automation.Dispose();
    }
}
