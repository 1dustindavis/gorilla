using Microsoft.Win32;
using Xunit;

namespace Gorilla.UI.App.WindowsUiTests;

public sealed class AppDetailsTests
{
    private const string FailureFixtureItemName = "Ps1Failure";
    private const string UpdateFixtureItemName = "RegistryUpdateFixture";
    private const string InstalledFixtureItemName = "RegistryInstalledFixture";
    private const string SlowFixtureItemName = "SlowInstallFixture";
    private const string UpdateRegistrySubKey = @"SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\GorillaUiUpdateFixture";
    private const string InstalledRegistrySubKey = @"SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\GorillaUiInstalledFixture";

    [Fact]
    [Trait("E2EPhase", "Healthy")]
    public void DetailsNavigationShowsIdentityFullDescriptionAndPreservesSearchOnBack()
    {
        SeedRegistryFixture(InstalledRegistrySubKey, "Gorilla UI Installed Fixture", "1.0.0");

        RunWithDiagnostics(nameof(DetailsNavigationShowsIdentityFullDescriptionAndPreservesSearchOnBack), session =>
        {
            var home = new HomePageDriver(session);
            home.Search("celestial amber telescope");
            _ = home.WaitForItem(InstalledFixtureItemName);
            Assert.False(home.HasDescriptionElement(InstalledFixtureItemName));

            home.OpenDetails(InstalledFixtureItemName);
            var details = new AppDetailsPageDriver(session);
            _ = details.Root;

            Assert.Equal("Installed Fixture", details.Name);
            Assert.Contains("Celestial amber telescope utility", details.Description, StringComparison.OrdinalIgnoreCase);
            Assert.Equal("Installed", details.ObservationText);
            Assert.Equal("1.0.0", details.InstalledVersion);
            details.GoBack();

            var returnedHome = new HomePageDriver(session);
            _ = returnedHome.WaitForItem(InstalledFixtureItemName);
            Assert.Equal("celestial amber telescope", returnedHome.SearchBox.Text);
        });
    }

    [Fact]
    [Trait("E2EPhase", "Healthy")]
    public void DetailsShowsTruthfulVersionsAndBothContextualActions()
    {
        SeedRegistryFixture(UpdateRegistrySubKey, "Gorilla UI Update Fixture", "1.0.0");
        SeedRegistryFixture(InstalledRegistrySubKey, "Gorilla UI Installed Fixture", "1.0.0");

        RunWithDiagnostics(nameof(DetailsShowsTruthfulVersionsAndBothContextualActions), session =>
        {
            var home = new HomePageDriver(session);
            home.WaitForItemStatus(UpdateFixtureItemName, "Update available", TimeSpan.FromSeconds(30));
            home.OpenDetails(UpdateFixtureItemName);

            var updateDetails = new AppDetailsPageDriver(session);
            updateDetails.WaitForObservation("Update available");
            Assert.Equal("2.0.0", updateDetails.AvailableVersion);
            Assert.Equal("1.0.0", updateDetails.InstalledVersion);
            Assert.Equal("Update", updateDetails.PrimaryAction.Name);
            Assert.Equal("Remove", updateDetails.SecondaryAction.Name);
            updateDetails.GoBack();

            home = new HomePageDriver(session);
            home.WaitForItemStatus(InstalledFixtureItemName, "Installed", TimeSpan.FromSeconds(30));
            home.OpenDetails(InstalledFixtureItemName);

            var installedDetails = new AppDetailsPageDriver(session);
            installedDetails.WaitForObservation("Installed");
            Assert.Equal("Keep Installed", installedDetails.PrimaryAction.Name);
            Assert.Equal("Remove", installedDetails.SecondaryAction.Name);
        });
    }

    [Fact]
    [Trait("E2EPhase", "Healthy")]
    public void CatalogOperationContinuesAcrossDetailsAndBack()
    {
        RunWithDiagnostics(nameof(CatalogOperationContinuesAcrossDetailsAndBack), session =>
        {
            var slowMarkerPath = RequiredPath("GORILLA_UI_E2E_SLOW_MARKER_PATH");
            var home = new HomePageDriver(session);
            EnsureSlowFixtureAbsent(session, home, slowMarkerPath);

            home.PrimaryActionButton(SlowFixtureItemName).Invoke();
            home.WaitForOperationContaining(SlowFixtureItemName, "Installing", TimeSpan.FromSeconds(30));
            home.OpenDetails(SlowFixtureItemName);

            var details = new AppDetailsPageDriver(session);
            details.WaitForActiveOperation("Install", TimeSpan.FromSeconds(30));
            details.GoBack();

            home = new HomePageDriver(session);
            home.WaitForOperationContaining(SlowFixtureItemName, "Installing", TimeSpan.FromSeconds(30));
            session.WaitUntil(() => File.Exists(slowMarkerPath), TimeSpan.FromSeconds(30));
            home.WaitForItemStatus(SlowFixtureItemName, "Installed", TimeSpan.FromSeconds(30));
        });
    }

    [Fact]
    [Trait("E2EPhase", "Healthy")]
    public void DetailsOperationContinuesAfterNavigatingBackToCatalog()
    {
        RunWithDiagnostics(nameof(DetailsOperationContinuesAfterNavigatingBackToCatalog), session =>
        {
            var slowMarkerPath = RequiredPath("GORILLA_UI_E2E_SLOW_MARKER_PATH");
            var home = new HomePageDriver(session);
            EnsureSlowFixtureAbsent(session, home, slowMarkerPath);
            home.OpenDetails(SlowFixtureItemName);

            var details = new AppDetailsPageDriver(session);
            Assert.Equal("Install", details.PrimaryAction.Name);
            details.PrimaryAction.Invoke();
            details.WaitForActiveOperation("Install", TimeSpan.FromSeconds(30));
            details.GoBack();

            home = new HomePageDriver(session);
            home.WaitForOperationContaining(SlowFixtureItemName, "Installing", TimeSpan.FromSeconds(30));
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
            details.WaitForLatestResult("Failed", TimeSpan.FromSeconds(30));
            Assert.Contains("Failed", details.LatestResult, StringComparison.OrdinalIgnoreCase);
            details.GoBack();

            home = new HomePageDriver(session);
            _ = home.WaitForItem(FailureFixtureItemName);
            Assert.DoesNotContain("Operation failed", home.WarningText, StringComparison.OrdinalIgnoreCase);
        });
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
