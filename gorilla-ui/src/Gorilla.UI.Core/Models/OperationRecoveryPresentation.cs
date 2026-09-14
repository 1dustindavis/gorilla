using System.Globalization;
using System.Text;
using Gorilla.UI.Client;
using Gorilla.UI.Client.AppCatalog;
using CatalogAction = Gorilla.UI.Client.AppCatalog.Action;

namespace Gorilla.UI.Core.Models;

public sealed record OperationRecoveryPresentation(
    bool IsRetryCandidate,
    string OutcomeTitle,
    string? UserMessage,
    string TechnicalDetails,
    bool CanRetry,
    string RetryLabel,
    string? RetryUnavailableReason
)
{
    public bool HasUserMessage => !string.IsNullOrWhiteSpace(UserMessage);
    public bool HasTechnicalDetails => !string.IsNullOrWhiteSpace(TechnicalDetails);
    public bool HasRetryUnavailableReason => !CanRetry && !string.IsNullOrWhiteSpace(RetryUnavailableReason);
}

public static class OperationRecoveryPresentationMapper
{
    private const int MaxPrimaryMessageLength = 300;

    private static readonly HashSet<string> KnownResultCodes = new(StringComparer.Ordinal)
    {
        "execution_failed",
        "postcondition_failed",
        "verification_unavailable",
        "execution_unknown",
        "execution_interrupted",
    };

    private static readonly HashSet<string> KnownDetailCodes = new(StringComparer.Ordinal)
    {
        "installer_failed",
        "installer_exit",
        "dependency_failed",
        "dependency_cycle",
        "invalid_dependency",
        "managed_run_failed",
        "service_canceled",
        "catalog_refresh_failed",
        "catalog_item_unavailable",
        "selection_read_failed",
        "install_selection_not_persisted",
        "install_selection_not_cleared",
        "persistent_uninstall_present",
        "requirement_unknown",
        "absence_unknown",
        "unsupported_action",
    };

    public static OperationRecoveryPresentation Map(
        UiOperationPresentation operation,
        UiOptionalInstallItem? currentItem,
        bool hasConflictingActiveOperation
    ) => MapCore(
        operation.OperationId,
        currentItem?.DisplayName,
        currentItem?.ItemName ?? string.Empty,
        operation.Action,
        operation.State,
        operation.Result,
        operation.Message,
        operation.TimestampUtc,
        currentItem,
        hasConflictingActiveOperation
    );

    public static OperationRecoveryPresentation Map(
        OperationStatusEvent operation,
        UiOptionalInstallItem? currentItem,
        bool hasConflictingActiveOperation
    ) => MapCore(
        operation.OperationId,
        currentItem?.DisplayName,
        operation.ItemName,
        operation.Action,
        operation.State,
        operation.Result,
        operation.Message,
        operation.TimestampUtc,
        currentItem,
        hasConflictingActiveOperation
    );

    private static OperationRecoveryPresentation MapCore(
        string operationId,
        string? displayName,
        string itemName,
        CatalogAction action,
        OperationState state,
        Result? result,
        string operationMessage,
        DateTimeOffset timestampUtc,
        UiOptionalInstallItem? currentItem,
        bool hasConflictingActiveOperation
    )
    {
        var outcome = result?.Outcome;
        var isCandidate = state == OperationState.Completed && outcome is
            Outcome.Failed or Outcome.Unverified or Outcome.Interrupted;

        var actionDecision = currentItem is null
            ? null
            : action == CatalogAction.Remove ? currentItem.RemoveDecision : currentItem.InstallDecision;
        var localRetryBlock = currentItem?.RetryBlockReasonFor(operationId);

        string? unavailableReason = null;
        var canRetry = false;
        if (isCandidate)
        {
            if (currentItem is null)
            {
                unavailableReason = "This app is no longer available in the current catalog.";
            }
            else if (hasConflictingActiveOperation || currentItem.IsBusy)
            {
                unavailableReason = "Another operation for this app is already active.";
            }
            else if (!string.IsNullOrWhiteSpace(localRetryBlock))
            {
                unavailableReason = localRetryBlock;
            }
            else if (actionDecision is not { Allowed: true })
            {
                unavailableReason = actionDecision is null
                    ? "Refresh the catalog to determine whether this action is currently available."
                    : ReasonText(actionDecision.Reason);
            }
            else
            {
                canRetry = true;
            }
        }

        return new OperationRecoveryPresentation(
            IsRetryCandidate: isCandidate,
            OutcomeTitle: OutcomeTitle(action, outcome),
            UserMessage: UserMessage(result, operationMessage),
            TechnicalDetails: TechnicalDetails(
                operationId,
                displayName,
                string.IsNullOrWhiteSpace(itemName) ? currentItem?.ItemName ?? string.Empty : itemName,
                action,
                state,
                result,
                operationMessage,
                timestampUtc
            ),
            CanRetry: canRetry,
            RetryLabel: "Retry",
            RetryUnavailableReason: unavailableReason
        );
    }

    public static bool IsRetryCandidate(Result? result, OperationState state)
        => state == OperationState.Completed && result?.Outcome is
            Outcome.Failed or Outcome.Unverified or Outcome.Interrupted;

    public static string ReasonText(string reason) => reason switch
    {
        "not_optional" => "This app is not optional software.",
        "policy_conflict" => "Conflicting managed policy currently prevents this action.",
        "managed_uninstall" => "This app is required to be removed by managed policy.",
        "required_install" => "This app is required to stay installed by managed policy.",
        "operation_active" => "Another operation for this app is already active.",
        "invalid_selection" => "The app's current managed selection does not permit this action.",
        "state_unknown" => "The current app state is unavailable.",
        "detection_failed" => "Gorilla could not determine the current app state.",
        "install_unavailable" => "No supported install action is currently available.",
        "already_selected" => "This app is already selected to stay installed and updated.",
        "required_dependency" => "Another managed app requires this app as a dependency.",
        "already_absent" => "This app is already absent and is not selected to stay installed.",
        "remove_unavailable" => "No supported remove action is currently available.",
        "Refresh required before installing." => "Refresh the catalog before trying to install this app.",
        "Refresh required before removing." => "Refresh the catalog before trying to remove this app.",
        _ when string.IsNullOrWhiteSpace(reason) => "This action is not currently available.",
        _ => reason,
    };

    private static string OutcomeTitle(CatalogAction action, Outcome? outcome)
    {
        var noun = action == CatalogAction.Remove ? "Removal" : "Installation";
        return outcome switch
        {
            Outcome.Failed => $"{noun} failed",
            Outcome.Unverified => $"{noun} couldn't be verified as successful",
            Outcome.Interrupted => $"{noun} was interrupted before confirmation",
            Outcome.Succeeded => $"{noun} succeeded",
            Outcome.AlreadySatisfied => $"{noun} was already satisfied",
            _ => $"{noun} completed",
        };
    }

    private static string? UserMessage(Result? result, string operationMessage)
    {
        if (result is null || !KnownResultCodes.Contains(result.Code))
        {
            return null;
        }

        if (!string.IsNullOrWhiteSpace(result.DetailCode) && !KnownDetailCodes.Contains(result.DetailCode))
        {
            return null;
        }

        return ConciseUserMessage(result.Message) ??
            (string.IsNullOrWhiteSpace(result.Message) ? ConciseUserMessage(operationMessage) : null);
    }

    private static string? ConciseUserMessage(string? message)
    {
        if (string.IsNullOrWhiteSpace(message))
        {
            return null;
        }

        var trimmed = message.Trim();
        if (trimmed.Length > MaxPrimaryMessageLength ||
            trimmed.Contains('\r', StringComparison.Ordinal) ||
            trimmed.Contains('\n', StringComparison.Ordinal) ||
            LooksLikeException(trimmed))
        {
            return null;
        }

        return trimmed;
    }

    private static bool LooksLikeException(string message)
        => message.Contains("Exception:", StringComparison.OrdinalIgnoreCase) ||
           message.StartsWith("System.", StringComparison.Ordinal) ||
           message.Contains(" at Gorilla.", StringComparison.Ordinal);

    private static string TechnicalDetails(
        string operationId,
        string? displayName,
        string itemName,
        CatalogAction action,
        OperationState state,
        Result? result,
        string operationMessage,
        DateTimeOffset timestampUtc
    )
    {
        var text = new StringBuilder();
        text.AppendLine("Gorilla App Catalog operation");
        if (!string.IsNullOrWhiteSpace(displayName))
        {
            text.AppendLine(CultureInfo.InvariantCulture, $"App: {displayName}");
        }
        text.AppendLine(CultureInfo.InvariantCulture, $"Item: {itemName}");
        text.AppendLine(CultureInfo.InvariantCulture, $"Action: {ActionLabel(action)}");
        text.AppendLine(CultureInfo.InvariantCulture, $"Operation ID: {operationId}");
        text.AppendLine(CultureInfo.InvariantCulture, $"State: {state}");
        if (result is not null)
        {
            text.AppendLine(CultureInfo.InvariantCulture, $"Outcome: {result.Outcome}");
            if (!string.IsNullOrWhiteSpace(result.Code))
            {
                text.AppendLine(CultureInfo.InvariantCulture, $"Code: {result.Code}");
            }
            if (!string.IsNullOrWhiteSpace(result.DetailCode))
            {
                text.AppendLine(CultureInfo.InvariantCulture, $"Detail code: {result.DetailCode}");
            }
            if (!string.IsNullOrWhiteSpace(result.Message))
            {
                text.AppendLine(CultureInfo.InvariantCulture, $"Result message: {result.Message}");
            }
        }
        if (!string.IsNullOrWhiteSpace(operationMessage) &&
            !string.Equals(operationMessage, result?.Message, StringComparison.Ordinal))
        {
            text.AppendLine(CultureInfo.InvariantCulture, $"Operation message: {operationMessage}");
        }
        text.AppendLine(CultureInfo.InvariantCulture, $"Timestamp: {timestampUtc:O}");
        text.Append(CultureInfo.CurrentCulture, $"Local time: {timestampUtc.ToLocalTime():g}");
        return text.ToString();
    }

    private static string ActionLabel(CatalogAction action)
        => action == CatalogAction.Remove ? "Remove" : "Install";
}
