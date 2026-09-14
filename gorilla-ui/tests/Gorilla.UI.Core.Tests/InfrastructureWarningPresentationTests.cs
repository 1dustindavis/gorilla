using Gorilla.UI.Client;
using Gorilla.UI.Client.AppCatalog;
using Gorilla.UI.Core;
using Gorilla.UI.Core.Models;
using Gorilla.UI.Core.Services;
using Gorilla.UI.Core.ViewModels;
using Xunit;
using AppCatalog = Gorilla.UI.Client.AppCatalog;

namespace Gorilla.UI.Core.Tests;

public sealed class InfrastructureWarningPresentationTests
{
    [Fact]
    public void Create_KeepsExceptionOutOfPrimaryMessageAndPreservesDiagnostics()
    {
        var exception = new IOException("raw transport diagnostic");

        var warning = InfrastructureWarningPresentation.Create(
            "Operation status is temporarily unavailable.",
            "Retained operation lookup",
            exception,
            operationId: "operation-17",
            itemName: "VLC",
            expectedAction: "Install"
        );

        Assert.Equal("Operation status is temporarily unavailable.", warning.Message);
        Assert.DoesNotContain(exception.Message, warning.Message, StringComparison.Ordinal);
        Assert.True(warning.HasTechnicalDetails);
        Assert.Contains("Retained operation lookup", warning.TechnicalDetails, StringComparison.Ordinal);
        Assert.Contains(typeof(IOException).FullName!, warning.TechnicalDetails, StringComparison.Ordinal);
        Assert.Contains(exception.Message, warning.TechnicalDetails, StringComparison.Ordinal);
        Assert.Contains("operation-17", warning.TechnicalDetails, StringComparison.Ordinal);
        Assert.Contains("VLC", warning.TechnicalDetails, StringComparison.Ordinal);
        Assert.Contains("Install", warning.TechnicalDetails, StringComparison.Ordinal);
    }

    [Fact]
    public void ViewModel_ChangingAndClearingWarningKeepsCompatibilitySurfaceSynchronized()
    {
        var client = new FakeClient();
        var viewModel = CreateViewModel(client);
        var exception = new InvalidOperationException("protocol detail");

        viewModel.ReportInfrastructureWarning(
            "Gorilla couldn't reconcile the operation after losing status updates.",
            "Reconciliation",
            exception
        );

        Assert.Equal(viewModel.InfrastructureWarning.Message, viewModel.WarningBanner);
        Assert.DoesNotContain(exception.Message, viewModel.WarningBanner, StringComparison.Ordinal);
        Assert.Contains(exception.Message, viewModel.InfrastructureWarning.TechnicalDetails, StringComparison.Ordinal);

        viewModel.ClearInfrastructureWarning();

        Assert.Empty(viewModel.WarningBanner);
        Assert.Equal(InfrastructureWarningPresentation.None, viewModel.InfrastructureWarning);
        Assert.False(viewModel.InfrastructureWarning.HasTechnicalDetails);
    }

    [Fact]
    public async Task Initialize_RetainedOperationLookupFailureRemainsInfrastructureUncertainty()
    {
        var lookupFailure = new IOException("retained operation pipe unavailable");
        var client = new FakeClient
        {
            ListOperationsAsyncImpl = _ => Task.FromException<IReadOnlyList<OperationStatusEvent>>(lookupFailure),
        };
        var viewModel = CreateViewModel(client);

        await viewModel.InitializeAsync(CancellationToken.None);

        Assert.False(viewModel.IsActivityLoaded);
        Assert.Empty(viewModel.ActivityItems);
        Assert.Equal("Operation status is temporarily unavailable.", viewModel.WarningBanner);
        Assert.DoesNotContain(lookupFailure.Message, viewModel.WarningBanner, StringComparison.Ordinal);
        Assert.Contains(
            "Retained operation lookup during App Catalog initialization",
            viewModel.InfrastructureWarning.TechnicalDetails,
            StringComparison.Ordinal
        );
        Assert.Contains(lookupFailure.Message, viewModel.InfrastructureWarning.TechnicalDetails, StringComparison.Ordinal);
        Assert.Contains(typeof(IOException).FullName!, viewModel.InfrastructureWarning.TechnicalDetails, StringComparison.Ordinal);
    }

    [Fact]
    public async Task ActionStartTransportFailure_DoesNotFabricateActivityOrOperationOutcome()
    {
        var transportFailure = new IOException("named pipe unavailable");
        var client = new FakeClient
        {
            InstallAsync = (_, _) => Task.FromException<OperationAccepted>(transportFailure),
        };
        var viewModel = CreateViewModel(client);
        var item = new UiOptionalInstallItem
        {
            ItemName = "VLC",
            DisplayName = "VLC",
        };

        var thrown = await Assert.ThrowsAsync<IOException>(
            () => viewModel.InstallAsync(item, CancellationToken.None)
        );

        Assert.Same(transportFailure, thrown);
        Assert.False(item.IsBusy);
        Assert.Empty(viewModel.ActivityItems);
        Assert.Null(item.ActiveOperation);
        Assert.Null(item.LatestOperation);

        viewModel.SetActionStartInfrastructureWarning(AppCatalog.Action.Install, item.ItemName, thrown);

        Assert.Equal(
            "Gorilla couldn't start that action. Refresh and try again.",
            viewModel.InfrastructureWarning.Message
        );
        Assert.DoesNotContain(transportFailure.Message, viewModel.InfrastructureWarning.Message, StringComparison.Ordinal);
        Assert.Contains(transportFailure.Message, viewModel.InfrastructureWarning.TechnicalDetails, StringComparison.Ordinal);
        Assert.Contains("VLC", viewModel.InfrastructureWarning.TechnicalDetails, StringComparison.Ordinal);
        Assert.Contains("Install", viewModel.InfrastructureWarning.TechnicalDetails, StringComparison.Ordinal);
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
        public Func<string, CancellationToken, Task<OperationAccepted>> InstallAsync { get; init; } =
            (_, _) => Task.FromException<OperationAccepted>(new NotSupportedException());
        public Func<CancellationToken, Task<IReadOnlyList<OperationStatusEvent>>> ListOperationsAsyncImpl { get; init; } =
            _ => Task.FromResult<IReadOnlyList<OperationStatusEvent>>([]);

        public Task<IReadOnlyList<OptionalInstallItem>> ListOptionalInstallsAsync(CancellationToken cancellationToken)
            => Task.FromResult<IReadOnlyList<OptionalInstallItem>>([]);

        public Task<OperationAccepted> InstallItemAsync(string itemName, CancellationToken cancellationToken)
            => InstallAsync(itemName, cancellationToken);

        public Task<OperationAccepted> RemoveItemAsync(string itemName, CancellationToken cancellationToken)
            => throw new NotSupportedException();

        public Task<IReadOnlyList<OperationStatusEvent>> ListOperationsAsync(CancellationToken cancellationToken)
            => ListOperationsAsyncImpl(cancellationToken);

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
