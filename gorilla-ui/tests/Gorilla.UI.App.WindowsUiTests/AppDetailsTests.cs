using FlaUI.Core.Definitions;
using Xunit;

namespace Gorilla.UI.App.WindowsUiTests;

public class AppDetailsTests
{
    private const string FailureFixtureItemName = "Ps1Failure";
    private const string SlowFixtureItemName = "SlowInstallFixture";
    private const string InstalledFixtureItemName = "RegistryInstalledFixture";
    private const string UpdateFixtureItemName = "RegistryUpdateFixture";

    [Fact]
    [Trait("E2EPhase", "Healthy")]
    public void DetailsShowsDescriptionAndCurrentState()
    {
        RunWithDiagnostics(nameof(DetailsShowsDescriptionAndCurrentState), session =>
        {
            var home = new HomePageDriver(session);
            _ = home.WaitForItem(InstalledFixtureItemName);
            home.OpenDetails(InstalledFixtureItemName);

            var details = new AppDetailsPageDriver(session);
            Assert.Equal("Installed Fixture", details.DisplayName);
            Assert.Contains("Installed", details.Status, StringComparison.OrdinalIgnoreCase);
            Assert.Contains("fixture", details.Description, StringComparison.OrdinalIgnoreCase);
            session.CaptureCheckpoint("details-installed-description", includeAutomationTree: true);
        });
    }

    [Fact]
    [Trait("E2EPhase", "Healthy")]
    public void DetailsShowsBothActionsForInstalledOptionalItem()
    {
        RunWithDiagnostics(nameof(DetailsShowsBothActionsForInstalledOptionalItem), session =>
        {
            var home = new HomePageDriver(session);
            _ = home.WaitForItem(InstalledFixtureItemName);
            home.OpenDetails(InstalledFixtureItemName);

            var details = new AppDetailsPageDriver(session);
            Assert.Equal("Keep Installed", details.PrimaryAction.Name);
            Assert.Equal("Remove", details.SecondaryAction.Name);
            session.CaptureCheckpoint("details-installed-dual-actions");
        });
    }

    [Fact]
    [Trait("E2EPhase", "Healthy")]
    public void DetailsShowsUpdateAndRemoveForUpdateAvailableItem()
    {
        RunWithDiagnostics(nameof(DetailsShowsUpdateAndRemoveForUpdateAvailableItem), session =>
        {
            var home = new HomePageDriver(session);
            _ = home.WaitForItem(UpdateFixtureItemName);
            home.OpenDetails(UpdateFixtureItemName);

            var details = new AppDetailsPageDriver(session);
            Assert.Equal("Update", details.PrimaryAction.Name);
            Assert.Equal("Remove", details.SecondaryAction.Name);
            session.CaptureCheckpoint("details-update-dual-actions", includeAutomationTree: true);
        });
    }

    [Fact]
    [Trait("E2EPhase", "Healthy")]
    public void DetailsCanInstallAndRemoveWhileKeepingCurrentStateTruthful()
    {
        RunWithDiagnostics(nameof(DetailsCanInstallAndRemoveWhileKeepingCurrentStateTruthful), session =>
        {
            var home = new HomePageDriver(session);
            var markerPath = Path.Combine(session.WorkRoot, "marker-ps1-v1.txt");
            if (File.Exists(markerPath))
            {
                File.Delete(markerPath);
            }

            _ = home.WaitForItem("Ps1V1");
            home.OpenDetails("Ps1V1");
            var details = new AppDetailsPageDriver(session);
            details.PrimaryAction.Invoke();
            details.WaitForStatus("Installed", TimeSpan.FromSeconds(30));
            Assert.True(File.Exists(markerPath));

            Assert.Equal("Keep Installed", details.PrimaryAction.Name);
            Assert.Equal("Remove", details.SecondaryAction.Name);
            details.SecondaryAction.Invoke();
            details.WaitForStatus("Not installed", TimeSpan.FromSeconds(30));
            Assert.False(File.Exists(markerPath));
        });
    }

    [Fact]
    [Trait("E2EPhase", "Healthy")]
    public void ActiveOperationStartedFromCatalogStaysVisibleOnDetails()
    {
        RunWithDiagnostics(nameof(ActiveOperationStartedFromCatalogStaysVisibleOnDetails), session =>
        {
            var slowMarkerPath = Path.Combine(session.WorkRoot, "marker-slow-install.txt");
            if (File.Exists(slowMarkerPath))
            {
                File.Delete(slowMarkerPath);
            }

            var home = new HomePageDriver(session);
            EnsureSlowFixtureAbsent(session, home, slowMarkerPath);
            home.PrimaryAction(SlowFixtureItemName).Invoke();
            var operationId = home.OperationId(SlowFixtureItemName);
            Assert.False(string.IsNullOrWhiteSpace(operationId));
            home.OpenDetails(SlowFixtureItemName);

            var details = new AppDetailsPageDriver(session);
            details.WaitForOperationContaining("Installing", TimeSpan.FromSeconds(30));
            Assert.Equal(operationId, details.OperationId);
            Assert.Contains("Installing", details.OperationText, StringComparison.OrdinalIgnoreCase);
            session.CaptureCheckpoint("details-active-from-catalog", includeAutomationTree: true);
            session.WaitUntil(() => File.Exists(slowMarkerPath), TimeSpan.FromSeconds(30));
            details.WaitForStatus("Installed", TimeSpan.FromSeconds(30));
        });
    }

    [Fact]
    [Trait("E2EPhase", "Healthy")]
    public void ActiveOperationStartedFromDetailsStaysVisibleOnCatalog()
    {
        RunWithDiagnostics(nameof(ActiveOperationStartedFromDetailsStaysVisibleOnCatalog), session =>
        {
            var slowMarkerPath = Path.Combine(session.WorkRoot, "marker-slow-install.txt");
            if (File.Exists(slowMarkerPath))
            {
                File.Delete(slowMarkerPath);
            }

            var home = new HomePageDriver(session);
            EnsureSlowFixtureAbsent(session, home, slowMarkerPath);
            home.OpenDetails(SlowFixtureItemName);
            var details = new AppDetailsPageDriver(session);
            details.PrimaryAction.Invoke();
            details.WaitForOperationContaining("Installing", TimeSpan.FromSeconds(30));
            var operationId = details.OperationId;
            Assert.False(string.IsNullOrWhiteSpace(operationId));
            session.CaptureCheckpoint("details-active-started-details", includeAutomationTree: true);
            details.GoBack();

            home = new HomePageDriver(session);
            home.WaitForOperationContaining(SlowFixtureItemName, "Installing", TimeSpan.FromSeconds(30));
            Assert.Equal(operationId, home.OperationId(SlowFixtureItemName));
            session.CaptureCheckpoint("catalog-active-from-details");
            session.WaitUntil(() => File.Exists(slowMarkerPath), TimeSpan.FromSeconds(30));
            home.WaitForItemStatus(SlowFixtureItemName, "Installed", TimeSpan.FromSeconds(30));
        });
    }

    [Fact]
    [Trait("E2EPhase", "Healthy")]
    public void DetailsShowsRetainedStructuredFailureWithoutGlobalOperationWarning()
    {
        RunWithDiagnostics(nameof(DetailsShowsRetainedStructuredFailureWithoutGlobalOperationWarning), session =>
        {
            var home = new HomePageDriver(session);
            _ = home.WaitForItem(FailureFixtureItemName);
            home.OpenDetails(FailureFixtureItemName);

            var details = new AppDetailsPageDriver(session);
            details.PrimaryAction.Invoke();
            details.WaitForLatestResult("Installation error: exit status 7", TimeSpan.FromSeconds(60));
            Assert.Equal("Installation failed", details.LatestResultHeading);
            Assert.Contains("Installation error: exit status 7", details.LatestResult, StringComparison.OrdinalIgnoreCase);
            session.CaptureCheckpoint("details-retained-failure", includeAutomationTree: true);
            details.GoBack();

            home = new HomePageDriver(session);
            _ = home.WaitForItem(FailureFixtureItemName);
            Assert.DoesNotContain("Operation failed", home.WarningText, StringComparison.OrdinalIgnoreCase);
            home.OpenDetails(FailureFixtureItemName);

            details = new AppDetailsPageDriver(session);
            details.WaitForLatestResult("Installation error: exit status 7", TimeSpan.FromSeconds(30));
            Assert.Equal("Installation failed", details.LatestResultHeading);
            Assert.Contains("Installation error: exit status 7", details.LatestResult, StringComparison.OrdinalIgnoreCase);
            session.CaptureCheckpoint("details-retained-failure-reopened");
        });
    }

    private static void EnsureSlowFixtureAbsent(GorillaAppSession session, HomePageDriver home, string slowMarkerPath)
    {
        _ = home.WaitForItem(SlowFixtureItemName);
        if (string.Equals(home.ItemStatus(SlowFixtureItemName), "Installed", StringComparison.OrdinalIgnoreCase))
        {
            home.SecondaryAction(SlowFixtureItemName).Invoke();
            home.WaitForItemStatus(SlowFixtureItemName, "Not installed", TimeSpan.FromSeconds(30));
        }
        if (File.Exists(slowMarkerPath))
        {
            File.Delete(slowMarkerPath);
        }
    }

    private static void RunWithDiagnostics(string testName, Action<GorillaAppSession> test)
    {
        using var session = GorillaAppSession.Start();
        try
        {
            test(session);
        }
        catch (Exception ex)
        {
            session.CaptureFailure(testName, ex);
            throw;
        }
    }
}
