namespace Gorilla.UI.Client;

public sealed record ListOptionalInstallsRequest();

public sealed record ListOptionalInstallsResponse(
    IReadOnlyList<OptionalInstallItem> Items
);

public sealed record InstallItemRequest(string ItemId);

public sealed record RemoveItemRequest(string ItemId);

public sealed record OperationAcceptedResponse(
    bool Accepted,
    DateTimeOffset QueuedAtUtc
);

public sealed record StreamOperationStatusRequest();

public sealed record StreamOperationStatusResponse(bool StreamAccepted);

public sealed record OperationStatusEventPayload(
    OperationState State,
    int ProgressPercent,
    string Message,
    string? ErrorCode = null,
    string? ErrorMessage = null,
    string? CanceledBy = null
);

public sealed record ErrorResponse(
    string ErrorCode,
    string ErrorMessage
);
