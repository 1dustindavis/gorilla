using Gorilla.UI.Client;
using Gorilla.UI.Client.AppCatalog;
using Gorilla.UI.Core.Models;
using CatalogAction = Gorilla.UI.Client.AppCatalog.Action;

namespace Gorilla.UI.Core.ViewModels;

public sealed record RetryAttemptResult(bool Started, string? Feedback)
{
    public static RetryAttemptResult Accepted { get; } = new(true, null);
    public static RetryAttemptResult NotStarted(string feedback) => new(false, feedback);
}

public sealed partial class HomeViewModel
{
    private const string RetryActiveFeedback = "Another operation for this app is already active.";

    private static readonly HashSet<string> ActionRejectionReasons = new(StringComparer.Ordinal)
    {
        "not_optional",
        "policy_conflict",
        "managed_uninstall",
        "required_install",
        "operation_active",
        "invalid_selection",
        "state_unknown",
        "detection_failed",
        "install_unavailable",
        "already_selected",
        "required_dependency",
        "already_absent",
        "remove_unavailable",
    };

    public void RefreshActivityRecoveryPresentations()
        => RebuildActivityProjection();

    public async Task<RetryAttemptResult> RetryAsync(string historicalOperationId, CancellationToken cancellationToken)
    {
        if (!_operationTracker.TryGetLatest(historicalOperationId, out var historical) || historical is null)
        {
            throw new InvalidOperationException("The historical operation is no longer retained by the service.");
        }

        if (!OperationRecoveryPresentationMapper.IsRetryCandidate(historical.Result, historical.State))
        {
            throw new InvalidOperationException("This operation is not eligible for retry.");
        }

        SetRetryAttemptFeedback(historicalOperationId, null);

        // Retry is a new intent. Resolve current canonical catalog truth immediately
        // before dispatch and never reuse historical operation or mutation identity.
        var item = FindCanonicalItem(historical.ItemName);
        if (item is null)
        {
            const string feedback = "This app is no longer available in the current catalog.";
            SetRetryAttemptFeedback(historicalOperationId, feedback);
            RebuildActivityProjection();
            return RetryAttemptResult.NotStarted(feedback);
        }

        var blockedReason = item.RetryBlockReasonFor(historicalOperationId);
        if (!string.IsNullOrWhiteSpace(blockedReason))
        {
            item.TransientFeedback = blockedReason;
            SetRetryAttemptFeedback(historicalOperationId, blockedReason);
            RebuildActivityProjection();
            return RetryAttemptResult.NotStarted(blockedReason);
        }

        var active = _operationTracker.GetActiveForItem(item.ItemName);
        if (active is not null || item.IsBusy)
        {
            // A stale/double UI activation must not submit a second mutation.
            item.TransientFeedback = RetryActiveFeedback;
            SetRetryAttemptFeedback(historicalOperationId, RetryActiveFeedback);
            RebuildActivityProjection();
            return RetryAttemptResult.NotStarted(RetryActiveFeedback);
        }

        var currentDecision = historical.Action == CatalogAction.Remove
            ? item.RemoveDecision
            : item.InstallDecision;
        if (!currentDecision.Allowed)
        {
            var feedback = OperationRecoveryPresentationMapper.ReasonText(currentDecision.Reason);
            item.TransientFeedback = feedback;
            SetRetryAttemptFeedback(historicalOperationId, feedback);
            RebuildActivityProjection();
            return RetryAttemptResult.NotStarted(feedback);
        }

        try
        {
            // Converge on the ordinary action path. The client creates a fresh mutation
            // identity for this user intent while preserving same-mutation transport retry.
            if (historical.Action == CatalogAction.Remove)
            {
                await RemoveAsync(item, cancellationToken);
            }
            else
            {
                await InstallAsync(item, cancellationToken);
            }
        }
        catch (ServiceErrorException ex) when (ActionRejectionReasons.Contains(ex.ErrorCode))
        {
            // The service owns final admission. A stale cached decision can therefore
            // be rejected after Retry is clicked. Preserve that fresher current-action
            // truth as an attempt-level guard until a successful catalog Refresh
            // replaces the cached snapshot; do not mutate the snapshot itself.
            var feedback = OperationRecoveryPresentationMapper.ReasonText(ex.ErrorCode);
            item.TransientFeedback = feedback;
            item.BlockRetry(historicalOperationId, feedback);
            SetRetryAttemptFeedback(historicalOperationId, feedback);
            RebuildActivityProjection();
            return RetryAttemptResult.NotStarted(feedback);
        }

        // OperationAccepted(false) is also an admission rejection. It lacks a
        // structured policy reason, but it is still fresher than the cached action
        // snapshot and must suppress another Retry until fresh catalog truth arrives.
        if (!string.IsNullOrWhiteSpace(item.TransientFeedback))
        {
            var feedback = item.TransientFeedback;
            item.BlockRetry(historicalOperationId, feedback);
            SetRetryAttemptFeedback(historicalOperationId, feedback);
            RebuildActivityProjection();
            return RetryAttemptResult.NotStarted(feedback);
        }

        SetRetryAttemptFeedback(historicalOperationId, null);
        RebuildActivityProjection();
        return RetryAttemptResult.Accepted;
    }

    private UiOptionalInstallItem? FindCanonicalItem(string itemName)
        => _catalogItems.TryGetValue(itemName, out var item) ? item : null;

    private void SetRetryAttemptFeedback(string operationId, string? feedback)
    {
        if (_activityItems.TryGetValue(operationId, out var activity))
        {
            activity.SetRetryAttemptFeedback(feedback);
        }
    }
}
