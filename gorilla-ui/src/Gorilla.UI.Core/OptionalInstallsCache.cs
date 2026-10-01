using System.Text.Json;
using System.Text.Json.Serialization;
using Gorilla.UI.Client;
using Gorilla.UI.Core.Models;

namespace Gorilla.UI.Core;

public sealed record OptionalInstallsCacheDocument(
    DateTimeOffset CachedAtUtc,
    [property: JsonRequired] DateTimeOffset SourceGeneratedAtUtc,
    IReadOnlyList<OptionalInstallItem> Items
);

public sealed record OptionalInstallsRefreshResult(
    DateTimeOffset RefreshedAtUtc,
    IReadOnlyList<OptionalInstallItem> Items,
    Exception? CacheWriteFailure = null
);

public sealed class CatalogRefreshException : Exception
{
    public CatalogRefreshException(string? errorCode = null)
        : base("Gorilla couldn't refresh the App Catalog.")
    {
        ErrorCode = errorCode;
    }

    public string? ErrorCode { get; }
}

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

        if (document is null || document.SourceGeneratedAtUtc == default || document.CachedAtUtc == default)
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

public sealed class OptionalInstallsCacheCoordinator : IDisposable
{
    private static readonly TimeSpan DefaultRefreshPollInterval = TimeSpan.FromMilliseconds(500);

    private readonly IGorillaServiceClient _client;
    private readonly IOptionalInstallsCacheStore _cacheStore;
    private readonly TimeSpan _refreshPollInterval;
    private readonly object _refreshLock = new();
    private readonly object _latestReadLock = new();
    private readonly object _cacheWriteLock = new();
    private readonly object _stateLock = new();
    private readonly object _stateNotificationLock = new();
    private readonly SemaphoreSlim _snapshotApplicationGate = new(1, 1);
    private Task<OptionalInstallsRefreshResult>? _refreshTask;
    private Task<OptionalInstallsRefreshResult>? _latestReadTask;
    private Task _cacheWriteTail = Task.CompletedTask;
    private CatalogDataState _state = CatalogDataState.InitialLoading;
    private SynchronizationContext? _stateNotificationContext;
    private Func<IReadOnlyList<OptionalInstallItem>, CancellationToken, Task>? _defaultSnapshotAcceptor;

    public OptionalInstallsCacheCoordinator(
        IGorillaServiceClient client,
        IOptionalInstallsCacheStore cacheStore,
        TimeSpan? refreshPollInterval = null
    )
    {
        _client = client;
        _cacheStore = cacheStore;
        _refreshPollInterval = refreshPollInterval ?? DefaultRefreshPollInterval;
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

        OptionalInstallsCacheDocument? cached;
        try
        {
            cached = await _cacheStore.LoadAsync(cancellationToken);
        }
        catch (OperationCanceledException) when (cancellationToken.IsCancellationRequested)
        {
            throw;
        }
        catch
        {
            UpdateState(state => state with { CacheFallback = CacheFallbackState.Unavailable });
            throw;
        }

        if (cached is null)
        {
            UpdateState(state => state with { CacheFallback = CacheFallbackState.Unavailable });
            return null;
        }

        UpdateState(state => new CatalogDataState(
            HasUsableData: true,
            DataSource: CatalogDataSource.Cached,
            IsInitialLoading: false,
            IsRefreshing: true,
            IsSuccessfulEmpty: cached.Items.Count == 0,
            LastSuccessfulRefreshUtc: cached.SourceGeneratedAtUtc,
            CachedAtUtc: cached.CachedAtUtc,
            RefreshFailure: null,
            LoadFailure: null,
            CacheWriteFailure: state.CacheWriteFailure,
            CacheFallback: CacheFallbackState.Available
        ));

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
        var acceptSnapshot = _defaultSnapshotAcceptor ?? ((IReadOnlyList<OptionalInstallItem> _, CancellationToken _) => Task.CompletedTask);
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

    public Task<OptionalInstallsRefreshResult> ReadLatestAsync(CancellationToken cancellationToken)
    {
        var acceptSnapshot = _defaultSnapshotAcceptor ?? ((IReadOnlyList<OptionalInstallItem> _, CancellationToken _) => Task.CompletedTask);
        return ReadLatestAsync(acceptSnapshot, cancellationToken);
    }

    public Task<OptionalInstallsRefreshResult> ReadLatestAsync(
        Func<IReadOnlyList<OptionalInstallItem>, CancellationToken, Task> acceptSnapshot,
        CancellationToken cancellationToken
    )
    {
        ArgumentNullException.ThrowIfNull(acceptSnapshot);
        CaptureStateNotificationContext();

        lock (_latestReadLock)
        {
            if (_latestReadTask is not null)
            {
                return _latestReadTask;
            }

            _latestReadTask = ReadLatestCoreAsync(acceptSnapshot, cancellationToken);
            return _latestReadTask;
        }
    }

    private async Task<OptionalInstallsRefreshResult> RefreshCoreAsync(
        Func<IReadOnlyList<OptionalInstallItem>, CancellationToken, Task> acceptSnapshot,
        CancellationToken cancellationToken
    )
    {
        await Task.Yield();

        var stateAtRefreshStart = State;
        var liveSnapshotAtRefreshStart = stateAtRefreshStart.DataSource == CatalogDataSource.Live
            ? stateAtRefreshStart.LastSuccessfulRefreshUtc
            : null;

        UpdateState(state => state with
        {
            IsRefreshing = true,
            IsInitialLoading = !state.HasUsableData,
            RefreshFailure = null,
            LoadFailure = null,
        });

        try
        {
            var response = await _client.ListOptionalInstallsAsync(refresh: true, cancellationToken);
            while (true)
            {
                var disposition = await ApplySnapshotIfEligibleAsync(response, acceptSnapshot, cancellationToken);

                if (response.RefreshState == CatalogRefreshState.Failed)
                {
                    throw new CatalogRefreshException(response.RefreshErrorCode);
                }

                if (response.RefreshState is CatalogRefreshState.Queued or CatalogRefreshState.Running)
                {
                    await Task.Delay(_refreshPollInterval, cancellationToken);
                    response = await _client.ListOptionalInstallsAsync(refresh: false, cancellationToken);
                    continue;
                }

                if (response.SnapshotGeneratedAtUtc is not DateTimeOffset generatedAtUtc)
                {
                    throw new CatalogRefreshException();
                }

                if (disposition == SnapshotDisposition.Older &&
                    IsSupersededByNewerLiveSnapshot(generatedAtUtc, liveSnapshotAtRefreshStart))
                {
                    // The terminal response still proves the requested regeneration ended,
                    // while live truth has advanced since this refresh began.
                }
                else if (disposition != SnapshotDisposition.Accepted)
                {
                    throw new CatalogRefreshException();
                }

                UpdateState(state => state with
                {
                    IsInitialLoading = false,
                    IsRefreshing = false,
                    RefreshFailure = null,
                    LoadFailure = null,
                });

                return new OptionalInstallsRefreshResult(
                    RefreshedAtUtc: generatedAtUtc,
                    Items: response.Items,
                    CacheWriteFailure: null
                );
            }
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
            RecordRefreshFailure(ex);
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

    private async Task<OptionalInstallsRefreshResult> ReadLatestCoreAsync(
        Func<IReadOnlyList<OptionalInstallItem>, CancellationToken, Task> acceptSnapshot,
        CancellationToken cancellationToken
    )
    {
        await Task.Yield();

        try
        {
            var response = await _client.ListOptionalInstallsAsync(refresh: false, cancellationToken);
            var disposition = await ApplySnapshotIfEligibleAsync(response, acceptSnapshot, cancellationToken);
            if (response.SnapshotGeneratedAtUtc is not DateTimeOffset generatedAtUtc)
            {
                throw new CatalogRefreshException(
                    response.RefreshState == CatalogRefreshState.Failed ? response.RefreshErrorCode : null
                );
            }

            if (disposition == SnapshotDisposition.Older && IsSupersededByNewerLiveSnapshot(generatedAtUtc))
            {
                // A newer live snapshot already won the application race. This read is
                // a successful no-op rather than evidence of regression or failure.
            }
            else if (disposition != SnapshotDisposition.Accepted)
            {
                throw new CatalogRefreshException(
                    response.RefreshState == CatalogRefreshState.Failed ? response.RefreshErrorCode : null
                );
            }

            return new OptionalInstallsRefreshResult(
                RefreshedAtUtc: generatedAtUtc,
                Items: response.Items,
                CacheWriteFailure: null
            );
        }
        catch (OperationCanceledException) when (cancellationToken.IsCancellationRequested)
        {
            throw;
        }
        catch (Exception ex)
        {
            RecordReadFailure(ex);
            throw;
        }
        finally
        {
            lock (_latestReadLock)
            {
                _latestReadTask = null;
            }
        }
    }

    private async Task<SnapshotDisposition> ApplySnapshotIfEligibleAsync(
        OptionalInstallsSnapshotResult response,
        Func<IReadOnlyList<OptionalInstallItem>, CancellationToken, Task> acceptSnapshot,
        CancellationToken cancellationToken
    )
    {
        if (!response.SnapshotAvailable || response.SnapshotGeneratedAtUtc is not DateTimeOffset generatedAtUtc)
        {
            return SnapshotDisposition.Unavailable;
        }

        await _snapshotApplicationGate.WaitAsync(cancellationToken);
        try
        {
            var current = State.LastSuccessfulRefreshUtc;
            if (current is DateTimeOffset currentGeneratedAtUtc && generatedAtUtc < currentGeneratedAtUtc)
            {
                return SnapshotDisposition.Older;
            }

            await acceptSnapshot(response.Items, cancellationToken);

            var document = new OptionalInstallsCacheDocument(
                CachedAtUtc: DateTimeOffset.UtcNow,
                SourceGeneratedAtUtc: generatedAtUtc,
                Items: response.Items
            );

            UpdateState(state => new CatalogDataState(
                HasUsableData: true,
                DataSource: CatalogDataSource.Live,
                IsInitialLoading: false,
                IsRefreshing: state.IsRefreshing,
                IsSuccessfulEmpty: response.Items.Count == 0,
                LastSuccessfulRefreshUtc: generatedAtUtc,
                CachedAtUtc: state.CachedAtUtc,
                RefreshFailure: null,
                LoadFailure: null,
                CacheWriteFailure: state.CacheWriteFailure,
                CacheFallback: state.CacheFallback
            ));

            QueueCachePersistence(document);
            return SnapshotDisposition.Accepted;
        }
        finally
        {
            _snapshotApplicationGate.Release();
        }
    }

    private bool IsSupersededByNewerLiveSnapshot(
        DateTimeOffset responseGeneratedAtUtc,
        DateTimeOffset? liveSnapshotAtRefreshStart = null
    )
    {
        var state = State;
        if (state.DataSource != CatalogDataSource.Live ||
            state.LastSuccessfulRefreshUtc is not DateTimeOffset currentGeneratedAtUtc ||
            currentGeneratedAtUtc <= responseGeneratedAtUtc)
        {
            return false;
        }

        return liveSnapshotAtRefreshStart is null || currentGeneratedAtUtc > liveSnapshotAtRefreshStart.Value;
    }

    private void RecordRefreshFailure(Exception exception)
    {
        UpdateState(state => FailureState(state, exception, isRefreshing: false));
    }

    private void RecordReadFailure(Exception exception)
    {
        if (IsRefreshTaskActive())
        {
            return;
        }

        UpdateState(state => FailureState(state, exception, isRefreshing: false));
    }

    private bool IsRefreshTaskActive()
    {
        lock (_refreshLock)
        {
            return _refreshTask is not null;
        }
    }

    private static CatalogDataState FailureState(
        CatalogDataState state,
        Exception exception,
        bool isRefreshing
    ) => state.HasUsableData
        ? state with
        {
            IsInitialLoading = false,
            IsRefreshing = isRefreshing,
            RefreshFailure = exception,
            LoadFailure = null,
            CacheWriteFailure = state.CacheWriteFailure,
        }
        : new CatalogDataState(
            HasUsableData: false,
            DataSource: CatalogDataSource.None,
            IsInitialLoading: isRefreshing,
            IsRefreshing: isRefreshing,
            IsSuccessfulEmpty: false,
            LastSuccessfulRefreshUtc: state.LastSuccessfulRefreshUtc,
            CachedAtUtc: null,
            RefreshFailure: null,
            LoadFailure: exception,
            CacheWriteFailure: state.CacheWriteFailure,
            CacheFallback: state.CacheFallback
        );

    private void QueueCachePersistence(OptionalInstallsCacheDocument document)
    {
        lock (_cacheWriteLock)
        {
            _cacheWriteTail = PersistCacheAfterAsync(_cacheWriteTail, document);
        }
    }

    private async Task PersistCacheAfterAsync(Task previousWrite, OptionalInstallsCacheDocument document)
    {
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

            UpdateState(state =>
            {
                var cachedAtUtc = state.CachedAtUtc is DateTimeOffset currentCachedAt && currentCachedAt > document.CachedAtUtc
                    ? currentCachedAt
                    : document.CachedAtUtc;
                var isCurrentLiveSnapshot = state.LastSuccessfulRefreshUtc == document.SourceGeneratedAtUtc;
                return state with
                {
                    CachedAtUtc = cachedAtUtc,
                    CacheWriteFailure = isCurrentLiveSnapshot ? null : state.CacheWriteFailure,
                    CacheFallback = CacheFallbackState.Available,
                };
            });
        }
        catch (Exception ex)
        {
            UpdateState(state => state.LastSuccessfulRefreshUtc == document.SourceGeneratedAtUtc
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

    public void Dispose()
    {
        _snapshotApplicationGate.Dispose();
    }

    private enum SnapshotDisposition
    {
        Accepted,
        Unavailable,
        Older,
    }
}
