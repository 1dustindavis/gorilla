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
            var operationId = activity.OperationId(entry);
            Assert.False(string.IsNullOrWhiteSpace(operationId));
            activity.WaitForOperationState(operationId, "Failed", TimeSpan.FromSeconds(30));
            Assert.Contains("Installation error: exit status 7", activity.DetailText(operationId), StringComparison.OrdinalIgnoreCase);
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
            Assert.Equal(1, activity.CountEntries(operationId));
            Assert.True(
                activity.StateText(operationId).Contains("Installing", StringComparison.OrdinalIgnoreCase)
                || activity.StateText(operationId).Contains("Succeeded", StringComparison.OrdinalIgnoreCase),
                $"Expected recovered operation {operationId} to be active or retained terminal, got '{activity.StateText(operationId)}'."
            );
            second.CaptureCheckpoint("activity-after-ui-relaunch", includeAutomationTree: true);
        }
        catch (Exception ex)
        {
            second.CaptureFailure(ex, nameof(UiRelaunchRecoversRetainedOperationWithoutCreatingDuplicateActivity));
            throw;
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
