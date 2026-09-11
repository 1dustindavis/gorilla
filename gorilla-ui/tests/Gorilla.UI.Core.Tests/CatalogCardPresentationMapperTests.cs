using Gorilla.UI.Client;
using Gorilla.UI.Client.AppCatalog;
using Gorilla.UI.Core.Models;
using Xunit;
using CatalogAction = Gorilla.UI.Client.AppCatalog.Action;

namespace Gorilla.UI.Core.Tests;

public sealed class CatalogCardPresentationMapperTests
{
    [Fact]
    public void Absent_WithInstallOnly_PresentsOnePrimaryInstallAction()
    {
        var item = Item(ObservedState.Absent, installAllowed: true, removeAllowed: false);

        var presentation = CatalogCardPresentationMapper.Map(item);

        Assert.Equal("Install", presentation.PrimaryAction?.Label);
        Assert.Equal(CatalogCardActionKind.Install, presentation.PrimaryAction?.Kind);
        Assert.Null(presentation.SecondaryAction);
    }

    [Fact]
    public void Installed_WithRemoveOnly_PresentsOnePrimaryRemoveAction()
    {
        var item = Item(ObservedState.Installed, installAllowed: false, removeAllowed: true);

        var presentation = CatalogCardPresentationMapper.Map(item);

        Assert.Equal("Remove", presentation.PrimaryAction?.Label);
        Assert.Equal(CatalogCardActionKind.Remove, presentation.PrimaryAction?.Kind);
        Assert.Null(presentation.SecondaryAction);
    }

    [Fact]
    public void UpdateAvailable_WithBothAllowed_PresentsUpdateThenRemove()
    {
        var item = Item(ObservedState.UpdateAvailable, installAllowed: true, removeAllowed: true);

        var presentation = CatalogCardPresentationMapper.Map(item);

        Assert.Equal("Update", presentation.PrimaryAction?.Label);
        Assert.Equal(CatalogCardActionKind.Install, presentation.PrimaryAction?.Kind);
        Assert.Equal("Remove", presentation.SecondaryAction?.Label);
        Assert.Equal(CatalogCardActionKind.Remove, presentation.SecondaryAction?.Kind);
    }

    [Fact]
    public void InstalledUnselected_WithBothAllowed_PresentsKeepInstalledThenRemove()
    {
        var item = Item(ObservedState.Installed, installAllowed: true, removeAllowed: true);
        item.Policy = new Policy(
            Optional: true,
            RequiredInstall: false,
            RequiredUninstall: false,
            RequiredDependency: false,
            Selection: Selection.None
        );

        var presentation = CatalogCardPresentationMapper.Map(item);

        Assert.Equal("Keep Installed", presentation.PrimaryAction?.Label);
        Assert.Equal(CatalogCardActionKind.Install, presentation.PrimaryAction?.Kind);
        Assert.Equal("Remove", presentation.SecondaryAction?.Label);
    }

    [Fact]
    public void NeitherAllowed_PresentsNoAction()
    {
        var presentation = CatalogCardPresentationMapper.Map(
            Item(ObservedState.Installed, installAllowed: false, removeAllowed: false)
        );

        Assert.Null(presentation.PrimaryAction);
        Assert.Null(presentation.SecondaryAction);
    }

    [Fact]
    public void BusyItem_PreservesActionsDisabledAndShowsOperationSeparately()
    {
        var item = Item(ObservedState.Installed, installAllowed: true, removeAllowed: true);
        item.IsBusy = true;
        item.ActiveOperation = new UiOperationPresentation(
            OperationId: "op-1",
            Action: CatalogAction.Remove,
            State: OperationState.Removing,
            ProgressPercent: null,
            Result: null,
            Message: "Removing files",
            TimestampUtc: DateTimeOffset.Parse("2026-09-11T15:00:00Z")
        );

        var presentation = CatalogCardPresentationMapper.Map(item);

        Assert.Equal("Installed", presentation.ObservationText);
        Assert.Equal("Removing…", presentation.OperationText);
        Assert.True(presentation.IsProgressIndeterminate);
        Assert.False(presentation.PrimaryAction!.Enabled);
        Assert.False(presentation.SecondaryAction!.Enabled);
    }

    [Theory]
    [InlineData(Outcome.Failed, "Installer exited with code 1", "Failed: Installer exited with code 1")]
    [InlineData(Outcome.Unverified, "Unable to confirm installed state", "Unable to verify: Unable to confirm installed state")]
    [InlineData(Outcome.Interrupted, "Service operation was interrupted", "Interrupted: Service operation was interrupted")]
    public void TerminalProblem_IsPresentedOnCard(
        Outcome outcome,
        string message,
        string expected
    )
    {
        var item = Item(ObservedState.Absent, installAllowed: true, removeAllowed: false);
        item.LatestOperation = TerminalOperation(outcome, message);

        var presentation = CatalogCardPresentationMapper.Map(item);

        Assert.Equal(expected, presentation.TerminalFeedbackText);
        Assert.True(presentation.HasTerminalFeedback);
    }

    [Theory]
    [InlineData(Outcome.Succeeded)]
    [InlineData(Outcome.AlreadySatisfied)]
    public void SuccessfulTerminalOutcome_DoesNotAddCardFeedback(Outcome outcome)
    {
        var item = Item(ObservedState.Installed, installAllowed: false, removeAllowed: true);
        item.LatestOperation = TerminalOperation(outcome, "Done");

        var presentation = CatalogCardPresentationMapper.Map(item);

        Assert.Null(presentation.TerminalFeedbackText);
        Assert.False(presentation.HasTerminalFeedback);
    }

    [Fact]
    public void NewActiveOperation_SuppressesStaleTerminalFeedback()
    {
        var item = Item(ObservedState.Absent, installAllowed: true, removeAllowed: false);
        item.LatestOperation = TerminalOperation(Outcome.Failed, "Previous failure");
        item.ActiveOperation = new UiOperationPresentation(
            OperationId: "op-new",
            Action: CatalogAction.Install,
            State: OperationState.Installing,
            ProgressPercent: null,
            Result: null,
            Message: "Installing",
            TimestampUtc: DateTimeOffset.Parse("2026-09-11T15:01:00Z")
        );

        var presentation = CatalogCardPresentationMapper.Map(item);

        Assert.Equal("Installing…", presentation.OperationText);
        Assert.Null(presentation.TerminalFeedbackText);
    }

    [Fact]
    public void UpdateVersion_UsesProvidedVersionsWithoutComparingThem()
    {
        var item = Item(ObservedState.UpdateAvailable, installAllowed: true, removeAllowed: true);
        item.Observation = item.Observation with { InstalledVersion = "1.7" };
        item.TargetVersion = "2.0";

        var presentation = CatalogCardPresentationMapper.Map(item);

        Assert.Equal("1.7 → 2.0", presentation.VersionText);
    }

    private static UiOperationPresentation TerminalOperation(Outcome outcome, string message) => new(
        OperationId: "op-terminal",
        Action: CatalogAction.Install,
        State: OperationState.Completed,
        ProgressPercent: null,
        Result: new Result(outcome, "terminal", Message: message),
        Message: message,
        TimestampUtc: DateTimeOffset.Parse("2026-09-11T15:00:00Z")
    );

    private static UiOptionalInstallItem Item(
        ObservedState observedState,
        bool installAllowed,
        bool removeAllowed
    ) => new()
    {
        ItemName = "Fixture",
        DisplayName = "Fixture App",
        Observation = new Observation(
            observedState,
            InstalledVersion: null,
            CheckedAtUtc: null,
            DetailCode: string.Empty,
            InstallRequirement: RequirementState.Unknown
        ),
        InstallDecision = new ActionDecision(installAllowed, string.Empty),
        RemoveDecision = new ActionDecision(removeAllowed, string.Empty),
    };
}
