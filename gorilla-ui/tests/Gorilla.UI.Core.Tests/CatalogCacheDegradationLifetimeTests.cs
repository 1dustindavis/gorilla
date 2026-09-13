using Gorilla.UI.Client;
using Gorilla.UI.Core;
using Xunit;

namespace Gorilla.UI.Core.Tests;

public sealed class CatalogCacheDegradationLifetimeTests
{
    [Fact]
    public async Task CacheWriteFailure_PersistsAcrossLiveRefreshUntilSuccessfulPersistenceCompletes()
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
        var responses = new Queue<IReadOnlyList<OptionalInstallItem>>();
        responses.Enqueue([Item("one")]);
        responses.Enqueue([Item("two")]);
        var client = new FakeClient
        {
            ListAsync = _ => Task.FromResult(responses.Dequeue()),
        };
        var coordinator = new OptionalInstallsCacheCoordinator(client, store);

        await coordinator.RefreshAsync(CancellationToken.None);
        await WaitUntilAsync(() => coordinator.State.HasCacheWriteFailure);
        var firstFailure = coordinator.State.CacheWriteFailure;
        Assert.NotNull(firstFailure);

        await coordinator.RefreshAsync(CancellationToken.None);
        await secondSaveStarted.Task.WaitAsync(TimeSpan.FromSeconds(2));

        Assert.True(coordinator.State.IsLive);
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
            ListAsync = _ => ++call == 1
                ? Task.FromResult<IReadOnlyList<OptionalInstallItem>>([Item("one")])
                : Task.FromException<IReadOnlyList<OptionalInstallItem>>(serviceFailure),
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
    public async Task OlderCacheCompletion_CannotOverwriteNewerRefreshState()
    {
        var firstSaveStarted = new TaskCompletionSource<bool>(TaskCreationOptions.RunContinuationsAsynchronously);
        var releaseFirstSave = new TaskCompletionSource<bool>(TaskCreationOptions.RunContinuationsAsynchronously);
        var secondResponse = new TaskCompletionSource<IReadOnlyList<OptionalInstallItem>>(
            TaskCreationOptions.RunContinuationsAsynchronously
        );
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
            ListAsync = _ => ++listAttempt == 1
                ? Task.FromResult<IReadOnlyList<OptionalInstallItem>>([Item("one")])
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

        secondResponse.TrySetResult([Item("two")]);
        await secondRefresh.WaitAsync(TimeSpan.FromSeconds(2));

        Assert.True(coordinator.State.IsLive);
        Assert.False(coordinator.State.IsRefreshing);
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

    private static OptionalInstallItem Item(string itemName)
    {
        var now = DateTimeOffset.Parse("2026-09-13T17:00:00Z");
        return new OptionalInstallItem(
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
            now,
            null
        );
    }

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
        public required Func<CancellationToken, Task<IReadOnlyList<OptionalInstallItem>>> ListAsync { get; init; }
        public int ListCalls { get; private set; }

        public Task<IReadOnlyList<OptionalInstallItem>> ListOptionalInstallsAsync(CancellationToken cancellationToken)
        {
            ListCalls++;
            return ListAsync(cancellationToken);
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
