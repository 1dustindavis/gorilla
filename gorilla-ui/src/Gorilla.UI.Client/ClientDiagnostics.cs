using System.Diagnostics;

namespace Gorilla.UI.Client;

internal static class ClientDiagnostics
{
    private static readonly object Gate = new();
    private static readonly string? LogPath = BuildLogPath();

    public static void Log(string message)
    {
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

    private static string? BuildLogPath()
    {
        try
        {
            var directory = Path.Combine(
                Environment.GetFolderPath(Environment.SpecialFolder.CommonApplicationData),
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
