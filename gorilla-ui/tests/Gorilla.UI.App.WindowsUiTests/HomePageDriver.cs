using FlaUI.Core.AutomationElements;
using FlaUI.Core.Definitions;
using FlaUI.Core.Exceptions;

namespace Gorilla.UI.App.WindowsUiTests;

internal sealed class HomePageDriver
{
    private readonly GorillaAppSession _session;

    public HomePageDriver(GorillaAppSession session)
    {
        _session = session;
    }

    public AutomationElement Heading => _session.WaitFor(() => ById("HomeHeading"));
    public AutomationElement CatalogItems => _session.WaitFor(() => ById("CatalogItems"));
    public AutomationElement ServiceWarning => _session.WaitFor(() => ById("ServiceWarning"));
    public TextBox SearchBox => _session.WaitFor(() => ById("CatalogSearchBox")?.AsTextBox());
    public string WarningText => SafeName(ServiceWarning);

    public AutomationElement WaitForItem(string itemName)
    {
        return _session.WaitFor(() => CatalogItems.FindFirstDescendant(cf => cf.ByAutomationId(itemName)));
    }

    public AutomationElement WaitForCard(string itemName)
    {
        var item = WaitForItem(itemName);
        return _session.WaitFor(() => item.FindFirstDescendant(cf => cf.ByAutomationId("CatalogCard")));
    }

    public Button InstallButton(string itemName) => ActionButton(itemName, "Install", "Update", "Keep Installed");
    public Button RemoveButton(string itemName) => ActionButton(itemName, "Remove");

    public string ItemStatus(string itemName)
    {
        var item = WaitForItem(itemName);
        var status = _session.WaitFor(() => item.FindFirstDescendant(cf => cf.ByAutomationId("CatalogObservation")));
        return SafeName(status);
    }

    public string? Description(string itemName)
    {
        var item = WaitForItem(itemName);
        var description = item.FindFirstDescendant(cf => cf.ByAutomationId("CatalogDescription"));
        return description is null ? null : SafeName(description);
    }

    public bool HasItem(string itemName)
        => CatalogItems.FindFirstDescendant(cf => cf.ByAutomationId(itemName)) is not null;

    public void Search(string query)
    {
        SearchBox.Text = query;
    }

    public void ClearSearch()
    {
        SearchBox.Text = string.Empty;
    }

    public AutomationElement WaitForSearchNoResults()
        => _session.WaitFor(() => ById("SearchNoResults"));

    public void WaitForItemStatus(string itemName, string expectedPrefix, TimeSpan? timeout = null)
    {
        _session.WaitUntil(
            () => ItemStatus(itemName).StartsWith(expectedPrefix, StringComparison.OrdinalIgnoreCase),
            timeout
        );
    }

    public void WaitForWarningContaining(string expected, TimeSpan? timeout = null)
    {
        _session.WaitUntil(
            () => WarningText.Contains(expected, StringComparison.OrdinalIgnoreCase),
            timeout
        );
    }

    public bool HasOperationFailureText()
    {
        return _session.MainWindow
            .FindAllDescendants(cf => cf.ByControlType(ControlType.Text))
            .Any(text => SafeName(text).StartsWith("Operation failed", StringComparison.OrdinalIgnoreCase));
    }

    private AutomationElement? ById(string automationId)
    {
        return _session.MainWindow.FindFirstDescendant(cf => cf.ByAutomationId(automationId));
    }

    private Button ActionButton(string itemName, params string[] labels)
    {
        var item = WaitForItem(itemName);
        return _session.WaitFor(() =>
        {
            foreach (var button in item.FindAllDescendants(cf => cf.ByControlType(ControlType.Button)))
            {
                var name = SafeName(button);
                if (labels.Any(label => string.Equals(name, label, StringComparison.OrdinalIgnoreCase)))
                {
                    return button.AsButton();
                }
            }
            return null;
        });
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
}
