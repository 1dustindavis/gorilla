using System.Security.Cryptography;
using System.Text;
using Xunit;

namespace Gorilla.UI.App.WindowsUiTests;

public sealed class CriticalPathTests
{
    private const string FixtureItemName = "Ps1V1";
    private const string FailureFixtureItemName = "Ps1Fail";

    [Fact]
    public void StartupShowsRealCatalogAndDetails()
    {
        RunWithDiagnostics(nameof(StartupShowsRealCatalogAndDetails), session =>
        {
            var home = new HomePageDriver(session);
            Assert.Equal("Available Software", home.Heading.Name);
            _ = home.WaitForItem(FixtureItemName);
            session.CaptureCheckpoint("healthy-startup", includeAutomationTree: true);

            home.OpenDetails(FixtureItemName);
            var details = new AppDetailsPageDriver(session);
            Assert.Equal(FixtureItemName, details.Root.Name);
            Assert.Contains("PowerShell", details.DescriptionText, StringComparison.OrdinalIgnoreCase);
            session.CaptureCheckpoint("details-installed-description", includeAutomationTree: true);
        });
    }

    [Fact]
    public void CatalogCardsExposeRealDescriptionsAndSearch()
    {
        RunWithDiagnostics(nameof(CatalogCardsExposeRealDescriptionsAndSearch), session =>
        {
            var home = new HomePageDriver(session);
            _ = home.WaitForItem(FixtureItemName);
            Assert.True(home.HasDescriptionElement(FixtureItemName));
            session.CaptureCheckpoint("catalog-cards", includeAutomationTree: true);

            home.Search("PowerShell");
            _ = home.WaitForItem(FixtureItemName);
            session.CaptureCheckpoint("catalog-search-description", includeAutomationTree: true);

            home.Search(FixtureItemName);
            _ = home.WaitForItem(FixtureItemName);
            session.CaptureCheckpoint("catalog-search-name", includeAutomationTree: true);

            home.Search("does-not-exist");
            _ = home.WaitForSearchNoResults();
            session.CaptureCheckpoint("catalog-search-no-results", includeAutomationTree: true);
        });
    }

    [Fact]
    public void ManualRefreshShowsTruthfulFreshnessAndCompletes()
    {
        RunWithDiagnostics(nameof(ManualRefreshShowsTruthfulFreshnessAndCompletes), session =>
        {
            var shell = new CatalogShellDriver(session);
            shell.WaitForRefreshComplete(TimeSpan.FromSeconds(30));
            shell.Refresh();
            shell.WaitForRefreshStarted(TimeSpan.FromSeconds(15));
            shell.WaitForRefreshComplete(TimeSpan.FromSeconds(30));
            shell.WaitForFreshnessContaining("Updated", TimeSpan.FromSeconds(15));
            session.CaptureCheckpoint("manual-refresh", includeAutomationTree: true);
        });
    }

    [Fact]
    public void InstallOperationRemainsSameIdentityAcrossCatalogDetailsAndActivity()
    {
        RunWithDiagnostics(nameof(InstallOperationRemainsSameIdentityAcrossCatalogDetailsAndActivity), session =>
        {
            var home = new HomePageDriver(session);
            var shell = new CatalogShellDriver(session);
            var markerPath = RequiredPath("GORILLA_UI_E2E_MARKER_PATH");
            EnsureFixtureAbsent(session, home, markerPath);

            home.PrimaryActionButton(FixtureItemName).Invoke();
            home.WaitForOperationContaining(FixtureItemName, "Running", TimeSpan.FromSeconds(30));
            var operationId = home.OperationId(FixtureItemName);
            Assert.False(string.IsNullOrWhiteSpace(operationId));
            session.CaptureCheckpoint("catalog-active-operation", includeAutomationTree: true);

            home.OpenDetails(FixtureItemName);
            var details = new AppDetailsPageDriver(session);
            details.WaitForOperationContaining("Running", TimeSpan.FromSeconds(30));
            Assert.Equal(operationId, details.OperationId());
            session.CaptureCheckpoint("details-active-from-catalog", includeAutomationTree: true);

            details.GoBack();
            _ = home.WaitForItem(FixtureItemName);
            var activity = ActivityPageDriver.OpenFromCatalog(session);
            activity.WaitForOperationState(operationId, "Running", TimeSpan.FromSeconds(30));
            session.CaptureCheckpoint("activity-active", includeAutomationTree: true);

            activity.GoBack();
            home.WaitForItemStatus(FixtureItemName, "Installed", TimeSpan.FromSeconds(60));
            shell.WaitForFreshnessContaining("Updated", TimeSpan.FromSeconds(30));
        });
    }

    [Fact]
    public void DetailsStartedOperationRemainsSameIdentityInCatalogAndActivity()
    {
        RunWithDiagnostics(nameof(DetailsStartedOperationRemainsSameIdentityInCatalogAndActivity), session =>
        {
            var home = new HomePageDriver(session);
            var markerPath = RequiredPath("GORILLA_UI_E2E_MARKER_PATH");
            EnsureFixtureAbsent(session, home, markerPath);

            home.OpenDetails(FixtureItemName);
            var details = new AppDetailsPageDriver(session);
            details.PrimaryActionButton.Invoke();
            details.WaitForOperationContaining("Running", TimeSpan.FromSeconds(30));
            var operationId = details.OperationId();
            Assert.False(string.IsNullOrWhiteSpace(operationId));
            session.CaptureCheckpoint("details-active-started-details", includeAutomationTree: true);

            details.GoBack();
            home.WaitForOperationContaining(FixtureItemName, "Running", TimeSpan.FromSeconds(30));
            Assert.Equal(operationId, home.OperationId(FixtureItemName));
            session.CaptureCheckpoint("catalog-active-from-details", includeAutomationTree: true);

            var activity = ActivityPageDriver.OpenFromCatalog(session);
            activity.WaitForOperationState(operationId, "Running", TimeSpan.FromSeconds(30));
            session.CaptureCheckpoint("activity-active", includeAutomationTree: true);
        });
    }

    [Fact]
    public void FailureKeepsCardGeometryStableAndReportsOutcome()
    {
        RunWithDiagnostics(nameof(FailureKeepsCardGeometryStableAndReportsOutcome), session =>
        {
            var home = new HomePageDriver(session);
            var actionTopBefore = home.PrimaryActionTop(FailureFixtureItemName);
            session.CaptureCheckpoint("failure-before-install", includeAutomationTree: true);

            home.PrimaryActionButton(FailureFixtureItemName).Invoke();
            home.WaitForTerminalFeedbackContaining(FailureFixtureItemName, "exit status", TimeSpan.FromSeconds(60));
            Assert.True(home.HasOperationFailureText());
            session.CaptureCheckpoint("failure-after-install", includeAutomationTree: true);

            Assert.InRange(home.PrimaryActionTop(FailureFixtureItemName), actionTopBefore - 1.0, actionTopBefore + 1.0);
            home.WaitForItemStatus(FailureFixtureItemName, "NotInstalled", TimeSpan.FromSeconds(30));
            home.EnsureItemVisible(FailureFixtureItemName);
            session.CaptureCheckpoint("failure-after-install", includeAutomationTree: true);
        });
    }

    [Fact]
    [Trait("E2EPhase", "ServiceUnavailable")]
    public void ServiceUnavailableShowsCachedThenNoCacheFailureStatesTruthfully()
    {
        var cachePath = RequiredPath("GORILLA_UI_E2E_CACHE_PATH");

        RunWithDiagnostics(nameof(ServiceUnavailableShowsCachedThenNoCacheFailureStatesTruthfully) + "-cached", session =>
        {
            var home = new HomePageDriver(session);
            var shell = new CatalogShellDriver(session);
            Assert.Equal("Available Software", home.Heading.Name);
            _ = home.WaitForItem(FixtureItemName);
            shell.WaitForFreshnessContaining("Showing saved data", TimeSpan.FromSeconds(15));
            shell.WaitForDegradedWarningContaining("couldn't refresh the catalog", TimeSpan.FromSeconds(15));
            Assert.DoesNotContain("Updated", shell.FreshnessText, StringComparison.OrdinalIgnoreCase);
            Assert.False(shell.HasNoCachedDataState());

            var action = home.PrimaryActionButton(FixtureItemName);
            Assert.True(action.IsEnabled);
            action.Invoke();
            shell.WaitForInfrastructureWarningContaining("couldn't start that action", TimeSpan.FromSeconds(15));

            Assert.Equal(1, shell.InfrastructureWarningPresentationCount());
            Assert.DoesNotContain("Exception", shell.InfrastructureWarningText, StringComparison.OrdinalIgnoreCase);
            var technicalDetails = shell.OpenAndReadInfrastructureTechnicalDetails();
            Assert.Contains("Exception type:", technicalDetails, StringComparison.OrdinalIgnoreCase);
            Assert.Contains("Exception message:", technicalDetails, StringComparison.OrdinalIgnoreCase);
            Assert.Contains(FixtureItemName, technicalDetails, StringComparison.OrdinalIgnoreCase);
            Assert.False(string.IsNullOrWhiteSpace(shell.DegradedWarningText));

            // Technical details are an inspection surface, not persistent navigation
            // state. Collapse them before verifying that the warning itself survives
            // Catalog -> Details -> Activity, so the small CI window still leaves a
            // usable catalog viewport for the navigation gesture under test.
            shell.CollapseInfrastructureTechnicalDetails();

            home.OpenDetails(FixtureItemName);
            _ = new AppDetailsPageDriver(session).Root;
            Assert.Contains("couldn't start that action", shell.InfrastructureWarningText, StringComparison.OrdinalIgnoreCase);
            Assert.Equal(1, shell.InfrastructureWarningPresentationCount());

            new AppDetailsPageDriver(session).GoBack();
            _ = home.WaitForItem(FixtureItemName);
            _ = ActivityPageDriver.OpenFromCatalog(session).Root;
            Assert.Contains("couldn't start that action", shell.InfrastructureWarningText, StringComparison.OrdinalIgnoreCase);
            Assert.Equal(1, shell.InfrastructureWarningPresentationCount());

            session.CaptureCheckpoint("cached-service-unavailable-infrastructure-warning", includeAutomationTree: true);
        });

        File.Delete(cachePath);

        RunWithDiagnostics(nameof(ServiceUnavailableShowsCachedThenNoCacheFailureStatesTruthfully) + "-no-cache", session =>
        {
            var shell = new CatalogShellDriver(session);
            var home = new HomePageDriver(session);

            shell.WaitForRefreshComplete(TimeSpan.FromSeconds(30));
            Assert.True(home.Heading.Name.Length > 0);
            _ = session.WaitFor(() => session.MainWindow.FindFirstDescendant(cf => cf.ByAutomationId("CatalogLoadFailed")));
            Assert.True(shell.HasLoadFailedState());
            Assert.True(shell.HasNoCachedDataState());
            Assert.False(shell.HasSuccessfulEmptyState());
            shell.WaitForDegradedWarningContaining("no saved catalog", TimeSpan.FromSeconds(15));
        });
    }

    private static void EnsureFixtureAbsent(GorillaAppSession session, HomePageDriver home, string markerPath)
    {
        if (File.Exists(markerPath))
        {
            File.Delete(markerPath);
        }

        var shell = new CatalogShellDriver(session);
        shell.Refresh();
        shell.WaitForRefreshComplete(TimeSpan.FromSeconds(30));
        home.WaitForItemStatus(FixtureItemName, "NotInstalled", TimeSpan.FromSeconds(30));
    }

    private static string RequiredPath(string variableName)
    {
        var value = Environment.GetEnvironmentVariable(variableName);
        if (string.IsNullOrWhiteSpace(value))
        {
            throw new InvalidOperationException($"Missing required environment variable {variableName}.");
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
        catch
        {
            session.CaptureFailure(testName);
            throw;
        }
    }
}
