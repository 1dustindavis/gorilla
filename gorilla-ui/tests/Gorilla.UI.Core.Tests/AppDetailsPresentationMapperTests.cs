using Gorilla.UI.Client;
using Gorilla.UI.Client.AppCatalog;
using Gorilla.UI.Core.Models;
using Xunit;
using AppCatalog = Gorilla.UI.Client.AppCatalog;

namespace Gorilla.UI.Core.Tests;

public sealed class AppDetailsPresentationMapperTests
{
    private static readonly DateTimeOffset Now = DateTimeOffset.Parse("2026-09-11T20:00:00Z");

    [Fact]
    public void Map_UsesIndependentAvailableAndObservedInstalledVersions()
    {
        var item = Item(ObservedState.UpdateAvailable, installedVersion: "1.7", targetVersion: "2.0");

        var details = AppDetailsPresentationMapper.Map(item);

        Assert.Equal("Update available", details.ObservationText);
        Assert.Equal("2.0", details.AvailableVersion);
        Assert.Equal("1.7", details.InstalledVersion);
    }

    [Fact]
    public void Map_DoesNotInventInstalledVersionFromTargetVersion()
    {
        var item = Item(ObservedState.Installed, installedVersion: null, targetVersion: "2.0");

        var details = AppDetailsPresentationMapper.Map(item);

        Assert.Equal("2.0", details.AvailableVersion);
        Assert.Null(details.InstalledVersion);
    }

    [Fact]
    public void Map_PreservesFullOptionalDescriptionWithoutPlaceholder()
    {
        var described = Item(ObservedState.Absent);
        described.Description = "A full description that belongs on the details surface and is not card-clamped.";
        var undescribed = Item(ObservedState.Absent);

        Assert.Equal(described.Description, AppDetailsPresentationMapper.Map(described).Description);
        Assert.True(AppDetailsPresentationMapper.Map(described).HasDescription);
        Assert.Null(AppDetailsPresentationMapper.Map(undescribed).Description);
        Assert.False(AppDetailsPresentationMapper.Map(undescribed).HasDescription);
    }

    [Fact]
    public void Map_UsesServiceDecisionReasonsForUnavailableActionExplanations()
    {
        var item = Item(ObservedState.Installed);
        item.InstallDecision = new ActionDecision(false, "required_install");
        item.RemoveDecision = new ActionDecision(false, "required_dependency");

        var details = AppDetailsPresentationMapper.Map(item);

        Assert.Equal("Install unavailable: This app is required to stay installed by managed policy.", details.InstallUnavailableExplanation);
        Assert.Equal("Remove unavailable: Another managed app requires this app as a dependency.", details.RemoveUnavailableExplanation);
        Assert.True(details.HasActionExplanation);
    }

    [Fact]
    public void Map_UpdateAvailableWithBothAuthorizedActionsUsesUpdateThenRemove()
    {
        var item = Item(ObservedState.UpdateAvailable);
        item.InstallDecision = new ActionDecision(true, string.Empty);
        item.RemoveDecision = new ActionDecision(true, string.Empty);

        var details = AppDetailsPresentationMapper.Map(item);

        Assert.Equal("Update", details.PrimaryAction?.Label);
        Assert.Equal(CatalogCardActionKind.Install, details.PrimaryAction?.Kind);
        Assert.Equal("Remove", details.SecondaryAction?.Label);
        Assert.Equal(CatalogCardActionKind.Remove, details.SecondaryAction?.Kind);
    }

    [Fact]
    public void Map_InstalledUnselectedWithBothAuthorizedActionsUsesKeepInstalledThenRemove()
    {
        var item = Item(ObservedState.Installed);
        item.Policy = new Policy(true, false, false, false, Selection.None);
        item.InstallDecision = new ActionDecision(true, string.Empty);
        item.RemoveDecision = new ActionDecision(true, string.Empty);

        var details = AppDetailsPresentationMapper.Map(item);

        Assert.Equal("Keep Installed", details.PrimaryAction?.Label);
        Assert.Equal("Remove", details.SecondaryAction?.Label);
    }

    [Fact]
    public void Map_ActiveOperationAndOlderTerminalResultRemainDistinct()
    {
        var item = Item(ObservedState.UpdateAvailable, installedVersion: "1.0", targetVersion: "2.0");
        item.ActiveOperation = new UiOperationPresentation(
            "active", AppCatalog.Action.Install, OperationState.Downloading, 42,
            null, "Downloading package", Now);
        item.LatestOperation = new UiOperationPresentation(
            "old", AppCatalog.Action.Remove, OperationState.Completed, null,
            new Result(Outcome.Failed, "execution_failed", Message: "Previous removal failed"),
            "Completed", Now.AddMinutes(-5));

        var details = AppDetailsPresentationMapper.Map(item);

        Assert.Equal("Update available", details.ObservationText);
        Assert.Equal("Update in progress", details.ActiveOperationTitle);
        Assert.Equal("Downloading", details.ActiveOperationState);
        Assert.Equal(42, details.ProgressPercent);
        Assert.Equal("Previous result: Remove — Failed", details.LatestResultHeading);
        Assert.Equal("Previous removal failed", details.LatestResultDetail);
    }

    [Theory]
    [InlineData(Outcome.Succeeded, "Succeeded")]
    [InlineData(Outcome.AlreadySatisfied, "Already satisfied")]
    [InlineData(Outcome.Failed, "Failed")]
    [InlineData(Outcome.Unverified, "Unable to verify")]
    [InlineData(Outcome.Interrupted, "Interrupted")]
    public void Map_ExposesEveryStructuredTerminalOutcome(Outcome outcome, string label)
    {
        var item = Item(ObservedState.Installed);
        item.LatestOperation = new UiOperationPresentation(
            "terminal", AppCatalog.Action.Install, OperationState.Completed, null,
            new Result(outcome, "result_code", Message: "Result detail"), "Completed", Now);

        var details = AppDetailsPresentationMapper.Map(item);

        Assert.Equal($"Latest result: Install — {label}", details.LatestResultHeading);
        Assert.Equal("Result detail", details.LatestResultDetail);
    }

    private static UiOptionalInstallItem Item(
        ObservedState state,
        string? installedVersion = null,
        string? targetVersion = "2.0")
        => new()
        {
            ItemName = "Example",
            DisplayName = "Example",
            TargetVersion = targetVersion,
            Observation = new Observation(state, installedVersion, Now, "detail", RequirementState.Unknown),
            Policy = new Policy(true, false, false, false, Selection.None),
            InstallDecision = new ActionDecision(false, "install_unavailable"),
            RemoveDecision = new ActionDecision(false, "remove_unavailable"),
        };
}
