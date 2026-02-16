using System.IO.Pipes;
using System.Runtime.CompilerServices;
using System.Text;
using System.Text.Json;

namespace Gorilla.UI.Client;

public sealed class NamedPipeGorillaServiceClient : IGorillaServiceClient
{
    private readonly NamedPipeClientOptions _options;

    public NamedPipeGorillaServiceClient(NamedPipeClientOptions? options = null)
    {
        _options = options ?? NamedPipeClientOptions.Default;
    }

    public async Task<IReadOnlyList<OptionalInstallItem>> ListOptionalInstallsAsync(CancellationToken cancellationToken)
    {
        var requestEnvelope = CreateRequestEnvelope(
            operation: ProtocolConstants.Operation.ListOptionalInstalls,
            operationId: string.Empty,
            payload: new ListOptionalInstallsRequest()
        );
        ClientDiagnostics.Log($"request:create operation={requestEnvelope.Operation} requestId={requestEnvelope.RequestId}");

        var responseEnvelope = await SendRequestAsync<ListOptionalInstallsRequest, ListOptionalInstallsResponse>(
            requestEnvelope,
            cancellationToken
        );

        var items = responseEnvelope.Payload.Items ?? [];
        foreach (var item in items)
        {
            ProtocolValidation.ValidateOptionalInstallItem(item);
        }

        return items;
    }

    public async Task<OperationAccepted> InstallItemAsync(string itemName, CancellationToken cancellationToken)
    {
        var requestEnvelope = CreateRequestEnvelope(
            operation: ProtocolConstants.Operation.InstallItem,
            operationId: string.Empty,
            payload: new InstallItemRequest(itemName)
        );
        ClientDiagnostics.Log($"request:create operation={requestEnvelope.Operation} requestId={requestEnvelope.RequestId} itemName={itemName}");

        var responseEnvelope = await SendRequestAsync<InstallItemRequest, OperationAcceptedResponse>(
            requestEnvelope,
            cancellationToken
        );

        return new OperationAccepted(
            OperationId: responseEnvelope.OperationId,
            Accepted: responseEnvelope.Payload.Accepted,
            QueuedAtUtc: responseEnvelope.Payload.QueuedAtUtc
        );
    }

    public async Task<OperationAccepted> RemoveItemAsync(string itemName, CancellationToken cancellationToken)
    {
        var requestEnvelope = CreateRequestEnvelope(
            operation: ProtocolConstants.Operation.RemoveItem,
            operationId: string.Empty,
            payload: new RemoveItemRequest(itemName)
        );
        ClientDiagnostics.Log($"request:create operation={requestEnvelope.Operation} requestId={requestEnvelope.RequestId} itemName={itemName}");

        var responseEnvelope = await SendRequestAsync<RemoveItemRequest, OperationAcceptedResponse>(
            requestEnvelope,
            cancellationToken
        );

        return new OperationAccepted(
            OperationId: responseEnvelope.OperationId,
            Accepted: responseEnvelope.Payload.Accepted,
            QueuedAtUtc: responseEnvelope.Payload.QueuedAtUtc
        );
    }

    public async IAsyncEnumerable<OperationStatusEvent> StreamOperationStatusAsync(
        string operationId,
        [EnumeratorCancellation] CancellationToken cancellationToken
    )
    {
        ClientDiagnostics.Log($"stream:begin operationId={operationId}");
        await using var pipe = await ConnectAsync(cancellationToken);

        using var linkedCts = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);
        linkedCts.CancelAfter(_options.RequestTimeout);

        await using var writer = new StreamWriter(pipe, new UTF8Encoding(false), leaveOpen: true) { AutoFlush = true };
        using var reader = new StreamReader(pipe, Encoding.UTF8, detectEncodingFromByteOrderMarks: false, leaveOpen: true);

        var requestEnvelope = CreateRequestEnvelope(
            operation: ProtocolConstants.Operation.StreamOperationStatus,
            operationId: operationId,
            payload: new StreamOperationStatusRequest()
        );

        await writer.WriteLineAsync(JsonSerializer.Serialize(requestEnvelope, ProtocolJson.Options));
        ClientDiagnostics.Log(
            $"stream:request:sent operation={requestEnvelope.Operation} requestId={requestEnvelope.RequestId} operationId={requestEnvelope.OperationId}"
        );

        var ackLine = await reader.ReadLineAsync(linkedCts.Token);
        ClientDiagnostics.Log($"stream:ack:raw {TruncateForLog(ackLine)}");
        if (string.IsNullOrWhiteSpace(ackLine))
        {
            throw new InvalidOperationException("No stream acknowledgement received from service.");
        }

        using (var ackDoc = JsonDocument.Parse(ackLine))
        {
            HandleErrorEnvelopeIfPresent(ackDoc);
            var ackEnvelope = JsonSerializer.Deserialize<ServiceEnvelope<StreamOperationStatusResponse>>(ackLine, ProtocolJson.Options)
                ?? throw new InvalidOperationException("Unable to decode stream acknowledgement envelope.");

            ProtocolValidation.ValidateEnvelopeHeader(ackEnvelope);
            ValidateExpectedEnvelope(
                envelope: ackEnvelope,
                expectedMessageType: ProtocolMessageType.Response,
                expectedOperation: requestEnvelope.Operation,
                expectedRequestId: requestEnvelope.RequestId
            );
            if (!string.Equals(ackEnvelope.OperationId, operationId, StringComparison.Ordinal))
            {
                throw new InvalidOperationException(
                    $"Unexpected operationId in stream acknowledgement. Expected '{operationId}', got '{ackEnvelope.OperationId}'."
                );
            }
            if (!ackEnvelope.Payload.StreamAccepted)
            {
                throw new InvalidOperationException("Service rejected StreamOperationStatus request.");
            }
            ClientDiagnostics.Log($"stream:ack:ok operationId={operationId}");
        }

        while (!cancellationToken.IsCancellationRequested)
        {
            var line = await reader.ReadLineAsync(cancellationToken);
            ClientDiagnostics.Log($"stream:event:raw {TruncateForLog(line)}");
            if (line is null)
            {
                throw new InvalidOperationException("Stream ended before terminal status event was received.");
            }

            if (string.IsNullOrWhiteSpace(line))
            {
                continue;
            }

            using var doc = JsonDocument.Parse(line);
            HandleErrorEnvelopeIfPresent(doc);

            var eventEnvelope = JsonSerializer.Deserialize<ServiceEnvelope<OperationStatusEventPayload>>(line, ProtocolJson.Options)
                ?? throw new InvalidOperationException("Unable to decode stream event envelope.");

            ProtocolValidation.ValidateEnvelopeHeader(eventEnvelope);
            ValidateExpectedEnvelope(
                envelope: eventEnvelope,
                expectedMessageType: ProtocolMessageType.Event,
                expectedOperation: requestEnvelope.Operation,
                expectedRequestId: null
            );
            if (!string.Equals(eventEnvelope.OperationId, operationId, StringComparison.Ordinal))
            {
                throw new InvalidOperationException(
                    $"Unexpected operationId in stream event. Expected '{operationId}', got '{eventEnvelope.OperationId}'."
                );
            }
            ProtocolValidation.ValidateStatusEvent(eventEnvelope.Payload);

            var ev = new OperationStatusEvent(
                OperationId: eventEnvelope.OperationId,
                State: eventEnvelope.Payload.State,
                ProgressPercent: eventEnvelope.Payload.ProgressPercent,
                Message: eventEnvelope.Payload.Message,
                TimestampUtc: eventEnvelope.TimestampUtc,
                ErrorCode: eventEnvelope.Payload.ErrorCode,
                ErrorMessage: eventEnvelope.Payload.ErrorMessage,
                CanceledBy: eventEnvelope.Payload.CanceledBy
            );

            yield return ev;
            ClientDiagnostics.Log($"stream:event operationId={ev.OperationId} state={ev.State} progress={ev.ProgressPercent}");

            if (IsTerminal(ev.State))
            {
                ClientDiagnostics.Log($"stream:end operationId={ev.OperationId} terminalState={ev.State}");
                yield break;
            }
        }
    }

    private static bool IsTerminal(OperationState state) =>
        state is OperationState.Succeeded or OperationState.Failed or OperationState.Canceled;

    private static ServiceEnvelope<TPayload> CreateRequestEnvelope<TPayload>(
        string operation,
        string operationId,
        TPayload payload
    )
    {
        return new ServiceEnvelope<TPayload>(
            Version: ProtocolConstants.Version,
            MessageType: ProtocolMessageType.Request,
            Operation: operation,
            RequestId: Guid.NewGuid().ToString(),
            OperationId: operationId,
            TimestampUtc: DateTimeOffset.UtcNow,
            Payload: payload
        );
    }

    private async Task<ServiceEnvelope<TResponse>> SendRequestAsync<TRequest, TResponse>(
        ServiceEnvelope<TRequest> requestEnvelope,
        CancellationToken cancellationToken
    )
    {
        await using var pipe = await ConnectAsync(cancellationToken);

        using var linkedCts = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);
        linkedCts.CancelAfter(_options.RequestTimeout);

        await using var writer = new StreamWriter(pipe, new UTF8Encoding(false), leaveOpen: true) { AutoFlush = true };
        using var reader = new StreamReader(pipe, Encoding.UTF8, detectEncodingFromByteOrderMarks: false, leaveOpen: true);

        await writer.WriteLineAsync(JsonSerializer.Serialize(requestEnvelope, ProtocolJson.Options));
        ClientDiagnostics.Log(
            $"request:sent operation={requestEnvelope.Operation} requestId={requestEnvelope.RequestId} operationId={requestEnvelope.OperationId}"
        );

        var line = await reader.ReadLineAsync(linkedCts.Token);
        ClientDiagnostics.Log($"response:raw {TruncateForLog(line)}");
        if (string.IsNullOrWhiteSpace(line))
        {
            throw new InvalidOperationException("No response received from service.");
        }

        using var doc = JsonDocument.Parse(line);
        HandleErrorEnvelopeIfPresent(doc);

        var responseEnvelope = JsonSerializer.Deserialize<ServiceEnvelope<TResponse>>(line, ProtocolJson.Options)
            ?? throw new InvalidOperationException("Unable to decode service response envelope.");

        ProtocolValidation.ValidateEnvelopeHeader(responseEnvelope);
        ValidateExpectedEnvelope(
            envelope: responseEnvelope,
            expectedMessageType: ProtocolMessageType.Response,
            expectedOperation: requestEnvelope.Operation,
            expectedRequestId: requestEnvelope.RequestId
        );
        ClientDiagnostics.Log(
            $"response:ok operation={responseEnvelope.Operation} requestId={responseEnvelope.RequestId} operationId={responseEnvelope.OperationId}"
        );

        return responseEnvelope;
    }

    private static void ValidateExpectedEnvelope<TPayload>(
        ServiceEnvelope<TPayload> envelope,
        ProtocolMessageType expectedMessageType,
        string expectedOperation,
        string? expectedRequestId
    )
    {
        if (envelope.MessageType != expectedMessageType)
        {
            throw new InvalidOperationException(
                $"Unexpected messageType. Expected '{expectedMessageType}', got '{envelope.MessageType}'."
            );
        }

        if (!string.Equals(envelope.Operation, expectedOperation, StringComparison.Ordinal))
        {
            throw new InvalidOperationException(
                $"Unexpected operation. Expected '{expectedOperation}', got '{envelope.Operation}'."
            );
        }

        if (expectedRequestId is not null && !string.Equals(envelope.RequestId, expectedRequestId, StringComparison.Ordinal))
        {
            throw new InvalidOperationException(
                $"Unexpected requestId. Expected '{expectedRequestId}', got '{envelope.RequestId}'."
            );
        }
    }

    private static void HandleErrorEnvelopeIfPresent(JsonDocument doc)
    {
        if (!doc.RootElement.TryGetProperty("messageType", out var messageTypeElement))
        {
            return;
        }

        var messageType = messageTypeElement.GetString();
        if (!string.Equals(messageType, ProtocolMessageType.Error.ToString(), StringComparison.OrdinalIgnoreCase))
        {
            return;
        }

        var err = JsonSerializer.Deserialize<ServiceEnvelope<ErrorResponse>>(doc.RootElement.GetRawText(), ProtocolJson.Options)
            ?? throw new InvalidOperationException("Unable to decode error response envelope.");

        if (err.Payload is null)
        {
            throw new InvalidOperationException("Service returned an error envelope with a missing payload.");
        }

        var message = string.IsNullOrWhiteSpace(err.Payload.ErrorMessage)
            ? "Service returned an error response."
            : err.Payload.ErrorMessage;

        throw new InvalidOperationException($"{err.Payload.ErrorCode}: {message}");
    }

    private async Task<NamedPipeClientStream> ConnectAsync(CancellationToken cancellationToken)
    {
        ClientDiagnostics.Log($"connect:start pipe={_options.PipeName}");
        var pipe = new NamedPipeClientStream(
            serverName: ".",
            pipeName: _options.PipeName,
            direction: PipeDirection.InOut,
            options: PipeOptions.Asynchronous
        );

        using var linkedCts = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);
        linkedCts.CancelAfter(_options.ConnectTimeout);

        try
        {
            await pipe.ConnectAsync(linkedCts.Token);
            ClientDiagnostics.Log($"connect:ok pipe={_options.PipeName}");
            return pipe;
        }
        catch (Exception ex)
        {
            await pipe.DisposeAsync();
            ClientDiagnostics.Log($"connect:failed pipe={_options.PipeName} error={ex.GetType().Name}:{ex.Message}");
            throw;
        }
    }

    private static string TruncateForLog(string? value)
    {
        if (value is null)
        {
            return "<null>";
        }

        const int max = 400;
        return value.Length <= max ? value : value[..max] + "...";
    }
}
