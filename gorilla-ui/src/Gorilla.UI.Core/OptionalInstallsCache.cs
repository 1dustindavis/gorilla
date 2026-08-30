using Gorilla.UI.Client;

namespace Gorilla.UI.Core;

public sealed record OptionalInstallsCacheDocument(
    DateTimeOffset CachedAtUtc,
    IReadOnlyList<OptionalInstallItem> Items
);

public interface IOptionalInstallsCacheStore
{
    Task<OptionalInstallsCacheDocument?> LoadAsync(CancellationToken cancellationToken);

    Task SaveAsync(OptionalInstallsCacheDocument document, CancellationToken cancellationToken);
}

public sealed class OptionalInstallsCacheCoordinator
{
    private readonly IGorillaServiceClient _client;
    private readonly IOptionalInstallsCacheStore _cacheStore;

    public OptionalInstallsCacheCoordinator(IGorillaServiceClient client, IOptionalInstallsCacheStore cacheStore)
    {
        _client = client;
        _cacheStore = cacheStore;
    }

    public Task<OptionalInstallsCacheDocument?> LoadCachedAsync(CancellationToken cancellationToken)
    {
        return _cacheStore.LoadAsync(cancellationToken);
    }

    public async Task<OptionalInstallsCacheDocument> RefreshAsync(CancellationToken cancellationToken)
    {
        var items = await _client.ListOptionalInstallsAsync(cancellationToken);
        var document = new OptionalInstallsCacheDocument(
            CachedAtUtc: DateTimeOffset.UtcNow,
            Items: items
        );

        await _cacheStore.SaveAsync(document, cancellationToken);
        return document;
    }
}
