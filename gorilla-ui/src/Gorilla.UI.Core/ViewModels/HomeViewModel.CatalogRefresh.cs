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
        var refreshed = await _cacheCoordinator.RefreshAsync(cancellationToken);
        ApplyItems(refreshed.Items);
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
