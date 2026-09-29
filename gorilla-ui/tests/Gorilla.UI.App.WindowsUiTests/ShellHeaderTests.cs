using FlaUI.Core.AutomationElements;
using Xunit;

namespace Gorilla.UI.App.WindowsUiTests;

public sealed class ShellHeaderTests
{
    private const string FixtureItemName = "Ps1V1";

    [Fact]
    [Trait("E2EPhase", "Healthy")]
    public void SharedShellHeaderSwitchesContextAndKeepsRefreshPersistent()
    {
        RunWithDiagnostics(nameof(SharedShellHeaderSwitchesContextAndKeepsRefreshPersistent), session =>
        {
            _ = session.WaitFor(() => ById(session, "CatalogSearchBox"));
            AssertPresent(session, "HomeHeading");
            AssertPresent(session, "ActivityNavigationButton");
            AssertAbsent(session, "DetailsBackButton");
            AssertAbsent(session, "ActivityBackButton");
            AssertAbsent(session, "ActivityHeading");
            AssertRefresh(session);

            var home = new HomePageDriver(session);
            home.OpenDetails(FixtureItemName);
            _ = session.WaitFor(() => ById(session, "AppDetailsRoot"));
            AssertPresent(session, "DetailsBackButton");
            AssertAbsent(session, "HomeHeading");
            AssertAbsent(session, "ActivityNavigationButton");
            AssertAbsent(session, "ActivityBackButton");
            AssertAbsent(session, "ActivityHeading");
            AssertRefresh(session);

            session.WaitFor(() => ById(session, "DetailsBackButton")?.AsButton()).Invoke();
            _ = session.WaitFor(() => ById(session, "CatalogSearchBox"));

            session.WaitFor(() => ById(session, "ActivityNavigationButton")?.AsButton()).Invoke();
            _ = session.WaitFor(() => ById(session, "ActivityPageRoot"));
            AssertPresent(session, "ActivityBackButton");
            AssertPresent(session, "ActivityHeading");
            AssertAbsent(session, "HomeHeading");
            AssertAbsent(session, "ActivityNavigationButton");
            AssertAbsent(session, "DetailsBackButton");
            AssertRefresh(session);
        });
    }

    private static void AssertRefresh(GorillaAppSession session)
    {
        var refresh = session.WaitFor(() => ById(session, "CatalogRefreshButton")?.AsButton());
        Assert.Equal("Refresh App Catalog", refresh.Name);
    }

    private static void AssertPresent(GorillaAppSession session, string automationId)
        => Assert.NotNull(ById(session, automationId));

    private static void AssertAbsent(GorillaAppSession session, string automationId)
    {
        session.WaitUntil(() => ById(session, automationId) is null, TimeSpan.FromSeconds(5));
        Assert.Null(ById(session, automationId));
    }

    private static AutomationElement? ById(GorillaAppSession session, string automationId)
        => session.MainWindow.FindFirstDescendant(cf => cf.ByAutomationId(automationId));

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
