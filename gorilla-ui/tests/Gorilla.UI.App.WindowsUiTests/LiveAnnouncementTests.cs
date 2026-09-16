using Xunit;

namespace Gorilla.UI.App.WindowsUiTests;

public sealed class LiveAnnouncementTests
{
    private const string FailureFixtureItemName = "Ps1Failure";

    [Fact]
    [Trait("E2EPhase", "Healthy")]
    public void DetailsTerminalFailureAnnouncementContainsTitleAndExplanation()
    {
        RunWithDiagnostics(nameof(DetailsTerminalFailureAnnouncementContainsTitleAndExplanation), session =>
        {
            var home = new HomePageDriver(session);
            _ = home.WaitForItem(FailureFixtureItemName);
            home.OpenDetails(FailureFixtureItemName);

            var details = new AppDetailsPageDriver(session);
            details.PrimaryAction.Invoke();
            details.WaitForLatestResult("Installation error: exit status 7", TimeSpan.FromSeconds(60));

            Assert.Equal(
                "Installation failed. Installation error: exit status 7",
                details.LatestFailureAnnouncement
            );
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
