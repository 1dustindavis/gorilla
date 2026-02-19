using System.Diagnostics;

namespace Gorilla.UI.Client;

internal static class ClientDiagnostics
{
    private static readonly object Gate = new();
    private const long DefaultMaxBytes = 10 * 1024 * 1024;

    public static void Log(string message)
    {
        if (!ResolveEnabled())
        {
            return;
        }

        var line = $"{DateTimeOffset.UtcNow:O} {message}";
        try
        {
            var logPath = BuildLogPath();
            if (string.IsNullOrWhiteSpace(logPath))
            {
                Trace.WriteLine(line);
                return;
            }

            lock (Gate)
            {
                EnsureDirectoryForPath(logPath);
                RotateIfNeeded(logPath);
                File.AppendAllText(logPath, line + Environment.NewLine);
            }
        }
        catch
        {
            // Diagnostics must never impact app behavior.
        }
    }

    internal static bool ResolveEnabled()
    {
        static bool ParseTrue(string? value) =>
            string.Equals(value, "1", StringComparison.OrdinalIgnoreCase) ||
            string.Equals(value, "true", StringComparison.OrdinalIgnoreCase) ||
            string.Equals(value, "yes", StringComparison.OrdinalIgnoreCase) ||
            string.Equals(value, "on", StringComparison.OrdinalIgnoreCase);

        return ParseTrue(Environment.GetEnvironmentVariable("GORILLA_UI_DEBUG"))
            || ParseTrue(Environment.GetEnvironmentVariable("GORILLA_DEBUG"));
    }

    internal static string? BuildLogPath()
    {
        try
        {
            var overridePath = Environment.GetEnvironmentVariable("GORILLA_UI_LOG_PATH");
            if (!string.IsNullOrWhiteSpace(overridePath))
            {
                return overridePath;
            }

            var directory = Path.Combine(
                Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
                "gorilla"
            );
            Directory.CreateDirectory(directory);
            return Path.Combine(directory, "ui-client.log");
        }
        catch
        {
            return null;
        }
    }

    private static long ResolveMaxBytes()
    {
        var raw = Environment.GetEnvironmentVariable("GORILLA_UI_LOG_MAX_BYTES");
        if (!string.IsNullOrWhiteSpace(raw) && long.TryParse(raw, out var parsed) && parsed > 0)
        {
            return parsed;
        }

        return DefaultMaxBytes;
    }

    private static void EnsureDirectoryForPath(string path)
    {
        var directory = Path.GetDirectoryName(path);
        if (!string.IsNullOrWhiteSpace(directory))
        {
            Directory.CreateDirectory(directory);
        }
    }

    private static void RotateIfNeeded(string path)
    {
        var maxBytes = ResolveMaxBytes();
        var info = new FileInfo(path);
        if (!info.Exists || info.Length < maxBytes)
        {
            return;
        }

        var backup = path + ".1";
        if (File.Exists(backup))
        {
            File.Delete(backup);
        }

        File.Move(path, backup);
    }
}
