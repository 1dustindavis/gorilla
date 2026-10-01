using Gorilla.UI.Client;
using Gorilla.UI.Core;
using Xunit;

namespace Gorilla.UI.Core.Tests;

public sealed class CatalogCacheDegradationLifetimeTests
{
    private static readonly DateTimeOffset T1 = DateTimeOffset.Parse("2026-09-13T17:00:00Z");
    private static readonly DateTimeOffset T2 = T1.AddMinutes(5);

    [Fact]
    public async Task CacheWriteFailure_PersistsAcrossNewerLiveRefreshUntilCurrentPersistenceCompletes()
    {
        var secondSaveStarted = new TaskCompletionSource<bool>(TaskCreationOptions.RunContinuationsAsynchronously);
        var releaseSecondSave = new TaskCompletionSource<bool>(TaskCreationOptions.RunContinuationsAsynchronously);
        var saveAttempt = 0;
        var store = new ControlledCacheStore
        {
            SaveAsyncImpl = async (_, _) =>
            {
                var attempt = Interlocked.Increment(ref saveAttempt);
                if (attempt == 1)
                {
                    throw new IOException("disk unavailable");
                }

                secondSaveStarted.TrySetResult(true);
                await releaseSecondSave.Task;
            },
        };
        var responses = new Queue<OptionalInstallsSnapshotResult>();
        responses.Enqueue(SnapshotTestData.Idle([Item("one")], T1));
        responses.Enqueue(SnapshotTestData.Idle([Item("two")], T2));
        var client = new FakeClient
        {
            ListAsync = (_, _) => Task.FromResult(responses.Dequeue()),
        };
        var coordinator = new OptionalInstallsCacheCoordinator(client, store);

        await coordinator.RefreshAsync(CancellationToken.None);
        await WaitUntilAsync(() => coordinator.State.HasCacheWriteFailure);
        var firstFailure = coordinator.State.CacheWriteFailure;
        Assert.NotNull(firstFailure);

        await coordinator.RefreshAsync(CancellationToken.None);
        await secondSaveStarted.Task.WaitAsync(TimeSpan.FromSeconds(2));

        Assert.True(coordinator.State.IsLive);
        Assert.Equal(T2, coordinator.State.LastSuccessfulRefreshUtc);
        Assert.True(coordinator.State.HasCacheWriteFailure);
        Assert.Same(firstFailure, coordinator.State.CacheWriteFailure);

        releaseSecondSave.TrySetResult(true);
        await WaitUntilAsync(() => !coordinator.State.HasCacheWriteFailure);

        Assert.False(coordinator.State.HasCacheWriteFailure);
    }

    [Fact]
    public async Task ServiceRefreshFailure_DoesNotClearExistingCacheWriteFailure()
    {
        var call = 0;
        var serviceFailure = new IOException("service unavailable");
        var store = new ControlledCacheStore
        {
            SaveAsyncImpl = (_, _) => Task.FromException(new IOException("disk unavailable")),
        };
        var client = new FakeClient
        {
            ListAsync = (_, _) => ++call == 1
                ? Task.FromResult(SnapshotTestData.Idle([Item("one")], T1))
                : Task.FromException<OptionalInstallsSnapshotResult>(serviceFailure),
        };
        var coordinator = new OptionalInstallsCacheCoordinator(client, store);

        await coordinator.RefreshAsync(CancellationToken.None);
        await WaitUntilAsync(() => coordinator.State.HasCacheWriteFailure);
        var cacheFailure = coordinator.State.CacheWriteFailure;

        await Assert.ThrowsAsync<IOException>(() => coordinator.RefreshAsync(CancellationToken.None));

        Assert.True(coordinator.State.HasRefreshFailure);
        Assert.Same(serviceFailure, coordinator.State.RefreshFailure);
        Assert.True(coordinator.State.HasCacheWriteFailure);
        Assert.Same(cacheFailure, coordinator.State.CacheWriteFailure);
    }

    [Fact]
    public async Task SupersededWriteFailure_CannotDegradeNewerPersistedSnapshot()
    {
        var firstSaveStarted = new TaskCompletionSource<bool>(TaskCreationOptions.RunContinuationsAsynchronously);
        var releaseFirstSave = new TaskCompletionSource<bool>(TaskCreationOptions.RunContinuationsAsynchronously);
        var saveAttempt = 0;
        var store = new ControlledCacheStore
        {
            SaveAsyncImpl = async (document, _) =>
            {
                if (Interlocked.Increment(ref saveAttempt) == 1)
                {
                    Assert.Equal(T1, document.SourceGeneratedAtUtc);
                    firstSaveStarted.TrySetResult(true);
                    await releaseFirstSave.Task;
                    throw new IOException("superseded write failed");
                }

                Assert.Equal(T2, document.SourceGeneratedAtUtc);
            },
        };
        var responses = new Queue<OptionalInstallsSnapshotResult>();
        responses.Enqueue(SnapshotTestData.Idle([Item("one")], T1));
        responses.Enqueue(SnapshotTestData.Idle([Item("two")], T2));
        var client = new FakeClient { ListAsync = (_, _) => Task.FromResult(responses.Dequeue()) };
        var coordinator = new OptionalInstallsCacheCoordinator(client, store);

        await coordinator.RefreshAsync(CancellationToken.None);
        await firstSaveStarted.Task.WaitAsync(TimeSpan.FromSeconds(2));
        await coordinator.RefreshAsync(CancellationToken.None);
        Assert.Equal(T2, coordinator.State.LastSuccessfulRefreshUtc);

        releaseFirstSave.TrySetResult(true);
        await WaitUntilAsync(() => saveAttempt >= 2);
        await WaitUntilAsync(() => !coordinator.State.HasCacheWriteFailure);

        Assert.False(coordinator.State.HasCacheWriteFailure);
        Assert.Equal(T2, coordinator.State.LastSuccessfulRefreshUtc);
    }

    [Fact]
    public async Task OlderCacheCompletion_CannotOverwriteNewerRefreshState()
    {
        var firstSaveStarted = new TaskCompletionSource<bool>(TaskCreationOptions.RunContinuationsAsynchronously);
        var releaseFirstSave = new TaskCompletionSource<bool>(TaskCreationOptions.RunContinuationsAsynchronously);
        var secondResponse = new TaskCompletionSource<OptionalInstallsSnapshotResult>(TaskCreationOptions.RunContinuationsAsynchronously);
        var saveAttempt = 0;
        var listAttempt = 0;
        var store = new ControlledCacheStore
        {
            SaveAsyncImpl = async (_, _) =>
            {
                if (Interlocked.Increment(ref saveAttempt) == 1)
                {
                    firstSaveStarted.TrySetResult(true);
                    await releaseFirstSave.Task;
                }
            },
        };
        var client = new FakeClient
        {
            ListAsync = (_, _) => ++listAttempt == 1
                ? Task.FromResult(SnapshotTestData.Idle([Item("one")], T1))
                : secondResponse.Task,
        };
        var coordinator = new OptionalInstallsCacheCoordinator(client, store);

        await coordinator.RefreshAsync(CancellationToken.None);
        await firstSaveStarted.Task.WaitAsync(TimeSpan.FromSeconds(2));

        var secondRefresh = coordinator.RefreshAsync(CancellationToken.None);
        await WaitUntilAsync(() => coordinator.State.IsRefreshing);
        Assert.True(coordinator.State.IsRefreshing);

        releaseFirstSave.TrySetResult(true);
        await Task.Delay(50);

        Assert.True(coordinator.State.IsRefreshing);

        secondResponse.TrySetResult(SnapshotTestData.Idle([Item("two")], T2));
        await secondRefresh.WaitAsync(TimeSpan.FromSeconds(2));

        Assert.True(coordinator.State.IsLive);
        Assert.False(coordinator.State.IsRefreshing);
        Assert.Equal(T2, coordinator.State.LastSuccessfulRefreshUtc);
        Assert.Equal(2, client.ListCalls);
    }

    private static async Task WaitUntilAsync(Func<bool> condition)
    {
        using var timeout = new CancellationTokenSource(TimeSpan.FromSeconds(2));
        while (!condition())
        {
            await Task.Delay(10, timeout.Token);
        }
    }

    private static OptionalInstallItem Item(string itemName) => new(
        itemName,
        itemName,
        "1.0.0",
        "testcatalog",
        "ps1",
        itemName,
        $"{itemName}.ps1",
        true,
        false,
        OptionalInstallStatus.NotInstalled,
        T1,
        null
    );

    private sealed class ControlledCacheStore : IOptionalInstallsCacheStore
    {
        public required Func<OptionalInstallsCacheDocument, CancellationToken, Task> SaveAsyncImpl { get; init; }

        public Task<OptionalInstallsCacheDocument?> LoadAsync(CancellationToken cancellationToken)
            => Task.FromResult<OptionalInstallsCacheDocument?>(null);

        public Task SaveAsync(OptionalInstallsCacheDocument document, CancellationToken cancellationToken)
            => SaveAsyncImpl(document, cancellationToken);
    }

    private sealed class FakeClient : IGorillaServiceClient
    {
        public required Func<bool, CancellationToken, Task<OptionalInstallsSnapshotResult>> ListAsync { get; init; }
        public int ListCalls { get; private set; }

        public Task<OptionalInstallsSnapshotResult> ListOptionalInstallsAsync(
            bool refresh,
            CancellationToken cancellationToken
        )
        {
            ListCalls++;
            return ListAsync(refresh, cancellationToken);
        }

        public Task<OperationAccepted> InstallItemAsync(string itemName, CancellationToken cancellationToken)
            => throw new NotSupportedException();

        public Task<OperationAccepted> RemoveItemAsync(string itemName, CancellationToken cancellationToken)
            => throw new NotSupportedException();

        public async IAsyncEnumerable<OperationStatusEvent> StreamOperationStatusAsync(
            string operationId,
            [System.Runtime.CompilerServices.EnumeratorCancellation] CancellationToken cancellationToken
        )
        {
            await Task.CompletedTask;
            yield break;
        }
    }
}
