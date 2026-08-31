using Xunit;

namespace Gorilla.UI.App.WindowsUiTests;

public sealed class CriticalPathTests
{
    private const string FixtureItemName = "Ps1V1";

    [Fact]
    [Trait("E2EPhase", "Healthy")]
    public void HealthyServiceInstallsAndRemovesFixtureThroughUi()
    {
        RunWithDiagnostics(nameof(HealthyServiceInstallsAndRemovesFixtureThroughUi), session =>
        {
            var markerPath = RequiredPath("GORILLA_UI_E2E_MARKER_PATH");
            var cachePath = RequiredPath("GORILLA_UI_E2E_CACHE_PATH");
            var home = new HomePageDriver(session);

            Assert.Equal("Available Software", home.Heading.Name);
            _ = home.WaitForItem(FixtureItemName);
            Assert.True(File.Exists(cachePath), $"Expected startup cache at {cachePath}.");
            Assert.False(File.Exists(markerPath), $"Fixture marker should be absent before install: {markerPath}");

            var startupCacheWrite = File.GetLastWriteTimeUtc(cachePath);
            home.InstallButton(FixtureItemName).Invoke();

            session.WaitUntil(() => File.Exists(markerPath), TimeSpan.FromSeconds(60));
            session.WaitUntil(() => File.GetLastWriteTimeUtc(cachePath) > startupCacheWrite, TimeSpan.FromSeconds(30));
            Assert.False(home.HasOperationFailureText());
            Assert.DoesNotContain("failed", home.ServiceWarning.Name, StringComparison.OrdinalIgnoreCase);

            var installRefreshWrite = File.GetLastWriteTimeUtc(cachePath);
            home.RemoveButton(FixtureItemName).Invoke();

            session.WaitUntil(() => !File.Exists(markerPath), TimeSpan.FromSeconds(60));
            session.WaitUntil(() => File.GetLastWriteTimeUtc(cachePath) > installRefreshWrite, TimeSpan.FromSeconds(30));
            Assert.False(home.HasOperationFailureText());
            Assert.DoesNotContain("failed", home.ServiceWarning.Name, StringComparison.OrdinalIgnoreCase);
        });
    }

    [Fact]
    [Trait("E2EPhase", "ServiceUnavailable")]
    public void ServiceUnavailableStartupKeepsCachedItemsVisible()
    {
        RunWithDiagnostics(nameof(ServiceUnavailableStartupKeepsCachedItemsVisible), session =>
        {
            var home = new HomePageDriver(session);
            Assert.Equal("Available Software", home.Heading.Name);
            _ = home.WaitForItem(FixtureItemName);
            home.WaitForWarningContaining("Showing cached data. Refresh failed", TimeSpan.FromSeconds(15));
        });
    }

    private static string RequiredPath(string variableName)
    {
        var value = Environment.GetEnvironmentVariable(variableName);
        if (string.IsNullOrWhiteSpace(value))
        {
            throw new InvalidOperationException($"{variableName} must be set by the E2E harness.");
        }
        return value;
    }

    private static void RunWithDiagnostics(string testName, Action<GorillaAppSession> test)
    {
        using var session = GorillaAppSession.Launch();
        try
        {
            test(session);
        }
        catch (Exception ex)
        {
            session.CaptureFailure(ex, testName);
            throw;
        }
    }
}
