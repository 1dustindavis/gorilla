using System.Text.Json.Serialization;

namespace Gorilla.UI.Client;

[JsonConverter(typeof(JsonStringEnumConverter))]
public enum ProtocolMessageType
{
    Request,
    Response,
    Event,
    Error,
}

public sealed record ServiceEnvelope<TPayload>(
    [property: JsonPropertyName("version")] string Version,
    [property: JsonPropertyName("messageType")] ProtocolMessageType MessageType,
    [property: JsonPropertyName("operation")] string Operation,
    [property: JsonPropertyName("requestId")] string RequestId,
    [property: JsonPropertyName("operationId")] string OperationId,
    [property: JsonPropertyName("timestampUtc")] DateTimeOffset TimestampUtc,
    [property: JsonPropertyName("payload")] TPayload Payload
);
