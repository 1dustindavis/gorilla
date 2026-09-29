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

    public int CountEntries(string operationId)
        => SnapshotOperationIdentities()
            .Count(entry => string.Equals(entry.OperationId, operationId, StringComparison.Ordinal));

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
        => SnapshotOperationIdentities()
            .Where(entry => entry.Name.Contains(itemName, StringComparison.OrdinalIgnoreCase))
            .Select(entry => entry.OperationId)
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
                if (scroll is null)
                {
                    Thread.Sleep(100);
                    continue;
                }

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

                    try
                    {
                        scroll.Scroll(ScrollAmount.NoAmount, ScrollAmount.LargeIncrement);
                    }
                    catch (ArgumentException)
                    {
                        scroll.Scroll(ScrollAmount.NoAmount, ScrollAmount.SmallIncrement);
                    }

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
                        break;
                    }
                }
            }
            catch (PropertyNotSupportedException ex)
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

    private IReadOnlyList<(string OperationId, string Name)> SnapshotOperationIdentities()
    {
        var items = Items;
        var identities = new Dictionary<string, string>(StringComparer.Ordinal);
        var scroll = items.Patterns.Scroll.PatternOrDefault;

        if (scroll is null)
        {
            CollectVisibleOperationIdentities(items, identities);
            return identities.Select(pair => (pair.Key, pair.Value)).ToArray();
        }

        var originalHorizontal = scroll.HorizontalScrollPercent;
        var originalVertical = scroll.VerticalScrollPercent;
        try
        {
            scroll.SetScrollPercent(originalHorizontal, 0);
            Thread.Sleep(50);

            while (true)
            {
                CollectVisibleOperationIdentities(items, identities);

                var before = scroll.VerticalScrollPercent;
                if (before < 0 || before >= 100)
                {
                    break;
                }

                try
                {
                    scroll.Scroll(ScrollAmount.NoAmount, ScrollAmount.LargeIncrement);
                }
                catch (ArgumentException)
                {
                    scroll.Scroll(ScrollAmount.NoAmount, ScrollAmount.SmallIncrement);
                }

                Thread.Sleep(50);
                var after = scroll.VerticalScrollPercent;
                if (after >= 100)
                {
                    CollectVisibleOperationIdentities(items, identities);
                    break;
                }

                if (Math.Abs(after - before) < 0.001)
                {
                    break;
                }
            }
        }
        finally
        {
            try
            {
                scroll.SetScrollPercent(originalHorizontal, originalVertical);
                Thread.Sleep(50);
            }
            catch (InvalidOperationException)
            {
                // The list can become temporarily non-scrollable while a live operation
                // changes its layout. Identity collection is still valid; callers will
                // reacquire the row they need before interacting with it.
            }
        }

        return identities.Select(pair => (pair.Key, pair.Value)).ToArray();
    }

    private static void CollectVisibleOperationIdentities(
        AutomationElement items,
        IDictionary<string, string> identities
    )
    {
        foreach (var item in ListEntries(items))
        {
            var operationId = OperationId(item);
            if (!string.IsNullOrWhiteSpace(operationId))
            {
                identities[operationId] = SafeName(item);
            }
        }
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
