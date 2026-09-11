using Gorilla.UI.Client;
using Gorilla.UI.Client.AppCatalog;
using Gorilla.UI.Core.Models;
using Xunit;
using CatalogAction = Gorilla.UI.Client.AppCatalog.Action;

namespace Gorilla.UI.Core.Tests;

public class CatalogCardTransientFeedbackTests
{
    [Fact]
    public void TransientActionFeedback_UsesCardResultSlotAndIsSuppressedByActiveWork()
    {
        var item = new UiOptionalInstallItem
        {
            ItemName = "Example",
            DisplayName = "Example",
            Observation = new Observation(
                ObservedState.Absent,
                InstalledVersion: null,
                CheckedAtUtc: DateTimeOffset.Parse("2026-09-11T20:00:00Z"),
                DetailCode: string.Empty,
                InstallRequirement: RequirementState.Unknown
            ),
            InstallDecision = new ActionDecision(true, string.Empty),
            RemoveDecision = new ActionDecision(false, "already_absent"),
        };

        item.TransientFeedback = "Install was not accepted for Example.";

        Assert.Equal(
            "Install was not accepted for Example.",
            item.CardPresentation.TerminalFeedbackText
        );
        Assert.True(item.CardPresentation.HasTerminalFeedback);

        item.ActiveOperation = new UiOperationPresentation(
            "op-1",
            CatalogAction.Install,
            OperationState.Installing,
            ProgressPercent: null,
            Result: null,
            Message: "Installing",
            TimestampUtc: DateTimeOffset.Parse("2026-09-11T20:00:01Z")
        );

        Assert.Null(item.CardPresentation.TerminalFeedbackText);
        Assert.False(item.CardPresentation.HasTerminalFeedback);
        Assert.Equal("Installing…", item.CardPresentation.OperationText);
    }

    [Fact]
    public void TransientActionFeedback_SupersedesRetainedTerminalFeedbackUntilCleared()
    {
        var item = new UiOptionalInstallItem
        {
            ItemName = "Example",
            DisplayName = "Example",
            Observation = new Observation(
                ObservedState.Absent,
                InstalledVersion: null,
                CheckedAtUtc: DateTimeOffset.Parse("2026-09-11T20:00:00Z"),
                DetailCode: string.Empty,
                InstallRequirement: RequirementState.Unknown
            ),
        };
        item.LatestOperation = new UiOperationPresentation(
            "op-old",
            CatalogAction.Install,
            OperationState.Completed,
            ProgressPercent: null,
            Result: new Result(Outcome.Failed, "execution_failed", Message: "old failure"),
            Message: "old failure",
            TimestampUtc: DateTimeOffset.Parse("2026-09-11T19:59:00Z")
        );

        item.TransientFeedback = "Install was not accepted for Example.";
        Assert.Equal("Install was not accepted for Example.", item.CardPresentation.TerminalFeedbackText);

        item.TransientFeedback = null;
        Assert.Equal("Failed: old failure", item.CardPresentation.TerminalFeedbackText);
    }
}
