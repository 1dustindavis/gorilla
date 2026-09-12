using FlaUI.Core.AutomationElements;
using FlaUI.Core.Exceptions;

namespace Gorilla.UI.App.WindowsUiTests;

internal sealed class CatalogShellDriver
{
    private readonly GorillaAppSession _session;

    public CatalogShellDriver(GorillaAppSession session)
    {
        _session = session;
    }

    public Button RefreshButton => _session.WaitFor(
        () => ById("CatalogRefreshButton")?.AsButton()
    );

    public string FreshnessText => SafeName(
        _session.WaitFor(() => ById("CatalogFreshnessStatus"))
    );

    public string DegradedWarningText
    {
        get
        {
            var warning = ById("CatalogDegradedWarningText");
            return warning is null ? string.Empty : SafeName(warning);
        }
    }

    public bool IsRefreshing => ById("CatalogRefreshProgress") is not null && !RefreshButton.IsEnabled;

    public void Refresh() => RefreshButton.Invoke();

    public void WaitForFreshnessContaining(string expected, TimeSpan? timeout = null)
    {
        _session.WaitUntil(
            () => FreshnessText.Contains(expected, StringComparison.OrdinalIgnoreCase),
            timeout
        );
    }

    public void WaitForDegradedWarningContaining(string expected, TimeSpan? timeout = null)
    {
        _session.WaitUntil(
            () => DegradedWarningText.Contains(expected, StringComparison.OrdinalIgnoreCase),
            timeout
        );
    }

    public void WaitForRefreshComplete(TimeSpan? timeout = null)
    {
        _session.WaitUntil(
            () => RefreshButton.IsEnabled && !FreshnessText.Contains("Refreshing", StringComparison.OrdinalIgnoreCase),
            timeout
        );
    }

    public bool HasInitialLoadingState() => ById("CatalogInitialLoading") is not null;

    public bool HasLoadFailedState() => ById("CatalogLoadFailed") is not null;

    public bool HasSuccessfulEmptyState() => ById("CatalogEmpty") is not null;

    private AutomationElement? ById(string automationId)
        => _session.MainWindow.FindFirstDescendant(cf => cf.ByAutomationId(automationId));

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
