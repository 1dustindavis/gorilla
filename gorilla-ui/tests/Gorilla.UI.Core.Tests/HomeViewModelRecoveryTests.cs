using System.Runtime.CompilerServices;
using Gorilla.UI.Client;
using Gorilla.UI.Client.AppCatalog;
using Gorilla.UI.Core;
using Gorilla.UI.Core.Services;
using Gorilla.UI.Core.ViewModels;
using Xunit;
using AppCatalog = Gorilla.UI.Client.AppCatalog;

namespace Gorilla.UI.Core.Tests;

public class HomeViewModelRecoveryTests
{
    private static readonly DateTimeOffset Now = DateTimeOffset.Parse("2026-09-10T05:00:00Z");

    [Fact]
    public async Task InitializeAsync_ProjectsRecoveredActiveOperationOntoRefreshedCard()
    {
        using var cancellation = new CancellationTokenSource();
        var streamStarted = new TaskCompletionSource<bool>(TaskCreationOptions.RunContinuationsAsynchronously);
        var client = new FakeClient
        {
            Operations =
            [
                ActiveOperation(),
            ],
            StreamAsync = (_, token) => ActiveStream(streamStarted, token),
        };
        var coordinator = new OptionalInstallsCacheCoordinator(client, new InMemoryCacheStore());
        var viewModel = new HomeViewModel(client, coordinator, new OperationTracker(client));

        await viewModel.InitializeAsync(cancellation.Token);
        await streamStarted.Task;

        var item = Assert.Single(viewModel.Items);
        Assert.Equal("VLC", item.ItemName);
        Assert.True(item.IsBusy);
        Assert.False(item.CanInstall);
        Assert.False(item.CanRemove);
        Assert.Equal("Installing: Installing item via managed run", item.Status);

        cancellation.Cancel();
    }

    [Fact]
    public async Task RecoveredOperation_StreamFailuresThenActiveReconciliation_ContinuesUntilCompletion()
    {
        using var cancellation = new CancellationTokenSource(TimeSpan.FromSeconds(10));
        var completedProjected = new TaskCompletionSource<bool>(TaskCreationOptions.RunContinuationsAsynchronously);
        var client = new FakeClient
        {
            Operations = [ActiveOperation()],
        };
        client.StreamAsync = (_, _) => client.StreamCalls switch
        {
            <= 2 => ThrowingStream(new IOException("pipe closed")),
            _ => CompletedStream(completedProjected),
        };

        var coordinator = new OptionalInstallsCacheCoordinator(client, new InMemoryCacheStore());
        var viewModel = new HomeViewModel(client, coordinator, new OperationTracker(client));

        await viewModel.InitializeAsync(cancellation.Token);
        await completedProjected.Task.WaitAsync(cancellation.Token);

        var item = Assert.Single(viewModel.Items);
        Assert.False(item.IsBusy);
        Assert.Equal("Succeeded: Installed", item.Status);
        Assert.True(client.StreamCalls >= 3);
        Assert.True(client.ListOperationsCalls >= 2);
    }

    private static OperationStatusEvent ActiveOperation() => new(
        "op-1",
        OperationState.Installing,
        null,
        "Installing item via managed run",
        Now,
        "VLC",
        AppCatalog.Action.Install
    );

    private static async IAsyncEnumerable<OperationStatusEvent> ActiveStream(
        TaskCompletionSource<bool> started,
        [EnumeratorCancellation] CancellationToken cancellationToken
    )
    {
        started.TrySetResult(true);
        yield return ActiveOperation();
        await Task.Delay(Timeout.InfiniteTimeSpan, cancellationToken);
    }

    private static async IAsyncEnumerable<OperationStatusEvent> CompletedStream(TaskCompletionSource<bool> projected)
    {
        yield return new OperationStatusEvent(
            "op-1",
            OperationState.Completed,
            null,
            "Installed",
            Now.AddSeconds(1),
            "VLC",
            AppCatalog.Action.Install,
            new Result(Outcome.Succeeded, "completed", Message: "Installed")
        );
        projected.TrySetResult(true);
        await Task.CompletedTask;
    }

    private static async IAsyncEnumerable<OperationStatusEvent> ThrowingStream(Exception exception)
    {
        await Task.Yield();
        throw exception;
#pragma warning disable CS0162
        yield break;
#pragma warning restore CS0162
    }

    private sealed class InMemoryCacheStore : IOptionalInstallsCacheStore
    {
        public Task<OptionalInstallsCacheDocument?> LoadAsync(CancellationToken cancellationToken)
            => Task.FromResult<OptionalInstallsCacheDocument?>(null);

        public Task SaveAsync(OptionalInstallsCacheDocument document, CancellationToken cancellationToken)
            => Task.CompletedTask;
    }

    private sealed class FakeClient : IGorillaServiceClient
    {
        public IReadOnlyList<OperationStatusEvent> Operations { get; init; } = [];
        public Func<string, CancellationToken, IAsyncEnumerable<OperationStatusEvent>> StreamAsync { get; set; }
            = (_, _) => EmptyStream();

        public int StreamCalls { get; private set; }
        public int ListOperationsCalls { get; private set; }

        public Task<IReadOnlyList<OptionalInstallItem>> ListOptionalInstallsAsync(CancellationToken cancellationToken)
            => Task.FromResult<IReadOnlyList<OptionalInstallItem>>
            ([
                new OptionalInstallItem(
                    "VLC",
                    "VLC",
                    "1.0.0",
                    "testcatalog",
                    "ps1",
                    "VLC",
                    "VLC.ps1",
                    true,
                    false,
                    OptionalInstallStatus.NotInstalled,
                    Now,
                    null,
                    Actions: new Actions(
                        new ActionDecision(true, "allowed"),
                        new ActionDecision(false, "not_installed")
                    )
                ),
            ]);

        public Task<OperationAccepted> InstallItemAsync(string itemName, CancellationToken cancellationToken)
            => throw new NotSupportedException();

        public Task<OperationAccepted> RemoveItemAsync(string itemName, CancellationToken cancellationToken)
            => throw new NotSupportedException();

        public Task<IReadOnlyList<OperationStatusEvent>> ListOperationsAsync(CancellationToken cancellationToken)
        {
            ListOperationsCalls++;
            return Task.FromResult(Operations);
        }

        public IAsyncEnumerable<OperationStatusEvent> StreamOperationStatusAsync(
            string operationId,
            CancellationToken cancellationToken
        )
        {
            StreamCalls++;
            return StreamAsync(operationId, cancellationToken);
        }

        private static async IAsyncEnumerable<OperationStatusEvent> EmptyStream()
        {
            await Task.CompletedTask;
            yield break;
        }
    }
}
