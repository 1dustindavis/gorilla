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
                new OperationStatusEvent(
                    "op-1",
                    OperationState.Installing,
                    null,
                    "Installing item via managed run",
                    Now,
                    "VLC",
                    AppCatalog.Action.Install
                ),
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

    private static async IAsyncEnumerable<OperationStatusEvent> ActiveStream(
        TaskCompletionSource<bool> started,
        [EnumeratorCancellation] CancellationToken cancellationToken
    )
    {
        started.TrySetResult(true);
        yield return new OperationStatusEvent(
            "op-1",
            OperationState.Installing,
            null,
            "Installing item via managed run",
            Now,
            "VLC",
            AppCatalog.Action.Install
        );
        await Task.Delay(Timeout.InfiniteTimeSpan, cancellationToken);
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
        public Func<string, CancellationToken, IAsyncEnumerable<OperationStatusEvent>> StreamAsync { get; init; }
            = (_, _) => EmptyStream();

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
            => Task.FromResult(Operations);

        public IAsyncEnumerable<OperationStatusEvent> StreamOperationStatusAsync(
            string operationId,
            CancellationToken cancellationToken
        ) => StreamAsync(operationId, cancellationToken);

        private static async IAsyncEnumerable<OperationStatusEvent> EmptyStream()
        {
            await Task.CompletedTask;
            yield break;
        }
    }
}
