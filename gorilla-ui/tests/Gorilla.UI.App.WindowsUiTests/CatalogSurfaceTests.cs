using FlaUI.Core.Definitions;
using Microsoft.Win32;
using Xunit;

namespace Gorilla.UI.App.WindowsUiTests;

public sealed class CatalogSurfaceTests
{
    private const string FixtureItemName = "Ps1V1";
    private const string FailureFixtureItemName = "Ps1Failure";
    private const string UpdateFixtureItemName = "RegistryUpdateFixture";
    private const string InstalledFixtureItemName = "RegistryInstalledFixture";
    private const string SlowFixtureItemName = "SlowInstallFixture";
    private const string UpdateRegistrySubKey = @"SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\GorillaUiUpdateFixture";
    private const string InstalledRegistrySubKey = @"SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\GorillaUiInstalledFixture";

    [Fact]
    [Trait("E2EPhase", "Healthy")]
    public void CatalogRendersCardsAndMissingDescriptionWithoutPlaceholder()
    {
        RunWithDiagnostics(nameof(CatalogRendersCardsAndMissingDescriptionWithoutPlaceholder), session =>
        {
            var home = new HomePageDriver(session);

            _ = home.WaitForCard(FixtureItemName);
            _ = home.WaitForCard(FailureFixtureItemName);
            _ = home.WaitForCard(InstalledFixtureItemName);
            Assert.Equal("Not installed", home.ItemStatus(FixtureItemName));
            Assert.Null(home.Description(FailureFixtureItemName));
            Assert.Contains("Celestial amber telescope", home.Description(InstalledFixtureItemName), StringComparison.OrdinalIgnoreCase);
            Assert.DoesNotContain(
                session.MainWindow.FindAllDescendants(cf => cf.ByControlType(ControlType.Text)),
                element => string.Equals(
                    HomePageDriver.AutomationName(element),
                    "No description available",
                    StringComparison.OrdinalIgnoreCase
                )
            );
            session.CaptureCheckpoint("catalog-cards", includeAutomationTree: true);
        });
    }

    [Fact]
    [Trait("E2EPhase", "Healthy")]
    public void SearchByNameFiltersAndClearingRestoresCatalog()
    {
        RunWithDiagnostics(nameof(SearchByNameFiltersAndClearingRestoresCatalog), session =>
        {
            var home = new HomePageDriver(session);
            _ = home.WaitForItem(FixtureItemName);
            _ = home.WaitForItem(FailureFixtureItemName);

            home.Search(FailureFixtureItemName);
            session.WaitUntil(() => home.HasItem(FailureFixtureItemName) && !home.HasItem(FixtureItemName));
            session.CaptureCheckpoint("catalog-search-name", includeAutomationTree: true);

            home.ClearSearch();
            session.WaitUntil(() => home.HasItem(FixtureItemName) && home.HasItem(FailureFixtureItemName));
        });
    }

    [Fact]
    [Trait("E2EPhase", "Healthy")]
    public void SearchByDescriptionFlowsFromCatalogThroughServiceAndCore()
    {
        RunWithDiagnostics(nameof(SearchByDescriptionFlowsFromCatalogThroughServiceAndCore), session =>
        {
            var home = new HomePageDriver(session);
            _ = home.WaitForItem(InstalledFixtureItemName);
            _ = home.WaitForItem(UpdateFixtureItemName);

            const string descriptionOnlyQuery = "celestial amber telescope";
            home.Search(descriptionOnlyQuery);

            session.WaitUntil(() => home.HasItem(InstalledFixtureItemName) && !home.HasItem(UpdateFixtureItemName));
            Assert.Contains(
                descriptionOnlyQuery,
                home.Description(InstalledFixtureItemName),
                StringComparison.OrdinalIgnoreCase
            );
            session.CaptureCheckpoint("catalog-search-description", includeAutomationTree: true);

            home.ClearSearch();
            _ = home.WaitForItem(UpdateFixtureItemName);
        });
    }

    [Fact]
    [Trait("E2EPhase", "Healthy")]
    public void SearchNoResultsUsesQuerySpecificStateAndClearingRestoresCatalog()
    {
        RunWithDiagnostics(nameof(SearchNoResultsUsesQuerySpecificStateAndClearingRestoresCatalog), session =>
        {
            var home = new HomePageDriver(session);
            _ = home.WaitForItem(FixtureItemName);

            const string query = "definitely-no-such-gorilla-app";
            home.Search(query);
            var noResults = home.WaitForSearchNoResults();
            Assert.Contains(query, noResults.Name, StringComparison.OrdinalIgnoreCase);
            session.CaptureCheckpoint("catalog-search-no-results", includeAutomationTree: true);

            home.ClearSearch();
            _ = home.WaitForItem(FixtureItemName);
        });
    }

    [Fact]
    [Trait("E2EPhase", "Healthy")]
    public void InstalledSelectedFixtureShowsOnePrimaryRemoveAction()
    {
        RunWithDiagnostics(nameof(InstalledSelectedFixtureShowsOnePrimaryRemoveAction), session =>
        {
            var slowMarkerPath = RequiredPath("GORILLA_UI_E2E_SLOW_MARKER_PATH");
            var home = new HomePageDriver(session);
            _ = home.WaitForItem(SlowFixtureItemName);

            if (!string.Equals(home.ItemStatus(SlowFixtureItemName), "Installed", StringComparison.OrdinalIgnoreCase))
            {
                home.InstallButton(SlowFixtureItemName).Invoke();
                session.WaitUntil(() => File.Exists(slowMarkerPath), TimeSpan.FromSeconds(30));
                home.WaitForItemStatus(SlowFixtureItemName, "Installed", TimeSpan.FromSeconds(30));
            }

            var card = home.WaitForCard(SlowFixtureItemName);
            var primary = card.FindFirstDescendant(cf => cf.ByAutomationId("PrimaryActionButton"));
            var secondary = card.FindFirstDescendant(cf => cf.ByAutomationId("SecondaryActionButton"));

            Assert.NotNull(primary);
            Assert.Equal("Remove", HomePageDriver.AutomationName(primary!));
            Assert.Null(secondary);
        });
    }

    [Fact]
    [Trait("E2EPhase", "Healthy")]
    public void UpdateAvailableFixtureShowsUpdatePrimaryAndRemoveSecondary()
    {
        SeedRegistryFixture(UpdateRegistrySubKey, "Gorilla UI Update Fixture", "1.0.0");

        RunWithDiagnostics(nameof(UpdateAvailableFixtureShowsUpdatePrimaryAndRemoveSecondary), session =>
        {
            var home = new HomePageDriver(session);
            home.WaitForItemStatus(UpdateFixtureItemName, "Update available", TimeSpan.FromSeconds(30));

            var primary = home.PrimaryActionButton(UpdateFixtureItemName);
            var secondary = home.SecondaryActionButton(UpdateFixtureItemName);

            Assert.Equal("Update", primary.Name);
            Assert.True(primary.IsEnabled);
            Assert.Equal("Remove", secondary.Name);
            Assert.True(secondary.IsEnabled);
            session.CaptureCheckpoint("catalog-update-dual-action", includeAutomationTree: true);
        });
    }

    [Fact]
    [Trait("E2EPhase", "Healthy")]
    public void InstalledUnselectedFixtureShowsKeepInstalledPrimaryAndRemoveSecondary()
    {
        SeedRegistryFixture(InstalledRegistrySubKey, "Gorilla UI Installed Fixture", "1.0.0");

        RunWithDiagnostics(nameof(InstalledUnselectedFixtureShowsKeepInstalledPrimaryAndRemoveSecondary), session =>
        {
            var home = new HomePageDriver(session);
            home.WaitForItemStatus(InstalledFixtureItemName, "Installed", TimeSpan.FromSeconds(30));

            var primary = home.PrimaryActionButton(InstalledFixtureItemName);
            var secondary = home.SecondaryActionButton(InstalledFixtureItemName);

            Assert.Equal("Keep Installed", primary.Name);
            Assert.True(primary.IsEnabled);
            Assert.Equal("Remove", secondary.Name);
            Assert.True(secondary.IsEnabled);
            session.CaptureCheckpoint("catalog-installed-unselected-dual-action", includeAutomationTree: true);
        });
    }

    [Fact]
    [Trait("E2EPhase", "Healthy")]
    public void ActiveOperationShowsBusyStateWithoutReplacingObservation()
    {
        RunWithDiagnostics(nameof(ActiveOperationShowsBusyStateWithoutReplacingObservation), session =>
        {
            var slowMarkerPath = RequiredPath("GORILLA_UI_E2E_SLOW_MARKER_PATH");
            File.Delete(slowMarkerPath);

            var home = new HomePageDriver(session);
            home.WaitForItemStatus(SlowFixtureItemName, "Not installed", TimeSpan.FromSeconds(30));

            home.PrimaryActionButton(SlowFixtureItemName).Invoke();

            home.WaitForOperationContaining(SlowFixtureItemName, "Installing", TimeSpan.FromSeconds(30));
            Assert.Equal("Not installed", home.ItemStatus(SlowFixtureItemName));
            Assert.False(home.PrimaryActionButton(SlowFixtureItemName).IsEnabled);
            session.CaptureCheckpoint("catalog-active-operation", includeAutomationTree: true);

            session.WaitUntil(() => File.Exists(slowMarkerPath), TimeSpan.FromSeconds(30));
            home.WaitForItemStatus(SlowFixtureItemName, "Installed", TimeSpan.FromSeconds(30));
            Assert.True(home.HasSecondaryAction(SlowFixtureItemName) || home.PrimaryActionButton(SlowFixtureItemName).Name == "Remove");
        });
    }

    private static void SeedRegistryFixture(string subKey, string displayName, string displayVersion)
    {
        using var baseKey = RegistryKey.OpenBaseKey(RegistryHive.LocalMachine, RegistryView.Registry64);
        using var key = baseKey.CreateSubKey(subKey, writable: true)
            ?? throw new InvalidOperationException($"Unable to create registry fixture HKLM\\{subKey}.");
        key.SetValue("DisplayName", displayName, RegistryValueKind.String);
        key.SetValue("DisplayVersion", displayVersion, RegistryValueKind.String);
        key.SetValue("UninstallString", "cmd.exe /c exit 0", RegistryValueKind.String);
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
