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

    public void EnsureItemVisible(string itemName)
    {
        var item = WaitForItem(itemName);
        item.Patterns.ScrollItem.PatternOrDefault?.ScrollIntoView();
        item.Focus();
        Thread.Sleep(250);
    }

    public void OpenDetails(string itemName)
    {
        EnsureItemVisible(itemName);
        WaitForItem(itemName).Click();
    }

    public Button InstallButton(string itemName) => ActionButton(itemName, "Install", "Update", "Keep Installed");
    public Button RemoveButton(string itemName) => ActionButton(itemName, "Remove");

    public Button PrimaryActionButton(string itemName)
        => ButtonByAutomationId(itemName, "PrimaryActionButton");

    public Button SecondaryActionButton(string itemName)
        => ButtonByAutomationId(itemName, "SecondaryActionButton");

    public bool HasSecondaryAction(string itemName)
    {
        var item = WaitForItem(itemName);
        return item.FindFirstDescendant(cf => cf.ByAutomationId("SecondaryActionButton")) is not null;
    }

    public string ItemStatus(string itemName)
    {
        var status = TryItemStatus(itemName);
        return status ?? string.Empty;
    }

    public string OperationText(string itemName)
    {
        var operation = FindOperation(itemName);
        return operation is null ? string.Empty : SafeName(operation);
    }

    public string OperationId(string itemName)
    {
        var operation = FindOperation(itemName);
        return operation is null ? string.Empty : SafeHelpText(operation);
    }

    public string TerminalFeedbackText(string itemName)
    {
        var catalog = ById("CatalogItems");
        var item = catalog?.FindFirstDescendant(cf => cf.ByAutomationId(itemName));
        var feedback = item?.FindFirstDescendant(cf => cf.ByAutomationId("CatalogTerminalFeedback"));
        return feedback is null ? string.Empty : SafeName(feedback);
    }

    public bool HasDescriptionElement(string itemName)
    {
        var item = WaitForItem(itemName);
        return item.FindFirstDescendant(cf => cf.ByAutomationId("CatalogDescription")) is not null;
    }

    public double CardWidth(string itemName) => WaitForCard(itemName).BoundingRectangle.Width;

    public double PrimaryActionTop(string itemName)
        => PrimaryActionButton(itemName).BoundingRectangle.Top;

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
            () =>
            {
                var status = TryItemStatus(itemName);
                return status is not null
                    && Normalize(status).StartsWith(Normalize(expectedPrefix), StringComparison.OrdinalIgnoreCase);
            },
            timeout
        );
    }

    public void WaitForOperationContaining(string itemName, string expected, TimeSpan? timeout = null)
    {
        _session.WaitUntil(
            () => OperationText(itemName).Contains(expected, StringComparison.OrdinalIgnoreCase),
            timeout
        );
    }

    public void WaitForTerminalFeedbackContaining(string itemName, string expected, TimeSpan? timeout = null)
    {
        _session.WaitUntil(
            () => TerminalFeedbackText(itemName).Contains(expected, StringComparison.OrdinalIgnoreCase),
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

    public static string AutomationName(AutomationElement element) => SafeName(element);

    private AutomationElement? FindOperation(string itemName)
    {
        var catalog = ById("CatalogItems");
        var item = catalog?.FindFirstDescendant(cf => cf.ByAutomationId(itemName));
        return item?.FindFirstDescendant(cf => cf.ByAutomationId("CatalogOperation"));
    }

    private string? TryItemStatus(string itemName)
    {
        var catalog = ById("CatalogItems");
        var item = catalog?.FindFirstDescendant(cf => cf.ByAutomationId(itemName));
        var status = item?.FindFirstDescendant(cf => cf.ByAutomationId("CatalogObservation"));
        return status is null ? null : SafeName(status);
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

    private Button ButtonByAutomationId(string itemName, string automationId)
    {
        var item = WaitForItem(itemName);
        return _session.WaitFor(() => item.FindFirstDescendant(cf => cf.ByAutomationId(automationId))?.AsButton());
    }

    private static string Normalize(string value)
        => value.Replace(" ", string.Empty, StringComparison.Ordinal);

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

    private static string SafeHelpText(AutomationElement element)
    {
        try
        {
            return element.Properties.HelpText.ValueOrDefault ?? string.Empty;
        }
        catch (PropertyNotSupportedException)
        {
            return string.Empty;
        }
    }
}
