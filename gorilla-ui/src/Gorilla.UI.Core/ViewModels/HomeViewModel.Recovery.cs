using Gorilla.UI.Client;
using Gorilla.UI.Client.AppCatalog;
using Gorilla.UI.Core.Models;
using CatalogAction = Gorilla.UI.Client.AppCatalog.Action;

namespace Gorilla.UI.Core.ViewModels;

public sealed partial class HomeViewModel
{
    public void RefreshActivityRecoveryPresentations()
    {
        foreach (var activity in ActivityItems)
        {
            if (!_operationTracker.TryGetLatest(activity.OperationId, out var operation) || operation is null)
            {
                continue;
            }

            var item = FindItem(operation.ItemName);
            var active = _operationTracker.GetActiveForItem(operation.ItemName);
            var conflictingActive = active is not null &&
                !string.Equals(active.OperationId, operation.OperationId, StringComparison.Ordinal);

            activity.ApplyRecovery(OperationRecoveryPresentationMapper.Map(
                operation,
                item,
                conflictingActive
            ));
        }
    }

    public OperationRecoveryPresentation GetRecoveryPresentation(UiOperationPresentation operation)
    {
        var itemName = SelectedItemName;
        var item = itemName is null ? null : FindItem(itemName);
        var active = item is null ? null : _operationTracker.GetActiveForItem(item.ItemName);
        var conflictingActive = active is not null &&
            !string.Equals(active.OperationId, operation.OperationId, StringComparison.Ordinal);
        return OperationRecoveryPresentationMapper.Map(operation, item, conflictingActive);
    }

    public async Task RetryAsync(string historicalOperationId, CancellationToken cancellationToken)
    {
        if (!_operationTracker.TryGetLatest(historicalOperationId, out var historical) || historical is null)
        {
            throw new InvalidOperationException("The historical operation is no longer retained by the service.");
        }

        if (!OperationRecoveryPresentationMapper.IsRetryCandidate(historical.Result, historical.State))
        {
            throw new InvalidOperationException("This operation is not eligible for retry.");
        }

        // Retry is a new intent. Resolve current canonical catalog truth immediately
        // before dispatch and never reuse the historical operation or mutation identity.
        var item = FindItem(historical.ItemName);
        if (item is null)
        {
            throw new InvalidOperationException("This app is no longer available in the current catalog.");
        }

        var active = _operationTracker.GetActiveForItem(item.ItemName);
        if (active is not null || item.IsBusy)
        {
            // A stale/double UI activation must not submit a second mutation or leave
            // transient failure-like feedback. The accepted active operation is the
            // current truth and all surfaces already project it.
            RefreshActivityRecoveryPresentations();
            return;
        }

        var currentDecision = historical.Action == CatalogAction.Remove
            ? item.RemoveDecision
            : item.InstallDecision;
        if (!currentDecision.Allowed)
        {
            item.TransientFeedback = OperationRecoveryPresentationMapper.ReasonText(currentDecision.Reason);
            RefreshActivityRecoveryPresentations();
            return;
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

        RefreshActivityRecoveryPresentations();
    }
}
