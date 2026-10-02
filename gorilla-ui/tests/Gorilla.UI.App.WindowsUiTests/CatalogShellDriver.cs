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

    public string InfrastructureWarningText
    {
        get
        {
            var warning = ById("InfrastructureWarningText");
            return warning is null ? string.Empty : SafeName(warning);
        }
    }

    public bool IsRefreshing => ById("CatalogRefreshProgress") is not null && !RefreshButton.IsEnabled;

    public void Refresh()
    {
        var refreshButton = RefreshButton;
        if (!refreshButton.IsEnabled)
        {
            WaitForRefreshComplete();
            return;
        }

        try
        {
            refreshButton.Invoke();
        }
        catch (ElementNotEnabledException)
        {
            // Startup can begin a refresh between the enabled check and Invoke.
            // In that case the requested refresh is already in flight, so wait for
            // it rather than starting a redundant second refresh afterward.
            WaitForRefreshComplete();
        }
    }

    public string OpenAndReadTechnicalDetails()
    {
        Expand("CatalogTechnicalDetails");
        return _session.WaitFor(() => ById("CatalogTechnicalDetailsContent")).AsTextBox().Text;
    }

    public string OpenAndReadInfrastructureTechnicalDetails()
    {
        Expand("InfrastructureTechnicalDetails");
        return _session.WaitFor(() => ById("InfrastructureTechnicalDetailsContent")).AsTextBox().Text;
    }

    public void CollapseInfrastructureTechnicalDetails()
        => Collapse("InfrastructureTechnicalDetails");

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

    public void WaitForInfrastructureWarningContaining(string expected, TimeSpan? timeout = null)
    {
        _session.WaitUntil(
            () => InfrastructureWarningText.Contains(expected, StringComparison.OrdinalIgnoreCase),
            timeout
        );
    }

    public void WaitForRefreshStarted(TimeSpan? timeout = null)
    {
        // Refresh regeneration can complete in only a few milliseconds. FlaUI may
        // therefore miss the transient disabled/progress state entirely even though
        // the refresh request was accepted and completed successfully. Tests that
        // care about the durable refresh result call WaitForRefreshComplete later;
        // here, accept either an observed in-progress state or an already-completed
        // state instead of requiring automation to sample a fleeting transition.
        _session.WaitUntil(
            () =>
                (IsRefreshing && FreshnessText.Contains("Refreshing", StringComparison.OrdinalIgnoreCase))
                || (RefreshButton.IsEnabled && !FreshnessText.Contains("Refreshing", StringComparison.OrdinalIgnoreCase)),
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

    public bool HasNoCachedDataState() => ById("CatalogNoCachedData") is not null;

    public bool HasSuccessfulEmptyState() => ById("CatalogEmpty") is not null;

    public int InfrastructureWarningPresentationCount()
        // The shell wrapper is a layout container and WinUI does not expose it as a
        // UI Automation element. Count the warning text, which is the stable exposed
        // element representing each rendered infrastructure-warning presentation.
        => _session.MainWindow.FindAllDescendants(
            cf => cf.ByAutomationId("InfrastructureWarningText")
        ).Length;

    private void Expand(string automationId)
    {
        var disclosure = _session.WaitFor(() => ById(automationId));
        disclosure.Patterns.ExpandCollapse.Pattern.Expand();
    }

    private void Collapse(string automationId)
    {
        var disclosure = _session.WaitFor(() => ById(automationId));
        disclosure.Patterns.ExpandCollapse.Pattern.Collapse();
    }

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
