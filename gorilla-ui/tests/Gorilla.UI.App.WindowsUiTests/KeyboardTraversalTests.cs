using FlaUI.Core.AutomationElements;
using FlaUI.Core.Input;
using FlaUI.Core.WindowsAPI;
using Xunit;

namespace Gorilla.UI.App.WindowsUiTests;

public sealed class KeyboardTraversalTests
{
    private const string FixtureItemName = "Ps1V1";

    [Fact]
    [Trait("E2EPhase", "Healthy")]
    public void TabTraversalReachesShellSearchCardAndEmbeddedActionInOrder()
    {
        RunWithDiagnostics(nameof(TabTraversalReachesShellSearchCardAndEmbeddedActionInOrder), session =>
        {
            var home = new HomePageDriver(session);
            _ = home.WaitForItem(FixtureItemName);
            var refresh = session.WaitFor(() => ById(session, "CatalogRefreshButton"));
            refresh.Focus();

            var visited = new List<string>();
            for (var i = 0; i < 14; i++)
            {
                Keyboard.Type(VirtualKeyShort.TAB);
                Thread.Sleep(100);
                var focused = FindFocusedElement(session);
                if (focused is not null && !string.IsNullOrWhiteSpace(focused.AutomationId))
                {
                    visited.Add(focused.AutomationId);
                }
                if (visited.Contains("PrimaryActionButton", StringComparer.Ordinal))
                {
                    break;
                }
            }

            AssertAppearsBefore(visited, "ActivityNavigationButton", "CatalogSearchBox");
            AssertAppearsBefore(visited, "CatalogSearchBox", FixtureItemName);
            AssertAppearsBefore(visited, FixtureItemName, "PrimaryActionButton");
        });
    }

    private static void AssertAppearsBefore(IReadOnlyList<string> visited, string first, string second)
    {
        var firstIndex = IndexOf(visited, first);
        var secondIndex = IndexOf(visited, second);
        Assert.True(firstIndex >= 0, $"Expected keyboard traversal to reach '{first}'. Visited: {string.Join(", ", visited)}");
        Assert.True(secondIndex >= 0, $"Expected keyboard traversal to reach '{second}'. Visited: {string.Join(", ", visited)}");
        Assert.True(firstIndex < secondIndex, $"Expected '{first}' before '{second}'. Visited: {string.Join(", ", visited)}");
    }

    private static int IndexOf(IReadOnlyList<string> values, string value)
    {
        for (var i = 0; i < values.Count; i++)
        {
            if (string.Equals(values[i], value, StringComparison.Ordinal))
            {
                return i;
            }
        }
        return -1;
    }

    private static AutomationElement? FindFocusedElement(GorillaAppSession session)
    {
        return session.MainWindow
            .FindAllDescendants()
            .FirstOrDefault(element =>
            {
                try
                {
                    return element.Properties.HasKeyboardFocus.ValueOrDefault;
                }
                catch
                {
                    return false;
                }
            });
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
