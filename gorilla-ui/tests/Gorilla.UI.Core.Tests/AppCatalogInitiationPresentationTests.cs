using Gorilla.UI.Client;
using Gorilla.UI.Client.AppCatalog;
using Gorilla.UI.Core.Models;
using Xunit;
using CatalogAction = Gorilla.UI.Client.AppCatalog.Action;

namespace Gorilla.UI.Core.Tests;

public sealed class AppCatalogInitiationPresentationTests
{
    private static readonly DateTimeOffset Now = DateTimeOffset.Parse("2026-10-02T06:30:00Z");

    [Fact]
    public void InstallInitiation_ShowsIndeterminatePreparingAndDisablesActions()
    {
        var item = Item(ObservedState.Absent, installAllowed: true, removeAllowed: true);
        item.InitiatingAction = CatalogAction.Install;
        item.IsBusy = true;

        var card = item.CardPresentation;

        Assert.True(item.HasCurrentActivity);
        Assert.Equal("Preparing…", card.OperationText);
        Assert.True(card.HasOperation);
        Assert.Null(card.ProgressPercent);
        Assert.True(card.IsProgressIndeterminate);
        Assert.False(card.PrimaryAction?.Enabled);
        Assert.False(card.SecondaryAction?.Enabled);
    }

    [Fact]
    public void RemoveInitiation_UsesExistingDetailsOperationPanel()
    {
        var item = Item(ObservedState.Installed, installAllowed: true, removeAllowed: true);
        item.InitiatingAction = CatalogAction.Remove;
        item.IsBusy = true;

        var details = item.DetailsPresentation;

        Assert.True(details.HasActiveOperation);
        Assert.Equal("Remove in progress", details.ActiveOperationTitle);
        Assert.Equal("Preparing", details.ActiveOperationState);
        Assert.Null(details.ActiveOperationMessage);
        Assert.Null(details.ProgressPercent);
        Assert.True(details.IsProgressIndeterminate);
    }

    [Fact]
    public void UpdateInitiation_KeepsUpdateTitle()
    {
        var item = Item(ObservedState.UpdateAvailable, installAllowed: true);
        item.InitiatingAction = CatalogAction.Install;

        Assert.Equal("Update in progress", item.DetailsPresentation.ActiveOperationTitle);
        Assert.Equal("Preparing", item.DetailsPresentation.ActiveOperationState);
    }

    [Fact]
    public void EnableUpdatesInitiation_KeepsEnableUpdatesTitle()
    {
        var item = Item(ObservedState.Installed, installAllowed: true);
        item.Policy = new Policy(true, false, false, false, Selection.None);
        item.InitiatingAction = CatalogAction.Install;

        Assert.Equal("Enabling updates", item.DetailsPresentation.ActiveOperationTitle);
        Assert.Equal("Preparing", item.DetailsPresentation.ActiveOperationState);
    }

    [Fact]
    public void RealOperation_TakesPrecedenceOverInitiation()
    {
        var item = Item(ObservedState.Absent, installAllowed: true);
        item.InitiatingAction = CatalogAction.Install;
        item.ActiveOperation = new UiOperationPresentation(
            "op-1", CatalogAction.Install, OperationState.Downloading, 37,
            null, "Downloading package", Now);

        Assert.Equal("Downloading…", item.CardPresentation.OperationText);
        Assert.Equal(37, item.CardPresentation.ProgressPercent);
        Assert.Equal("Downloading", item.DetailsPresentation.ActiveOperationState);
        Assert.Equal("Downloading package", item.DetailsPresentation.ActiveOperationMessage);
    }

    [Fact]
    public void Initiation_SuppressesOlderTerminalFailureAndConflictingRetry()
    {
        var item = Item(ObservedState.Absent, installAllowed: true);
        item.LatestOperation = new UiOperationPresentation(
            "old", CatalogAction.Install, OperationState.Completed, null,
            new Result(Outcome.Failed, "execution_failed", Message: "Previous install failed"),
            "Completed", Now.AddMinutes(-5));
        item.InitiatingAction = CatalogAction.Install;

        var card = item.CardPresentation;
        var details = item.DetailsPresentation;

        Assert.Null(card.TerminalFeedbackText);
        Assert.Equal("Previous result: Install — Failed", details.LatestResultHeading);
        Assert.NotNull(details.LatestRecovery);
        Assert.False(details.CanRetryLatest);
    }

    [Fact]
    public void IdleAndServiceOwnedOperationPresentationsRemainUnchanged()
    {
        var idle = Item(ObservedState.Absent);
        Assert.False(idle.HasCurrentActivity);
        Assert.Null(idle.CardPresentation.OperationText);
        Assert.False(idle.DetailsPresentation.HasActiveOperation);

        idle.ActiveOperation = new UiOperationPresentation(
            "op-2", CatalogAction.Remove, OperationState.Removing, null,
            null, "Removing files", Now);
        Assert.Equal("Removing…", idle.CardPresentation.OperationText);
        Assert.Equal("Removing", idle.DetailsPresentation.ActiveOperationState);
    }

    private static UiOptionalInstallItem Item(
        ObservedState state,
        bool installAllowed = false,
        bool removeAllowed = false)
        => new()
        {
            ItemName = "Example",
            DisplayName = "Example",
            TargetVersion = "2.0",
            Observation = new Observation(state, state == ObservedState.Absent ? null : "1.0", Now, "detail", RequirementState.Unknown),
            Policy = new Policy(true, false, false, false, Selection.None),
            InstallDecision = new ActionDecision(installAllowed, installAllowed ? string.Empty : "install_unavailable"),
            RemoveDecision = new ActionDecision(removeAllowed, removeAllowed ? string.Empty : "remove_unavailable"),
        };
}
