using Gorilla.UI.Client;
using Gorilla.UI.Client.AppCatalog;
using CatalogAction = Gorilla.UI.Client.AppCatalog.Action;

namespace Gorilla.UI.Core.Models;

public enum CatalogCardActionKind
{
    Install,
    Remove,
}

public sealed record CatalogCardActionPresentation(
    CatalogCardActionKind Kind,
    string Label,
    bool Enabled
);

public sealed record CatalogCardPresentation(
    string ObservationText,
    bool HasDescription,
    string? VersionText,
    string? OperationText,
    string? TerminalFeedbackText,
    int? ProgressPercent,
    CatalogCardActionPresentation? PrimaryAction,
    CatalogCardActionPresentation? SecondaryAction
)
{
    public bool HasVersion => !string.IsNullOrWhiteSpace(VersionText);
    public bool HasOperation => !string.IsNullOrWhiteSpace(OperationText);
    public bool HasTerminalFeedback => !string.IsNullOrWhiteSpace(TerminalFeedbackText);
    public bool IsProgressIndeterminate => ProgressPercent is null;
    public double ProgressValue => ProgressPercent ?? 0;
    public bool HasPrimaryAction => PrimaryAction is not null;
    public bool HasSecondaryAction => SecondaryAction is not null;
}

public static class CatalogCardPresentationMapper
{
    public static CatalogCardPresentation Map(UiOptionalInstallItem item)
    {
        var (primary, secondary) = MapActions(item);
        return new CatalogCardPresentation(
            ObservationText: ObservationText(item.ObservedState),
            HasDescription: !string.IsNullOrWhiteSpace(item.Description),
            VersionText: VersionText(item),
            OperationText: OperationText(item.ActiveOperation),
            TerminalFeedbackText: TerminalFeedbackText(item),
            ProgressPercent: item.ActiveOperation?.ProgressPercent,
            PrimaryAction: primary,
            SecondaryAction: secondary
        );
    }

    private static (CatalogCardActionPresentation? Primary, CatalogCardActionPresentation? Secondary) MapActions(
        UiOptionalInstallItem item
    )
    {
        var install = item.InstallAllowed
            ? new CatalogCardActionPresentation(
                CatalogCardActionKind.Install,
                InstallLabel(item),
                item.CanInstall
            )
            : null;
        var remove = item.RemoveAllowed
            ? new CatalogCardActionPresentation(
                CatalogCardActionKind.Remove,
                "Remove",
                item.CanRemove
            )
            : null;

        if (install is null)
        {
            return (remove, null);
        }
        if (remove is null)
        {
            return (install, null);
        }

        // Allowedness is service-owned. This only chooses presentation hierarchy
        // between two actions the service already authorized.
        return (install, remove);
    }

    private static string InstallLabel(UiOptionalInstallItem item)
    {
        if (item.ObservedState == ObservedState.UpdateAvailable)
        {
            return "Update";
        }

        if (item.ObservedState == ObservedState.Installed && item.Policy?.Selection == Selection.None)
        {
            return "Keep Installed";
        }

        return "Install";
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

    private static string? VersionText(UiOptionalInstallItem item)
    {
        if (string.IsNullOrWhiteSpace(item.TargetVersion))
        {
            return null;
        }

        if (item.ObservedState == ObservedState.UpdateAvailable && !string.IsNullOrWhiteSpace(item.InstalledVersion))
        {
            return $"{item.InstalledVersion} → {item.TargetVersion}";
        }

        return $"Version {item.TargetVersion}";
    }

    private static string? OperationText(UiOperationPresentation? operation)
    {
        if (operation is null)
        {
            return null;
        }

        return operation.State switch
        {
            OperationState.Queued or OperationState.Validating => "Preparing…",
            OperationState.Downloading => "Downloading…",
            OperationState.Installing => "Installing…",
            OperationState.Removing => "Removing…",
            OperationState.Completed => operation.Action == CatalogAction.Remove ? "Removing…" : "Installing…",
            _ => "Working…",
        };
    }

    private static string? TerminalFeedbackText(UiOptionalInstallItem item)
    {
        // A new active attempt supersedes retained terminal feedback from the prior
        // operation. Once the new operation becomes terminal, LatestOperation will
        // replace it with the new authoritative result.
        if (item.ActiveOperation is not null || item.LatestOperation?.Result is not { } result)
        {
            return null;
        }

        var details = OperationDisplay.Details(result);
        return result.Outcome switch
        {
            Outcome.Failed => $"Failed: {details}",
            Outcome.Unverified => $"Unable to verify: {details}",
            Outcome.Interrupted => $"Interrupted: {details}",
            _ => null,
        };
    }
}
