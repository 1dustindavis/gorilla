using System.Security.Cryptography;
using Xunit;

namespace Gorilla.UI.App.WindowsUiTests;

public sealed class ActivityTests
{
    private const string FailureFixtureItemName = "Ps1Failure";
    private const string SlowFixtureItemName = "SlowInstallFixture";

    [Fact]
    [Trait("E2EPhase", "Healthy")]
    public void ActiveOperationTransitionsInPlaceAndNavigatesToSameDetailsOperation()
    {
        RunWithDiagnostics(nameof(ActiveOperationTransitionsInPlaceAndNavigatesToSameDetailsOperation), session =>
        {
            var slowMarkerPath = RequiredPath("GORILLA_UI_E2E_SLOW_MARKER_PATH");
            var home = new HomePageDriver(session);
            EnsureSlowFixtureAbsent(session, home, slowMarkerPath);

            home.PrimaryActionButton(SlowFixtureItemName).Invoke();
            home.WaitForOperationContaining(SlowFixtureItemName, "Installing", TimeSpan.FromSeconds(30));
            var operationId = home.OperationId(SlowFixtureItemName);
            Assert.False(string.IsNullOrWhiteSpace(operationId));

            var activity = ActivityPageDriver.OpenFromCatalog(session);
            activity.WaitForOperationState(operationId, "Installing", TimeSpan.FromSeconds(30));
            Assert.Equal(1, activity.CountEntries(operationId));
            Assert.Equal("Install", activity.ActionText(operationId));
            session.CaptureCheckpoint("activity-active", includeAutomationTree: true);

            activity.OpenDetails(operationId);
            var details = new AppDetailsPageDriver(session);
            details.WaitForActiveOperation("Install", TimeSpan.FromSeconds(30));
            Assert.Equal(operationId, details.ActiveOperationId);
            session.CaptureCheckpoint("activity-details-same-operation");

            session.WaitUntil(() => File.Exists(slowMarkerPath), TimeSpan.FromSeconds(30));
            details.WaitForObservation("Installed", TimeSpan.FromSeconds(30));
            details.GoBack();

            activity = new ActivityPageDriver(session);
            activity.WaitForOperationState(operationId, "Succeeded", TimeSpan.FromSeconds(30));
            Assert.Equal(1, activity.CountEntries(operationId));
            session.CaptureCheckpoint("activity-terminal-same-entry", includeAutomationTree: true);
        });
    }

    [Fact]
    [Trait("E2EPhase", "Healthy")]
    public void CatalogRefreshPreservesActiveOperationAndActivityIdentity()
    {
        RunWithDiagnostics(nameof(CatalogRefreshPreservesActiveOperationAndActivityIdentity), session =>
        {
            var slowMarkerPath = RequiredPath("GORILLA_UI_E2E_SLOW_MARKER_PATH");
            var home = new HomePageDriver(session);
            var shell = new CatalogShellDriver(session);
            EnsureSlowFixtureAbsent(session, home, slowMarkerPath);

            home.PrimaryActionButton(SlowFixtureItemName).Invoke();
            home.WaitForOperationContaining(SlowFixtureItemName, "Installing", TimeSpan.FromSeconds(30));
            var operationId = home.OperationId(SlowFixtureItemName);
            Assert.False(string.IsNullOrWhiteSpace(operationId));

            shell.Refresh();
            shell.WaitForRefreshStarted(TimeSpan.FromSeconds(30));
            Assert.Equal(operationId, home.OperationId(SlowFixtureItemName));

            var activity = ActivityPageDriver.OpenFromCatalog(session);
            activity.WaitForOperationState(operationId, "Installing", TimeSpan.FromSeconds(30));
            Assert.Equal(1, activity.CountEntries(operationId));
            Assert.Equal("Install", activity.ActionText(operationId));
            session.CaptureCheckpoint("activity-during-catalog-refresh", includeAutomationTree: true);

            session.WaitUntil(() => File.Exists(slowMarkerPath), TimeSpan.FromSeconds(30));
            activity.WaitForOperationState(operationId, "Succeeded", TimeSpan.FromSeconds(30));
            Assert.Equal(1, activity.CountEntries(operationId));

            shell.WaitForRefreshComplete(TimeSpan.FromSeconds(30));
            Assert.Equal(1, activity.CountEntries(operationId));
            Assert.Equal("Succeeded", activity.StateText(operationId));
            session.CaptureCheckpoint("activity-after-catalog-refresh", includeAutomationTree: true);
        });
    }

    [Fact]
    [Trait("E2EPhase", "Healthy")]
    public void RetainedFailureStaysLocalAndShowsStructuredDetail()
    {
        RunWithDiagnostics(nameof(RetainedFailureStaysLocalAndShowsStructuredDetail), session =>
        {
            var home = new HomePageDriver(session);
            _ = home.WaitForItem(FailureFixtureItemName);
            home.PrimaryActionButton(FailureFixtureItemName).Invoke();
            home.WaitForTerminalFeedbackContaining(FailureFixtureItemName, "Installation error: exit status 7", TimeSpan.FromSeconds(60));
            Assert.DoesNotContain("Operation failed", home.WarningText, StringComparison.OrdinalIgnoreCase);

            var activity = ActivityPageDriver.OpenFromCatalog(session);
            var entry = activity.WaitForEntryWithDetail("Installation error: exit status 7", TimeSpan.FromSeconds(30));
            var operationId = ActivityPageDriver.OperationId(entry);
            Assert.False(string.IsNullOrWhiteSpace(operationId));
            activity.WaitForOperationState(operationId, "Failed", TimeSpan.FromSeconds(30));
            Assert.Equal("Installation failed", activity.FailureTitle(operationId));
            Assert.True(activity.HasRetryButton(operationId));
            var technical = activity.OpenAndReadTechnicalDetails(operationId);
            Assert.Contains($"Operation ID: {operationId}", technical, StringComparison.OrdinalIgnoreCase);
            Assert.Contains("Outcome: Failed", technical, StringComparison.OrdinalIgnoreCase);
            Assert.Contains("Installation error: exit status 7", technical, StringComparison.OrdinalIgnoreCase);
            Assert.Equal(1, activity.CountEntries(operationId));
            session.CaptureCheckpoint("activity-retained-failure", includeAutomationTree: true);
        });
    }

    [Fact]
    [Trait("E2EPhase", "Healthy")]
    public void UiRelaunchRecoversRetainedOperationWithoutCreatingDuplicateActivity()
    {
        var slowMarkerPath = RequiredPath("GORILLA_UI_E2E_SLOW_MARKER_PATH");
        string operationId;

        using (var first = GorillaAppSession.Launch())
        {
            try
            {
                var home = new HomePageDriver(first);
                EnsureSlowFixtureAbsent(first, home, slowMarkerPath);

                home.PrimaryActionButton(SlowFixtureItemName).Invoke();
                home.WaitForOperationContaining(SlowFixtureItemName, "Installing", TimeSpan.FromSeconds(30));
                operationId = home.OperationId(SlowFixtureItemName);
                Assert.False(string.IsNullOrWhiteSpace(operationId));
                first.CaptureCheckpoint("activity-before-ui-relaunch");
            }
            catch (Exception ex)
            {
                first.CaptureFailure(ex, nameof(UiRelaunchRecoversRetainedOperationWithoutCreatingDuplicateActivity));
                throw;
            }
        }

        using var second = GorillaAppSession.Launch();
        try
        {
            var home = new HomePageDriver(second);
            _ = home.WaitForItem(SlowFixtureItemName);
            var activity = ActivityPageDriver.OpenFromCatalog(second);
            _ = activity.WaitForOperation(operationId, TimeSpan.FromSeconds(30));

            // Activity is a virtualized ListView, so UI Automation only exposes
            // currently realized containers. The stable duplicate invariant is that
            // the recovered service OperationId appears exactly once.
            Assert.Equal(1, activity.CountEntries(operationId));
            Assert.True(
                activity.StateText(operationId).Contains("Installing", StringComparison.OrdinalIgnoreCase)
                || activity.StateText(operationId).Contains("Succeeded", StringComparison.OrdinalIgnoreCase),
                $"Expected recovered operation {operationId} to be active or retained terminal, got '{activity.StateText(operationId)}'."
            );
            second.CaptureCheckpoint("activity-after-ui-relaunch", includeAutomationTree: true);

            // The relaunch assertion above intentionally observes an operation that
            // may still be active. Finish that same recovered operation before this
            // test releases the shared E2E service/fixture state to the next test.
            second.WaitUntil(() => File.Exists(slowMarkerPath), TimeSpan.FromSeconds(30));
            activity.WaitForOperationState(operationId, "Succeeded", TimeSpan.FromSeconds(30));
        }
        catch (Exception ex)
        {
            second.CaptureFailure(ex, nameof(UiRelaunchRecoversRetainedOperationWithoutCreatingDuplicateActivity));
            throw;
        }
    }

    [Fact]
    [Trait("E2EPhase", "Healthy")]
    public void FailedInstallRelaunchRetryCreatesNewOperationAndSucceeds()
    {
        var slowMarkerPath = RequiredPath("GORILLA_UI_E2E_SLOW_MARKER_PATH");
        if (!TryResolveMutableFixture(out var installScriptPath, out var catalogPath))
        {
            // This scenario requires the mutable HTTP fixture repository. Harnesses
            // that do not provide that explicit contract cannot exercise it safely.
            return;
        }

        var originalScript = File.ReadAllText(installScriptPath);
        var originalCatalog = File.ReadAllText(catalogPath);
        var originalHash = Sha256(installScriptPath);
        string failedOperationId = string.Empty;

        try
        {
            using (var first = GorillaAppSession.Launch())
            {
                try
                {
                    var home = new HomePageDriver(first);
                    var shell = new CatalogShellDriver(first);
                    EnsureSlowFixtureAbsent(first, home, slowMarkerPath);

                    File.WriteAllText(
                        installScriptPath,
                        "Write-Error \"Intentional retry fixture failure\"\r\nexit 9"
                    );
                    var failingHash = Sha256(installScriptPath);
                    Assert.Contains(originalHash, originalCatalog, StringComparison.OrdinalIgnoreCase);
                    File.WriteAllText(
                        catalogPath,
                        originalCatalog.Replace(originalHash, failingHash, StringComparison.OrdinalIgnoreCase)
                    );

                    shell.Refresh();
                    shell.WaitForRefreshComplete(TimeSpan.FromSeconds(30));
                    home.PrimaryActionButton(SlowFixtureItemName).Invoke();
                    home.WaitForTerminalFeedbackContaining(SlowFixtureItemName, "exit status 9", TimeSpan.FromSeconds(60));

                    var activity = ActivityPageDriver.OpenFromCatalog(first);
                    var failedEntry = activity.WaitForEntryWithDetail("exit status 9", TimeSpan.FromSeconds(30));
                    failedOperationId = ActivityPageDriver.OperationId(failedEntry);
                    Assert.False(string.IsNullOrWhiteSpace(failedOperationId));
                    activity.WaitForOperationState(failedOperationId, "Failed", TimeSpan.FromSeconds(30));
                    Assert.Equal("Installation failed", activity.FailureTitle(failedOperationId));
                    Assert.True(activity.HasRetryButton(failedOperationId));

                    var technical = activity.OpenAndReadTechnicalDetails(failedOperationId);
                    Assert.Contains($"Operation ID: {failedOperationId}", technical, StringComparison.OrdinalIgnoreCase);
                    Assert.Contains("Outcome: Failed", technical, StringComparison.OrdinalIgnoreCase);
                    Assert.Contains("exit status 9", technical, StringComparison.OrdinalIgnoreCase);

                    activity.OpenDetails(failedOperationId);
                    var details = new AppDetailsPageDriver(first);
                    Assert.Equal("Installation failed", details.FailureTitle(failedOperationId));
                    Assert.True(details.HasRetryButton(failedOperationId));
                    Assert.Contains(
                        $"Operation ID: {failedOperationId}",
                        details.OpenAndReadTechnicalDetails(failedOperationId),
                        StringComparison.OrdinalIgnoreCase
                    );
                    details.GoBack();

                    // A Refresh must be able to remove Retry when current truth changes,
                    // without rewriting the retained failure. Simulate an external
                    // install: the failed request left the item selected, so present +
                    // selected makes current Install disallowed as already_selected.
                    Directory.CreateDirectory(Path.GetDirectoryName(slowMarkerPath)!);
                    File.WriteAllText(slowMarkerPath, "externally-installed");
                    shell.Refresh();
                    shell.WaitForRefreshComplete(TimeSpan.FromSeconds(30));
                    first.WaitUntil(() => !activity.HasRetryButton(failedOperationId), TimeSpan.FromSeconds(30));
                    Assert.Contains("already selected", activity.RetryUnavailableText(failedOperationId), StringComparison.OrdinalIgnoreCase);
                    Assert.Equal("Failed", activity.StateText(failedOperationId));

                    File.Delete(slowMarkerPath);
                    shell.Refresh();
                    shell.WaitForRefreshComplete(TimeSpan.FromSeconds(30));
                    first.WaitUntil(() => activity.HasRetryButton(failedOperationId), TimeSpan.FromSeconds(30));
                    first.CaptureCheckpoint("retry-failure-before-relaunch", includeAutomationTree: true);
                }
                catch (Exception ex)
                {
                    first.CaptureFailure(ex, nameof(FailedInstallRelaunchRetryCreatesNewOperationAndSucceeds) + "-before-relaunch");
                    throw;
                }
            }

            // Restore the successful installer before relaunch. The retained failed
            // operation remains service truth, while current catalog truth now permits
            // the user to submit a new Install intent.
            File.WriteAllText(installScriptPath, originalScript);
            File.WriteAllText(catalogPath, originalCatalog);

            using var second = GorillaAppSession.Launch();
            try
            {
                var home = new HomePageDriver(second);
                var shell = new CatalogShellDriver(second);
                _ = home.WaitForItem(SlowFixtureItemName);
                shell.Refresh();
                shell.WaitForRefreshComplete(TimeSpan.FromSeconds(30));

                var activity = ActivityPageDriver.OpenFromCatalog(second);
                activity.WaitForOperationState(failedOperationId, "Failed", TimeSpan.FromSeconds(30));
                Assert.Equal(1, activity.CountEntries(failedOperationId));
                Assert.True(activity.HasRetryButton(failedOperationId));

                activity.RetryButton(failedOperationId).Invoke();

                var retryEntry = activity.WaitForDifferentOperation(
                    "Slow Install Fixture",
                    failedOperationId,
                    TimeSpan.FromSeconds(30)
                );
                var retryOperationId = ActivityPageDriver.OperationId(retryEntry);

                Assert.False(string.IsNullOrWhiteSpace(retryOperationId));
                Assert.NotEqual(failedOperationId, retryOperationId);
                activity.WaitForOperationState(retryOperationId, "Installing", TimeSpan.FromSeconds(30));
                Assert.Equal("Install", activity.ActionText(retryOperationId));

                activity.WaitForOperationState(retryOperationId, "Succeeded", TimeSpan.FromSeconds(60));
                Assert.Equal("Failed", activity.StateText(failedOperationId));
                Assert.Equal(1, activity.CountEntries(failedOperationId));
                Assert.Equal(1, activity.CountEntries(retryOperationId));
                second.WaitUntil(() => File.Exists(slowMarkerPath), TimeSpan.FromSeconds(30));

                activity.OpenDetails(retryOperationId);
                var details = new AppDetailsPageDriver(second);
                details.WaitForObservation("Installed", TimeSpan.FromSeconds(30));
                details.GoBack();
                activity = new ActivityPageDriver(second);
                activity.GoBack();

                home = new HomePageDriver(second);
                home.WaitForItemStatus(SlowFixtureItemName, "Installed", TimeSpan.FromSeconds(30));
                home.RemoveButton(SlowFixtureItemName).Invoke();
                second.WaitUntil(() => !File.Exists(slowMarkerPath), TimeSpan.FromSeconds(30));
                home.WaitForItemStatus(SlowFixtureItemName, "Not installed", TimeSpan.FromSeconds(30));
                second.CaptureCheckpoint("retry-success-new-operation", includeAutomationTree: true);
            }
            catch (Exception ex)
            {
                second.CaptureFailure(ex, nameof(FailedInstallRelaunchRetryCreatesNewOperationAndSucceeds) + "-after-relaunch");
                throw;
            }
        }
        finally
        {
            File.WriteAllText(installScriptPath, originalScript);
            File.WriteAllText(catalogPath, originalCatalog);
            File.Delete(slowMarkerPath);
        }
    }

    private static void EnsureSlowFixtureAbsent(GorillaAppSession session, HomePageDriver home, string slowMarkerPath)
    {
        _ = home.WaitForItem(SlowFixtureItemName);
        if (string.Equals(home.ItemStatus(SlowFixtureItemName), "Installed", StringComparison.OrdinalIgnoreCase))
        {
            home.RemoveButton(SlowFixtureItemName).Invoke();
            session.WaitUntil(() => !File.Exists(slowMarkerPath), TimeSpan.FromSeconds(30));
            home.WaitForItemStatus(SlowFixtureItemName, "Not installed", TimeSpan.FromSeconds(30));
        }
        else
        {
            home.WaitForItemStatus(SlowFixtureItemName, "Not installed", TimeSpan.FromSeconds(30));
        }

        session.WaitUntil(
            () => home.PrimaryActionButton(SlowFixtureItemName).IsEnabled,
            TimeSpan.FromSeconds(30)
        );
    }

    private static bool TryResolveMutableFixture(out string installScriptPath, out string catalogPath)
    {
        installScriptPath = string.Empty;
        catalogPath = string.Empty;
        var fixtureRoot = Environment.GetEnvironmentVariable("GORILLA_UI_E2E_FIXTURE_ROOT");
        if (string.IsNullOrWhiteSpace(fixtureRoot))
        {
            return false;
        }

        fixtureRoot = Path.GetFullPath(fixtureRoot);
        installScriptPath = Path.Combine(fixtureRoot, "packages", "scripts", "ui-slow-install.ps1");
        catalogPath = Path.Combine(fixtureRoot, "catalogs", "integration.yaml");
        return File.Exists(installScriptPath) && File.Exists(catalogPath);
    }

    private static string Sha256(string path)
        => Convert.ToHexString(SHA256.HashData(File.ReadAllBytes(path))).ToLowerInvariant();

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
