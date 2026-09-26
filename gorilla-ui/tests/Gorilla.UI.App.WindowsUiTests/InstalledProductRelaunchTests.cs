using System.Text;
using Gorilla.UI.Client;
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
        string operationId;
        int installRequestsBeforeSubmit;
        int installRequestsAtClose;
        int listRequestsAtClose;
        int logicalOperationCountAtClose;

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

                logicalOperationCountAtClose = CountLogicalOperations(operationId);
                Assert.Equal(1, logicalOperationCountAtClose);

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

            var logicalOperationCountAfterRelaunch = CountLogicalOperations(operationId);
            Assert.Equal(1, logicalOperationCountAfterRelaunch);

            var activity = ActivityPageDriver.OpenFromCatalog(second);
            _ = activity.WaitForOperation(operationId, TimeSpan.FromSeconds(30));
            Assert.True(
                activity.StateText(operationId).Contains("Installing", StringComparison.OrdinalIgnoreCase)
                || activity.StateText(operationId).Contains("Succeeded", StringComparison.OrdinalIgnoreCase),
                $"Expected recovered operation {operationId} to be active or retained terminal, got '{activity.StateText(operationId)}'."
            );

            Assert.Equal(installRequestsAtClose, CountServiceRequests(serviceLogPath, "InstallItem"));
            second.CaptureCheckpoint("installed-relaunch-after-recovery", includeAutomationTree: true);

            second.WaitUntil(() => File.Exists(slowMarkerPath), TimeSpan.FromSeconds(30));
            activity.WaitForOperationState(operationId, "Succeeded", TimeSpan.FromSeconds(30));
            var installRequestsAfterCompletion = CountServiceRequests(serviceLogPath, "InstallItem");
            Assert.Equal(installRequestsAtClose, installRequestsAfterCompletion);

            WriteRelaunchEvidence(
                operationId,
                logicalOperationCountAtClose,
                logicalOperationCountAfterRelaunch,
                installRequestsBeforeSubmit,
                installRequestsAtClose,
                installRequestsAfterCompletion,
                listRequestsAtClose,
                CountServiceRequests(serviceLogPath, "ListOperations")
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

    private static int CountLogicalOperations(string operationId)
    {
        using var cts = new CancellationTokenSource(TimeSpan.FromSeconds(10));
        var client = new NamedPipeGorillaServiceClient();
        var operations = client.ListOperationsAsync(cts.Token).GetAwaiter().GetResult();
        return operations.Count(operation => string.Equals(operation.OperationId, operationId, StringComparison.Ordinal));
    }

    private static int CountServiceRequests(string logPath, string operation)
    {
        try
        {
            if (!File.Exists(logPath))
            {
                return 0;
            }

            // The LocalSystem service keeps gorilla.log open while these assertions
            // run. Read it with sharing enabled instead of File.ReadLines, whose
            // default FileShare.Read can conflict with the service writer on Windows.
            using var stream = new FileStream(
                logPath,
                FileMode.Open,
                FileAccess.Read,
                FileShare.ReadWrite | FileShare.Delete
            );
            using var reader = new StreamReader(stream, Encoding.UTF8, detectEncodingFromByteOrderMarks: true);
            var count = 0;
            while (reader.ReadLine() is { } line)
            {
                if (line.Contains($"named pipe request: {operation} ", StringComparison.Ordinal))
                {
                    count++;
                }
            }
            return count;
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
        int logicalOperationCountAtClose,
        int logicalOperationCountAfterRelaunch,
        int installRequestsBeforeSubmit,
        int installRequestsAtClose,
        int installRequestsAfterCompletion,
        int listRequestsAtClose,
        int listRequestsAfterRelaunch
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
                $"LogicalOperationCountAtClose: {logicalOperationCountAtClose}",
                $"LogicalOperationCountAfterRelaunch: {logicalOperationCountAfterRelaunch}",
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
