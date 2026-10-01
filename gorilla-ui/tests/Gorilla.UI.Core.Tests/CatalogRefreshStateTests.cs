using System.Runtime.CompilerServices;
using Gorilla.UI.Client;
using Gorilla.UI.Core.Models;
using Xunit;

namespace Gorilla.UI.Core.Tests;

public sealed class CatalogRefreshStateTests
{
    private static readonly DateTimeOffset T0900 = DateTimeOffset.Parse("2026-09-30T09:00:00Z");
    private static readonly DateTimeOffset T1000 = DateTimeOffset.Parse("2026-09-30T10:00:00Z");
    private static readonly DateTimeOffset T1005 = DateTimeOffset.Parse("2026-09-30T10:05:00Z");

    [Fact]
    public async Task RefreshAsync_AppliesCurrentSnapshotWhileRunning_ThenNewIdleSnapshot()
    {
        var secondResponse = new TaskCompletionSource<OptionalInstallsSnapshotResult>(TaskCreationOptions.RunContinuationsAsynchronously);
        var secondCallStarted = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
        var client = new SequenceClient();
        client.Enqueue(Result("current", T1000, CatalogRefreshState.Running));
        client.Enqueue(async (_, token) =>
        {
            secondCallStarted.TrySetResult();
            return await secondResponse.Task.WaitAsync(token);
        });
        var coordinator = Coordinator(client);
        var applied = new List<string>();

        var refresh = coordinator.RefreshAsync(
            (items, _) =>
            {
                applied.Add(items.Single().DisplayName);
                return Task.CompletedTask;
            },
            CancellationToken.None
        );

        await secondCallStarted.Task;
        Assert.Equal(["current"], applied);
        Assert.True(coordinator.State.IsRefreshing);
        Assert.Equal(T1000, coordinator.State.LastSuccessfulRefreshUtc);
        Assert.Equal(CatalogDataSource.Live, coordinator.State.DataSource);

        secondResponse.SetResult(Result("new", T1005, CatalogRefreshState.Idle));
        await refresh;

        Assert.Equal(["current", "new"], applied);
        Assert.Equal([true, false], client.RefreshArguments);
        Assert.False(coordinator.State.IsRefreshing);
        Assert.Equal(T1005, coordinator.State.LastSuccessfulRefreshUtc);
        Assert.Null(coordinator.State.RefreshFailure);
    }

    [Fact]
    public async Task RefreshAsync_NoSnapshotQueuedAndRunning_DoesNotAcceptEmptyUntilSnapshotExists()
    {
        var client = new SequenceClient();
        client.Enqueue(Unavailable(CatalogRefreshState.Queued));
        client.Enqueue(Unavailable(CatalogRefreshState.Running));
        client.Enqueue(Result("ready", T1005, CatalogRefreshState.Idle));
        var coordinator = Coordinator(client);
        var accepted = new List<IReadOnlyList<OptionalInstallItem>>();

        await coordinator.RefreshAsync(
            (items, _) =>
            {
                accepted.Add(items);
                return Task.CompletedTask;
            },
            CancellationToken.None
        );

        Assert.Single(accepted);
        Assert.Equal("ready", Assert.Single(accepted[0]).DisplayName);
        Assert.Equal([true, false, false], client.RefreshArguments);
        Assert.True(coordinator.State.HasUsableData);
        Assert.False(coordinator.State.IsSuccessfulEmpty);
    }

    [Fact]
    public async Task RefreshAsync_OlderRunningSnapshotNeverReplacesNewerCache()
    {
        var store = new RecordingCacheStore
        {
            Loaded = new OptionalInstallsCacheDocument(
                CachedAtUtc: T1000.AddMinutes(1),
                SourceGeneratedAtUtc: T1000,
                Items: [Item("cached")]
            )
        };
        var client = new SequenceClient();
        client.Enqueue(Result("old", T0900, CatalogRefreshState.Running));
        client.Enqueue(Result("new", T1005, CatalogRefreshState.Idle));
        var coordinator = Coordinator(client, store);
        var applied = new List<string>();

        var cached = await coordinator.LoadCachedAsync(CancellationToken.None);
        Assert.NotNull(cached);
        applied.Add(cached!.Items.Single().DisplayName);

        await coordinator.RefreshAsync(
            (items, _) =>
            {
                applied.Add(items.Single().DisplayName);
                return Task.CompletedTask;
            },
            CancellationToken.None
        );

        Assert.Equal(["cached", "new"], applied);
        Assert.DoesNotContain(store.Saved, document => document.SourceGeneratedAtUtc == T0900);
        Assert.Equal(T1005, coordinator.State.LastSuccessfulRefreshUtc);
    }

    [Fact]
    public async Task RefreshAsync_FinalIdleOlderThanDisplayedCachedSnapshot_IsRefreshFailureWithoutRegression()
    {
        var store = new RecordingCacheStore
        {
            Loaded = new OptionalInstallsCacheDocument(T1000.AddMinutes(1), T1000, [Item("cached")])
        };
        var client = new SequenceClient();
        client.Enqueue(Result("old", T0900, CatalogRefreshState.Idle));
        var coordinator = Coordinator(client, store);
        await coordinator.LoadCachedAsync(CancellationToken.None);
        var acceptCalls = 0;

        await Assert.ThrowsAsync<CatalogRefreshException>(() => coordinator.RefreshAsync(
            (_, _) =>
            {
                acceptCalls++;
                return Task.CompletedTask;
            },
            CancellationToken.None
        ));

        Assert.Equal(0, acceptCalls);
        Assert.Equal(T1000, coordinator.State.LastSuccessfulRefreshUtc);
        Assert.Equal(CatalogDataSource.Cached, coordinator.State.DataSource);
        Assert.IsType<CatalogRefreshException>(coordinator.State.RefreshFailure);
    }

    [Fact]
    public async Task RefreshAsync_OlderTerminalSnapshotSupersededByConcurrentNewerLiveRead_CompletesSuccessfully()
    {
        var terminalCallStarted = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
        var releaseTerminalResponse = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
        var newerAt = T1005.AddMinutes(5);
        var client = new SequenceClient();
        client.Enqueue(Result("current", T1000, CatalogRefreshState.Running));
        client.Enqueue(async (_, token) =>
        {
            terminalCallStarted.TrySetResult();
            await releaseTerminalResponse.Task.WaitAsync(token);
            return Result("terminal", T1005, CatalogRefreshState.Idle);
        });
        client.Enqueue(Result("newer", newerAt, CatalogRefreshState.Running));
        var coordinator = Coordinator(client);
        var applied = new List<string>();

        var refresh = coordinator.RefreshAsync(
            (items, _) =>
            {
                applied.Add(items.Single().DisplayName);
                return Task.CompletedTask;
            },
            CancellationToken.None
        );

        await terminalCallStarted.Task.WaitAsync(TimeSpan.FromSeconds(2));
        await coordinator.ReadLatestAsync(
            (items, _) =>
            {
                applied.Add(items.Single().DisplayName);
                return Task.CompletedTask;
            },
            CancellationToken.None
        );
        Assert.Equal(newerAt, coordinator.State.LastSuccessfulRefreshUtc);

        releaseTerminalResponse.TrySetResult();
        var result = await refresh.WaitAsync(TimeSpan.FromSeconds(2));

        Assert.Equal(["current", "newer"], applied);
        Assert.Equal(T1005, result.RefreshedAtUtc);
        Assert.Equal(newerAt, coordinator.State.LastSuccessfulRefreshUtc);
        Assert.True(coordinator.State.IsLive);
        Assert.False(coordinator.State.IsRefreshing);
        Assert.Null(coordinator.State.RefreshFailure);
    }

    [Fact]
    public async Task RefreshAsync_FailedWithExistingData_KeepsDataAndRecordsSafeRefreshFailure()
    {
        var store = new RecordingCacheStore
        {
            Loaded = new OptionalInstallsCacheDocument(T1000.AddMinutes(1), T1000, [Item("cached")])
        };
        var client = new SequenceClient();
        client.Enqueue(Unavailable(CatalogRefreshState.Failed));
        var coordinator = Coordinator(client, store);
        await coordinator.LoadCachedAsync(CancellationToken.None);

        var exception = await Assert.ThrowsAsync<CatalogRefreshException>(
            () => coordinator.RefreshAsync((_, _) => Task.CompletedTask, CancellationToken.None)
        );

        Assert.Equal("refresh_failed", exception.ErrorCode);
        Assert.True(coordinator.State.HasUsableData);
        Assert.Equal(T1000, coordinator.State.LastSuccessfulRefreshUtc);
        Assert.Same(exception, coordinator.State.RefreshFailure);
        Assert.Null(coordinator.State.LoadFailure);
    }

    [Fact]
    public async Task RefreshAsync_FailedWithoutData_RecordsLoadFailure()
    {
        var client = new SequenceClient();
        client.Enqueue(Unavailable(CatalogRefreshState.Failed));
        var coordinator = Coordinator(client);

        var exception = await Assert.ThrowsAsync<CatalogRefreshException>(
            () => coordinator.RefreshAsync((_, _) => Task.CompletedTask, CancellationToken.None)
        );

        Assert.False(coordinator.State.HasUsableData);
        Assert.Same(exception, coordinator.State.LoadFailure);
        Assert.Null(coordinator.State.RefreshFailure);
    }

    [Fact]
    public async Task RefreshAsync_AuthoritativeEmptySnapshot_IsUsableEmptyData()
    {
        var client = new SequenceClient();
        client.Enqueue(new OptionalInstallsSnapshotResult(
            Items: [],
            SnapshotAvailable: true,
            SnapshotGeneratedAtUtc: T1005,
            RefreshState: CatalogRefreshState.Idle,
            RefreshRequestedAtUtc: null,
            RefreshCompletedAtUtc: null,
            RefreshErrorCode: null
        ));
        var store = new RecordingCacheStore();
        var coordinator = Coordinator(client, store);
        var accepts = 0;

        await coordinator.RefreshAsync(
            (items, _) =>
            {
                Assert.Empty(items);
                accepts++;
                return Task.CompletedTask;
            },
            CancellationToken.None
        );

        Assert.Equal(1, accepts);
        Assert.True(coordinator.State.HasUsableData);
        Assert.True(coordinator.State.IsSuccessfulEmpty);
        Assert.Equal(T1005, coordinator.State.LastSuccessfulRefreshUtc);
        await store.WaitForSaveAsync();
        Assert.Contains(store.Saved, document => document.SourceGeneratedAtUtc == T1005 && document.Items.Count == 0);
    }

    [Fact]
    public async Task RefreshAsync_CancellationWhilePolling_StopsWithoutRefreshFailure()
    {
        var store = new RecordingCacheStore
        {
            Loaded = new OptionalInstallsCacheDocument(T1000.AddMinutes(1), T1000, [Item("cached")])
        };
        var client = new SequenceClient();
        client.Enqueue(Unavailable(CatalogRefreshState.Running));
        var coordinator = new OptionalInstallsCacheCoordinator(client, store, TimeSpan.FromHours(1));
        await coordinator.LoadCachedAsync(CancellationToken.None);
        using var cts = new CancellationTokenSource();

        var refresh = coordinator.RefreshAsync((_, _) => Task.CompletedTask, cts.Token);
        await client.FirstCallObserved.Task;
        cts.Cancel();

        await Assert.ThrowsAnyAsync<OperationCanceledException>(() => refresh);
        Assert.Single(client.RefreshArguments);
        Assert.Equal([true], client.RefreshArguments);
        Assert.False(coordinator.State.IsRefreshing);
        Assert.Null(coordinator.State.RefreshFailure);
        Assert.Null(coordinator.State.LoadFailure);
        Assert.True(coordinator.State.HasUsableData);
        Assert.Equal(T1000, coordinator.State.LastSuccessfulRefreshUtc);
    }

    [Fact]
    public async Task ReadLatestAsync_UsesFalseOnly_AndDoesNotPollRunningState()
    {
        var client = new SequenceClient();
        client.Enqueue(Result("published", T1005, CatalogRefreshState.Running));
        var coordinator = Coordinator(client);
        var accepted = new List<string>();

        var result = await coordinator.ReadLatestAsync(
            (items, _) =>
            {
                accepted.Add(items.Single().DisplayName);
                return Task.CompletedTask;
            },
            CancellationToken.None
        );

        Assert.Equal([false], client.RefreshArguments);
        Assert.Equal(["published"], accepted);
        Assert.Equal(T1005, result.RefreshedAtUtc);
    }

    [Fact]
    public async Task ReadLatestAsync_FailureDuringActiveRefresh_DoesNotEndOrFailRefreshLifecycle()
    {
        var readFailure = new IOException("transient read failed");
        var client = new SequenceClient();
        client.Enqueue(Result("current", T1000, CatalogRefreshState.Running));
        client.Enqueue((_, _) => Task.FromException<OptionalInstallsSnapshotResult>(readFailure));
        var coordinator = new OptionalInstallsCacheCoordinator(
            client,
            new RecordingCacheStore(),
            TimeSpan.FromHours(1)
        );
        using var cts = new CancellationTokenSource();

        var refresh = coordinator.RefreshAsync((_, _) => Task.CompletedTask, cts.Token);
        await WaitUntilAsync(() => coordinator.State.IsRefreshing && coordinator.State.LastSuccessfulRefreshUtc == T1000);

        var thrown = await Assert.ThrowsAsync<IOException>(
            () => coordinator.ReadLatestAsync((_, _) => Task.CompletedTask, CancellationToken.None)
        );

        Assert.Same(readFailure, thrown);
        Assert.True(coordinator.State.IsRefreshing);
        Assert.Null(coordinator.State.RefreshFailure);
        Assert.Null(coordinator.State.LoadFailure);

        cts.Cancel();
        await Assert.ThrowsAnyAsync<OperationCanceledException>(() => refresh);
        Assert.False(coordinator.State.IsRefreshing);
    }

    [Fact]
    public async Task RefreshAsync_CoalescesConcurrentRegenerationCallers()
    {
        var response = new TaskCompletionSource<OptionalInstallsSnapshotResult>(TaskCreationOptions.RunContinuationsAsynchronously);
        var client = new SequenceClient();
        client.Enqueue((_, token) => response.Task.WaitAsync(token));
        var coordinator = Coordinator(client);

        var first = coordinator.RefreshAsync((_, _) => Task.CompletedTask, CancellationToken.None);
        var second = coordinator.RefreshAsync((_, _) => Task.CompletedTask, CancellationToken.None);
        await client.FirstCallObserved.Task;

        Assert.Same(first, second);
        Assert.Equal([true], client.RefreshArguments);

        response.SetResult(Result("ready", T1005, CatalogRefreshState.Idle));
        await first;
    }

    private static async Task WaitUntilAsync(Func<bool> condition)
    {
        using var timeout = new CancellationTokenSource(TimeSpan.FromSeconds(2));
        while (!condition())
        {
            await Task.Delay(10, timeout.Token);
        }
    }

    private static OptionalInstallsCacheCoordinator Coordinator(
        SequenceClient client,
        RecordingCacheStore? store = null
    ) => new(client, store ?? new RecordingCacheStore(), TimeSpan.Zero);

    private static OptionalInstallsSnapshotResult Result(
        string name,
        DateTimeOffset generatedAt,
        CatalogRefreshState state
    ) => new(
        Items: [Item(name)],
        SnapshotAvailable: true,
        SnapshotGeneratedAtUtc: generatedAt,
        RefreshState: state,
        RefreshRequestedAtUtc: null,
        RefreshCompletedAtUtc: null,
        RefreshErrorCode: null
    );

    private static OptionalInstallsSnapshotResult Unavailable(CatalogRefreshState state) => new(
        Items: [],
        SnapshotAvailable: false,
        SnapshotGeneratedAtUtc: null,
        RefreshState: state,
        RefreshRequestedAtUtc: null,
        RefreshCompletedAtUtc: null,
        RefreshErrorCode: state == CatalogRefreshState.Failed ? "refresh_failed" : null
    );

    private static OptionalInstallItem Item(string name) => new(
        ItemName: name,
        DisplayName: name,
        Version: "1.0",
        Catalog: "test",
        InstallerType: "msi",
        InstallerPackageId: name,
        InstallerLocation: $"{name}.msi",
        IsManaged: false,
        IsInstalled: false,
        Status: OptionalInstallStatus.NotInstalled,
        StatusUpdatedAtUtc: T0900,
        LastOperationId: null
    );

    private sealed class SequenceClient : IGorillaServiceClient
    {
        private readonly Queue<Func<bool, CancellationToken, Task<OptionalInstallsSnapshotResult>>> _responses = new();

        public List<bool> RefreshArguments { get; } = [];
        public TaskCompletionSource FirstCallObserved { get; } = new(TaskCreationOptions.RunContinuationsAsynchronously);

        public void Enqueue(OptionalInstallsSnapshotResult response)
            => Enqueue((_, _) => Task.FromResult(response));

        public void Enqueue(Func<bool, CancellationToken, Task<OptionalInstallsSnapshotResult>> response)
            => _responses.Enqueue(response);

        public async Task<OptionalInstallsSnapshotResult> ListOptionalInstallsAsync(
            bool refresh,
            CancellationToken cancellationToken
        )
        {
            RefreshArguments.Add(refresh);
            FirstCallObserved.TrySetResult();
            if (_responses.Count == 0)
            {
                throw new InvalidOperationException("No queued snapshot response.");
            }
            return await _responses.Dequeue()(refresh, cancellationToken);
        }

        public Task<OperationAccepted> InstallItemAsync(string itemName, CancellationToken cancellationToken)
            => throw new NotSupportedException();

        public Task<OperationAccepted> RemoveItemAsync(string itemName, CancellationToken cancellationToken)
            => throw new NotSupportedException();

        public async IAsyncEnumerable<OperationStatusEvent> StreamOperationStatusAsync(
            string operationId,
            [EnumeratorCancellation] CancellationToken cancellationToken
        )
        {
            await Task.CompletedTask;
            yield break;
        }
    }

    private sealed class RecordingCacheStore : IOptionalInstallsCacheStore
    {
        private readonly TaskCompletionSource _saveObserved = new(TaskCreationOptions.RunContinuationsAsynchronously);

        public OptionalInstallsCacheDocument? Loaded { get; init; }
        public List<OptionalInstallsCacheDocument> Saved { get; } = [];

        public Task<OptionalInstallsCacheDocument?> LoadAsync(CancellationToken cancellationToken)
            => Task.FromResult(Loaded);

        public Task SaveAsync(OptionalInstallsCacheDocument document, CancellationToken cancellationToken)
        {
            Saved.Add(document);
            _saveObserved.TrySetResult();
            return Task.CompletedTask;
        }

        public Task WaitForSaveAsync() => _saveObserved.Task;
    }
}
