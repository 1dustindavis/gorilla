using System.Runtime.CompilerServices;
using Gorilla.UI.Client;
using Gorilla.UI.Client.AppCatalog;
using Gorilla.UI.Core;
using Gorilla.UI.Core.Models;
using Gorilla.UI.Core.Services;
using Gorilla.UI.Core.ViewModels;
using Xunit;
using CatalogAction = Gorilla.UI.Client.AppCatalog.Action;

namespace Gorilla.UI.Core.Tests;

public class Stage6RecoveryBoundaryTests
{
    private static readonly DateTimeOffset Now = DateTimeOffset.Parse("2026-09-14T00:00:00Z");

    [Fact]
    public async Task RetryAsync_ServicePolicyRejectionBecomesAttemptFeedbackWithoutNewOperation()
    {
        var client = new RejectingClient("already_selected") { Catalog = [ProtocolItem()] };
        var viewModel = CreateViewModel(client);
        await viewModel.InitializeAsync(CancellationToken.None);

        var before = Assert.Single(viewModel.ActivityItems);
        Assert.True(before.CanRetry);

        var result = await viewModel.RetryAsync("old-op", CancellationToken.None);

        Assert.False(result.Started);
        Assert.NotNull(result.Feedback);
        Assert.Contains("already selected", result.Feedback!, StringComparison.OrdinalIgnoreCase);
        Assert.Equal(1, client.InstallCalls);
        var activity = Assert.Single(viewModel.ActivityItems);
        Assert.Equal("old-op", activity.OperationId);
        Assert.Equal(Outcome.Failed, activity.Result?.Outcome);
        Assert.NotNull(activity.RetryAttemptFeedback);
        Assert.Contains("already selected", activity.RetryAttemptFeedback!, StringComparison.OrdinalIgnoreCase);
        var item = Assert.IsType<UiOptionalInstallItem>(viewModel.FindItem("VLC"));
        Assert.NotNull(item.TransientFeedback);
        Assert.Contains("already selected", item.TransientFeedback!, StringComparison.OrdinalIgnoreCase);
    }

    [Fact]
    public async Task RetryAsync_UnknownServiceErrorStillPropagatesAsInfrastructureFailure()
    {
        var client = new RejectingClient("unexpected_backend_error") { Catalog = [ProtocolItem()] };
        var viewModel = CreateViewModel(client);
        await viewModel.InitializeAsync(CancellationToken.None);

        var error = await Assert.ThrowsAsync<ServiceErrorException>(
            () => viewModel.RetryAsync("old-op", CancellationToken.None)
        );

        Assert.Equal("unexpected_backend_error", error.ErrorCode);
        Assert.Single(viewModel.ActivityItems);
        Assert.Equal("old-op", viewModel.ActivityItems[0].OperationId);
    }

    [Fact]
    public void RecoveryPresentation_UnknownSingleLineDetailStaysTechnicalOnly()
    {
        const string diagnostic = "Future backend diagnostic that should not become primary UI";
        var operation = new UiOperationPresentation(
            "op-future",
            CatalogAction.Install,
            OperationState.Completed,
            null,
            new Result(Outcome.Failed, "execution_failed", "future_backend_detail", diagnostic),
            diagnostic,
            Now
        );

        var recovery = OperationRecoveryPresentationMapper.Map(operation, UiItem(), false);

        Assert.Null(recovery.UserMessage);
        Assert.Equal("Installation failed", recovery.OutcomeTitle);
        Assert.Contains("Detail code: future_backend_detail", recovery.TechnicalDetails);
        Assert.Contains(diagnostic, recovery.TechnicalDetails);
    }

    [Fact]
    public void RecoveryPresentation_ExceptionLikeKnownMessageStaysTechnicalOnly()
    {
        const string diagnostic = "System.InvalidOperationException: installer bridge failed";
        var operation = new UiOperationPresentation(
            "op-exception",
            CatalogAction.Install,
            OperationState.Completed,
            null,
            new Result(Outcome.Failed, "execution_failed", "installer_failed", diagnostic),
            diagnostic,
            Now
        );

        var recovery = OperationRecoveryPresentationMapper.Map(operation, UiItem(), false);

        Assert.Null(recovery.UserMessage);
        Assert.Contains(diagnostic, recovery.TechnicalDetails);
    }

    private static HomeViewModel CreateViewModel(RejectingClient client)
        => new(client, new OptionalInstallsCacheCoordinator(client, new InMemoryCacheStore()), new OperationTracker(client));

    private static UiOptionalInstallItem UiItem() => new()
    {
        ItemName = "VLC",
        DisplayName = "VLC",
        TargetVersion = "4.0",
        Observation = new Observation(ObservedState.Absent, null, Now, string.Empty, RequirementState.NotSatisfied),
        Policy = new Policy(true, false, false, false, Selection.None),
        InstallDecision = new ActionDecision(true, string.Empty),
        RemoveDecision = new ActionDecision(false, "already_absent"),
    };

    private static OptionalInstallItem ProtocolItem()
        => new(
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
            Actions: new Actions(new ActionDecision(true, string.Empty), new ActionDecision(false, "already_absent")),
            Description: "VLC media player"
        );

    private static OperationStatusEvent Historical()
        => new(
            "old-op",
            OperationState.Completed,
            null,
            "Installation error: exit status 7",
            Now,
            "VLC",
            CatalogAction.Install,
            new Result(Outcome.Failed, "execution_failed", "installer_failed", "Installation error: exit status 7")
        );

    private sealed class InMemoryCacheStore : IOptionalInstallsCacheStore
    {
        public Task<OptionalInstallsCacheDocument?> LoadAsync(CancellationToken cancellationToken)
            => Task.FromResult<OptionalInstallsCacheDocument?>(null);

        public Task SaveAsync(OptionalInstallsCacheDocument document, CancellationToken cancellationToken)
            => Task.CompletedTask;
    }

    private sealed class RejectingClient : IGorillaServiceClient
    {
        private readonly string _errorCode;

        public RejectingClient(string errorCode)
        {
            _errorCode = errorCode;
        }

        public IReadOnlyList<OptionalInstallItem> Catalog { get; init; } = [];
        public int InstallCalls { get; private set; }

        public Task<IReadOnlyList<OptionalInstallItem>> ListOptionalInstallsAsync(CancellationToken cancellationToken)
            => Task.FromResult(Catalog);

        public Task<IReadOnlyList<OperationStatusEvent>> ListOperationsAsync(CancellationToken cancellationToken)
            => Task.FromResult<IReadOnlyList<OperationStatusEvent>>([Historical()]);

        public Task<OperationAccepted> InstallItemAsync(string itemName, CancellationToken cancellationToken)
        {
            InstallCalls++;
            throw new ServiceErrorException(_errorCode, "Current service truth rejected the request.");
        }

        public Task<OperationAccepted> RemoveItemAsync(string itemName, CancellationToken cancellationToken)
            => throw new InvalidOperationException("Unexpected remove request.");

        public IAsyncEnumerable<OperationStatusEvent> StreamOperationStatusAsync(
            string operationId,
            CancellationToken cancellationToken
        ) => Empty(cancellationToken);

        private static async IAsyncEnumerable<OperationStatusEvent> Empty(
            [EnumeratorCancellation] CancellationToken cancellationToken = default
        )
        {
            await Task.CompletedTask;
            yield break;
        }
    }
}
