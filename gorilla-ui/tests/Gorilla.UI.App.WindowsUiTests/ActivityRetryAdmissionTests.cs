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

                // The service intentionally retains recent operations across UI test
                // processes. Capture the existing Ps1Failure identities so the later
                // assertion targets the operation created by this test rather than an
                // older retained row with the same failure message.
                var activity = ActivityPageDriver.OpenFromCatalog(session);
                var existingOperationIds = activity.OperationIdsForItem(FailureFixtureItemName);
                activity.GoBack();

                home.PrimaryActionButton(FailureFixtureItemName).Invoke();
                home.WaitForTerminalFeedbackContaining(
                    FailureFixtureItemName,
                    "Installation error: exit status 7",
                    TimeSpan.FromSeconds(60)
                );

                activity = ActivityPageDriver.OpenFromCatalog(session);
                var failedEntry = activity.WaitForNewEntryWithDetail(
                    "Installation error: exit status 7",
                    existingOperationIds,
                    TimeSpan.FromSeconds(30)
                );
                var operationId = ActivityPageDriver.OperationId(failedEntry);
                Assert.False(string.IsNullOrWhiteSpace(operationId));
                activity.WaitForOperationState(operationId, "Failed", TimeSpan.FromSeconds(30));

                // The operation reaches its terminal service state before the initiating
                // Core action necessarily finishes its post-terminal catalog refresh and
                // clears IsBusy. Retry is intentionally suppressed during that short
                // cleanup window, so wait for the actual actionable control rather than
                // asserting synchronously at the first Failed projection.
                var retryButton = activity.RetryButton(operationId);

                // Change detection truth after the UI has projected Retry eligibility,
                // without refreshing the client. The service must revalidate the new
                // Install intent and reject it before creating another operation.
                Directory.CreateDirectory(Path.GetDirectoryName(FailureMarkerPath)!);
                File.WriteAllText(FailureMarkerPath, "externally-installed");

                retryButton.Invoke();
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
