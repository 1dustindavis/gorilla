using Gorilla.UI.Client;
using Gorilla.UI.Client.AppCatalog;
using Gorilla.UI.Core;
using Gorilla.UI.Core.Models;
using Gorilla.UI.Core.Services;
using Gorilla.UI.Core.ViewModels;
using Xunit;
using CatalogAction = Gorilla.UI.Client.AppCatalog.Action;

namespace Gorilla.UI.Core.Tests;

public sealed class HomeViewModelRecoveryProjectionTests
{
    private static readonly DateTimeOffset Now = DateTimeOffset.Parse("2026-09-13T17:42:31.1234567Z");

    [Fact]
    public async Task InitializeAsync_ProjectsRetainedFailureRecoveryWithoutCallerRepair()
    {
        var client = new FakeClient();
        var viewModel = new HomeViewModel(
            client,
            new OptionalInstallsCacheCoordinator(client, new InMemoryCacheStore()),
            new OperationTracker(client)
        );

        await viewModel.InitializeAsync(CancellationToken.None);

        var activity = Assert.Single(viewModel.ActivityItems);
        Assert.Equal("retained-failure", activity.OperationId);
        Assert.True(activity.CanRetry);
        Assert.True(activity.CanNavigate);
        Assert.Null(activity.RetryUnavailableReason);
    }

    [Fact]
    public void RecoveryTechnicalDetails_PreserveExactTimestamp()
    {
        var operation = new UiOperationPresentation(
            "op-exact-time",
            CatalogAction.Install,
            OperationState.Completed,
            null,
            new Result(Outcome.Failed, "execution_failed", "installer_exit", "Installer failed."),
            "operation message",
            Now
        );
        var item = new UiOptionalInstallItem
        {
            ItemName = "VLC",
            DisplayName = "VLC",
            Observation = new Observation(ObservedState.Absent, null, Now, string.Empty, RequirementState.NotSatisfied),
            Policy = new Policy(true, false, false, false, Selection.None),
            InstallDecision = new ActionDecision(true, "allowed"),
            RemoveDecision = new ActionDecision(false, "remove_unavailable"),
        };

        var recovery = OperationRecoveryPresentationMapper.Map(operation, item, false);

        Assert.Contains($"Timestamp: {Now:O}", recovery.TechnicalDetails, StringComparison.Ordinal);
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
        public Task<IReadOnlyList<OptionalInstallItem>> ListOptionalInstallsAsync(CancellationToken cancellationToken)
            => Task.FromResult<IReadOnlyList<OptionalInstallItem>>([
                new OptionalInstallItem(
                    "VLC",
                    "VLC",
                    "4.0",
                    "testcatalog",
                    "nupkg",
                    "VLC",
                    "packages/VLC/VLC.nupkg",
                    true,
                    false,
                    OptionalInstallStatus.NotInstalled,
                    Now,
                    null,
                    TargetVersion: "4.0",
                    Observation: new Observation(ObservedState.Absent, null, Now, string.Empty, RequirementState.NotSatisfied),
                    Policy: new Policy(true, false, false, false, Selection.None),
                    Actions: new Actions(
                        new ActionDecision(true, "allowed"),
                        new ActionDecision(false, "remove_unavailable")
                    ),
                    Description: "VLC media player"
                ),
            ]);

        public Task<IReadOnlyList<OperationStatusEvent>> ListOperationsAsync(CancellationToken cancellationToken)
            => Task.FromResult<IReadOnlyList<OperationStatusEvent>>([
                new OperationStatusEvent(
                    "retained-failure",
                    OperationState.Completed,
                    null,
                    "failed",
                    Now,
                    "VLC",
                    CatalogAction.Install,
                    new Result(Outcome.Failed, "execution_failed", "installer_exit", "Installer failed.")
                ),
            ]);

        public Task<OperationAccepted> InstallItemAsync(string itemName, CancellationToken cancellationToken)
            => throw new NotSupportedException();

        public Task<OperationAccepted> RemoveItemAsync(string itemName, CancellationToken cancellationToken)
            => throw new NotSupportedException();

        public IAsyncEnumerable<OperationStatusEvent> StreamOperationStatusAsync(
            string operationId,
            CancellationToken cancellationToken)
            => throw new NotSupportedException();
    }
}
