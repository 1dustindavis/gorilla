using Xunit;

namespace Gorilla.UI.App.WindowsUiTests;

public sealed class InstalledProductRelaunchTests
{
    private const string SlowFixtureItemName = "SlowInstallFixture";
    private const string SlowFixtureDisplayName = "Slow Install Fixture";

    [Fact]
    [Trait("E2EPhase", "Healthy")]
    public void PackagedUiRelaunchRecoversSameServiceOperationWithoutResubmission()
    {
        var appUserModelId = Environment.GetEnvironmentVariable("GORILLA_UI_APP_ID");
        if (string.IsNullOrWhiteSpace(appUserModelId))
        {
            // This is deliberately an installed-product assertion. The source-built
            // suite retains its existing relaunch coverage without taking on this
            // produced-MSIX boundary proof.
            return;
        }

        var slowMarkerPath = RequiredPath("GORILLA_UI_E2E_SLOW_MARKER_PATH");
        var clientLogPath = RequiredPath("GORILLA_UI_LOG_PATH");
        string operationId;
        int installMutationsAtClose;
        int listResponsesAtClose;

        using (var first = GorillaAppSession.Launch())
        {
            try
            {
                var home = new HomePageDriver(first);
                EnsureSlowFixtureAbsent(first, home, slowMarkerPath);

                var beforeActivity = ActivityPageDriver.OpenFromCatalog(first);
                var existingOperationIds = beforeActivity.OperationIdsForItem(SlowFixtureDisplayName);
                beforeActivity.GoBack();

                home = new HomePageDriver(first);
                home.PrimaryActionButton(SlowFixtureItemName).Invoke();

                var activity = ActivityPageDriver.OpenFromCatalog(first);
                var newEntry = activity.WaitForNewOperation(
                    SlowFixtureDisplayName,
                    "Install",
                    existingOperationIds,
                    TimeSpan.FromSeconds(30)
                );
                operationId = ActivityPageDriver.OperationId(newEntry);
                Assert.False(string.IsNullOrWhiteSpace(operationId));
                activity.WaitForOperationState(operationId, "Installing", TimeSpan.FromSeconds(30));
                Assert.False(
                    File.Exists(slowMarkerPath),
                    "The slow fixture completed before App Catalog closed, so the run did not exercise UI-independent work."
                );

                installMutationsAtClose = CountSlowInstallMutations(clientLogPath);
                Assert.True(installMutationsAtClose > 0);
                listResponsesAtClose = CountListOperationResponses(clientLogPath);
                first.CaptureCheckpoint("installed-relaunch-before-close", includeAutomationTree: true);
            }
            catch (Exception ex)
            {
                first.CaptureFailure(ex, nameof(PackagedUiRelaunchRecoversSameServiceOperationWithoutResubmission) + "-before-close");
                throw;
            }
        }

        using var second = GorillaAppSession.Launch();
        try
        {
            var home = new HomePageDriver(second);
            _ = home.WaitForItem(SlowFixtureItemName);

            // Startup recovery asks the service for retained operations. Wait for a
            // fresh ListOperations response from the relaunched process, then assert
            // logical identity in that service response rather than counting realized
            // virtualized Activity rows.
            second.WaitUntil(
                () => CountListOperationResponses(clientLogPath) > listResponsesAtClose,
                TimeSpan.FromSeconds(30)
            );
            Assert.Equal(1, CountOperationInLatestListResponse(clientLogPath, operationId));

            var activity = ActivityPageDriver.OpenFromCatalog(second);
            _ = activity.WaitForOperation(operationId, TimeSpan.FromSeconds(30));
            Assert.True(
                activity.StateText(operationId).Contains("Installing", StringComparison.OrdinalIgnoreCase)
                || activity.StateText(operationId).Contains("Succeeded", StringComparison.OrdinalIgnoreCase),
                $"Expected recovered operation {operationId} to be active or retained terminal, got '{activity.StateText(operationId)}'."
            );

            Assert.Equal(installMutationsAtClose, CountSlowInstallMutations(clientLogPath));
            second.CaptureCheckpoint("installed-relaunch-after-recovery", includeAutomationTree: true);

            second.WaitUntil(() => File.Exists(slowMarkerPath), TimeSpan.FromSeconds(30));
            activity.WaitForOperationState(operationId, "Succeeded", TimeSpan.FromSeconds(30));
            Assert.Equal(installMutationsAtClose, CountSlowInstallMutations(clientLogPath));

            WriteRelaunchEvidence(
                operationId,
                installMutationsAtClose,
                CountSlowInstallMutations(clientLogPath),
                CountOperationInLatestListResponse(clientLogPath, operationId)
            );

            activity.GoBack();
            home = new HomePageDriver(second);
            home.WaitForItemStatus(SlowFixtureItemName, "Installed", TimeSpan.FromSeconds(30));
            home.RemoveButton(SlowFixtureItemName).Invoke();
            second.WaitUntil(() => !File.Exists(slowMarkerPath), TimeSpan.FromSeconds(30));
            home.WaitForItemStatus(SlowFixtureItemName, "Not installed", TimeSpan.FromSeconds(30));
        }
        catch (Exception ex)
        {
            second.CaptureFailure(ex, nameof(PackagedUiRelaunchRecoversSameServiceOperationWithoutResubmission) + "-after-relaunch");
            throw;
        }
    }

    private static int CountSlowInstallMutations(string logPath)
        => ReadLogLines(logPath).Count(line =>
            line.Contains("mutation:request:create operation=InstallItem", StringComparison.Ordinal)
            && line.Contains($"itemName={SlowFixtureItemName}", StringComparison.Ordinal)
        );

    private static int CountListOperationResponses(string logPath)
        => ReadLogLines(logPath).Count(IsListOperationsResponse);

    private static int CountOperationInLatestListResponse(string logPath, string operationId)
    {
        var latest = ReadLogLines(logPath).LastOrDefault(IsListOperationsResponse);
        if (latest is null)
        {
            return 0;
        }

        var needle = $"\"operationId\":\"{operationId}\"";
        var count = 0;
        var offset = 0;
        while ((offset = latest.IndexOf(needle, offset, StringComparison.Ordinal)) >= 0)
        {
            count++;
            offset += needle.Length;
        }
        return count;
    }

    private static bool IsListOperationsResponse(string line)
        => line.Contains("response:raw", StringComparison.Ordinal)
            && line.Contains("\"operation\":\"ListOperations\"", StringComparison.Ordinal);

    private static string[] ReadLogLines(string logPath)
    {
        try
        {
            return File.Exists(logPath) ? File.ReadAllLines(logPath) : [];
        }
        catch (IOException)
        {
            // The UI logger may have the file open while appending. Treat a transient
            // read-sharing race as no observation yet so WaitUntil can retry it.
            return [];
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

    private static string RequiredPath(string variableName)
    {
        var value = Environment.GetEnvironmentVariable(variableName);
        if (string.IsNullOrWhiteSpace(value))
        {
            throw new InvalidOperationException($"{variableName} must be set by the installed-product harness.");
        }
        return value;
    }

    private static void WriteRelaunchEvidence(
        string operationId,
        int installMutationsAtClose,
        int installMutationsAfterCompletion,
        int retainedOperationOccurrences
    )
    {
        var artifactsDirectory = Environment.GetEnvironmentVariable("WINDOWS_UI_TEST_ARTIFACTS_DIR");
        if (string.IsNullOrWhiteSpace(artifactsDirectory))
        {
            return;
        }

        Directory.CreateDirectory(artifactsDirectory);
        File.WriteAllLines(
            Path.Combine(artifactsDirectory, "ui-relaunch-operation-identity.txt"),
            [
                $"OperationId: {operationId}",
                "PreCloseState: Installing",
                $"RecoveredListOperationsOccurrences: {retainedOperationOccurrences}",
                $"InstallMutationCountAtClose: {installMutationsAtClose}",
                $"InstallMutationCountAfterCompletion: {installMutationsAfterCompletion}",
                $"SecondMutationSubmitted: {installMutationsAfterCompletion != installMutationsAtClose}"
            ]
        );
    }
}
