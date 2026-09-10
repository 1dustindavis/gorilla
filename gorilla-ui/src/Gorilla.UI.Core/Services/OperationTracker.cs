using System.Collections.Concurrent;
using Gorilla.UI.Client;

namespace Gorilla.UI.Core.Services;

public sealed class OperationTracker
{
    private const int StreamAttemptLimit = 3;
    private readonly IGorillaServiceClient _client;
    private readonly ConcurrentDictionary<string, OperationStatusEvent> _latest = new(StringComparer.Ordinal);

    public OperationTracker(IGorillaServiceClient client)
    {
        _client = client;
    }

    public async Task<IReadOnlyList<OperationStatusEvent>> RefreshKnownOperationsAsync(CancellationToken cancellationToken)
    {
        var snapshots = await _client.ListOperationsAsync(cancellationToken);
        var retainedIds = snapshots.Select(operation => operation.OperationId).ToHashSet(StringComparer.Ordinal);

        foreach (var operation in snapshots)
        {
            _latest[operation.OperationId] = operation;
        }

        // The service operation registry is authoritative. Missing IDs mean the
        // operation aged out or the service restarted; neither is a terminal
        // success/failure result and neither should remain projected as active.
        foreach (var operationId in _latest.Keys)
        {
            if (!retainedIds.Contains(operationId))
            {
                _latest.TryRemove(operationId, out _);
            }
        }

        return snapshots;
    }

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
            .FirstOrDefault();
    }

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
                    _latest[operationId] = update;
                    if (delivered.Add(EventIdentity(update)))
                    {
                        onUpdate(update);
                    }
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
                // The service replays retained events from the beginning on a
                // reconnect. delivered prevents lifecycle regressions in the UI.
            }
        }
    }

    private static bool IsReconnectable(Exception ex)
        => ex is IOException or TimeoutException or OperationCanceledException;

    private static string EventIdentity(OperationStatusEvent update)
        => string.Join(
            "\u001f",
            update.State,
            update.ProgressPercent?.ToString() ?? string.Empty,
            update.Message,
            update.ItemName,
            update.Action,
            update.Result?.Outcome.ToString() ?? string.Empty,
            update.Result?.Code ?? string.Empty,
            update.Result?.Message ?? string.Empty
        );
}
