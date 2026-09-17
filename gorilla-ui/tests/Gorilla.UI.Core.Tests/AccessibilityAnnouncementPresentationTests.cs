using Gorilla.UI.Core.Models;
using Xunit;

namespace Gorilla.UI.Core.Tests;

public sealed class AccessibilityAnnouncementPresentationTests
{
    [Fact]
    public void DetailsActiveAnnouncementChangesWhenOnlyMessageChanges()
    {
        var before = Details(activeState: "Installing", activeMessage: "Phase one");
        var after = before with { ActiveOperationMessage = "Phase two" };

        Assert.Equal(before.ActiveOperationState, after.ActiveOperationState);
        Assert.Equal("Installing. Phase one", before.ActiveOperationAnnouncement);
        Assert.Equal("Installing. Phase two", after.ActiveOperationAnnouncement);
    }

    [Fact]
    public void DetailsTerminalAnnouncementContainsTitleAndUserExplanation()
    {
        var recovery = new OperationRecoveryPresentation(
            IsRetryCandidate: true,
            OutcomeTitle: "Installation failed",
            UserMessage: "Installation error: exit status 7",
            TechnicalDetails: "technical",
            CanRetry: true,
            RetryLabel: "Retry",
            RetryUnavailableReason: null
        );
        var details = Details(activeState: null, activeMessage: null) with
        {
            LatestResultHeading = "Latest result: Install — Failed",
            LatestRecovery = recovery,
        };

        Assert.Equal(
            "Installation failed. Installation error: exit status 7",
            details.LatestResultAnnouncement
        );
    }

    private static AppDetailsPresentation Details(string? activeState, string? activeMessage)
        => new(
            Description: null,
            ObservationText: "Not installed",
            AvailableVersion: "1.0.0",
            InstalledVersion: null,
            ActiveOperationTitle: activeState is null ? null : "Install in progress",
            ActiveOperationState: activeState,
            ActiveOperationMessage: activeMessage,
            ProgressPercent: null,
            LatestResultHeading: null,
            LatestResultDetail: null,
            LatestRecovery: null,
            LatestOperationId: null,
            PrimaryAction: null,
            SecondaryAction: null,
            InstallUnavailableExplanation: null,
            RemoveUnavailableExplanation: null,
            ActionFeedbackText: null
        );
}
