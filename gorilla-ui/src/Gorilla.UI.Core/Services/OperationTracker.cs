using System.Collections.Concurrent;
using System.Globalization;
using Gorilla.UI.Client;

namespace Gorilla.UI.Core.Services;

public sealed class OperationTracker
{
    private const int StreamAttemptLimit = 2;
    private readonly IGorillaServiceClient _client;
    private readonly ConcurrentDictionary<string, OperationStatusEvent> _latest = new(StringComparer.Ordinal);

    public OperationTracker(IGorillaServiceClient client)
    {
        _client = client;
    }

    public event EventHandler? OperationsChanged;

    public async Task<IReadOnlyList<OperationStatusEvent>> RefreshKnownOperationsAsync(CancellationToken cancellationToken)
    {
        var beforeLookup = _latest.ToArray();
        var snapshots = await _client.ListOperationsAsync(cancellationToken);
        var retainedIds = snapshots.Select(operation => operation.OperationId).ToHashSet(StringComparer.Ordinal);
        var changed = false;

        foreach (var operation in snapshots)
        {
            while (true)
            {
                if (!_latest.TryGetValue(operation.OperationId, out var existing))
                {
                    if (_latest.TryAdd(operation.OperationId, operation))
                    {
                        changed = true;
                        break;
                    }
                    continue;
                }
                if (existing == operation || IsOlderThanCurrent(existing, operation))
                {
                    break;
                }
                if (_latest.TryUpdate(operation.OperationId, operation, existing))
                {
                    changed = true;
                    break;
                }
            }
        }

        // The service operation registry is authoritative. Missing IDs mean the
        // operation aged out or the service restarted; neither is a terminal
        // success/failure result and neither should remain projected as activity.
        foreach (var entry in beforeLookup)
        {
            // Do not erase work started or advanced while the lookup was in flight.
            if (!retainedIds.Contains(entry.Key) &&
                ((ICollection<KeyValuePair<string, OperationStatusEvent>>)_latest).Remove(entry))
            {
                changed = true;
            }
        }

        if (changed)
        {
            OnOperationsChanged();
        }

        // A stream may have advanced these operations during the lookup. Return
        // the canonical, reconciled states so callers do not restart stale work.
        return _latest.Values.ToArray();
    }

    // Status tracking is best-effort UI state. If both the stream and service
    // lookup fail, release the stale busy projection rather than requiring
    // subsequent refreshes to reconstruct a historical outcome.
    public void ForgetOperation(string operationId)
    {
        if (_latest.TryRemove(operationId, out _))
        {
            OnOperationsChanged();
        }
    }

    // Canonical Activity source. This is a snapshot of the same operation-ID keyed
    // registry used by the card/details projections; Activity must not maintain a
    // separate operation/history truth.
    public IReadOnlyList<OperationStatusEvent> GetRetainedOperations()
        => _latest.Values.ToArray();

    public bool TryGetLatest(string operationId, out OperationStatusEvent? operation)
    {
        var found = _latest.TryGetValue(operationId, out var value);
        operation = value;
        return found;
    }

    public OperationStatusEvent? GetActiveForItem(string itemName)
    {
        return _latest.Values
            .Where(operation => operation.State != OperationState.Completed)
            .Where(operation => string.Equals(operation.ItemName, itemName, StringComparison.OrdinalIgnoreCase))
            .OrderByDescending(operation => operation.TimestampUtc)
            .ThenByDescending(operation => operation.OperationId, StringComparer.Ordinal)
            .FirstOrDefault();
    }

    public OperationStatusEvent? GetLatestTerminalForItem(string itemName)
    {
        return _latest.Values
            .Where(operation => operation.State == OperationState.Completed)
            .Where(operation => string.Equals(operation.ItemName, itemName, StringComparison.OrdinalIgnoreCase))
            .OrderByDescending(operation => operation.TimestampUtc)
            .ThenByDescending(operation => operation.OperationId, StringComparer.Ordinal)
            .FirstOrDefault();
    }

    // This is an item-oriented projection over the operation-ID registry. The
    // registry itself remains keyed by operation ID so replay and reconciliation
    // semantics do not depend on catalog objects.
    public OperationStatusEvent? GetCurrentOrLatestForItem(string itemName)
        => GetActiveForItem(itemName) ?? GetLatestTerminalForItem(itemName);

    public IReadOnlyList<OperationStatusEvent> GetActiveOperations()
    {
        return _latest.Values
            .Where(operation => operation.State != OperationState.Completed)
            .OrderBy(operation => operation.ItemName, StringComparer.OrdinalIgnoreCase)
            .ToArray();
    }

    public async Task TrackAsync(
        string operationId,
        Action<OperationStatusEvent> onUpdate,
        CancellationToken cancellationToken
    )
    {
        var delivered = new HashSet<string>(StringComparer.Ordinal);
        for (var attempt = 1; attempt <= StreamAttemptLimit; attempt++)
        {
            try
            {
                await foreach (var update in _client.StreamOperationStatusAsync(operationId, cancellationToken))
                {
                    var identity = EventIdentity(update);
                    if (!delivered.Add(identity))
                    {
                        if (update.State == OperationState.Completed)
                        {
                            return;
                        }
                        continue;
                    }

                    // Recovery begins with a ListOperations snapshot, while a newly
                    // opened status stream may replay older retained events first.
                    // Keep the operation-ID registry monotonic so Activity and the
                    // existing card/details projections never move backwards.
                    var accepted = false;
                    var changed = false;
                    while (true)
                    {
                        if (!_latest.TryGetValue(operationId, out var current))
                        {
                            if (_latest.TryAdd(operationId, update))
                            {
                                accepted = true;
                                changed = true;
                                break;
                            }
                        }
                        else if (IsOlderThanCurrent(current, update))
                        {
                            // A concurrent lookup or stream already published newer evidence.
                            break;
                        }
                        else if (current == update)
                        {
                            // Still deliver a replayed terminal event to complete tracking.
                            accepted = true;
                            break;
                        }
                        else if (_latest.TryUpdate(operationId, update, current))
                        {
                            accepted = true;
                            changed = true;
                            break;
                        }
                    }

                    if (!accepted)
                    {
                        continue;
                    }
                    if (changed)
                    {
                        OnOperationsChanged();
                    }
                    onUpdate(update);
                    if (update.State == OperationState.Completed)
                    {
                        return;
                    }
                }
                return;
            }
            catch (OperationCanceledException) when (cancellationToken.IsCancellationRequested)
            {
                throw;
            }
            catch (Exception ex) when (attempt < StreamAttemptLimit && IsReconnectable(ex))
            {
                // Retry the status stream once. The service replays retained
                // events from the beginning; delivered prevents UI regressions.
            }
        }
    }

    private void OnOperationsChanged()
        => OperationsChanged?.Invoke(this, EventArgs.Empty);

    private static bool IsOlderThanCurrent(OperationStatusEvent current, OperationStatusEvent update)
    {
        if (current.State == OperationState.Completed && update.State != OperationState.Completed)
        {
            return true;
        }
        if (update.TimestampUtc < current.TimestampUtc)
        {
            return true;
        }
        if (update.TimestampUtc > current.TimestampUtc)
        {
            return false;
        }
        return StateRank(update.State) < StateRank(current.State);
    }

    private static int StateRank(OperationState state) => state switch
    {
        OperationState.Queued => 0,
        OperationState.Completed => 2,
        _ => 1,
    };

    private static bool IsReconnectable(Exception ex)
        => ex is IOException or TimeoutException or OperationCanceledException;

    private static string EventIdentity(OperationStatusEvent update)
        => string.Join(
            "\u001f",
            update.State,
            update.ProgressPercent?.ToString(CultureInfo.InvariantCulture) ?? string.Empty,
            update.Message,
            update.ItemName,
            update.Action,
            update.Result?.Outcome.ToString() ?? string.Empty,
            update.Result?.Code ?? string.Empty,
            update.Result?.Message ?? string.Empty
        );
}
