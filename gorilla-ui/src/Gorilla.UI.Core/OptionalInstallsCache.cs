using System.Text.Json;
using Gorilla.UI.Client;
using Gorilla.UI.Core.Models;

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
    private readonly object _refreshLock = new();
    private readonly SemaphoreSlim _cacheWriteLock = new(1, 1);
    private Task<OptionalInstallsRefreshResult>? _refreshTask;
    private CatalogDataState _state = CatalogDataState.InitialLoading;

    public OptionalInstallsCacheCoordinator(IGorillaServiceClient client, IOptionalInstallsCacheStore cacheStore)
    {
        _client = client;
        _cacheStore = cacheStore;
    }

    public CatalogDataState State => _state;

    public event EventHandler? StateChanged;

    public async Task<OptionalInstallsCacheDocument?> LoadCachedAsync(CancellationToken cancellationToken)
    {
        var cached = await _cacheStore.LoadAsync(cancellationToken);
        if (cached is not null)
        {
            SetState(new CatalogDataState(
                HasUsableData: true,
                DataSource: CatalogDataSource.Cached,
                IsInitialLoading: false,
                IsRefreshing: true,
                IsSuccessfulEmpty: false,
                LastSuccessfulRefreshUtc: _state.LastSuccessfulRefreshUtc,
                CachedAtUtc: cached.CachedAtUtc,
                RefreshFailure: null,
                LoadFailure: null,
                CacheWriteFailure: null
            ));
        }

        return cached;
    }

    public Task<OptionalInstallsRefreshResult> RefreshAsync(CancellationToken cancellationToken)
    {
        lock (_refreshLock)
        {
            if (_refreshTask is not null)
            {
                return _refreshTask;
            }

            _refreshTask = RefreshCoreAsync(cancellationToken);
            return _refreshTask;
        }
    }

    private async Task<OptionalInstallsRefreshResult> RefreshCoreAsync(CancellationToken cancellationToken)
    {
        // Do not raise StateChanged synchronously while RefreshAsync still owns the
        // coordination lock and has not yet published _refreshTask. A subscriber may
        // legitimately observe refresh state and request Refresh again; yielding first
        // guarantees that request joins the already-published task instead of racing it.
        await Task.Yield();

        SetState(_state with
        {
            IsRefreshing = true,
            IsInitialLoading = !_state.HasUsableData && _state.LastSuccessfulRefreshUtc is null,
            RefreshFailure = null,
            LoadFailure = null,
        });

        try
        {
            var items = await _client.ListOptionalInstallsAsync(cancellationToken);
            var refreshedAtUtc = DateTimeOffset.UtcNow;
            var document = new OptionalInstallsCacheDocument(refreshedAtUtc, items);

            // A successful live response is immediately authoritative. Cache
            // persistence is secondary durability work and must never delay fresh data
            // becoming usable or keep the Refresh UI spinning.
            SetState(new CatalogDataState(
                HasUsableData: true,
                DataSource: CatalogDataSource.Live,
                IsInitialLoading: false,
                IsRefreshing: false,
                IsSuccessfulEmpty: items.Count == 0,
                LastSuccessfulRefreshUtc: refreshedAtUtc,
                CachedAtUtc: _state.CachedAtUtc,
                RefreshFailure: null,
                LoadFailure: null,
                CacheWriteFailure: null
            ));

            _ = PersistCacheAsync(document);

            return new OptionalInstallsRefreshResult(
                refreshedAtUtc,
                items,
                CacheWriteFailure: null
            );
        }
        catch (OperationCanceledException) when (cancellationToken.IsCancellationRequested)
        {
            SetState(_state with
            {
                IsInitialLoading = false,
                IsRefreshing = false,
            });
            throw;
        }
        catch (Exception ex)
        {
            if (_state.HasUsableData)
            {
                SetState(_state with
                {
                    IsInitialLoading = false,
                    IsRefreshing = false,
                    RefreshFailure = ex,
                    LoadFailure = null,
                    CacheWriteFailure = null,
                });
            }
            else
            {
                SetState(new CatalogDataState(
                    HasUsableData: false,
                    DataSource: CatalogDataSource.None,
                    IsInitialLoading: false,
                    IsRefreshing: false,
                    IsSuccessfulEmpty: false,
                    LastSuccessfulRefreshUtc: _state.LastSuccessfulRefreshUtc,
                    CachedAtUtc: null,
                    RefreshFailure: null,
                    LoadFailure: ex,
                    CacheWriteFailure: null
                ));
            }

            throw;
        }
        finally
        {
            lock (_refreshLock)
            {
                _refreshTask = null;
            }
        }
    }

    private async Task PersistCacheAsync(OptionalInstallsCacheDocument document)
    {
        await _cacheWriteLock.WaitAsync(CancellationToken.None);
        try
        {
            try
            {
                await _cacheStore.SaveAsync(document, CancellationToken.None);

                var state = _state;
                var cachedAtUtc = state.CachedAtUtc is DateTimeOffset currentCachedAt && currentCachedAt > document.CachedAtUtc
                    ? currentCachedAt
                    : document.CachedAtUtc;
                var isCurrentLiveSnapshot = state.LastSuccessfulRefreshUtc == document.CachedAtUtc;
                SetState(state with
                {
                    CachedAtUtc = cachedAtUtc,
                    CacheWriteFailure = isCurrentLiveSnapshot ? null : state.CacheWriteFailure,
                });
            }
            catch (Exception ex)
            {
                var state = _state;
                if (state.LastSuccessfulRefreshUtc == document.CachedAtUtc)
                {
                    SetState(state with { CacheWriteFailure = ex });
                }
            }
        }
        finally
        {
            _cacheWriteLock.Release();
        }
    }

    private void SetState(CatalogDataState state)
    {
        if (Equals(_state, state))
        {
            return;
        }

        _state = state;
        StateChanged?.Invoke(this, EventArgs.Empty);
    }
}
