using FlaUI.Core.AutomationElements;
using FlaUI.Core.Definitions;
using FlaUI.Core.Exceptions;
using FlaUI.Core.Input;
using FlaUI.Core.WindowsAPI;
using Xunit;

namespace Gorilla.UI.App.WindowsUiTests;

public sealed class KeyboardTraversalTests
{
    [Fact]
    [Trait("E2EPhase", "Healthy")]
    public void TabTraversalReachesShellSearchCardAndEmbeddedActionInOrder()
    {
        RunWithDiagnostics(nameof(TabTraversalReachesShellSearchCardAndEmbeddedActionInOrder), session =>
        {
            var home = new HomePageDriver(session);
            _ = home.CatalogItems;
            var refresh = session.WaitFor(() => ById(session, "CatalogRefreshButton"));
            session.FocusForKeyboard(refresh);

            Keyboard.Type(VirtualKeyShort.TAB);
            var activity = WaitForFocused(session, "ActivityNavigationButton");
            Assert.Equal(ControlType.Button, activity.ControlType);

            Keyboard.Type(VirtualKeyShort.TAB);
            var search = WaitForFocused(session, "CatalogSearchBox");
            Assert.Equal(ControlType.Edit, search.ControlType);

            Keyboard.Type(VirtualKeyShort.TAB);
            var card = session.WaitFor(() =>
            {
                var focused = session.FocusedElement();
                return focused.ControlType == ControlType.ListItem &&
                    !string.IsNullOrWhiteSpace(SafeAutomationId(focused))
                        ? focused
                        : null;
            }, TimeSpan.FromSeconds(5));
            var itemName = SafeAutomationId(card);
            Assert.NotNull(home.CatalogItems.FindFirstDescendant(cf => cf.ByAutomationId(itemName)));

            Keyboard.Type(VirtualKeyShort.TAB);
            var action = WaitForFocused(session, "PrimaryActionButton");
            Assert.Equal(ControlType.Button, action.ControlType);

            // Repeated child IDs are intentionally scoped by their stable ItemName
            // container. Prove the actual focused action is descended from the exact
            // card reached by the immediately preceding Tab.
            var actionCard = FindListItemAncestor(action);
            Assert.NotNull(actionCard);
            Assert.Equal(itemName, SafeAutomationId(actionCard!));
        });
    }

    private static AutomationElement? FindListItemAncestor(AutomationElement element)
    {
        var current = element.Parent;
        while (current is not null)
        {
            if (current.ControlType == ControlType.ListItem)
            {
                return current;
            }
            current = current.Parent;
        }
        return null;
    }

    private static AutomationElement WaitForFocused(GorillaAppSession session, string automationId)
        => session.WaitFor(() =>
        {
            var focused = session.FocusedElement();
            return string.Equals(SafeAutomationId(focused), automationId, StringComparison.Ordinal)
                ? focused
                : null;
        }, TimeSpan.FromSeconds(5));

    private static string SafeAutomationId(AutomationElement element)
    {
        try
        {
            return element.AutomationId;
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
