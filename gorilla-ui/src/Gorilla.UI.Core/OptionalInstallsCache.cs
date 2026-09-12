using System.Text.Json;
using Gorilla.UI.Client;

namespace Gorilla.UI.Core;

public sealed record OptionalInstallsCacheDocument(
    DateTimeOffset CachedAtUtc,
    IReadOnlyList<OptionalInstallItem> Items
);

public sealed record OptionalInstallsRefreshResult(
    DateTimeOffset RefreshedAtUtc,
    IReadOnlyList<OptionalInstallItem> Items,
    Exception? CacheWriteFailure = null
);

public interface IOptionalInstallsCacheStore
{
    Task<OptionalInstallsCacheDocument?> LoadAsync(CancellationToken cancellationToken);

    Task SaveAsync(OptionalInstallsCacheDocument document, CancellationToken cancellationToken);
}

public sealed class JsonFileOptionalInstallsCacheStore : IOptionalInstallsCacheStore
{
    private readonly string _cacheFilePath;

    public JsonFileOptionalInstallsCacheStore(string cacheFilePath)
    {
        _cacheFilePath = cacheFilePath;
    }

    public async Task<OptionalInstallsCacheDocument?> LoadAsync(CancellationToken cancellationToken)
    {
        if (!File.Exists(_cacheFilePath))
        {
            return null;
        }

        OptionalInstallsCacheDocument? document;
        try
        {
            await using var stream = File.OpenRead(_cacheFilePath);
            document = await JsonSerializer.DeserializeAsync<OptionalInstallsCacheDocument>(
                stream,
                ProtocolJson.Options,
                cancellationToken
            );
        }
        catch (JsonException)
        {
            return null;
        }
        catch (IOException)
        {
            return null;
        }

        if (document is null)
        {
            return null;
        }

        var items = document.Items ?? [];
        foreach (var item in items)
        {
            ProtocolValidation.ValidateOptionalInstallItem(item);
        }

        return document with { Items = items };
    }

    public async Task SaveAsync(OptionalInstallsCacheDocument document, CancellationToken cancellationToken)
    {
        var directory = Path.GetDirectoryName(_cacheFilePath);
        if (!string.IsNullOrWhiteSpace(directory))
        {
            Directory.CreateDirectory(directory);
        }

        await using var stream = File.Create(_cacheFilePath);
        await JsonSerializer.SerializeAsync(stream, document, ProtocolJson.Options, cancellationToken);
    }
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

    public async Task<OptionalInstallsRefreshResult> RefreshAsync(CancellationToken cancellationToken)
    {
        var items = await _client.ListOptionalInstallsAsync(cancellationToken);
        var refreshedAtUtc = DateTimeOffset.UtcNow;
        var document = new OptionalInstallsCacheDocument(refreshedAtUtc, items);

        Exception? cacheWriteFailure = null;
        try
        {
            await _cacheStore.SaveAsync(document, cancellationToken);
        }
        catch (OperationCanceledException) when (cancellationToken.IsCancellationRequested)
        {
            throw;
        }
        catch (Exception ex)
        {
            // Fresh service data remains authoritative and usable even when the
            // fallback cache cannot be updated.
            cacheWriteFailure = ex;
        }

        return new OptionalInstallsRefreshResult(
            refreshedAtUtc,
            items,
            cacheWriteFailure
        );
    }
}
