using Gorilla.UI.Client;
using Xunit;

namespace Gorilla.UI.Client.Tests;

[Collection("ClientDiagnosticsEnv")]
public sealed class ClientDiagnosticsTests : IDisposable
{
    private readonly string? _originalUiDebug = Environment.GetEnvironmentVariable("GORILLA_UI_DEBUG");
    private readonly string? _originalGlobalDebug = Environment.GetEnvironmentVariable("GORILLA_DEBUG");
    private readonly string? _originalPath = Environment.GetEnvironmentVariable("GORILLA_UI_LOG_PATH");
    private readonly string? _originalMaxBytes = Environment.GetEnvironmentVariable("GORILLA_UI_LOG_MAX_BYTES");

    [Fact]
    public void ResolveEnabled_TrueWhenUiDebugEnabled()
    {
        Environment.SetEnvironmentVariable("GORILLA_UI_DEBUG", "1");
        Environment.SetEnvironmentVariable("GORILLA_DEBUG", null);

        Assert.True(ClientDiagnostics.ResolveEnabled());
    }

    [Fact]
    public void Log_RotatesFile_WhenCapReached()
    {
        var tempDir = Path.Combine(Path.GetTempPath(), Guid.NewGuid().ToString("N"));
        Directory.CreateDirectory(tempDir);

        var logPath = Path.Combine(tempDir, "ui-client.log");
        Environment.SetEnvironmentVariable("GORILLA_UI_DEBUG", "1");
        Environment.SetEnvironmentVariable("GORILLA_UI_LOG_PATH", logPath);
        Environment.SetEnvironmentVariable("GORILLA_UI_LOG_MAX_BYTES", "120");

        ClientDiagnostics.Log(new string('A', 200));
        ClientDiagnostics.Log("second line triggers rotation check");

        Assert.True(File.Exists(logPath + ".1"));
        Assert.True(File.Exists(logPath));
        Directory.Delete(tempDir, recursive: true);
    }

    public void Dispose()
    {
        Environment.SetEnvironmentVariable("GORILLA_UI_DEBUG", _originalUiDebug);
        Environment.SetEnvironmentVariable("GORILLA_DEBUG", _originalGlobalDebug);
        Environment.SetEnvironmentVariable("GORILLA_UI_LOG_PATH", _originalPath);
        Environment.SetEnvironmentVariable("GORILLA_UI_LOG_MAX_BYTES", _originalMaxBytes);
    }
}

[CollectionDefinition("ClientDiagnosticsEnv", DisableParallelization = true)]
public sealed class ClientDiagnosticsEnvGroup;
