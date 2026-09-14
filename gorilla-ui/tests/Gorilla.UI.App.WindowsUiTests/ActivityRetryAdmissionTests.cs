using Xunit;

namespace Gorilla.UI.App.WindowsUiTests;

public sealed class ActivityRetryAdmissionTests
{
    private const string FailureFixtureItemName = "Ps1Failure";
    private const string FailureMarkerPath = @"C:\ProgramData\gorilla-it\ps1-failure.txt";

    [Fact]
    [Trait("E2EPhase", "Healthy")]
    public void RetryRejectedByCurrentServiceTruthShowsFeedbackOnActivityRow()
    {
        RunWithDiagnostics(nameof(RetryRejectedByCurrentServiceTruthShowsFeedbackOnActivityRow), session =>
        {
            try
            {
                File.Delete(FailureMarkerPath);

                var home = new HomePageDriver(session);
                var shell = new CatalogShellDriver(session);
                _ = home.WaitForItem(FailureFixtureItemName);
                shell.Refresh();
                shell.WaitForRefreshComplete(TimeSpan.FromSeconds(30));
                home.WaitForItemStatus(FailureFixtureItemName, "Not installed", TimeSpan.FromSeconds(30));

                home.PrimaryActionButton(FailureFixtureItemName).Invoke();
                home.WaitForTerminalFeedbackContaining(
                    FailureFixtureItemName,
                    "Installation error: exit status 7",
                    TimeSpan.FromSeconds(60)
                );

                var activity = ActivityPageDriver.OpenFromCatalog(session);
                var failedEntry = activity.WaitForEntryWithDetail(
                    "Installation error: exit status 7",
                    TimeSpan.FromSeconds(30)
                );
                var operationId = ActivityPageDriver.OperationId(failedEntry);
                Assert.False(string.IsNullOrWhiteSpace(operationId));
                activity.WaitForOperationState(operationId, "Failed", TimeSpan.FromSeconds(30));
                Assert.True(activity.HasRetryButton(operationId));

                // Change detection truth after the UI has projected Retry eligibility,
                // without refreshing the client. The service must revalidate the new
                // Install intent and reject it before creating another operation.
                Directory.CreateDirectory(Path.GetDirectoryName(FailureMarkerPath)!);
                File.WriteAllText(FailureMarkerPath, "externally-installed");

                activity.RetryButton(operationId).Invoke();
                session.WaitUntil(
                    () => activity.RetryAttemptFeedback(operationId)
                        .Contains("already selected", StringComparison.OrdinalIgnoreCase),
                    TimeSpan.FromSeconds(30)
                );
                session.WaitUntil(
                    () => !activity.HasRetryButton(operationId),
                    TimeSpan.FromSeconds(30)
                );

                Assert.Equal("Failed", activity.StateText(operationId));
                Assert.Equal(1, activity.CountEntries(operationId));
                Assert.Contains(
                    "already selected",
                    activity.RetryAttemptFeedback(operationId),
                    StringComparison.OrdinalIgnoreCase
                );
                Assert.Contains(
                    "already selected",
                    activity.RetryUnavailableText(operationId),
                    StringComparison.OrdinalIgnoreCase
                );
                Assert.False(activity.HasRetryButton(operationId));
                session.CaptureCheckpoint("activity-retry-admission-rejected", includeAutomationTree: true);
            }
            finally
            {
                File.Delete(FailureMarkerPath);
            }
        });
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
