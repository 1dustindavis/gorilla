using Gorilla.UI.Client;
using Gorilla.UI.Client.AppCatalog;
using Gorilla.UI.Core;
using Gorilla.UI.Core.Services;
using Gorilla.UI.Core.ViewModels;
using Xunit;
using AppCatalog = Gorilla.UI.Client.AppCatalog;

namespace Gorilla.UI.Core.Tests;

public sealed class RetainedOperationRefreshRecoveryTests
{
    private static readonly DateTimeOffset Now = DateTimeOffset.Parse("2026-09-14T20:00:00Z");

    [Fact]
    public async Task StartupLookupFailure_CanRecoverActivityWithoutRestart()
    {
        var client = new FakeClient { FailRetainedLookups = 1 };
        var viewModel = CreateViewModel(client);

        await viewModel.InitializeAsync(CancellationToken.None);

        Assert.False(viewModel.IsActivityLoaded);
        Assert.Equal("Operation status is temporarily unavailable.", viewModel.WarningBanner);

        await viewModel.RetryActivityLoadAsync(CancellationToken.None);

        Assert.True(viewModel.IsActivityLoaded);
        Assert.Single(viewModel.ActivityItems);
        Assert.Empty(viewModel.WarningBanner);
        Assert.Equal(2, client.ListOperationsCalls);
    }

    [Fact]
    public async Task FailedActivityRetry_RemainsUnavailableAndUpdatesWarning()
    {
        var client = new FakeClient { FailRetainedLookups = 2 };
        var viewModel = CreateViewModel(client);

        await viewModel.InitializeAsync(CancellationToken.None);
        await viewModel.RetryActivityLoadAsync(CancellationToken.None);

        Assert.False(viewModel.IsActivityLoaded);
        Assert.Equal("Operation status is temporarily unavailable.", viewModel.WarningBanner);
        Assert.Equal(
            "Retained operation lookup during App Catalog refresh",
            viewModel.InfrastructureWarning.Context
        );
        Assert.Equal(2, client.ListOperationsCalls);
    }

    [Fact]
    public async Task SuccessfulActivityRetry_DoesNotClearUnrelatedInfrastructureWarning()
    {
        var client = new FakeClient { FailRetainedLookups = 1 };
        var viewModel = CreateViewModel(client);

        await viewModel.InitializeAsync(CancellationToken.None);
        viewModel.SetActionStartInfrastructureWarning(
            AppCatalog.Action.Install,
            "VLC",
            new IOException("action pipe unavailable")
        );

        await viewModel.RetryActivityLoadAsync(CancellationToken.None);

        Assert.True(viewModel.IsActivityLoaded);
        Assert.Equal("Gorilla couldn't start that action. Refresh and try again.", viewModel.WarningBanner);
    }

    private static HomeViewModel CreateViewModel(FakeClient client)
        => new(
            client,
            new OptionalInstallsCacheCoordinator(client, new EmptyCacheStore()),
            new OperationTracker(client)
        );

    private sealed class EmptyCacheStore : IOptionalInstallsCacheStore
    {
        public Task<OptionalInstallsCacheDocument?> LoadAsync(CancellationToken cancellationToken)
            => Task.FromResult<OptionalInstallsCacheDocument?>(null);

        public Task SaveAsync(OptionalInstallsCacheDocument document, CancellationToken cancellationToken)
            => Task.CompletedTask;
    }

    private sealed class FakeClient : IGorillaServiceClient
    {
        public int FailRetainedLookups { get; init; }
        public int ListOperationsCalls { get; private set; }

        public Task<IReadOnlyList<OptionalInstallItem>> ListOptionalInstallsAsync(CancellationToken cancellationToken)
            => Task.FromResult<IReadOnlyList<OptionalInstallItem>>([]);

        public Task<OperationAccepted> InstallItemAsync(string itemName, CancellationToken cancellationToken)
            => throw new NotSupportedException();

        public Task<OperationAccepted> RemoveItemAsync(string itemName, CancellationToken cancellationToken)
            => throw new NotSupportedException();

        public Task<IReadOnlyList<OperationStatusEvent>> ListOperationsAsync(CancellationToken cancellationToken)
        {
            ListOperationsCalls++;
            if (ListOperationsCalls <= FailRetainedLookups)
            {
                return Task.FromException<IReadOnlyList<OperationStatusEvent>>(
                    new IOException("retained operation pipe unavailable")
                );
            }

            return Task.FromResult<IReadOnlyList<OperationStatusEvent>>
            ([
                new OperationStatusEvent(
                    "op-1",
                    OperationState.Completed,
                    null,
                    "Installed",
                    Now,
                    "VLC",
                    AppCatalog.Action.Install,
                    new Result(Outcome.Succeeded, "completed", Message: "Installed")
                ),
            ]);
        }

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
