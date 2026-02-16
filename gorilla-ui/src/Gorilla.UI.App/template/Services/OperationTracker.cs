using Gorilla.UI.Client;

namespace Gorilla.UI.App.Services;

public sealed class OperationTracker
{
    private readonly IGorillaServiceClient _client;

    public OperationTracker(IGorillaServiceClient client)
    {
        _client = client;
    }

    public async Task TrackAsync(
        string operationId,
        Action<OperationStatusEvent> onUpdate,
        CancellationToken cancellationToken
    )
    {
        await foreach (var update in _client.StreamOperationStatusAsync(operationId, cancellationToken))
        {
            onUpdate(update);
        }
    }
}
