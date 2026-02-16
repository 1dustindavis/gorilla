using System.Diagnostics;

namespace Gorilla.UI.Client;

internal static class ClientDiagnostics
{
    private static readonly object Gate = new();
    private static readonly bool Enabled = ResolveEnabled();
    private static readonly string? LogPath = Enabled ? BuildLogPath() : null;

    public static void Log(string message)
    {
        if (!Enabled)
        {
            return;
        }

        var line = $"{DateTimeOffset.UtcNow:O} {message}";
        try
        {
            if (string.IsNullOrWhiteSpace(LogPath))
            {
                Trace.WriteLine(line);
                return;
            }

            lock (Gate)
            {
                File.AppendAllText(LogPath, line + Environment.NewLine);
            }
        }
        catch
        {
            // Diagnostics must never impact app behavior.
        }
    }

    private static bool ResolveEnabled()
    {
        static bool ParseTrue(string? value) =>
            string.Equals(value, "1", StringComparison.OrdinalIgnoreCase) ||
            string.Equals(value, "true", StringComparison.OrdinalIgnoreCase) ||
            string.Equals(value, "yes", StringComparison.OrdinalIgnoreCase) ||
            string.Equals(value, "on", StringComparison.OrdinalIgnoreCase);

        return ParseTrue(Environment.GetEnvironmentVariable("GORILLA_UI_DEBUG"))
            || ParseTrue(Environment.GetEnvironmentVariable("GORILLA_DEBUG"));
    }

    private static string? BuildLogPath()
    {
        try
        {
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
}
