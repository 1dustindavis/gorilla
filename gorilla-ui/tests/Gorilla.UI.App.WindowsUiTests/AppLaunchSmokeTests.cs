using Xunit;

[assembly: CollectionBehavior(DisableTestParallelization = true)]

namespace Gorilla.UI.App.WindowsUiTests;

public sealed class AppLaunchSmokeTests
{
    [Fact]
    public void AppLaunchesAndExposesStableHomeAutomationContract()
    {
        RunWithDiagnostics(nameof(AppLaunchesAndExposesStableHomeAutomationContract), session =>
        {
            var home = new HomePageDriver(session);
            Assert.Equal("App Catalog", session.MainWindow.Title);
            Assert.Equal("Available Software", home.Heading.Name);
            Assert.Equal("Available software", home.ItemsList.Name);
            Assert.False(home.HasOperationFailureText());
        });
    }

    [Fact]
    public void AppLaunchesAndRemainsRunningAfterStartup()
    {
        RunWithDiagnostics(nameof(AppLaunchesAndRemainsRunningAfterStartup), session =>
        {
            session.AssertStillRunning(TimeSpan.FromSeconds(5));
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
