using FlaUI.Core.AutomationElements;
using FlaUI.Core.Exceptions;
using FlaUI.Core.Input;
using FlaUI.Core.WindowsAPI;
using Xunit;

namespace Gorilla.UI.App.WindowsUiTests;

public sealed class GridKeyboardSemanticsTests
{
    private static readonly string[] FixtureItemNames =
    {
        "Ps1V1",
        "Ps1Failure",
        "RegistryUpdateFixture",
        "RegistryInstalledFixture",
        "SlowInstallFixture"
    };

    [Fact]
    [Trait("E2EPhase", "Healthy")]
    public void SpaceOnFocusedCatalogItemOpensDetails()
    {
        RunWithDiagnostics(nameof(SpaceOnFocusedCatalogItemOpensDetails), session =>
        {
            var home = new HomePageDriver(session);
            var item = home.WaitForItem("Ps1V1");
            item.Focus();
            Keyboard.Type(VirtualKeyShort.SPACE);
            _ = session.WaitFor(() => ById(session, "AppDetailsRoot"));
        });
    }

    [Fact]
    [Trait("E2EPhase", "Healthy")]
    public void ArrowNavigationUsesGridViewSemanticsAndActivationTargetsFocusedItem()
    {
        RunWithDiagnostics(nameof(ArrowNavigationUsesGridViewSemanticsAndActivationTargetsFocusedItem), session =>
        {
            var home = new HomePageDriver(session);
            var first = home.WaitForItem(FixtureItemNames[0]);
            first.Focus();

            Keyboard.Type(VirtualKeyShort.RIGHT);
            Thread.Sleep(150);

            var focused = FixtureItemNames
                .Skip(1)
                .Select(name => TryFindItem(home, name))
                .FirstOrDefault(element => element is not null && HasKeyboardFocus(element));
            Assert.NotNull(focused);
            var expectedName = SafeName(focused!);
            Assert.False(string.IsNullOrWhiteSpace(expectedName));
            var expectedItemName = focused!.AutomationId;
            Assert.Contains(expectedItemName, FixtureItemNames);

            Keyboard.Type(VirtualKeyShort.ENTER);
            _ = session.WaitFor(() => ById(session, "AppDetailsRoot"));
            var detailsName = session.WaitFor(() =>
            {
                var element = ById(session, "DetailsDisplayName");
                return element is not null && string.Equals(SafeName(element), expectedName, StringComparison.Ordinal)
                    ? element
                    : null;
            });
            Assert.Equal(expectedName, SafeName(detailsName));
        });
    }

    private static AutomationElement? TryFindItem(HomePageDriver home, string itemName)
    {
        try
        {
            return home.WaitForItem(itemName);
        }
        catch (TimeoutException)
        {
            return null;
        }
    }

    private static bool HasKeyboardFocus(AutomationElement element)
    {
        try
        {
            return element.Properties.HasKeyboardFocus.ValueOrDefault;
        }
        catch
        {
            return false;
        }
    }

    private static string SafeName(AutomationElement element)
    {
        try
        {
            return element.Name;
        }
        catch (PropertyNotSupportedException)
        {
            return string.Empty;
        }
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
