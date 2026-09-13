using Xunit;

namespace Gorilla.UI.App.WindowsUiTests;

public sealed class CriticalPathTests
{
    private const string FixtureItemName = "Ps1V1";
    private const string FailureFixtureItemName = "Ps1Failure";

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
            home.WaitForItemStatus(FixtureItemName, "NotInstalled");
            Assert.True(File.Exists(cachePath), $"Expected startup cache at {cachePath}.");
            Assert.False(File.Exists(markerPath), $"Fixture marker should be absent before install: {markerPath}");
            home.EnsureItemVisible(FixtureItemName);
            session.CaptureCheckpoint("healthy-startup", includeAutomationTree: true);

            var startupCacheWrite = File.GetLastWriteTimeUtc(cachePath);
            home.InstallButton(FixtureItemName).Invoke();

            session.WaitUntil(() => File.Exists(markerPath), TimeSpan.FromSeconds(60));
            session.WaitUntil(() => File.GetLastWriteTimeUtc(cachePath) > startupCacheWrite, TimeSpan.FromSeconds(30));
            home.WaitForItemStatus(FixtureItemName, "Installed", TimeSpan.FromSeconds(30));
            Assert.False(home.HasOperationFailureText());
            Assert.DoesNotContain("failed", home.WarningText, StringComparison.OrdinalIgnoreCase);
            home.EnsureItemVisible(FixtureItemName);
            session.CaptureCheckpoint("after-install");

            var installRefreshWrite = File.GetLastWriteTimeUtc(cachePath);
            home.RemoveButton(FixtureItemName).Invoke();

            session.WaitUntil(() => !File.Exists(markerPath), TimeSpan.FromSeconds(60));
            session.WaitUntil(() => File.GetLastWriteTimeUtc(cachePath) > installRefreshWrite, TimeSpan.FromSeconds(30));
            home.WaitForItemStatus(FixtureItemName, "NotInstalled", TimeSpan.FromSeconds(30));
            Assert.False(home.HasOperationFailureText());
            Assert.DoesNotContain("failed", home.WarningText, StringComparison.OrdinalIgnoreCase);
            home.EnsureItemVisible(FixtureItemName);
            session.CaptureCheckpoint("after-remove");
        });
    }

    [Fact]
    [Trait("E2EPhase", "Healthy")]
    public void ManualRefreshPreservesSearchAndReturnsToFreshState()
    {
        RunWithDiagnostics(nameof(ManualRefreshPreservesSearchAndReturnsToFreshState), session =>
        {
            var home = new HomePageDriver(session);
            var shell = new CatalogShellDriver(session);
            _ = home.WaitForItem(FixtureItemName);
            shell.WaitForFreshnessContaining("Updated", TimeSpan.FromSeconds(30));

            home.Search(FixtureItemName);
            session.WaitUntil(() => home.HasItem(FixtureItemName));
            shell.Refresh();
            Assert.True(home.HasItem(FixtureItemName));
            shell.WaitForRefreshComplete(TimeSpan.FromSeconds(30));

            Assert.Equal(FixtureItemName, home.SearchBox.Text);
            Assert.True(home.HasItem(FixtureItemName));
            Assert.Contains("Updated", shell.FreshnessText, StringComparison.OrdinalIgnoreCase);
            Assert.True(string.IsNullOrWhiteSpace(shell.DegradedWarningText));
            session.CaptureCheckpoint("manual-refresh", includeAutomationTree: true);
        });
    }

    [Fact]
    [Trait("E2EPhase", "Healthy")]
    public void CacheWriteFailureKeepsFreshCatalogVisibleAndActionable()
    {
        var cachePath = RequiredPath("GORILLA_UI_E2E_CACHE_PATH");
        RunWithDiagnostics(nameof(CacheWriteFailureKeepsFreshCatalogVisibleAndActionable), session =>
        {
            var home = new HomePageDriver(session);
            var shell = new CatalogShellDriver(session);
            _ = home.WaitForItem(FixtureItemName);
            shell.WaitForFreshnessContaining("Updated", TimeSpan.FromSeconds(30));
            Assert.True(File.Exists(cachePath), $"Expected startup cache at {cachePath}.");

            var originalAttributes = File.GetAttributes(cachePath);
            try
            {
                File.SetAttributes(cachePath, originalAttributes | FileAttributes.ReadOnly);
                shell.Refresh();
                shell.WaitForRefreshComplete(TimeSpan.FromSeconds(30));
                shell.WaitForDegradedWarningContaining("couldn't save the latest catalog", TimeSpan.FromSeconds(15));

                Assert.True(home.HasItem(FixtureItemName));
                Assert.True(home.PrimaryActionButton(FixtureItemName).IsEnabled);
                Assert.Contains("Updated", shell.FreshnessText, StringComparison.OrdinalIgnoreCase);
                Assert.DoesNotContain("couldn't refresh", shell.DegradedWarningText, StringComparison.OrdinalIgnoreCase);
                session.CaptureCheckpoint("cache-write-degraded", includeAutomationTree: true);
            }
            finally
            {
                if (File.Exists(cachePath))
                {
                    File.SetAttributes(cachePath, originalAttributes);
                }
            }
        });
    }

    [Fact]
    [Trait("E2EPhase", "Healthy")]
    public void InstalledAndRemovedStateSurvivesAppRelaunch()
    {
        var markerPath = RequiredPath("GORILLA_UI_E2E_MARKER_PATH");

        RunWithDiagnostics(nameof(InstalledAndRemovedStateSurvivesAppRelaunch) + "-install", session =>
        {
            var home = new HomePageDriver(session);

            Assert.Equal("Available Software", home.Heading.Name);
            _ = home.WaitForItem(FixtureItemName);
            home.WaitForItemStatus(FixtureItemName, "NotInstalled");
            Assert.False(File.Exists(markerPath), $"Fixture marker should be absent before install: {markerPath}");

            home.InstallButton(FixtureItemName).Invoke();

            session.WaitUntil(() => File.Exists(markerPath), TimeSpan.FromSeconds(60));
            home.WaitForItemStatus(FixtureItemName, "Installed", TimeSpan.FromSeconds(30));
            Assert.False(home.HasOperationFailureText());
            home.EnsureItemVisible(FixtureItemName);
            session.CaptureCheckpoint("reopen-after-install-before-close", includeAutomationTree: true);
        });

        RunWithDiagnostics(nameof(InstalledAndRemovedStateSurvivesAppRelaunch) + "-verify-installed", session =>
        {
            var home = new HomePageDriver(session);

            Assert.Equal("Available Software", home.Heading.Name);
            _ = home.WaitForItem(FixtureItemName);
            home.WaitForItemStatus(FixtureItemName, "Installed", TimeSpan.FromSeconds(30));
            Assert.True(File.Exists(markerPath), $"Fixture marker should remain present after UI relaunch: {markerPath}");
            Assert.False(home.HasOperationFailureText());
            home.EnsureItemVisible(FixtureItemName);
            session.CaptureCheckpoint("reopen-installed", includeAutomationTree: true);

            home.RemoveButton(FixtureItemName).Invoke();

            session.WaitUntil(() => !File.Exists(markerPath), TimeSpan.FromSeconds(60));
            home.WaitForItemStatus(FixtureItemName, "NotInstalled", TimeSpan.FromSeconds(30));
            Assert.False(home.HasOperationFailureText());
            home.EnsureItemVisible(FixtureItemName);
            session.CaptureCheckpoint("reopen-after-remove-before-close", includeAutomationTree: true);
        });

        RunWithDiagnostics(nameof(InstalledAndRemovedStateSurvivesAppRelaunch) + "-verify-removed", session =>
        {
            var home = new HomePageDriver(session);

            Assert.Equal("Available Software", home.Heading.Name);
            _ = home.WaitForItem(FixtureItemName);
            home.WaitForItemStatus(FixtureItemName, "NotInstalled", TimeSpan.FromSeconds(30));
            Assert.False(File.Exists(markerPath), $"Fixture marker should remain absent after UI relaunch: {markerPath}");
            Assert.False(home.HasOperationFailureText());
            home.EnsureItemVisible(FixtureItemName);
            session.CaptureCheckpoint("reopen-not-installed", includeAutomationTree: true);
        });
    }

    [Fact]
    [Trait("E2EPhase", "Healthy")]
    public void DeliberateInstallerFailureIsDisplayedOnItsCard()
    {
        RunWithDiagnostics(nameof(DeliberateInstallerFailureIsDisplayedOnItsCard), session =>
        {
            var home = new HomePageDriver(session);

            Assert.Equal("Available Software", home.Heading.Name);
            _ = home.WaitForItem(FailureFixtureItemName);
            home.WaitForItemStatus(FailureFixtureItemName, "NotInstalled");
            var actionTopBefore = home.PrimaryActionTop(FailureFixtureItemName);
            home.EnsureItemVisible(FailureFixtureItemName);
            session.CaptureCheckpoint("failure-before-install", includeAutomationTree: true);

            home.InstallButton(FailureFixtureItemName).Invoke();

            home.WaitForTerminalFeedbackContaining(FailureFixtureItemName, "Installation error: exit status 7", TimeSpan.FromSeconds(60));
            Assert.Contains(
                "Installation error: exit status 7",
                home.TerminalFeedbackText(FailureFixtureItemName),
                StringComparison.OrdinalIgnoreCase
            );
            Assert.True(
                string.IsNullOrWhiteSpace(home.WarningText),
                $"Item-specific failure should not populate the page warning: {home.WarningText}"
            );
            Assert.InRange(home.PrimaryActionTop(FailureFixtureItemName), actionTopBefore - 1.0, actionTopBefore + 1.0);
            home.WaitForItemStatus(FailureFixtureItemName, "NotInstalled", TimeSpan.FromSeconds(30));
            home.EnsureItemVisible(FailureFixtureItemName);
            session.CaptureCheckpoint("failure-after-install", includeAutomationTree: true);
        });
    }

    [Fact]
    [Trait("E2EPhase", "ServiceUnavailable")]
    public void ServiceUnavailableShowsCachedThenNoCacheFailureStatesTruthfully()
    {
        var cachePath = RequiredPath("GORILLA_UI_E2E_CACHE_PATH");

        RunWithDiagnostics(nameof(ServiceUnavailableShowsCachedThenNoCacheFailureStatesTruthfully) + "-cached", session =>
        {
            var home = new HomePageDriver(session);
            var shell = new CatalogShellDriver(session);
            Assert.Equal("Available Software", home.Heading.Name);
            _ = home.WaitForItem(FixtureItemName);
            shell.WaitForFreshnessContaining("Showing saved data", TimeSpan.FromSeconds(15));
            shell.WaitForDegradedWarningContaining("couldn't refresh the catalog", TimeSpan.FromSeconds(15));
            Assert.DoesNotContain("Updated", shell.FreshnessText, StringComparison.OrdinalIgnoreCase);
            home.EnsureItemVisible(FixtureItemName);
            session.CaptureCheckpoint("cached-service-unavailable", includeAutomationTree: true);
        });

        File.Delete(cachePath);

        RunWithDiagnostics(nameof(ServiceUnavailableShowsCachedThenNoCacheFailureStatesTruthfully) + "-no-cache", session =>
        {
            var shell = new CatalogShellDriver(session);
            session.WaitUntil(() => shell.HasLoadFailedState(), TimeSpan.FromSeconds(15));

            Assert.False(shell.HasSuccessfulEmptyState());
            Assert.True(shell.RefreshButton.IsEnabled);
            Assert.Contains("unavailable", shell.FreshnessText, StringComparison.OrdinalIgnoreCase);
            session.CaptureCheckpoint("service-unavailable-no-cache", includeAutomationTree: true);
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
