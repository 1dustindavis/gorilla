using System.Text.Json;
using System.Reflection;
using Gorilla.UI.Client;
using Xunit;

namespace Gorilla.UI.Client.Tests;

public class ContractsSmokeTests
{
    [Fact]
    public void OperationState_HasTerminalValues()
    {
        Assert.Contains(OperationState.Succeeded, Enum.GetValues<OperationState>());
        Assert.Contains(OperationState.Failed, Enum.GetValues<OperationState>());
        Assert.Contains(OperationState.Canceled, Enum.GetValues<OperationState>());
    }

    [Fact]
    public void Envelope_RoundTrips_WithProtocolJsonOptions()
    {
        var item = new OptionalInstallItem(
            ItemName: "GoogleChrome",
            DisplayName: "Google Chrome",
            Version: "68.0.3440.106",
            Catalog: "testcatalog",
            InstallerType: "nupkg",
            InstallerPackageId: "GoogleChrome",
            InstallerLocation: "packages/google-chrome/GoogleChrome.68.0.3440.106.nupkg",
            IsManaged: true,
            IsInstalled: false,
            Status: OptionalInstallStatus.NotInstalled,
            StatusUpdatedAtUtc: DateTimeOffset.Parse("2026-02-14T18:10:00Z"),
            LastOperationId: null
        );

        var envelope = new ServiceEnvelope<ListOptionalInstallsResponse>(
            Version: ProtocolConstants.Version,
            MessageType: ProtocolMessageType.Response,
            Operation: ProtocolConstants.Operation.ListOptionalInstalls,
            RequestId: "req-1",
            OperationId: string.Empty,
            TimestampUtc: DateTimeOffset.Parse("2026-02-14T18:10:00Z"),
            Payload: new ListOptionalInstallsResponse(new[] { item })
        );

        var json = JsonSerializer.Serialize(envelope, ProtocolJson.Options);
        var copy = JsonSerializer.Deserialize<ServiceEnvelope<ListOptionalInstallsResponse>>(json, ProtocolJson.Options);

        Assert.NotNull(copy);
        Assert.Equal(ProtocolConstants.Version, copy!.Version);
        Assert.Equal(ProtocolMessageType.Response, copy.MessageType);
        Assert.Equal(ProtocolConstants.Operation.ListOptionalInstalls, copy.Operation);
        Assert.Single(copy.Payload.Items);
        Assert.Equal(OptionalInstallStatus.NotInstalled, copy.Payload.Items[0].Status);
    }

    [Fact]
    public void ValidateEnvelopeHeader_RejectsUnknownOperation()
    {
        var envelope = new ServiceEnvelope<ListOptionalInstallsRequest>(
            Version: ProtocolConstants.Version,
            MessageType: ProtocolMessageType.Request,
            Operation: "UnknownOperation",
            RequestId: "req-2",
            OperationId: string.Empty,
            TimestampUtc: DateTimeOffset.UtcNow,
            Payload: new ListOptionalInstallsRequest()
        );

        var ex = Assert.Throws<ProtocolValidationException>(() => ProtocolValidation.ValidateEnvelopeHeader(envelope));

        Assert.Contains("Unsupported operation", ex.Message);
    }

    [Fact]
    public void ValidateStatusEvent_RejectsOutOfRangeProgress()
    {
        var payload = new OperationStatusEventPayload(
            State: OperationState.Downloading,
            ProgressPercent: 101,
            Message: "Downloading package"
        );

        var ex = Assert.Throws<ProtocolValidationException>(() => ProtocolValidation.ValidateStatusEvent(payload));

        Assert.Contains("progressPercent", ex.Message);
    }

    [Fact]
    public void HandleErrorEnvelopeIfPresent_NullPayload_ThrowsProtocolError()
    {
        using var doc = JsonDocument.Parse(
            "{\"version\":\"v1\",\"messageType\":\"Error\",\"operation\":\"InstallItem\",\"requestId\":\"req-1\",\"operationId\":\"\",\"timestampUtc\":\"2026-02-14T18:10:00Z\",\"payload\":null}"
        );

        var method = typeof(NamedPipeGorillaServiceClient).GetMethod(
            "HandleErrorEnvelopeIfPresent",
            BindingFlags.NonPublic | BindingFlags.Static
        );
        Assert.NotNull(method);

        var ex = Assert.Throws<TargetInvocationException>(() => method!.Invoke(null, new object[] { doc }));
        Assert.NotNull(ex.InnerException);
        Assert.Contains("missing payload", ex.InnerException!.Message);
    }
}
