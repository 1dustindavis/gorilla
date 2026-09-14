using Gorilla.UI.Core.Models;

namespace Gorilla.UI.Core.ViewModels;

public sealed partial class HomeViewModel
{
    private bool _catalogStateSubscribed;

    public CatalogDataState CatalogState
    {
        get
        {
            EnsureCatalogStateSubscription();
            return _cacheCoordinator.State;
        }
    }

    public async Task RefreshCatalogAsync(CancellationToken cancellationToken)
    {
        EnsureCatalogStateSubscription();
        await _cacheCoordinator.RefreshAsync(
            (items, _) =>
            {
                // Attempt-level feedback and service-admission retry blocks describe
                // the truth observed by a specific user action against the prior
                // catalog snapshot. A successful manual Refresh supplies new canonical
                // catalog truth, so clear those local guards before recomputing recovery.
                foreach (var activity in _activityItems.Values)
                {
                    activity.SetRetryAttemptFeedback(null);
                }
                foreach (var item in _catalogItems.Values)
                {
                    item.ClearRetryBlock();
                    item.TransientFeedback = null;
                }

                ApplyItems(items);
                return Task.CompletedTask;
            },
            cancellationToken
        );
    }

    private void EnsureCatalogStateSubscription()
    {
        if (_catalogStateSubscribed)
        {
            return;
        }

        _cacheCoordinator.StateChanged += CacheCoordinator_StateChanged;
        _catalogStateSubscribed = true;
    }

    private void CacheCoordinator_StateChanged(object? sender, EventArgs e)
    {
        OnPropertyChanged(nameof(CatalogState));
    }
}
