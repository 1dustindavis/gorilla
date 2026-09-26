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
            // suite retains its existing relaunch coverage unchanged.
            return;
        }

        var slowMarkerPath = RequiredPath("GORILLA_UI_E2E_SLOW_MARKER_PATH");
        var appDataPath = Path.GetDirectoryName(slowMarkerPath)!;
        var serviceLogPath = Path.Combine(appDataPath, "gorilla.log");
        var operationsPath = Path.Combine(appDataPath, "operations");
        string operationId;
        int installRequestsBeforeSubmit;
        int installRequestsAtClose;
        int listRequestsAtClose;

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
                installRequestsBeforeSubmit = CountServiceRequests(serviceLogPath, "InstallItem");
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

                first.WaitUntil(
                    () => CountServiceRequests(serviceLogPath, "InstallItem") > installRequestsBeforeSubmit,
                    TimeSpan.FromSeconds(30)
                );
                installRequestsAtClose = CountServiceRequests(serviceLogPath, "InstallItem");
                Assert.Equal(installRequestsBeforeSubmit + 1, installRequestsAtClose);
                Assert.Equal(1, CountOperationFiles(operationsPath, operationId));

                listRequestsAtClose = CountServiceRequests(serviceLogPath, "ListOperations");
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

            // Startup recovery asks the real installed service for retained operations.
            second.WaitUntil(
                () => CountServiceRequests(serviceLogPath, "ListOperations") > listRequestsAtClose,
                TimeSpan.FromSeconds(30)
            );

            var activity = ActivityPageDriver.OpenFromCatalog(second);
            _ = activity.WaitForOperation(operationId, TimeSpan.FromSeconds(30));
            Assert.True(
                activity.StateText(operationId).Contains("Installing", StringComparison.OrdinalIgnoreCase)
                || activity.StateText(operationId).Contains("Succeeded", StringComparison.OrdinalIgnoreCase),
                $"Expected recovered operation {operationId} to be active or retained terminal, got '{activity.StateText(operationId)}'."
            );

            // The service-owned operation store is the logical identity invariant.
            // Do not use the count of currently realized virtualized Activity rows.
            Assert.Equal(1, CountOperationFiles(operationsPath, operationId));
            Assert.Equal(installRequestsAtClose, CountServiceRequests(serviceLogPath, "InstallItem"));
            second.CaptureCheckpoint("installed-relaunch-after-recovery", includeAutomationTree: true);

            second.WaitUntil(() => File.Exists(slowMarkerPath), TimeSpan.FromSeconds(30));
            activity.WaitForOperationState(operationId, "Succeeded", TimeSpan.FromSeconds(30));
            Assert.Equal(1, CountOperationFiles(operationsPath, operationId));
            var installRequestsAfterCompletion = CountServiceRequests(serviceLogPath, "InstallItem");
            Assert.Equal(installRequestsAtClose, installRequestsAfterCompletion);

            WriteRelaunchEvidence(
                operationId,
                installRequestsBeforeSubmit,
                installRequestsAtClose,
                installRequestsAfterCompletion,
                listRequestsAtClose,
                CountServiceRequests(serviceLogPath, "ListOperations"),
                CountOperationFiles(operationsPath, operationId)
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

    private static int CountServiceRequests(string logPath, string operation)
    {
        try
        {
            return File.Exists(logPath)
                ? File.ReadLines(logPath).Count(line =>
                    line.Contains($"named pipe request: {operation} ", StringComparison.Ordinal))
                : 0;
        }
        catch (IOException)
        {
            return 0;
        }
    }

    private static int CountOperationFiles(string operationsPath, string operationId)
    {
        try
        {
            return Directory.Exists(operationsPath)
                ? Directory.GetFiles(operationsPath, $"{operationId}-*.yaml").Length
                : 0;
        }
        catch (IOException)
        {
            return 0;
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
        int installRequestsBeforeSubmit,
        int installRequestsAtClose,
        int installRequestsAfterCompletion,
        int listRequestsAtClose,
        int listRequestsAfterRelaunch,
        int retainedOperationFileCount
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
                $"RetainedOperationFileCount: {retainedOperationFileCount}",
                $"InstallItemRequestsBeforeSubmit: {installRequestsBeforeSubmit}",
                $"InstallItemRequestsAtClose: {installRequestsAtClose}",
                $"InstallItemRequestsAfterCompletion: {installRequestsAfterCompletion}",
                $"ListOperationsRequestsAtClose: {listRequestsAtClose}",
                $"ListOperationsRequestsAfterRelaunch: {listRequestsAfterRelaunch}",
                $"SecondMutationSubmitted: {installRequestsAfterCompletion != installRequestsAtClose}"
            ]
        );
    }
}
