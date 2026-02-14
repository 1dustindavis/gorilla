namespace Gorilla.UI.Client;

public enum OperationState
{
    Queued,
    Validating,
    Downloading,
    Installing,
    Removing,
    Succeeded,
    Failed,
    Canceled,
}

public sealed record OptionInstallItem(
    string ItemId,
    string DisplayName,
    string Version,
    bool IsInstalled,
    bool IsManaged,
    string Action
);

public sealed record OperationAccepted(
    string OperationId,
    bool Accepted,
    DateTimeOffset QueuedAtUtc
);

public sealed record OperationStatusEvent(
    string OperationId,
    OperationState State,
    int ProgressPercent,
    string Message,
    DateTimeOffset TimestampUtc,
    string? ErrorCode = null,
    string? ErrorMessage = null
);

public interface IGorillaServiceClient
{
    Task<IReadOnlyList<OptionInstallItem>> ListOptionalInstallsAsync(CancellationToken cancellationToken);

    Task<OperationAccepted> InstallItemAsync(string itemId, CancellationToken cancellationToken);

    Task<OperationAccepted> RemoveItemAsync(string itemId, CancellationToken cancellationToken);

    IAsyncEnumerable<OperationStatusEvent> StreamOperationStatusAsync(
        string operationId,
        CancellationToken cancellationToken
    );
}
