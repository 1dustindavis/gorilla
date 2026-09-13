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
    private readonly object _cacheWriteLock = new();
    private readonly object _stateLock = new();
    private readonly object _stateNotificationLock = new();
    private Task<OptionalInstallsRefreshResult>? _refreshTask;
    private Task _cacheWriteTail = Task.CompletedTask;
    private CatalogDataState _state = CatalogDataState.InitialLoading;
    private SynchronizationContext? _stateNotificationContext;
    private Func<IReadOnlyList<OptionalInstallItem>, CancellationToken, Task>? _defaultSnapshotAcceptor;

    public OptionalInstallsCacheCoordinator(IGorillaServiceClient client, IOptionalInstallsCacheStore cacheStore)
    {
        _client = client;
        _cacheStore = cacheStore;
    }

    public CatalogDataState State
    {
        get
        {
            lock (_stateLock)
            {
                return _state;
            }
        }
    }

    public event EventHandler? StateChanged;

    public async Task<OptionalInstallsCacheDocument?> LoadCachedAsync(CancellationToken cancellationToken)
    {
        CaptureStateNotificationContext();

        var cached = await _cacheStore.LoadAsync(cancellationToken);
        if (cached is not null)
        {
            UpdateState(state => new CatalogDataState(
                HasUsableData: true,
                DataSource: CatalogDataSource.Cached,
                IsInitialLoading: false,
                IsRefreshing: true,
                IsSuccessfulEmpty: false,
                LastSuccessfulRefreshUtc: state.LastSuccessfulRefreshUtc,
                CachedAtUtc: cached.CachedAtUtc,
                RefreshFailure: null,
                LoadFailure: null,
                CacheWriteFailure: state.CacheWriteFailure
            ));
        }

        return cached;
    }

    public void RegisterSnapshotAcceptor(
        Func<IReadOnlyList<OptionalInstallItem>, CancellationToken, Task> acceptSnapshot
    )
    {
        ArgumentNullException.ThrowIfNull(acceptSnapshot);
        _defaultSnapshotAcceptor = acceptSnapshot;
    }

    public Task<OptionalInstallsRefreshResult> RefreshAsync(CancellationToken cancellationToken)
    {
        Func<IReadOnlyList<OptionalInstallItem>, CancellationToken, Task> acceptSnapshot =
            _defaultSnapshotAcceptor ?? ((_, _) => Task.CompletedTask);
        return RefreshAsync(acceptSnapshot, cancellationToken);
    }

    public Task<OptionalInstallsRefreshResult> RefreshAsync(
        Func<IReadOnlyList<OptionalInstallItem>, CancellationToken, Task> acceptSnapshot,
        CancellationToken cancellationToken
    )
    {
        ArgumentNullException.ThrowIfNull(acceptSnapshot);
        CaptureStateNotificationContext();

        lock (_refreshLock)
        {
            if (_refreshTask is not null)
            {
                return _refreshTask;
            }

            _refreshTask = RefreshCoreAsync(acceptSnapshot, cancellationToken);
            return _refreshTask;
        }
    }

    private async Task<OptionalInstallsRefreshResult> RefreshCoreAsync(
        Func<IReadOnlyList<OptionalInstallItem>, CancellationToken, Task> acceptSnapshot,
        CancellationToken cancellationToken
    )
    {
        // Do not raise StateChanged synchronously while RefreshAsync still owns the
        // coordination lock and has not yet published _refreshTask. A subscriber may
        // legitimately observe refresh state and request Refresh again; yielding first
        // guarantees that request joins the already-published task instead of racing it.
        await Task.Yield();

        UpdateState(state => state with
        {
            IsRefreshing = true,
            IsInitialLoading = !state.HasUsableData && state.LastSuccessfulRefreshUtc is null,
            RefreshFailure = null,
            LoadFailure = null,
        });

        try
        {
            var items = await _client.ListOptionalInstallsAsync(cancellationToken);
            var refreshedAtUtc = DateTimeOffset.UtcNow;
            var document = new OptionalInstallsCacheDocument(refreshedAtUtc, items);

            // The service response is not user-visible freshness truth until the
            // canonical model has accepted it. Keep the previous provenance and the
            // refresh-in-progress state while reconciliation runs so shell freshness,
            // catalog contents, details, and action availability change atomically
            // from the user's perspective.
            await acceptSnapshot(items, cancellationToken);

            UpdateState(state => new CatalogDataState(
                HasUsableData: true,
                DataSource: CatalogDataSource.Live,
                IsInitialLoading: false,
                IsRefreshing: false,
                IsSuccessfulEmpty: items.Count == 0,
                LastSuccessfulRefreshUtc: refreshedAtUtc,
                CachedAtUtc: state.CachedAtUtc,
                RefreshFailure: null,
                LoadFailure: null,
                // Cache persistence is an independent degradation axis. A new live
                // response does not prove fallback durability has recovered.
                CacheWriteFailure: state.CacheWriteFailure
            ));

            // Persistence is secondary durability work. It begins only after the
            // snapshot is accepted, but remains independent of refresh completion.
            QueueCachePersistence(document);

            return new OptionalInstallsRefreshResult(
                refreshedAtUtc,
                items,
                CacheWriteFailure: null
            );
        }
        catch (OperationCanceledException) when (cancellationToken.IsCancellationRequested)
        {
            UpdateState(state => state with
            {
                IsInitialLoading = false,
                IsRefreshing = false,
            });
            throw;
        }
        catch (Exception ex)
        {
            UpdateState(state => state.HasUsableData
                ? state with
                {
                    IsInitialLoading = false,
                    IsRefreshing = false,
                    RefreshFailure = ex,
                    LoadFailure = null,
                    // An unrelated service failure must not erase cache degradation.
                    CacheWriteFailure = state.CacheWriteFailure,
                }
                : new CatalogDataState(
                    HasUsableData: false,
                    DataSource: CatalogDataSource.None,
                    IsInitialLoading: false,
                    IsRefreshing: false,
                    IsSuccessfulEmpty: false,
                    LastSuccessfulRefreshUtc: state.LastSuccessfulRefreshUtc,
                    CachedAtUtc: null,
                    RefreshFailure: null,
                    LoadFailure: ex,
                    CacheWriteFailure: state.CacheWriteFailure
                ));

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

    private void QueueCachePersistence(OptionalInstallsCacheDocument document)
    {
        lock (_cacheWriteLock)
        {
            _cacheWriteTail = PersistCacheAfterAsync(_cacheWriteTail, document);
        }
    }

    private async Task PersistCacheAfterAsync(Task previousWrite, OptionalInstallsCacheDocument document)
    {
        // Preserve write ordering, but never let an unexpected failure in an older
        // best-effort persistence task prevent a newer live snapshot from being saved.
        try
        {
            await previousWrite.ConfigureAwait(false);
        }
        catch
        {
        }

        try
        {
            await _cacheStore.SaveAsync(document, CancellationToken.None).ConfigureAwait(false);

            // Merge only persistence-owned fields into the current state atomically.
            // An older save completion must never overwrite newer refresh/provenance
            // state. Clear degradation only if this save durably persisted the current
            // live snapshot.
            UpdateState(state =>
            {
                var cachedAtUtc = state.CachedAtUtc is DateTimeOffset currentCachedAt && currentCachedAt > document.CachedAtUtc
                    ? currentCachedAt
                    : document.CachedAtUtc;
                var isCurrentLiveSnapshot = state.LastSuccessfulRefreshUtc == document.CachedAtUtc;
                return state with
                {
                    CachedAtUtc = cachedAtUtc,
                    CacheWriteFailure = isCurrentLiveSnapshot ? null : state.CacheWriteFailure,
                };
            });
        }
        catch (Exception ex)
        {
            // Only a failed attempt to persist the current live snapshot establishes
            // current cache degradation. Failures from superseded queued saves cannot
            // overwrite the state of a newer snapshot.
            UpdateState(state => state.LastSuccessfulRefreshUtc == document.CachedAtUtc
                ? state with { CacheWriteFailure = ex }
                : state);
        }
    }

    private void CaptureStateNotificationContext()
    {
        var current = SynchronizationContext.Current;
        if (current is null)
        {
            return;
        }

        lock (_stateNotificationLock)
        {
            _stateNotificationContext ??= current;
        }
    }

    private void UpdateState(Func<CatalogDataState, CatalogDataState> update)
    {
        EventHandler? handler;
        lock (_stateLock)
        {
            var next = update(_state);
            if (Equals(_state, next))
            {
                return;
            }

            _state = next;
            handler = StateChanged;
        }

        if (handler is null)
        {
            return;
        }

        SynchronizationContext? notificationContext;
        lock (_stateNotificationLock)
        {
            notificationContext = _stateNotificationContext;
        }

        if (notificationContext is not null && !ReferenceEquals(SynchronizationContext.Current, notificationContext))
        {
            notificationContext.Post(
                static payload =>
                {
                    var (owner, stateChanged) = ((OptionalInstallsCacheCoordinator Owner, EventHandler StateChanged))payload!;
                    stateChanged(owner, EventArgs.Empty);
                },
                (this, handler)
            );
            return;
        }

        handler(this, EventArgs.Empty);
    }
}
