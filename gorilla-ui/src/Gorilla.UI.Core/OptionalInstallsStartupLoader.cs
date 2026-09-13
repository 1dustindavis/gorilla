using Gorilla.UI.Client;

namespace Gorilla.UI.Core;

public sealed class OptionalInstallsStartupLoader
{
    private readonly OptionalInstallsCacheCoordinator _cacheCoordinator;

    public OptionalInstallsStartupLoader(OptionalInstallsCacheCoordinator cacheCoordinator)
    {
        _cacheCoordinator = cacheCoordinator;
    }

    public async Task<string> InitializeAsync(
        Action<IReadOnlyList<OptionalInstallItem>> applyCachedItems,
        Action<IReadOnlyList<OptionalInstallItem>> applyRefreshedItems,
        CancellationToken cancellationToken
    )
    {
        OptionalInstallsCacheDocument? cached = null;
        try
        {
            cached = await _cacheCoordinator.LoadCachedAsync(cancellationToken);
        }
        catch (OperationCanceledException) when (cancellationToken.IsCancellationRequested)
        {
            throw;
        }
        catch
        {
            // An unreadable or invalid fallback cache is not authoritative. Treat it
            // as absent and continue to the live service instead of aborting session
            // initialization before a live catalog request can be attempted.
        }

        if (cached is not null)
        {
            applyCachedItems(cached.Items);
        }

        try
        {
            var refreshed = await _cacheCoordinator.RefreshAsync(cancellationToken);
            applyRefreshedItems(refreshed.Items);
        }
        catch (OperationCanceledException) when (cancellationToken.IsCancellationRequested)
        {
            throw;
        }
        catch
        {
            // Catalog load/refresh failures are represented by the coordinator's
            // explicit CatalogDataState. WarningBanner remains reserved for other
            // service/operation infrastructure warnings.
        }

        return string.Empty;
    }
}
