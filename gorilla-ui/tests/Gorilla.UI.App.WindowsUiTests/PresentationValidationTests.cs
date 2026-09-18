using FlaUI.Core.AutomationElements;
using Xunit;

namespace Gorilla.UI.App.WindowsUiTests;

public sealed class PresentationValidationTests
{
    private const string FixtureItemName = "Ps1V1";
    private const string FailureFixtureItemName = "Ps1Failure";

    [Fact]
    [Trait("E2EPhase", "Healthy")]
    public void RepresentativeCatalogAndDetailsSurfacesRemainVisibleAndReachable()
    {
        RunWithDiagnostics(nameof(RepresentativeCatalogAndDetailsSurfacesRemainVisibleAndReachable), session =>
        {
            var home = new HomePageDriver(session);

            AssertVisible(home.Heading, "Catalog heading");
            AssertVisible(home.SearchBox, "Catalog search");
            AssertVisible(ById(session, "ActivityNavigationButton"), "Activity navigation");
            AssertVisible(ById(session, "CatalogRefreshButton"), "Refresh action");

            home.EnsureItemVisible(FixtureItemName);
            AssertVisible(home.WaitForItem(FixtureItemName), "Catalog item");
            AssertVisible(home.PrimaryActionButton(FixtureItemName), "Catalog primary action");
            session.CaptureCheckpoint("stage7-presentation-catalog", includeAutomationTree: true);

            home.OpenDetails(FailureFixtureItemName);
            var details = new AppDetailsPageDriver(session);

            AssertVisible(details.Root, "Details root");
            AssertVisible(details.BackButton, "Details back action");
            AssertVisible(details.DisplayName, "Details heading");

            var primaryAction = details.PrimaryAction;
            primaryAction.Patterns.ScrollItem.PatternOrDefault?.ScrollIntoView();
            primaryAction = details.PrimaryAction;
            AssertVisible(primaryAction, "Details primary action");
            primaryAction.Invoke();
            details.WaitForLatestResult("Installation error: exit status 7", TimeSpan.FromSeconds(60));

            var latestResult = ById(session, "DetailsLatestResult");
            latestResult.Patterns.ScrollItem.PatternOrDefault?.ScrollIntoView();
            latestResult = ById(session, "DetailsLatestResult");
            AssertVisible(latestResult, "Details failure result");

            session.CaptureCheckpoint("stage7-presentation-details-failure", includeAutomationTree: true);
        });
    }

    private static AutomationElement ById(GorillaAppSession session, string automationId)
        => session.WaitFor(() => session.MainWindow.FindFirstDescendant(cf => cf.ByAutomationId(automationId)));

    private static void AssertVisible(AutomationElement element, string description)
    {
        Assert.False(element.IsOffscreen, $"{description} should be on-screen.");

        var bounds = element.BoundingRectangle;
        Assert.True(
            bounds.Width > 0 && bounds.Height > 0,
            $"{description} should have a non-empty UIA bounding rectangle; actual={bounds}."
        );
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
