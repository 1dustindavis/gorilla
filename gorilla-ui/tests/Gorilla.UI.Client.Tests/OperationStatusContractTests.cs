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
        Assert.Null(payload.Result);
    }

    [Fact]
    public void StatusEvent_DeserializesStructuredCompletedResult()
    {
        const string json = """
        {
          "state":"Completed",
          "message":"Installed state did not satisfy the selected requirement",
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
        Assert.Equal(OperationState.Completed, payload.State);
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
            Message: "Downloading package",
            ItemName: "Example",
            Action: Gorilla.UI.Client.AppCatalog.Action.Install
        );

        var ex = Assert.Throws<ProtocolValidationException>(() => ProtocolValidation.ValidateStatusEvent(payload));

        Assert.Contains("progressPercent", ex.Message);
    }

    [Fact]
    public void StatusEvent_RejectsMissingItemIdentity()
    {
        var payload = new OperationStatusEventPayload(
            State: OperationState.Installing,
            ProgressPercent: null,
            Message: "Installing",
            Action: Gorilla.UI.Client.AppCatalog.Action.Install
        );

        var ex = Assert.Throws<ProtocolValidationException>(() => ProtocolValidation.ValidateStatusEvent(payload));

        Assert.Contains("itemName", ex.Message);
    }

    [Fact]
    public void StatusEvent_RejectsMissingActionIdentity()
    {
        var payload = new OperationStatusEventPayload(
            State: OperationState.Installing,
            ProgressPercent: null,
            Message: "Installing",
            ItemName: "Example"
        );

        var ex = Assert.Throws<ProtocolValidationException>(() => ProtocolValidation.ValidateStatusEvent(payload));

        Assert.Contains("action", ex.Message);
    }

    [Fact]
    public void StatusEvent_RejectsCompletedWithoutResult()
    {
        var payload = new OperationStatusEventPayload(
            State: OperationState.Completed,
            ProgressPercent: null,
            Message: "Done",
            ItemName: "Example",
            Action: Gorilla.UI.Client.AppCatalog.Action.Install
        );

        var ex = Assert.Throws<ProtocolValidationException>(() => ProtocolValidation.ValidateStatusEvent(payload));

        Assert.Contains("result is required", ex.Message);
    }

    [Fact]
    public void StatusEvent_RejectsResultBeforeCompleted()
    {
        var payload = new OperationStatusEventPayload(
            State: OperationState.Installing,
            ProgressPercent: null,
            Message: "Installing",
            ItemName: "Example",
            Action: Gorilla.UI.Client.AppCatalog.Action.Install,
            Result: new Result(Outcome.Succeeded, "completed")
        );

        var ex = Assert.Throws<ProtocolValidationException>(() => ProtocolValidation.ValidateStatusEvent(payload));

        Assert.Contains("only allowed", ex.Message);
    }

    [Fact]
    public void StatusEvent_RejectsCompletedResultWithoutCode()
    {
        var payload = new OperationStatusEventPayload(
            State: OperationState.Completed,
            ProgressPercent: null,
            Message: "Done",
            ItemName: "Example",
            Action: Gorilla.UI.Client.AppCatalog.Action.Install,
            Result: new Result(Outcome.Succeeded, "")
        );

        var ex = Assert.Throws<ProtocolValidationException>(() => ProtocolValidation.ValidateStatusEvent(payload));

        Assert.Contains("result.code", ex.Message);
    }
}
