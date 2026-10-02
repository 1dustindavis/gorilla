using System.Runtime.CompilerServices;
using Gorilla.UI.Client;
using Gorilla.UI.Client.AppCatalog;
using Gorilla.UI.Core.Models;
using Gorilla.UI.Core.Services;
using Gorilla.UI.Core.ViewModels;
using Xunit;
using AppCatalog = Gorilla.UI.Client.AppCatalog;

namespace Gorilla.UI.Core.Tests;

public sealed class AppCatalogInitiationLifecycleTests
{
    private static readonly DateTimeOffset Now = DateTimeOffset.Parse("2026-10-02T06:30:00Z");

    [Fact]
    public async Task InstallAsync_ShowsPreparingBeforeAdmissionCompletes()
    {
        var admissionEntered = NewSignal();
        var releaseAdmission = new TaskCompletionSource<OperationAccepted>(TaskCreationOptions.RunContinuationsAsynchronously);
        var client = new FakeClient
        {
            InstallAsync = async (_, _) =>
            {
                admissionEntered.TrySetResult(true);
                return await releaseAdmission.Task;
            },
        };
        var viewModel = CreateViewModel(client);
        var item = Item(ObservedState.Absent, installAllowed: true, removeAllowed: true);

        var task = viewModel.InstallAsync(item, CancellationToken.None);
        await admissionEntered.Task;

        Assert.Equal(AppCatalog.Action.Install, item.InitiatingAction);
        Assert.True(item.IsBusy);
        Assert.Equal("Preparing…", item.CardPresentation.OperationText);
        Assert.False(item.CardPresentation.PrimaryAction?.Enabled);
        Assert.False(item.CardPresentation.SecondaryAction?.Enabled);

        releaseAdmission.TrySetResult(new OperationAccepted("op-1", false, Now));
        await task;
        Assert.Null(item.InitiatingAction);
        Assert.False(item.IsBusy);
        Assert.Equal("Install was not accepted for Example.", item.TransientFeedback);
    }

    [Fact]
    public async Task RemoveAsync_ShowsPreparingBeforeAdmissionCompletes()
    {
        var admissionEntered = NewSignal();
        var releaseAdmission = new TaskCompletionSource<OperationAccepted>(TaskCreationOptions.RunContinuationsAsynchronously);
        var client = new FakeClient
        {
            RemoveAsync = async (_, _) =>
            {
                admissionEntered.TrySetResult(true);
                return await releaseAdmission.Task;
            },
        };
        var viewModel = CreateViewModel(client);
        var item = Item(ObservedState.Installed, installAllowed: true, removeAllowed: true);

        var task = viewModel.RemoveAsync(item, CancellationToken.None);
        await admissionEntered.Task;

        Assert.Equal(AppCatalog.Action.Remove, item.InitiatingAction);
        Assert.Equal("Remove in progress", item.DetailsPresentation.ActiveOperationTitle);
        Assert.Equal("Preparing", item.DetailsPresentation.ActiveOperationState);

        releaseAdmission.TrySetResult(new OperationAccepted("op-2", false, Now));
        await task;
        Assert.Null(item.InitiatingAction);
        Assert.False(item.IsBusy);
    }

    [Fact]
    public async Task FirstRealEvent_HandsOffWithoutLeavingInitiationStuck()
    {
        var eventApplied = NewSignal();
        using var cancellation = new CancellationTokenSource();
        var client = new FakeClient
        {
            InstallAsync = (_, _) => Task.FromResult(new OperationAccepted("op-1", true, Now)),
            StreamAsync = (_, token) => QueuedThenWait(eventApplied, token),
        };
        var viewModel = CreateViewModel(client);
        var item = Item(ObservedState.Absent, installAllowed: true);

        var task = viewModel.InstallAsync(item, cancellation.Token);
        await eventApplied.Task;

        Assert.Null(item.InitiatingAction);
        Assert.NotNull(item.ActiveOperation);
        Assert.Equal(OperationState.Queued, item.ActiveOperation!.State);
        Assert.Equal("Preparing…", item.CardPresentation.OperationText);
        Assert.True(item.IsBusy);

        cancellation.Cancel();
        await Assert.ThrowsAnyAsync<OperationCanceledException>(() => task);
        Assert.Null(item.InitiatingAction);
    }

    [Fact]
    public async Task CancellationBeforeFirstEvent_ClearsInitiationWithoutFakeTerminalResult()
    {
        var streamStarted = NewSignal();
        using var cancellation = new CancellationTokenSource();
        var client = new FakeClient
        {
            InstallAsync = (_, _) => Task.FromResult(new OperationAccepted("op-1", true, Now)),
            StreamAsync = (_, token) => WaitForCancellation(streamStarted, token),
        };
        var viewModel = CreateViewModel(client);
        var item = Item(ObservedState.Absent, installAllowed: true);

        var task = viewModel.InstallAsync(item, cancellation.Token);
        await streamStarted.Task;
        Assert.Equal(AppCatalog.Action.Install, item.InitiatingAction);
        Assert.Null(item.ActiveOperation);

        cancellation.Cancel();
        await Assert.ThrowsAnyAsync<OperationCanceledException>(() => task);

        Assert.Null(item.InitiatingAction);
        Assert.Null(item.ActiveOperation);
        Assert.Null(item.LatestOperation);
        Assert.False(item.IsBusy);
    }

    [Fact]
    public async Task ActionStartException_ClearsInitiation()
    {
        var client = new FakeClient
        {
            InstallAsync = (_, _) => Task.FromException<OperationAccepted>(new IOException("pipe unavailable")),
        };
        var viewModel = CreateViewModel(client);
        var item = Item(ObservedState.Absent, installAllowed: true);

        await Assert.ThrowsAsync<IOException>(() => viewModel.InstallAsync(item, CancellationToken.None));

        Assert.Null(item.InitiatingAction);
        Assert.False(item.IsBusy);
        Assert.Null(item.ActiveOperation);
    }

    [Fact]
    public async Task CatalogRefresh_DoesNotEraseInitiationBeforeFirstEvent()
    {
        var admissionEntered = NewSignal();
        var releaseAdmission = new TaskCompletionSource<OperationAccepted>(TaskCreationOptions.RunContinuationsAsynchronously);
        var snapshot = SnapshotTestData.Idle([ProtocolItem()], Now);
        var client = new FakeClient
        {
            ListAsync = (_, _) => Task.FromResult(snapshot),
            InstallAsync = async (_, _) =>
            {
                admissionEntered.TrySetResult(true);
                return await releaseAdmission.Task;
            },
        };
        var viewModel = CreateViewModel(client);
        await viewModel.InitializeAsync(CancellationToken.None);
        var item = Assert.Single(viewModel.Items);

        var task = viewModel.InstallAsync(item, CancellationToken.None);
        await admissionEntered.Task;
        await viewModel.RefreshCatalogAsync(CancellationToken.None);

        var canonical = Assert.Single(viewModel.Items);
        Assert.Same(item, canonical);
        Assert.Equal(AppCatalog.Action.Install, canonical.InitiatingAction);
        Assert.True(canonical.IsBusy);
        Assert.Equal("Preparing…", canonical.CardPresentation.OperationText);

        releaseAdmission.TrySetResult(new OperationAccepted("op-1", false, Now));
        await task;
    }

    private static HomeViewModel CreateViewModel(FakeClient client)
    {
        var coordinator = new OptionalInstallsCacheCoordinator(client, new InMemoryCacheStore());
        return new HomeViewModel(client, coordinator, new OperationTracker(client));
    }

    private static TaskCompletionSource<bool> NewSignal() => new(TaskCreationOptions.RunContinuationsAsynchronously);

    private static UiOptionalInstallItem Item(ObservedState state, bool installAllowed = false, bool removeAllowed = false)
        => new()
        {
            ItemName = "Example",
            DisplayName = "Example",
            TargetVersion = "2.0",
            Observation = new Observation(state, state == ObservedState.Absent ? null : "1.0", Now, "detail", RequirementState.Unknown),
            Policy = new Policy(true, false, false, false, Selection.None),
            InstallDecision = new ActionDecision(installAllowed, installAllowed ? string.Empty : "install_unavailable"),
            RemoveDecision = new ActionDecision(removeAllowed, removeAllowed ? string.Empty : "remove_unavailable"),
        };

    private static OptionalInstallItem ProtocolItem() => new(
        "Example", "Example", "2.0", "testcatalog", "nupkg", "Example",
        "packages/Example/Example.nupkg", true, false,
        OptionalInstallStatus.NotInstalled, Now, null
    );

    private static async IAsyncEnumerable<OperationStatusEvent> QueuedThenWait(
        TaskCompletionSource<bool> applied,
        [EnumeratorCancellation] CancellationToken cancellationToken)
    {
        yield return new OperationStatusEvent(
            "op-1", OperationState.Queued, null, "Queued", Now,
            "Example", AppCatalog.Action.Install);
        applied.TrySetResult(true);
        await Task.Delay(Timeout.InfiniteTimeSpan, cancellationToken);
    }

    private static async IAsyncEnumerable<OperationStatusEvent> WaitForCancellation(
        TaskCompletionSource<bool> started,
        [EnumeratorCancellation] CancellationToken cancellationToken)
    {
        started.TrySetResult(true);
        await Task.Delay(Timeout.InfiniteTimeSpan, cancellationToken);
        yield break;
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
        public Func<bool, CancellationToken, Task<OptionalInstallsSnapshotResult>> ListAsync { get; init; } =
            (_, _) => Task.FromResult(SnapshotTestData.Idle([], Now));
        public Func<string, CancellationToken, Task<OperationAccepted>> InstallAsync { get; init; } =
            (_, _) => Task.FromResult(new OperationAccepted("op-install", true, Now));
        public Func<string, CancellationToken, Task<OperationAccepted>> RemoveAsync { get; init; } =
            (_, _) => Task.FromResult(new OperationAccepted("op-remove", true, Now));
        public Func<string, CancellationToken, IAsyncEnumerable<OperationStatusEvent>> StreamAsync { get; init; } =
            (_, _) => EmptyStream();

        public Task<OptionalInstallsSnapshotResult> ListOptionalInstallsAsync(bool refresh, CancellationToken cancellationToken)
            => ListAsync(refresh, cancellationToken);
        public Task<OperationAccepted> InstallItemAsync(string itemName, CancellationToken cancellationToken)
            => InstallAsync(itemName, cancellationToken);
        public Task<OperationAccepted> RemoveItemAsync(string itemName, CancellationToken cancellationToken)
            => RemoveAsync(itemName, cancellationToken);
        public IAsyncEnumerable<OperationStatusEvent> StreamOperationStatusAsync(string operationId, CancellationToken cancellationToken)
            => StreamAsync(operationId, cancellationToken);

        private static async IAsyncEnumerable<OperationStatusEvent> EmptyStream()
        {
            await Task.Yield();
            yield break;
        }
    }
}
