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
        if (string.IsNullOrWhiteSpace(item.ItemId))
        {
            throw new ProtocolValidationException("itemId is required.");
        }

        if (item.StatusUpdatedAtUtc == default)
        {
            throw new ProtocolValidationException("statusUpdatedAtUtc is required.");
        }
    }

    public static void ValidateStatusEvent(OperationStatusEventPayload payload)
    {
        if (payload.ProgressPercent is < 0 or > 100)
        {
            throw new ProtocolValidationException("progressPercent must be between 0 and 100.");
        }

        if (payload.State == OperationState.Failed && string.IsNullOrWhiteSpace(payload.ErrorMessage))
        {
            throw new ProtocolValidationException("errorMessage is required when state is Failed.");
        }

        if (payload.State == OperationState.Canceled && string.IsNullOrWhiteSpace(payload.CanceledBy))
        {
            throw new ProtocolValidationException("canceledBy is required when state is Canceled.");
        }
    }
}
