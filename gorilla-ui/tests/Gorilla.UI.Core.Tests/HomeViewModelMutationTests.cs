using System.Runtime.CompilerServices;
using Gorilla.UI.Client;
using Gorilla.UI.Core;
using Gorilla.UI.Core.Models;
using Gorilla.UI.Core.Services;
using Gorilla.UI.Core.ViewModels;
using Xunit;

namespace Gorilla.UI.Core.Tests;

public class HomeViewModelMutationTests
{
    private static readonly DateTimeOffset Now = DateTimeOffset.Parse("2026-02-19T18:10:00Z");

    [Fact]
    public void WarningBanner_InitiallyEmpty()
    {
        var viewModel = CreateViewModel(new FakeClient());

        Assert.Equal(string.Empty, viewModel.WarningBanner);
    }

    [Fact]
    public void SetWarningBanner_RaisesPropertyChangedForWarningBanner()
    {
        var viewModel = CreateViewModel(new FakeClient());
        var propertyNames = new List<string?>();
        viewModel.PropertyChanged += (_, args) => propertyNames.Add(args.PropertyName);

        viewModel.SetWarningBanner("service unavailable");

        Assert.Contains(nameof(HomeViewModel.WarningBanner), propertyNames);
    }

    [Fact]
    public async Task InstallAsync_RejectedOperation_DoesNotTrackOrRefresh()
    {
        var client = new FakeClient
        {
            InstallAsync = (_, _) => Task.FromResult(new OperationAccepted("op-1", false, Now)),
        };
        var viewModel = CreateViewModel(client);
        var item = MakeUiItem("VLC");

        await viewModel.InstallAsync(item, CancellationToken.None);

        Assert.False(item.IsBusy);
        Assert.Equal("Install was not accepted for VLC.", viewModel.WarningBanner);
        Assert.Equal(0, client.StreamCalls);
        Assert.Equal(0, client.ListCalls);
    }

    [Fact]
    public async Task RemoveAsync_RejectedOperation_DoesNotTrackOrRefresh()
    {
        var client = new FakeClient
        {
            RemoveAsync = (_, _) => Task.FromResult(new OperationAccepted("op-2", false, Now)),
        };
        var viewModel = CreateViewModel(client);
        var item = MakeUiItem("VLC", installed: true);

        await viewModel.RemoveAsync(item, CancellationToken.None);

        Assert.False(item.IsBusy);
        Assert.Equal("Remove was not accepted for VLC.", viewModel.WarningBanner);
        Assert.Equal(0, client.StreamCalls);
        Assert.Equal(0, client.ListCalls);
    }

    [Fact]
    public async Task InstallAsync_SetsBusyWhileOperationIsActiveThenClearsIt()
    {
        var streamStarted = NewSignal();
        var releaseStream = NewSignal();
        var client = new FakeClient
        {
            InstallAsync = (_, _) => Task.FromResult(new OperationAccepted("op-1", true, Now)),
            StreamAsync = (_, _) => BlockingTerminalStream(streamStarted, releaseStream, OperationState.Succeeded),
            ListAsync = _ => Task.FromResult<IReadOnlyList<OptionalInstallItem>>([]),
        };
        var viewModel = CreateViewModel(client);
        var item = MakeUiItem("VLC");

        var installTask = viewModel.InstallAsync(item, CancellationToken.None);
        await streamStarted.Task;

        Assert.True(item.IsBusy);

        releaseStream.TrySetResult(true);
        await installTask;

        Assert.False(item.IsBusy);
    }

    [Fact]
    public async Task RemoveAsync_SetsBusyWhileOperationIsActiveThenClearsIt()
    {
        var streamStarted = NewSignal();
        var releaseStream = NewSignal();
        var client = new FakeClient
        {
            RemoveAsync = (_, _) => Task.FromResult(new OperationAccepted("op-2", true, Now)),
            StreamAsync = (_, _) => BlockingTerminalStream(streamStarted, releaseStream, OperationState.Succeeded),
            ListAsync = _ => Task.FromResult<IReadOnlyList<OptionalInstallItem>>([]),
        };
        var viewModel = CreateViewModel(client);
        var item = MakeUiItem("VLC", installed: true);

        var removeTask = viewModel.RemoveAsync(item, CancellationToken.None);
        await streamStarted.Task;

        Assert.True(item.IsBusy);

        releaseStream.TrySetResult(true);
        await removeTask;

        Assert.False(item.IsBusy);
    }

    [Fact]
    public async Task RemoveAsync_StreamFailure_UsesRemoveSpecificWarningAndDoesNotRefresh()
    {
        var client = new FakeClient
        {
            RemoveAsync = (_, _) => Task.FromResult(new OperationAccepted("op-2", true, Now)),
            StreamAsync = (_, _) => ThrowingStream(new IOException("pipe closed")),
        };
        var viewModel = CreateViewModel(client);
        var item = MakeUiItem("VLC", installed: true);

        await viewModel.RemoveAsync(item, CancellationToken.None);

        Assert.Contains("Remove queued, but live status stream failed:", viewModel.WarningBanner);
        Assert.Contains("pipe closed", viewModel.WarningBanner);
        Assert.Equal(0, client.ListCalls);
        Assert.False(item.IsBusy);
    }

    [Fact]
    public async Task FindItem_NoMatch_ReturnsNull()
    {
        var client = new FakeClient
        {
            ListAsync = _ => Task.FromResult<IReadOnlyList<OptionalInstallItem>>([MakeProtocolItem("GoogleChrome", false)]),
        };
        var viewModel = CreateViewModel(client);
        await viewModel.InitializeAsync(CancellationToken.None);

        Assert.Null(viewModel.FindItem("VLC"));
    }

    [Fact]
    public async Task InstallAsync_NonTerminalStream_DoesNotRefreshItems()
    {
        var client = new FakeClient
        {
            InstallAsync = (_, _) => Task.FromResult(new OperationAccepted("op-1", true, Now)),
            StreamAsync = (_, _) => Stream(
                new OperationStatusEvent("op-1", OperationState.Installing, 50, "Copying files", Now)
            ),
        };
        var viewModel = CreateViewModel(client);
        var item = MakeUiItem("VLC");

        await viewModel.InstallAsync(item, CancellationToken.None);

        Assert.Equal(0, client.ListCalls);
        Assert.Equal("Installing: Copying files", item.Status);
    }

    [Fact]
    public async Task InstallAsync_RefreshFailureAfterOperationFailure_PreservesOperationWarning()
    {
        var client = new FakeClient
        {
            InstallAsync = (_, _) => Task.FromResult(new OperationAccepted("op-1", true, Now)),
            StreamAsync = (_, _) => Stream(
                new OperationStatusEvent("op-1", OperationState.Failed, 40, "Install failed", Now, "installer_failed", "exit code 1")
            ),
            ListAsync = _ => Task.FromException<IReadOnlyList<OptionalInstallItem>>(new IOException("refresh unavailable")),
        };
        var viewModel = CreateViewModel(client);
        var item = MakeUiItem("VLC");

        await viewModel.InstallAsync(item, CancellationToken.None);

        Assert.Equal("Operation for VLC ended with Failed: exit code 1", viewModel.WarningBanner);
        Assert.Equal(1, client.ListCalls);
    }

    [Theory]
    [InlineData(OperationState.Failed, "Install failed", "exit code 1", "Operation for VLC ended with Failed: exit code 1")]
    [InlineData(OperationState.Canceled, "Canceled by user", "", "Operation for VLC ended with Canceled: Canceled by user")]
    public async Task InstallAsync_FailedOrCanceledOperation_UsesErrorMessageWithMessageFallback(
        OperationState state,
        string message,
        string errorMessage,
        string expectedWarning)
    {
        var client = new FakeClient
        {
            InstallAsync = (_, _) => Task.FromResult(new OperationAccepted("op-1", true, Now)),
            StreamAsync = (_, _) => Stream(
                new OperationStatusEvent("op-1", state, 100, message, Now, "operation_error", errorMessage)
            ),
            ListAsync = _ => Task.FromResult<IReadOnlyList<OptionalInstallItem>>([]),
        };
        var viewModel = CreateViewModel(client);
        var item = MakeUiItem("VLC");

        await viewModel.InstallAsync(item, CancellationToken.None);

        Assert.Equal(expectedWarning, viewModel.WarningBanner);
    }

    [Fact]
    public async Task InstallAsync_CancellationDuringStatusTracking_IsPropagated()
    {
        var streamStarted = NewSignal();
        using var cancellation = new CancellationTokenSource();
        var client = new FakeClient
        {
            InstallAsync = (_, _) => Task.FromResult(new OperationAccepted("op-1", true, Now)),
            StreamAsync = (_, token) => WaitForCancellationStream(streamStarted, token),
        };
        var viewModel = CreateViewModel(client);
        var item = MakeUiItem("VLC");

        var installTask = viewModel.InstallAsync(item, cancellation.Token);
        await streamStarted.Task;
        cancellation.Cancel();

        await Assert.ThrowsAnyAsync<OperationCanceledException>(() => installTask);
        Assert.False(item.IsBusy);
        Assert.Equal(0, client.ListCalls);
    }

    [Fact]
    public async Task InstallAsync_CancellationDuringPostOperationRefresh_IsPropagated()
    {
        var refreshStarted = NewSignal();
        using var cancellation = new CancellationTokenSource();
        var client = new FakeClient
        {
            InstallAsync = (_, _) => Task.FromResult(new OperationAccepted("op-1", true, Now)),
            StreamAsync = (_, _) => Stream(
                new OperationStatusEvent("op-1", OperationState.Succeeded, 100, "Installed", Now)
            ),
            ListAsync = async token =>
            {
                refreshStarted.TrySetResult(true);
                await Task.Delay(Timeout.InfiniteTimeSpan, token);
                return Array.Empty<OptionalInstallItem>();
            },
        };
        var viewModel = CreateViewModel(client);
        var item = MakeUiItem("VLC");

        var installTask = viewModel.InstallAsync(item, cancellation.Token);
        await refreshStarted.Task;
        cancellation.Cancel();

        await Assert.ThrowsAnyAsync<OperationCanceledException>(() => installTask);
        Assert.False(item.IsBusy);
        Assert.Equal(1, client.ListCalls);
    }

    private static HomeViewModel CreateViewModel(FakeClient client)
    {
        var coordinator = new OptionalInstallsCacheCoordinator(client, new InMemoryCacheStore());
        return new HomeViewModel(client, coordinator, new OperationTracker(client));
    }

    private static TaskCompletionSource<bool> NewSignal() =>
        new(TaskCreationOptions.RunContinuationsAsynchronously);

    private static UiOptionalInstallItem MakeUiItem(string itemName, bool installed = false) => new()
    {
        ItemName = itemName,
        DisplayName = itemName,
        Version = "1.0.0",
        Status = installed ? "Installed" : "NotInstalled",
        IsInstalled = installed,
    };

    private static OptionalInstallItem MakeProtocolItem(string itemName, bool installed) => new(
        itemName,
        itemName,
        "1.0.0",
        "testcatalog",
        "nupkg",
        itemName,
        $"packages/{itemName}/{itemName}.nupkg",
        true,
        installed,
        installed ? OptionalInstallStatus.Installed : OptionalInstallStatus.NotInstalled,
        Now,
        null
    );

    private static async IAsyncEnumerable<OperationStatusEvent> Stream(params OperationStatusEvent[] events)
    {
        foreach (var update in events)
        {
            await Task.Yield();
            yield return update;
        }
    }

    private static async IAsyncEnumerable<OperationStatusEvent> BlockingTerminalStream(
        TaskCompletionSource<bool> started,
        TaskCompletionSource<bool> release,
        OperationState terminalState)
    {
        started.TrySetResult(true);
        await release.Task;
        yield return new OperationStatusEvent("op", terminalState, 100, "done", Now);
    }

    private static async IAsyncEnumerable<OperationStatusEvent> WaitForCancellationStream(
        TaskCompletionSource<bool> started,
        [EnumeratorCancellation] CancellationToken cancellationToken)
    {
        started.TrySetResult(true);
        await Task.Delay(Timeout.InfiniteTimeSpan, cancellationToken);
        yield break;
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
        private OptionalInstallsCacheDocument? _document;

        public Task<OptionalInstallsCacheDocument?> LoadAsync(CancellationToken cancellationToken) => Task.FromResult(_document);

        public Task SaveAsync(OptionalInstallsCacheDocument document, CancellationToken cancellationToken)
        {
            _document = document;
            return Task.CompletedTask;
        }
    }

    private sealed class FakeClient : IGorillaServiceClient
    {
        public Func<CancellationToken, Task<IReadOnlyList<OptionalInstallItem>>> ListAsync { get; init; } = _ => Task.FromResult<IReadOnlyList<OptionalInstallItem>>([]);
        public Func<string, CancellationToken, Task<OperationAccepted>> InstallAsync { get; init; } = (_, _) => Task.FromResult(new OperationAccepted("op-install", true, Now));
        public Func<string, CancellationToken, Task<OperationAccepted>> RemoveAsync { get; init; } = (_, _) => Task.FromResult(new OperationAccepted("op-remove", true, Now));
        public Func<string, CancellationToken, IAsyncEnumerable<OperationStatusEvent>> StreamAsync { get; init; } = (_, _) => Stream();

        public int ListCalls { get; private set; }
        public int StreamCalls { get; private set; }

        public Task<IReadOnlyList<OptionalInstallItem>> ListOptionalInstallsAsync(CancellationToken cancellationToken)
        {
            ListCalls++;
            return ListAsync(cancellationToken);
        }

        public Task<OperationAccepted> InstallItemAsync(string itemName, CancellationToken cancellationToken) => InstallAsync(itemName, cancellationToken);
        public Task<OperationAccepted> RemoveItemAsync(string itemName, CancellationToken cancellationToken) => RemoveAsync(itemName, cancellationToken);

        public IAsyncEnumerable<OperationStatusEvent> StreamOperationStatusAsync(string operationId, CancellationToken cancellationToken)
        {
            StreamCalls++;
            return StreamAsync(operationId, cancellationToken);
        }
    }
}
