using System.Text.Json;
using Gorilla.UI.Client;
using Gorilla.UI.Client.AppCatalog;
using Xunit;

namespace Gorilla.UI.Client.Tests;

public class OperationStatusContractTests
{
    [Fact]
    public void StatusEvent_AllowsOmittedProgressForIndeterminateWork()
    {
        const string json = """
        {
          "state":"Installing",
          "message":"Installing item via managed run",
          "itemName":"Example",
          "action":"Install"
        }
        """;

        var payload = JsonSerializer.Deserialize<OperationStatusEventPayload>(json, ProtocolJson.Options)!;

        ProtocolValidation.ValidateStatusEvent(payload);
        Assert.Null(payload.ProgressPercent);
        Assert.Equal("Example", payload.ItemName);
        Assert.Equal(Gorilla.UI.Client.AppCatalog.Action.Install, payload.Action);
    }

    [Fact]
    public void StatusEvent_DeserializesStructuredTerminalResult()
    {
        const string json = """
        {
          "state":"Failed",
          "message":"Operation failed",
          "errorCode":"postcondition_failed",
          "errorMessage":"Installed state did not satisfy the selected requirement",
          "itemName":"Example",
          "action":"Install",
          "result":{
            "outcome":"Failed",
            "code":"postcondition_failed",
            "detailCode":"version_requirement_unsatisfied",
            "message":"Installed state did not satisfy the selected requirement"
          }
        }
        """;

        var payload = JsonSerializer.Deserialize<OperationStatusEventPayload>(json, ProtocolJson.Options)!;

        ProtocolValidation.ValidateStatusEvent(payload);
        Assert.Null(payload.ProgressPercent);
        Assert.Equal("Example", payload.ItemName);
        Assert.Equal(Gorilla.UI.Client.AppCatalog.Action.Install, payload.Action);
        Assert.NotNull(payload.Result);
        Assert.Equal(Outcome.Failed, payload.Result!.Outcome);
        Assert.Equal("postcondition_failed", payload.Result.Code);
        Assert.Equal("version_requirement_unsatisfied", payload.Result.DetailCode);
    }

    [Fact]
    public void StatusEvent_RejectsOutOfRangeMeasuredProgress()
    {
        var payload = new OperationStatusEventPayload(
            State: OperationState.Downloading,
            ProgressPercent: 101,
            Message: "Downloading package"
        );

        var ex = Assert.Throws<ProtocolValidationException>(() => ProtocolValidation.ValidateStatusEvent(payload));

        Assert.Contains("progressPercent", ex.Message);
    }
}
