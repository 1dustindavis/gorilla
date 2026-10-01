using System.Text.Json.Serialization;

namespace Gorilla.UI.Client;

public sealed record ListOptionalInstallsRequest(bool Refresh);

public sealed record ListOptionalInstallsResponse(
    [property: JsonRequired] IReadOnlyList<OptionalInstallItem> Items,
    [property: JsonRequired] bool SnapshotAvailable,
    [property: JsonRequired] DateTimeOffset? SnapshotGeneratedAtUtc,
    [property: JsonRequired] string RefreshState,
    DateTimeOffset? RefreshRequestedAtUtc = null,
    DateTimeOffset? RefreshCompletedAtUtc = null,
    string? RefreshErrorCode = null
);

public sealed record InstallItemRequest(string ItemName, string MutationId);

public sealed record RemoveItemRequest(string ItemName, string MutationId);

public sealed record OperationAcceptedResponse(
    bool Accepted,
    DateTimeOffset QueuedAtUtc
);

public sealed record ListOperationsRequest();

public sealed record OperationSnapshotPayload(
    string OperationId,
    OperationStatusEventPayload Status
);

public sealed record ListOperationsResponse(
    IReadOnlyList<OperationSnapshotPayload> Operations
);

public sealed record StreamOperationStatusRequest();

public sealed record StreamOperationStatusResponse(bool StreamAccepted);

public sealed record OperationStatusEventPayload(
    OperationState State,
    int? ProgressPercent,
    string Message,
    string? ItemName = null,
    AppCatalog.Action? Action = null,
    AppCatalog.Result? Result = null
);

public sealed record ErrorResponse(
    string ErrorCode,
    string ErrorMessage
);
