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

public enum OptionalInstallStatus
{
    Installed,
    NotInstalled,
    InstallPending,
    RemovePending,
    Unknown,
}

public sealed record OptionalInstallItem(
    string ItemName,
    string DisplayName,
    string Version,
    string Catalog,
    string InstallerType,
    string InstallerPackageId,
    string InstallerLocation,
    bool IsManaged,
    bool IsInstalled,
    OptionalInstallStatus Status,
    DateTimeOffset StatusUpdatedAtUtc,
    string? LastOperationId
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
    string? ErrorMessage = null,
    string? CanceledBy = null
);

public interface IGorillaServiceClient
{
    Task<IReadOnlyList<OptionalInstallItem>> ListOptionalInstallsAsync(CancellationToken cancellationToken);

    Task<OperationAccepted> InstallItemAsync(string itemName, CancellationToken cancellationToken);

    Task<OperationAccepted> RemoveItemAsync(string itemName, CancellationToken cancellationToken);

    IAsyncEnumerable<OperationStatusEvent> StreamOperationStatusAsync(
        string operationId,
        CancellationToken cancellationToken
    );
}
