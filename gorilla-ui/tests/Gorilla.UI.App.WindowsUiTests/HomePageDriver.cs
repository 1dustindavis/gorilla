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
    public AutomationElement ItemsList => _session.WaitFor(() => ById("ItemsList"));
    public AutomationElement ServiceWarning => _session.WaitFor(() => ById("ServiceWarning"));
    public string WarningText => SafeName(ServiceWarning);

    public AutomationElement WaitForItem(string itemName)
    {
        return _session.WaitFor(() => ItemsList.FindFirstDescendant(cf => cf.ByAutomationId(itemName)));
    }

    public Button InstallButton(string itemName) => ButtonFor(itemName, "InstallButton");
    public Button RemoveButton(string itemName) => ButtonFor(itemName, "RemoveButton");

    public string ItemStatus(string itemName)
    {
        var item = WaitForItem(itemName);
        var status = _session.WaitFor(() => item.FindFirstDescendant(cf => cf.ByAutomationId("ItemStatus")));
        return SafeName(status);
    }

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

    private Button ButtonFor(string itemName, string automationId)
    {
        var item = WaitForItem(itemName);
        return _session.WaitFor(() => item.FindFirstDescendant(cf => cf.ByAutomationId(automationId))?.AsButton());
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
