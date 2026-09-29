using System.Diagnostics;
using FlaUI.Core.AutomationElements;
using FlaUI.Core.Definitions;
using FlaUI.Core.Exceptions;
using FlaUI.Core.Input;
using FlaUI.Core.WindowsAPI;

namespace Gorilla.UI.App.WindowsUiTests;

internal sealed class ActivityPageDriver
{
    private readonly GorillaAppSession _session;

    public ActivityPageDriver(GorillaAppSession session)
    {
        _session = session;
    }

    public AutomationElement Root => _session.WaitFor(() => ById("ActivityPageRoot"));
    public AutomationElement Heading => _session.WaitFor(() => ById("ActivityHeading"));
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
        => FindEntryAcrossVirtualizedList(
            item => string.Equals(OperationId(item), operationId, StringComparison.Ordinal),
            $"Activity operation '{operationId}'",
            timeout
        );

    public void WaitForOperationState(string operationId, string expected, TimeSpan? timeout = null)
    {
        _session.WaitUntil(
            () => StateText(operationId).Contains(expected, StringComparison.OrdinalIgnoreCase),
            timeout
        );
    }

    // StateText intentionally exposes the coarse visual state used by the legacy
    // behavior tests. Stage 7 gives the same live TextBlock a richer UIA Name that
    // combines state/outcome and detail; accessibility tests assert that semantic
    // Name directly instead of conflating it with the visible coarse state.
    public string StateText(string operationId)
        => CoarseState(NameOfDescendant(operationId, $"ActivityState-{operationId}"));

    public string SemanticStateText(string operationId)
        => NameOfDescendant(operationId, $"ActivityState-{operationId}");

    public string ActionText(string operationId)
        => NameOfDescendant(operationId, $"ActivityAction-{operationId}");

    public string DetailText(string operationId)
        => NameOfDescendant(operationId, $"ActivityDetail-{operationId}");

    public string FailureTitle(string operationId)
        => LeadingSentence(NameOfDescendant(operationId, $"ActivityFailureTitle-{operationId}"));

    public string SemanticFailureTitle(string operationId)
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

    // This is intentionally a realized-container count. Callers first locate the
    // logical operation, which brings that row into the realized UIA window, then
    // use this assertion to guard against duplicate peers for the same OperationId.
    public int CountEntries(string operationId)
        => Items.FindAllDescendants(cf => cf.ByAutomationId($"ActivityOperation-{operationId}")).Length;

    public AutomationElement WaitForEntryContaining(string text, TimeSpan? timeout = null)
        => FindEntryAcrossVirtualizedList(
            item => SafeName(item).Contains(text, StringComparison.OrdinalIgnoreCase),
            $"Activity entry containing '{text}'",
            timeout
        );

    public AutomationElement WaitForEntryWithDetail(string detail, TimeSpan? timeout = null)
        => FindEntryAcrossVirtualizedList(
            item =>
            {
                if (!EntryContainsDetail(item, detail))
                {
                    return false;
                }

                var operationId = OperationId(item);
                return !string.IsNullOrWhiteSpace(operationId)
                    && item.FindFirstDescendant(cf => cf.ByAutomationId($"ActivityRetry-{operationId}")) is not null;
            },
            $"Activity entry with detail '{detail}'",
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
    ) => FindEntryAcrossVirtualizedList(
        item =>
        {
            var operationId = OperationId(item);
            return !string.IsNullOrWhiteSpace(operationId)
                && !existingOperationIds.Contains(operationId)
                && EntryContainsDetail(item, detail);
        },
        $"new Activity entry with detail '{detail}'",
        timeout
    );

    public AutomationElement WaitForDifferentOperation(string itemName, string previousOperationId, TimeSpan? timeout = null)
        => FindEntryAcrossVirtualizedList(
            item =>
                !string.Equals(SafeHelpText(item), previousOperationId, StringComparison.Ordinal)
                && SafeName(item).Contains(itemName, StringComparison.OrdinalIgnoreCase),
            $"Activity operation for '{itemName}' different from '{previousOperationId}'",
            timeout
        );

    public AutomationElement WaitForNewOperation(
        string itemName,
        string action,
        IReadOnlySet<string> existingOperationIds,
        TimeSpan? timeout = null
    ) => FindEntryAcrossVirtualizedList(
        item =>
        {
            var operationId = OperationId(item);
            if (string.IsNullOrWhiteSpace(operationId)
                || existingOperationIds.Contains(operationId)
                || !SafeName(item).Contains(itemName, StringComparison.OrdinalIgnoreCase))
            {
                return false;
            }

            var actionElement = item.FindFirstDescendant(
                cf => cf.ByAutomationId($"ActivityAction-{operationId}")
            );
            return actionElement is not null
                && string.Equals(SafeName(actionElement), action, StringComparison.OrdinalIgnoreCase);
        },
        $"new {action} Activity operation for '{itemName}'",
        timeout
    );

    public static string OperationId(AutomationElement entry) => SafeHelpText(entry);

    public void OpenDetails(string operationId)
    {
        var timeout = TimeSpan.FromSeconds(30);
        var stopwatch = Stopwatch.StartNew();
        Exception? lastError = null;

        while (stopwatch.Elapsed < timeout)
        {
            try
            {
                var entry = ScrollOperationIntoView(operationId);

                var technicalDetails = entry.FindFirstDescendant(
                    cf => cf.ByAutomationId($"OperationTechnicalDetails-{operationId}")
                );
                var technicalDetailsContent = entry.FindFirstDescendant(
                    cf => cf.ByAutomationId($"OperationTechnicalDetailsContent-{operationId}")
                );
                if (technicalDetails is not null && technicalDetailsContent is not null)
                {
                    technicalDetails.Patterns.ExpandCollapse.Pattern.Collapse();
                    entry = ScrollOperationIntoView(operationId);
                }

                // Use the ListViewItem activation contract instead of a child TextBlock
                // clickable point. Virtualized child peers can temporarily be present
                // in UIA without exposing a mouse point, while the entry itself remains
                // the stable keyboard activation surface.
                _session.FocusForKeyboard(entry, TimeSpan.FromSeconds(5));
                Keyboard.Type(VirtualKeyShort.SPACE);

                _ = _session.WaitFor(() => ById("AppDetailsRoot"), TimeSpan.FromSeconds(2));
                return;
            }
            catch (TimeoutException ex)
            {
                lastError = ex;
            }
        }

        throw new TimeoutException(
            $"Timed out after {timeout.TotalSeconds:n0}s opening Activity details for operation '{operationId}'.",
            lastError
        );
    }

    public void GoBack()
    {
        _session.WaitFor(() => ById("ActivityBackButton")?.AsButton()).Invoke();
        _ = _session.WaitFor(() => ById("CatalogSearchBox"));
    }

    public void ScrollToEnd()
    {
        var scroll = Items.Patterns.Scroll.PatternOrDefault;
        if (scroll is null || !scroll.VerticallyScrollable)
        {
            return;
        }

        scroll.SetScrollPercent(scroll.HorizontalScrollPercent, 100);
        Thread.Sleep(100);
    }

    private AutomationElement ScrollOperationIntoView(string operationId)
    {
        var entry = WaitForOperation(operationId);
        entry.AsListBoxItem().ScrollIntoView();

        _session.WaitUntil(() =>
        {
            var realized = Items.FindFirstDescendant(
                cf => cf.ByAutomationId($"ActivityOperation-{operationId}")
            );
            return realized is not null && !realized.IsOffscreen;
        });

        return WaitForOperation(operationId);
    }

    private AutomationElement FindEntryAcrossVirtualizedList(
        Func<AutomationElement, bool> predicate,
        string description,
        TimeSpan? timeout = null
    )
    {
        var effectiveTimeout = timeout ?? TimeSpan.FromSeconds(30);
        var stopwatch = Stopwatch.StartNew();
        Exception? lastError = null;

        while (stopwatch.Elapsed < effectiveTimeout)
        {
            var items = Items;
            try
            {
                var current = ListEntries(items).FirstOrDefault(predicate);
                if (current is not null)
                {
                    return current;
                }

                var scroll = items.Patterns.Scroll.PatternOrDefault;
                if (scroll is null || !scroll.VerticallyScrollable)
                {
                    Thread.Sleep(100);
                    continue;
                }

                // UI Automation only exposes realized ListView containers. Search
                // deterministically from the start of the logical list, advancing the
                // viewport until WinUI realizes the target OperationId. This avoids the
                // circular old behavior of requiring a row to be realized before it
                // could be scrolled into view.
                scroll.SetScrollPercent(scroll.HorizontalScrollPercent, 0);
                Thread.Sleep(50);

                while (stopwatch.Elapsed < effectiveTimeout)
                {
                    current = ListEntries(items).FirstOrDefault(predicate);
                    if (current is not null)
                    {
                        return current;
                    }

                    var before = scroll.VerticalScrollPercent;
                    if (before < 0 || before >= 100)
                    {
                        break;
                    }

                    scroll.Scroll(ScrollAmount.NoAmount, ScrollAmount.LargeIncrement);
                    Thread.Sleep(50);

                    var after = scroll.VerticalScrollPercent;
                    if (after >= 100)
                    {
                        current = ListEntries(items).FirstOrDefault(predicate);
                        if (current is not null)
                        {
                            return current;
                        }
                        break;
                    }

                    if (Math.Abs(after - before) < 0.001)
                    {
                        // Some WinUI ListView providers report the same percentage for
                        // one large increment near a virtualization boundary. A small
                        // increment gives the provider one more chance to realize the
                        // next container without converting a UIA timing issue into an
                        // arbitrary longer timeout.
                        scroll.Scroll(ScrollAmount.NoAmount, ScrollAmount.SmallIncrement);
                        Thread.Sleep(50);
                        if (Math.Abs(scroll.VerticalScrollPercent - before) < 0.001)
                        {
                            break;
                        }
                    }
                }
            }
            catch (PropertyNotSupportedException ex)
            {
                lastError = ex;
            }
            catch (ElementNotAvailableException ex)
            {
                lastError = ex;
            }
            catch (InvalidOperationException ex)
            {
                lastError = ex;
            }

            Thread.Sleep(100);
        }

        throw new TimeoutException(
            $"Timed out after {effectiveTimeout.TotalSeconds:n0}s finding {description} across the virtualized Activity list.",
            lastError
        );
    }

    private AutomationElement[] ListEntries()
        => ListEntries(Items);

    private static AutomationElement[] ListEntries(AutomationElement items)
        => items.FindAllDescendants(cf => cf.ByControlType(ControlType.ListItem));

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

    private static string CoarseState(string semanticName)
    {
        if (semanticName.StartsWith("Installation failed", StringComparison.OrdinalIgnoreCase) ||
            semanticName.StartsWith("Removal failed", StringComparison.OrdinalIgnoreCase))
        {
            return "Failed";
        }

        if (semanticName.Contains("couldn't be verified", StringComparison.OrdinalIgnoreCase))
        {
            return "Unverified";
        }

        if (semanticName.Contains("was interrupted", StringComparison.OrdinalIgnoreCase))
        {
            return "Interrupted";
        }

        return LeadingSentence(semanticName);
    }

    private static string LeadingSentence(string value)
    {
        var separator = value.IndexOf(". ", StringComparison.Ordinal);
        return separator < 0 ? value : value[..separator];
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
