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

        // Ordinary Install/Remove admission rejection is intentionally non-operation
        // feedback. Surface it on both Details/card item state and the Activity row
        // where Retry was initiated, without manufacturing retained operation history.
        if (!string.IsNullOrWhiteSpace(item.TransientFeedback))
        {
            var feedback = item.TransientFeedback;
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
