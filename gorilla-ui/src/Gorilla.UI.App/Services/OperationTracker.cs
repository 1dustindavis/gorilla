using System;
using System.Threading;
using System.Threading.Tasks;
using Gorilla.UI.Client;

namespace Gorilla.UI.App.Services;

public sealed class OperationTracker
{
    private readonly IGorillaServiceClient _client;

    public OperationTracker(IGorillaServiceClient client)
    {
        _client = client;
    }

    public async Task<OperationStatusEvent?> TrackAsync(
        string operationId,
        Action<OperationStatusEvent> onUpdate,
        CancellationToken cancellationToken
    )
    {
        OperationStatusEvent? latest = null;
        await foreach (var update in _client.StreamOperationStatusAsync(operationId, cancellationToken))
        {
            latest = update;
            onUpdate(update);
        }
        return latest;
    }
}
