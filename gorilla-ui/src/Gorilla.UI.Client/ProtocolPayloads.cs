namespace Gorilla.UI.Client;

public sealed record ListOptionalInstallsRequest();

public sealed record ListOptionalInstallsResponse(
    IReadOnlyList<OptionalInstallItem> Items
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
