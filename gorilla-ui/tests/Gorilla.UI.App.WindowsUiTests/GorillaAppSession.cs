using System.Diagnostics;
using System.Text;
using FlaUI.Core;
using FlaUI.Core.AutomationElements;
using FlaUI.Core.Capturing;
using FlaUI.UIA3;

namespace Gorilla.UI.App.WindowsUiTests;

internal sealed class GorillaAppSession : IDisposable
{
    private readonly string _artifactsDirectory;
    private readonly Application _application;
    private readonly Process _process;
    private readonly UIA3Automation _automation;

    private GorillaAppSession(Application application, Process process, UIA3Automation automation, string artifactsDirectory)
    {
        _application = application;
        _process = process;
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
        var process = Process.GetProcessById(application.ProcessId);
        var automation = new UIA3Automation();
        var session = new GorillaAppSession(application, process, automation, artifactsDirectory);
        try
        {
            session.MainWindow = session.WaitForMainWindow(TimeSpan.FromSeconds(30));
            return session;
        }
        catch (Exception ex)
        {
            session.CaptureFailure(ex, "app-launch");
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

    public void CaptureCheckpoint(string name, bool includeAutomationTree = false)
    {
        BestEffort(() =>
        {
            var screenshotsDirectory = Path.Combine(_artifactsDirectory, "screenshots");
            Directory.CreateDirectory(screenshotsDirectory);
            using var image = MainWindow.Capture();
            image.Save(Path.Combine(screenshotsDirectory, SafeFileName(name) + ".png"));
        });

        if (includeAutomationTree)
        {
            CaptureAutomationTree("automation-tree.txt");
        }
        CaptureProcessInfo();
    }

    public void CaptureAutomationTree(string fileName = "automation-tree.txt")
    {
        BestEffort(() =>
        {
            var builder = new StringBuilder();
            AppendAutomationElement(builder, MainWindow, 0);
            File.WriteAllText(Path.Combine(_artifactsDirectory, fileName), builder.ToString());
        });
    }

    public void CaptureProcessInfo(string fileName = "process-info.txt")
    {
        BestEffort(() => File.WriteAllText(Path.Combine(_artifactsDirectory, fileName), BuildProcessInfo()));
    }

    public void CaptureFailure(Exception exception, string testName)
    {
        BestEffort(() =>
        {
            File.WriteAllText(
                Path.Combine(_artifactsDirectory, $"failure-{SafeFileName(testName)}.txt"),
                exception + Environment.NewLine + BuildProcessInfo());
        });
        BestEffort(() =>
        {
            var screenshotsDirectory = Path.Combine(_artifactsDirectory, "screenshots");
            Directory.CreateDirectory(screenshotsDirectory);
            using var image = MainWindow.Capture();
            image.Save(Path.Combine(screenshotsDirectory, $"failure-{SafeFileName(testName)}.png"));
        });
        CaptureAutomationTree("automation-tree-failure.txt");
        CaptureProcessInfo();
    }

    private string BuildProcessInfo()
    {
        var lines = new List<string>
        {
            $"ProcessId: {_process.Id}",
            $"ProcessName: {BestEffortValue(() => _process.ProcessName)}",
            $"StartTimeUtc: {BestEffortValue(() => _process.StartTime.ToUniversalTime().ToString("O"))}",
            $"HasExited: {BestEffortValue(() => _process.HasExited.ToString())}"
        };
        if (BestEffortValue(() => _process.HasExited.ToString()) == bool.TrueString)
        {
            lines.Add($"ExitCode: {BestEffortValue(() => _process.ExitCode.ToString())}");
            lines.Add($"ExitTimeUtc: {BestEffortValue(() => _process.ExitTime.ToUniversalTime().ToString("O"))}");
        }
        return string.Join(Environment.NewLine, lines) + Environment.NewLine;
    }

    private static void AppendAutomationElement(StringBuilder builder, AutomationElement element, int depth)
    {
        var indent = new string(' ', depth * 2);
        builder.Append(indent);
        builder.Append("ControlType=").Append(BestEffortValue(() => element.ControlType.ToString()));
        builder.Append(" AutomationId=").Append(Quote(BestEffortValue(() => element.AutomationId)));
        builder.Append(" Name=").Append(Quote(BestEffortValue(() => element.Name)));
        builder.Append(" Enabled=").Append(BestEffortValue(() => element.IsEnabled.ToString()));
        builder.Append(" Offscreen=").Append(BestEffortValue(() => element.IsOffscreen.ToString()));
        builder.AppendLine();

        AutomationElement[] children;
        try
        {
            children = element.FindAllChildren();
        }
        catch
        {
            return;
        }
        foreach (var child in children)
        {
            AppendAutomationElement(builder, child, depth + 1);
        }
    }

    private static string Quote(string value) => $"\"{value.Replace("\"", "\\\"")}\"";

    private static string SafeFileName(string value) =>
        string.Concat(value.Select(c => Path.GetInvalidFileNameChars().Contains(c) ? '_' : c));

    private static void BestEffort(Action action)
    {
        try
        {
            action();
        }
        catch
        {
            // Diagnostics must never mask the original test result.
        }
    }

    private static string BestEffortValue(Func<string> value)
    {
        try
        {
            return value();
        }
        catch (Exception ex)
        {
            return $"<unavailable: {ex.GetType().Name}>";
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
            if (!_process.HasExited)
            {
                return;
            }
            throw new InvalidOperationException($"Gorilla.UI.App exited unexpectedly. ExitCode={_process.ExitCode}.");
        }
        catch (InvalidOperationException)
        {
            throw;
        }
        catch
        {
            throw new InvalidOperationException("Gorilla.UI.App process is unavailable.");
        }
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
        _process.Dispose();
        _automation.Dispose();
    }
}
