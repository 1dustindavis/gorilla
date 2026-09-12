using Gorilla.UI.Client;
using Gorilla.UI.Client.AppCatalog;
using CatalogAction = Gorilla.UI.Client.AppCatalog.Action;

namespace Gorilla.UI.Core.Models;

public sealed record AppDetailsPresentation(
    string ObservationText,
    string? AvailableVersion,
    string? InstalledVersion,
    string? ActiveOperationTitle,
    string? ActiveOperationState,
    string? ActiveOperationMessage,
    int? ProgressPercent,
    string? LatestResultHeading,
    string? LatestResultDetail,
    CatalogCardActionPresentation? PrimaryAction,
    CatalogCardActionPresentation? SecondaryAction,
    string? InstallUnavailableExplanation,
    string? RemoveUnavailableExplanation,
    string? ActionFeedbackText
)
{
    public bool HasAvailableVersion => !string.IsNullOrWhiteSpace(AvailableVersion);
    public bool HasInstalledVersion => !string.IsNullOrWhiteSpace(InstalledVersion);
    public bool HasActiveOperation => !string.IsNullOrWhiteSpace(ActiveOperationTitle);
    public bool HasActiveOperationMessage => !string.IsNullOrWhiteSpace(ActiveOperationMessage);
    public bool IsProgressIndeterminate => ProgressPercent is null;
    public double ProgressValue => ProgressPercent ?? 0;
    public bool HasLatestResult => !string.IsNullOrWhiteSpace(LatestResultHeading);
    public bool HasLatestResultDetail => !string.IsNullOrWhiteSpace(LatestResultDetail);
    public bool HasPrimaryAction => PrimaryAction is not null;
    public bool HasSecondaryAction => SecondaryAction is not null;
    public bool HasInstallUnavailableExplanation => !string.IsNullOrWhiteSpace(InstallUnavailableExplanation);
    public bool HasRemoveUnavailableExplanation => !string.IsNullOrWhiteSpace(RemoveUnavailableExplanation);
    public bool HasActionExplanation => HasInstallUnavailableExplanation || HasRemoveUnavailableExplanation;
    public bool HasActionFeedback => !string.IsNullOrWhiteSpace(ActionFeedbackText);
}

public static class AppDetailsPresentationMapper
{
    public static AppDetailsPresentation Map(UiOptionalInstallItem item)
    {
        var card = item.CardPresentation;
        var active = item.ActiveOperation;
        var latest = item.LatestOperation;

        return new AppDetailsPresentation(
            ObservationText: ObservationText(item.ObservedState),
            AvailableVersion: EmptyToNull(item.TargetVersion),
            InstalledVersion: EmptyToNull(item.Observation.InstalledVersion),
            ActiveOperationTitle: active is null ? null : ActiveOperationTitle(active.Action),
            ActiveOperationState: active is null ? null : ActiveOperationState(active.State),
            ActiveOperationMessage: active is null ? null : EmptyToNull(active.Message),
            ProgressPercent: active?.ProgressPercent,
            LatestResultHeading: LatestResultHeading(active, latest),
            LatestResultDetail: LatestResultDetail(latest),
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

    private static string ActiveOperationTitle(CatalogAction action) => action switch
    {
        CatalogAction.Remove => "Remove in progress",
        _ => "Install in progress",
    };

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

    private static string? LatestResultHeading(
        UiOperationPresentation? active,
        UiOperationPresentation? latest
    )
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

        return $"{actionLabel} unavailable: {ReasonText(decision.Reason)}";
    }

    // Service reason codes remain authoritative. This is intentionally only a
    // deterministic wording map; it never inspects policy or observation to infer
    // a different decision.
    private static string ReasonText(string reason) => reason switch
    {
        "not_optional" => "This app is not optional software.",
        "policy_conflict" => "Conflicting managed policy currently prevents this action.",
        "managed_uninstall" => "This app is required to be removed by managed policy.",
        "required_install" => "This app is required to stay installed by managed policy.",
        "operation_active" => "Another operation for this app is already active.",
        "invalid_selection" => "The app's current managed selection does not permit this action.",
        "state_unknown" => "The current app state is unavailable.",
        "detection_failed" => "Gorilla could not determine the current app state.",
        "install_unavailable" => "No supported install action is available.",
        "already_selected" => "This app is already selected to stay installed and updated.",
        "required_dependency" => "Another managed app requires this app as a dependency.",
        "already_absent" => "This app is already absent and is not selected to stay installed.",
        "remove_unavailable" => "No supported remove action is available.",
        _ => reason,
    };

    private static string? EmptyToNull(string? value)
        => string.IsNullOrWhiteSpace(value) ? null : value;
}
