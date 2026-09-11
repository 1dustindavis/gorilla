using FlaUI.Core.Definitions;
using Xunit;

namespace Gorilla.UI.App.WindowsUiTests;

public sealed class CatalogSurfaceTests
{
    private const string FixtureItemName = "Ps1V1";
    private const string FailureFixtureItemName = "Ps1Failure";

    [Fact]
    [Trait("E2EPhase", "Healthy")]
    public void CatalogRendersCardsAndMissingDescriptionWithoutPlaceholder()
    {
        RunWithDiagnostics(nameof(CatalogRendersCardsAndMissingDescriptionWithoutPlaceholder), session =>
        {
            var home = new HomePageDriver(session);

            _ = home.WaitForCard(FixtureItemName);
            _ = home.WaitForCard(FailureFixtureItemName);
            Assert.Equal("Not installed", home.ItemStatus(FixtureItemName));
            Assert.Null(home.Description(FailureFixtureItemName));
            Assert.DoesNotContain(
                session.MainWindow.FindAllDescendants(cf => cf.ByControlType(ControlType.Text)),
                element => string.Equals(element.Name, "No description available", StringComparison.OrdinalIgnoreCase)
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
    public void AbsentFixtureShowsOnePrimaryInstallAction()
    {
        RunWithDiagnostics(nameof(AbsentFixtureShowsOnePrimaryInstallAction), session =>
        {
            var home = new HomePageDriver(session);
            var card = home.WaitForCard(FailureFixtureItemName);
            var primary = card.FindFirstDescendant(cf => cf.ByAutomationId("PrimaryActionButton"));
            var secondary = card.FindFirstDescendant(cf => cf.ByAutomationId("SecondaryActionButton"));

            Assert.NotNull(primary);
            Assert.Equal("Install", primary!.Name);
            Assert.Null(secondary);
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
