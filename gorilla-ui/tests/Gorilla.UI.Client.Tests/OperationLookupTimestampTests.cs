using System.IO.Pipes;
using System.Text.Json;
using Gorilla.UI.Client;
using Xunit;

namespace Gorilla.UI.Client.Tests;

public class OperationLookupTimestampTests
{
    [Theory]
    [InlineData(true)]
    [InlineData(false)]
    public async Task ListOperations_PreservesEventTimeAndSupportsOlderServices(bool includeTimestamp)
    {
        using var cancellation = new CancellationTokenSource(TimeSpan.FromSeconds(10));
        var pipeName = $"gorilla-timestamp-test-{Guid.NewGuid():N}";
        using var server = new NamedPipeServerStream(pipeName, PipeDirection.InOut, 1,
            PipeTransmissionMode.Byte, PipeOptions.Asynchronous);
        var eventTime = DateTimeOffset.Parse("2026-09-10T05:00:00Z");
        var responseTime = eventTime.AddHours(1);
        var serve = RespondAsync();
        var client = new NamedPipeGorillaServiceClient(new NamedPipeClientOptions(
            pipeName, TimeSpan.FromSeconds(5), TimeSpan.FromSeconds(5)));

        var operations = await client.ListOperationsAsync(cancellation.Token);
        await serve;

        Assert.Equal(includeTimestamp ? eventTime : responseTime, Assert.Single(operations).TimestampUtc);

        async Task RespondAsync()
        {
            await server.WaitForConnectionAsync(cancellation.Token);
            using var reader = new StreamReader(server, leaveOpen: true);
            using var writer = new StreamWriter(server, leaveOpen: true) { AutoFlush = true };
            var request = JsonSerializer.Deserialize<ServiceEnvelope<JsonElement>>(
                (await reader.ReadLineAsync(cancellation.Token))!, ProtocolJson.Options)!;
            var status = new OperationStatusEventPayload(OperationState.Installing, null, "Installing",
                "VLC", Gorilla.UI.Client.AppCatalog.Action.Install);
            var response = new ServiceEnvelope<ListOperationsResponse>(ProtocolConstants.Version,
                ProtocolMessageType.Response, request.Operation, request.RequestId, "", responseTime,
                new ListOperationsResponse([new OperationSnapshotPayload("op-1", status,
                    includeTimestamp ? eventTime : null)]));
            await writer.WriteLineAsync(JsonSerializer.Serialize(response, ProtocolJson.Options));
        }
    }
}
