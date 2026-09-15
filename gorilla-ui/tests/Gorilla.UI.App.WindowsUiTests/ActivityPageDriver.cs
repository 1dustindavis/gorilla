using System.Diagnostics;
using FlaUI.Core.AutomationElements;
using FlaUI.Core.Definitions;
using FlaUI.Core.Exceptions;

namespace Gorilla.UI.App.WindowsUiTests;

internal sealed class ActivityPageDriver
{
    private readonly GorillaAppSession _session;

    public ActivityPageDriver(GorillaAppSession session)
    {
        _session = session;
    }

    public AutomationElement Root => _session.WaitFor(() => ById("ActivityHeading"));
    public AutomationElement Items => _session.WaitFor(() => ById("ActivityItems"));
    public AutomationElement EmptyState => _session.WaitFor(() => ById("ActivityEmptyState"));

    public static ActivityPageDriver OpenFromCatalog(GorillaAppSession session)
    {
        var button = session.WaitFor(
            () => session.MainWindow.FindFirstDescendant(cf => cf.ByAutomationId("ActivityNavigationButton"))?.AsButton()
        );
        button.Invoke();
        var driver = new ActivityPageDriver(session);
        _ = driver.Root;
        return driver;
    }

    public AutomationElement WaitForOperation(string operationId, TimeSpan? timeout = null)
        => _session.WaitFor(
            () => Items.FindFirstDescendant(cf => cf.ByAutomationId($"ActivityOperation-{operationId}")),
            timeout
        );

    public void WaitForOperationState(string operationId, string expected, TimeSpan? timeout = null)
    {
        _session.WaitUntil(
            () => StateText(operationId).Contains(expected, StringComparison.OrdinalIgnoreCase),
            timeout
        );
    }

    public string StateText(string operationId)
        => NameOfDescendant(operationId, $"ActivityState-{operationId}");

    public string ActionText(string operationId)
        => NameOfDescendant(operationId, $"ActivityAction-{operationId}");

    public string DetailText(string operationId)
        => NameOfDescendant(operationId, $"ActivityDetail-{operationId}");

    public string FailureTitle(string operationId)
        => NameOfDescendant(operationId, $"ActivityFailureTitle-{operationId}");

    public string RetryAttemptFeedback(string operationId)
        => NameOfDescendant(operationId, $"ActivityRetryFeedback-{operationId}");

    public string RetryUnavailableText(string operationId)
        => NameOfDescendant(operationId, $"ActivityRetryUnavailable-{operationId}");

    public Button RetryButton(string operationId)
    {
        var entry = ScrollOperationIntoView(operationId);
        return _session.WaitFor(
            () => entry.FindFirstDescendant(cf => cf.ByAutomationId($"ActivityRetry-{operationId}"))?.AsButton()
        );
    }

    public bool HasRetryButton(string operationId)
        => WaitForOperation(operationId)
            .FindFirstDescendant(cf => cf.ByAutomationId($"ActivityRetry-{operationId}")) is not null;

    public AutomationElement TechnicalDetailsDisclosure(string operationId)
    {
        var entry = ScrollOperationIntoView(operationId);
        return _session.WaitFor(
            () => entry.FindFirstDescendant(cf => cf.ByAutomationId($"OperationTechnicalDetails-{operationId}"))
        );
    }

    public string OpenAndReadTechnicalDetails(string operationId)
    {
        var disclosure = TechnicalDetailsDisclosure(operationId);
        var expandCollapse = disclosure.Patterns.ExpandCollapse.Pattern;
        expandCollapse.Expand();

        return _session.WaitFor(
            () => ScrollOperationIntoView(operationId)
                .FindFirstDescendant(cf => cf.ByAutomationId($"OperationTechnicalDetailsContent-{operationId}"))
                ?.AsTextBox()
        ).Text;
    }

    public int CountEntries(string operationId)
        => Items.FindAllDescendants(cf => cf.ByAutomationId($"ActivityOperation-{operationId}")).Length;

    public AutomationElement WaitForEntryContaining(string text, TimeSpan? timeout = null)
        => _session.WaitFor(
            () => ListEntries().FirstOrDefault(item => SafeName(item).Contains(text, StringComparison.OrdinalIgnoreCase)),
            timeout
        );

    public AutomationElement WaitForEntryWithDetail(string detail, TimeSpan? timeout = null)
        => _session.WaitFor(
            () => ListEntries().FirstOrDefault(item =>
            {
                if (!EntryContainsDetail(item, detail))
                {
                    return false;
                }

                // Existing Stage 6 callers use this helper to inspect a retryable
                // retained failure immediately after creating it. Older retained rows
                // may carry the same detail while a new operation is still active,
                // which temporarily suppresses Retry for those rows. Wait for the
                // matching retryable row instead of returning stale text identity.
                var operationId = OperationId(item);
                return !string.IsNullOrWhiteSpace(operationId)
                    && item.FindFirstDescendant(cf => cf.ByAutomationId($"ActivityRetry-{operationId}")) is not null;
            }),
            timeout
        );

    public IReadOnlySet<string> OperationIdsForItem(string itemName)
        => ListEntries()
            .Where(item => SafeName(item).Contains(itemName, StringComparison.OrdinalIgnoreCase))
            .Select(OperationId)
            .Where(id => !string.IsNullOrWhiteSpace(id))
            .ToHashSet(StringComparer.Ordinal);

    public AutomationElement WaitForNewEntryWithDetail(
        string detail,
        IReadOnlySet<string> existingOperationIds,
        TimeSpan? timeout = null
    ) => _session.WaitFor(
        () => ListEntries().FirstOrDefault(item =>
        {
            var operationId = OperationId(item);
            return !string.IsNullOrWhiteSpace(operationId)
                && !existingOperationIds.Contains(operationId)
                && EntryContainsDetail(item, detail);
        }),
        timeout
    );

    public AutomationElement WaitForDifferentOperation(string itemName, string previousOperationId, TimeSpan? timeout = null)
        => _session.WaitFor(
            () => ListEntries().FirstOrDefault(item =>
                !string.Equals(SafeHelpText(item), previousOperationId, StringComparison.Ordinal)
                && SafeName(item).Contains(itemName, StringComparison.OrdinalIgnoreCase)),
            timeout
        );

    public static string OperationId(AutomationElement entry) => SafeHelpText(entry);

    public void OpenDetails(string operationId)
    {
        var timeout = TimeSpan.FromSeconds(30);
        var stopwatch = Stopwatch.StartNew();

        while (stopwatch.Elapsed < timeout)
        {
            var entry = ScrollOperationIntoView(operationId);
            var nonActionTarget = _session.WaitFor(
                () => entry.FindFirstDescendant(cf => cf.ByAutomationId($"ActivityApp-{operationId}"))
            );
            nonActionTarget.Click();

            try
            {
                _ = _session.WaitFor(() => ById("AppDetailsRoot"), TimeSpan.FromSeconds(2));
                return;
            }
            catch (TimeoutException) when (stopwatch.Elapsed < timeout)
            {
                // Match HomePageDriver's established WinUI/FlaUI boundary: pointer
                // delivery can race ListView settling after ScrollIntoView, especially
                // when an expanded operation row changes the realized layout. Reacquire
                // the current row and retry the same non-action target.
            }
        }

        throw new TimeoutException(
            $"Timed out after {timeout.TotalSeconds:n0}s opening Activity details for operation '{operationId}'."
        );
    }

    public void GoBack()
    {
        _session.WaitFor(() => ById("ActivityBackButton")?.AsButton()).Invoke();
        _ = _session.WaitFor(() => ById("HomeHeading"));
    }

    private AutomationElement ScrollOperationIntoView(string operationId)
    {
        var entry = WaitForOperation(operationId);
        entry.AsListBoxItem().ScrollIntoView();

        // ScrollIntoView can cause WinUI ListView virtualization to recycle the
        // realized container and its descendants. Do not keep using the pre-scroll
        // automation proxy: wait for, and then return, the currently realized row.
        _session.WaitUntil(() =>
        {
            var realized = Items.FindFirstDescendant(
                cf => cf.ByAutomationId($"ActivityOperation-{operationId}")
            );
            return realized is not null && !realized.IsOffscreen;
        });

        return WaitForOperation(operationId);
    }

    private AutomationElement[] ListEntries()
        => Items.FindAllDescendants(cf => cf.ByControlType(ControlType.ListItem));

    private static bool EntryContainsDetail(AutomationElement item, string detail)
        => item.FindAllDescendants(cf => cf.ByControlType(ControlType.Text))
            .Any(text => SafeName(text).Contains(detail, StringComparison.OrdinalIgnoreCase));

    private string NameOfDescendant(string operationId, string automationId)
    {
        var entry = WaitForOperation(operationId);
        var element = entry.FindFirstDescendant(cf => cf.ByAutomationId(automationId));
        return element is null ? string.Empty : SafeName(element);
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
