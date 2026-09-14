using Gorilla.UI.Client;
using Gorilla.UI.Core;
using Gorilla.UI.Core.Models;
using Gorilla.UI.Core.Services;
using Gorilla.UI.Core.ViewModels;
using Xunit;

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
        var viewModel = new HomeViewModel(
            client,
            new OptionalInstallsCacheCoordinator(client, new EmptyCacheStore()),
            new OperationTracker(client)
        );
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

    private sealed class EmptyCacheStore : IOptionalInstallsCacheStore
    {
        public Task<OptionalInstallsCacheDocument?> LoadAsync(CancellationToken cancellationToken)
            => Task.FromResult<OptionalInstallsCacheDocument?>(null);

        public Task SaveAsync(OptionalInstallsCacheDocument document, CancellationToken cancellationToken)
            => Task.CompletedTask;
    }

    private sealed class FakeClient : IGorillaServiceClient
    {
        public Task<IReadOnlyList<OptionalInstallItem>> ListOptionalInstallsAsync(CancellationToken cancellationToken)
            => Task.FromResult<IReadOnlyList<OptionalInstallItem>>([]);

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
