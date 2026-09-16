using Gorilla.UI.Client;
using Gorilla.UI.Client.AppCatalog;
using CatalogAction = Gorilla.UI.Client.AppCatalog.Action;

namespace Gorilla.UI.Core.Models;

public sealed record AppDetailsPresentation(
    string? Description,
    string ObservationText,
    string? AvailableVersion,
    string? InstalledVersion,
    string? ActiveOperationTitle,
    string? ActiveOperationState,
    string? ActiveOperationMessage,
    int? ProgressPercent,
    string? LatestResultHeading,
    string? LatestResultDetail,
    OperationRecoveryPresentation? LatestRecovery,
    string? LatestOperationId,
    CatalogCardActionPresentation? PrimaryAction,
    CatalogCardActionPresentation? SecondaryAction,
    string? InstallUnavailableExplanation,
    string? RemoveUnavailableExplanation,
    string? ActionFeedbackText
)
{
    public bool HasDescription => !string.IsNullOrWhiteSpace(Description);
    public bool HasAvailableVersion => !string.IsNullOrWhiteSpace(AvailableVersion);
    public bool HasInstalledVersion => !string.IsNullOrWhiteSpace(InstalledVersion);
    public bool HasActiveOperation => !string.IsNullOrWhiteSpace(ActiveOperationTitle);
    public bool HasActiveOperationMessage => !string.IsNullOrWhiteSpace(ActiveOperationMessage);
    public string ActiveOperationAnnouncement => JoinAnnouncement(ActiveOperationState, ActiveOperationMessage);
    public bool IsProgressIndeterminate => ProgressPercent is null;
    public double ProgressValue => ProgressPercent ?? 0;
    public bool HasLatestResult => !string.IsNullOrWhiteSpace(LatestResultHeading);
    public bool HasLatestResultDetail => !string.IsNullOrWhiteSpace(LatestResultDetail);
    public bool HasLatestRecovery => LatestRecovery is not null;
    public bool CanRetryLatest => LatestRecovery?.CanRetry == true;
    public bool HasRetryUnavailableReason => LatestRecovery?.HasRetryUnavailableReason == true;
    public bool HasLatestTechnicalDetails => LatestRecovery?.HasTechnicalDetails == true;
    public string LatestFailureTitle => LatestRecovery?.OutcomeTitle ?? LatestResultHeading ?? string.Empty;
    // Once a terminal result is represented by the recovery model, that model owns
    // primary failure text. A null UserMessage is deliberate: unknown/future detail
    // codes and exception-like diagnostics belong only in Technical details and must
    // not leak back through the legacy LatestResultDetail fallback.
    public string? LatestFailureMessage => LatestRecovery is not null
        ? LatestRecovery.UserMessage
        : LatestResultDetail;
    public bool HasLatestFailureMessage => !string.IsNullOrWhiteSpace(LatestFailureMessage);
    public string LatestResultAnnouncement => JoinAnnouncement(LatestFailureTitle, LatestFailureMessage);
    public string RetryLabel => LatestRecovery?.RetryLabel ?? "Retry";
    public string? RetryUnavailableReason => LatestRecovery?.RetryUnavailableReason;
    public string LatestTechnicalDetails => LatestRecovery?.TechnicalDetails ?? string.Empty;
    public string LatestFailureTitleAutomationId => $"DetailsFailureTitle-{LatestOperationId}";
    public string LatestRetryAutomationId => $"DetailsRetry-{LatestOperationId}";
    public string LatestRetryUnavailableAutomationId => $"DetailsRetryUnavailable-{LatestOperationId}";
    public string LatestTechnicalDetailsAutomationId => $"DetailsTechnicalDetails-{LatestOperationId}";
    public string LatestTechnicalDetailsContentAutomationId => $"DetailsTechnicalDetailsContent-{LatestOperationId}";
    public bool HasPrimaryAction => PrimaryAction is not null;
    public bool HasSecondaryAction => SecondaryAction is not null;
    public bool HasInstallUnavailableExplanation => !string.IsNullOrWhiteSpace(InstallUnavailableExplanation);
    public bool HasRemoveUnavailableExplanation => !string.IsNullOrWhiteSpace(RemoveUnavailableExplanation);
    public bool HasActionExplanation => HasInstallUnavailableExplanation || HasRemoveUnavailableExplanation;
    public bool HasActionFeedback => !string.IsNullOrWhiteSpace(ActionFeedbackText);

    private static string JoinAnnouncement(string? primary, string? detail)
    {
        var first = primary?.Trim();
        var second = detail?.Trim();
        if (string.IsNullOrWhiteSpace(first))
        {
            return second ?? string.Empty;
        }
        if (string.IsNullOrWhiteSpace(second))
        {
            return first;
        }
        return $"{first}. {second}";
    }
}

public static class AppDetailsPresentationMapper
{
    public static AppDetailsPresentation Map(UiOptionalInstallItem item)
    {
        var card = item.CardPresentation;
        var active = item.ActiveOperation;
        var latest = item.LatestOperation;
        OperationRecoveryPresentation? recovery = null;
        if (latest is not null && OperationRecoveryPresentationMapper.IsRetryCandidate(latest.Result, latest.State))
        {
            recovery = OperationRecoveryPresentationMapper.Map(
                latest,
                item,
                hasConflictingActiveOperation: active is not null &&
                    !string.Equals(active.OperationId, latest.OperationId, StringComparison.Ordinal)
            );
        }

        return new AppDetailsPresentation(
            Description: EmptyToNull(item.Description),
            ObservationText: ObservationText(item.ObservedState),
            AvailableVersion: EmptyToNull(item.TargetVersion),
            InstalledVersion: EmptyToNull(item.Observation.InstalledVersion),
            ActiveOperationTitle: active is null ? null : ActiveOperationTitle(item, active.Action),
            ActiveOperationState: active is null ? null : ActiveOperationState(active.State),
            ActiveOperationMessage: active is null ? null : EmptyToNull(active.Message),
            ProgressPercent: active?.ProgressPercent,
            LatestResultHeading: LatestResultHeading(active, latest),
            LatestResultDetail: LatestResultDetail(latest),
            LatestRecovery: recovery,
            LatestOperationId: latest?.OperationId,
            PrimaryAction: card.PrimaryAction,
            SecondaryAction: card.SecondaryAction,
            InstallUnavailableExplanation: Explanation("Install", item.InstallDecision),
            RemoveUnavailableExplanation: Explanation("Remove", item.RemoveDecision),
            ActionFeedbackText: active is null ? EmptyToNull(item.TransientFeedback) : null
        );
    }

    private static string ObservationText(ObservedState state) => state switch
    {
        ObservedState.Absent => "Not installed",
        ObservedState.Installed => "Installed",
        ObservedState.UpdateAvailable => "Update available",
        ObservedState.Unknown => "Status unavailable",
        ObservedState.DetectionFailed => "Unable to determine status",
        _ => "Status unavailable",
    };

    private static string ActiveOperationTitle(UiOptionalInstallItem item, CatalogAction action)
    {
        if (action == CatalogAction.Remove)
        {
            return "Remove in progress";
        }
        if (item.ObservedState == ObservedState.UpdateAvailable)
        {
            return "Update in progress";
        }
        if (item.ObservedState == ObservedState.Installed && item.Policy?.Selection == Selection.None)
        {
            return "Keep Installed in progress";
        }
        return "Install in progress";
    }

    private static string ActiveOperationState(OperationState state) => state switch
    {
        OperationState.Queued => "Queued",
        OperationState.Validating => "Preparing",
        OperationState.Downloading => "Downloading",
        OperationState.Installing => "Installing",
        OperationState.Removing => "Removing",
        OperationState.Completed => "Completing",
        _ => "Working",
    };

    private static string? LatestResultHeading(UiOperationPresentation? active, UiOperationPresentation? latest)
    {
        if (latest?.Result is not { } result)
        {
            return null;
        }

        var prefix = active is null ? "Latest result" : "Previous result";
        return $"{prefix}: {ActionLabel(latest.Action)} — {OutcomeLabel(result.Outcome)}";
    }

    private static string? LatestResultDetail(UiOperationPresentation? latest)
    {
        if (latest?.Result is not { } result)
        {
            return null;
        }

        return EmptyToNull(OperationDisplay.Details(result));
    }

    private static string ActionLabel(CatalogAction action) => action switch
    {
        CatalogAction.Remove => "Remove",
        _ => "Install",
    };

    private static string OutcomeLabel(Outcome outcome) => outcome switch
    {
        Outcome.Succeeded => "Succeeded",
        Outcome.AlreadySatisfied => "Already satisfied",
        Outcome.Failed => "Failed",
        Outcome.Unverified => "Unable to verify",
        Outcome.Interrupted => "Interrupted",
        _ => outcome.ToString(),
    };

    private static string? Explanation(string actionLabel, ActionDecision decision)
    {
        if (decision.Allowed || string.IsNullOrWhiteSpace(decision.Reason))
        {
            return null;
        }

        return $"{actionLabel} unavailable: {OperationRecoveryPresentationMapper.ReasonText(decision.Reason)}";
    }

    private static string? EmptyToNull(string? value)
        => string.IsNullOrWhiteSpace(value) ? null : value;
}
