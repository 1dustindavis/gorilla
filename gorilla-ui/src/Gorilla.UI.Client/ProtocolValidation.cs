namespace Gorilla.UI.Client;

public sealed class ProtocolValidationException : Exception
{
    public ProtocolValidationException(string message)
        : base(message) { }
}

public static class ProtocolValidation
{
    public static void ValidateEnvelopeHeader<TPayload>(ServiceEnvelope<TPayload> envelope)
    {
        if (envelope.Version != ProtocolConstants.Version)
        {
            throw new ProtocolValidationException($"Unsupported protocol version '{envelope.Version}'.");
        }

        if (!ProtocolConstants.AllOperations.Contains(envelope.Operation))
        {
            throw new ProtocolValidationException($"Unsupported operation '{envelope.Operation}'.");
        }

        if (envelope.TimestampUtc == default)
        {
            throw new ProtocolValidationException("timestampUtc is required.");
        }

        if (envelope.MessageType is not ProtocolMessageType.Event && string.IsNullOrWhiteSpace(envelope.RequestId))
        {
            throw new ProtocolValidationException("requestId is required for non-event messages.");
        }
    }

    public static void ValidateOptionalInstallItem(OptionalInstallItem item)
    {
        if (string.IsNullOrWhiteSpace(item.ItemName))
        {
            throw new ProtocolValidationException("itemName is required.");
        }

        if (item.StatusUpdatedAtUtc == default)
        {
            throw new ProtocolValidationException("statusUpdatedAtUtc is required.");
        }
    }

    public static CatalogRefreshState ValidateListOptionalInstallsResponse(ListOptionalInstallsResponse payload)
    {
        if (payload.Items is null)
        {
            throw new ProtocolValidationException("items is required.");
        }

        var refreshState = payload.RefreshState switch
        {
            "Idle" => CatalogRefreshState.Idle,
            "Queued" => CatalogRefreshState.Queued,
            "Running" => CatalogRefreshState.Running,
            "Failed" => CatalogRefreshState.Failed,
            _ => throw new ProtocolValidationException($"Unsupported refreshState '{payload.RefreshState}'."),
        };

        if (payload.SnapshotAvailable)
        {
            if (payload.SnapshotGeneratedAtUtc is null || payload.SnapshotGeneratedAtUtc == default)
            {
                throw new ProtocolValidationException(
                    "snapshotGeneratedAtUtc is required when snapshotAvailable is true."
                );
            }
        }
        else
        {
            if (payload.SnapshotGeneratedAtUtc is not null)
            {
                throw new ProtocolValidationException(
                    "snapshotGeneratedAtUtc must be null when snapshotAvailable is false."
                );
            }

            if (payload.Items.Count != 0)
            {
                throw new ProtocolValidationException(
                    "items must be empty when snapshotAvailable is false."
                );
            }
        }

        if (payload.RefreshRequestedAtUtc is DateTimeOffset requestedAt && requestedAt == default)
        {
            throw new ProtocolValidationException("refreshRequestedAtUtc must be a valid timestamp when provided.");
        }

        if (payload.RefreshCompletedAtUtc is DateTimeOffset completedAt && completedAt == default)
        {
            throw new ProtocolValidationException("refreshCompletedAtUtc must be a valid timestamp when provided.");
        }

        if (refreshState == CatalogRefreshState.Failed)
        {
            if (payload.RefreshErrorCode is not null &&
                !string.Equals(payload.RefreshErrorCode, "refresh_failed", StringComparison.Ordinal))
            {
                throw new ProtocolValidationException(
                    $"Unsupported refreshErrorCode '{payload.RefreshErrorCode}'."
                );
            }
        }
        else if (payload.RefreshErrorCode is not null)
        {
            throw new ProtocolValidationException(
                "refreshErrorCode is only allowed when refreshState is Failed."
            );
        }

        return refreshState;
    }

    public static void ValidateStatusEvent(OperationStatusEventPayload payload)
    {
        if (payload.ProgressPercent is < 0 or > 100)
        {
            throw new ProtocolValidationException("progressPercent must be between 0 and 100 when provided.");
        }

        if (string.IsNullOrWhiteSpace(payload.ItemName))
        {
            throw new ProtocolValidationException("itemName is required for operation status events.");
        }

        if (payload.Action is null)
        {
            throw new ProtocolValidationException("action is required for operation status events.");
        }

        if (payload.State == OperationState.Completed)
        {
            if (payload.Result is null)
            {
                throw new ProtocolValidationException("result is required when state is Completed.");
            }
            if (string.IsNullOrWhiteSpace(payload.Result.Code))
            {
                throw new ProtocolValidationException("result.code is required when state is Completed.");
            }
            return;
        }

        if (payload.Result is not null)
        {
            throw new ProtocolValidationException("result is only allowed when state is Completed.");
        }
    }

    public static void ValidateOperationAccepted(string operationId, OperationAcceptedResponse payload)
    {
        if (!payload.Accepted)
        {
            return;
        }

        if (string.IsNullOrWhiteSpace(operationId))
        {
            throw new ProtocolValidationException("operationId is required when operation is accepted.");
        }

        if (payload.QueuedAtUtc == default)
        {
            throw new ProtocolValidationException("queuedAtUtc is required when operation is accepted.");
        }
    }
}
