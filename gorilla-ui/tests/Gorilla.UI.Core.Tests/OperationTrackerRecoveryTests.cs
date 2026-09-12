using System.Runtime.CompilerServices;
using Gorilla.UI.Client;
using Gorilla.UI.Client.AppCatalog;
using Gorilla.UI.Core.Services;
using Xunit;
using AppCatalog = Gorilla.UI.Client.AppCatalog;

namespace Gorilla.UI.Core.Tests;

public class OperationTrackerRecoveryTests
{
    private static readonly DateTimeOffset Now = DateTimeOffset.Parse("2026-09-10T05:00:00Z");

    [Fact]
    public async Task TrackAsync_ReconnectsDroppedStreamWithoutReplayingDeliveredLifecycle()
    {
        var client = new FakeClient();
        client.StreamAsync = (_, _, attempt) => attempt == 1
            ? DroppedStream()
            : ReplayedCompletedStream();
        var tracker = new OperationTracker(client);
        var delivered = new List<OperationStatusEvent>();

        await tracker.TrackAsync("op-1", delivered.Add, CancellationToken.None);

        Assert.Equal(2, client.StreamCalls);
        Assert.Equal(
            new[] { OperationState.Queued, OperationState.Installing, OperationState.Completed },
            delivered.Select(update => update.State).ToArray()
        );
        Assert.Null(tracker.GetActiveForItem("VLC"));
    }

    [Fact]
    public async Task TrackAsync_DoesNotRegressRecoveredSnapshotWhenStreamReplaysOlderEvents()
    {
        var recovered = new OperationStatusEvent(
            "op-1",
            OperationState.Installing,
            50,
            "Installing",
            Now.AddSeconds(10),
            "VLC",
            AppCatalog.Action.Install
        );
        var client = new FakeClient
        {
            ListOperationsAsyncImpl = _ => Task.FromResult<IReadOnlyList<OperationStatusEvent>>([recovered]),
            StreamAsync = (_, _, _) => ReplayedAfterRecoveredSnapshot(),
        };
        var tracker = new OperationTracker(client);
        await tracker.RefreshKnownOperationsAsync(CancellationToken.None);
        var delivered = new List<OperationStatusEvent>();

        await tracker.TrackAsync("op-1", delivered.Add, CancellationToken.None);

        Assert.Single(delivered);
        Assert.Equal(OperationState.Completed, delivered[0].State);
        Assert.Null(tracker.GetActiveForItem("VLC"));
        Assert.Equal(OperationState.Completed, tracker.GetLatestTerminalForItem("VLC")?.State);
    }

    [Fact]
    public async Task RefreshKnownOperations_RemovesVanishedOperationWithoutInventingTerminalResult()
    {
        var active = Event(OperationState.Installing, "Copying files");
        var client = new FakeClient
        {
            ListOperationsAsyncImpl = call => Task.FromResult<IReadOnlyList<OperationStatusEvent>>(
                call == 1 ? [active] : []
            ),
        };
        var tracker = new OperationTracker(client);

        await tracker.RefreshKnownOperationsAsync(CancellationToken.None);
        Assert.Equal("op-1", tracker.GetActiveForItem("VLC")?.OperationId);

        await tracker.RefreshKnownOperationsAsync(CancellationToken.None);
        Assert.Null(tracker.GetActiveForItem("VLC"));
        Assert.False(tracker.TryGetLatest("op-1", out _));
    }

    [Fact]
    public async Task ItemLookup_PrefersActiveAndKeepsLatestTerminalSeparate()
    {
        var terminal = new OperationStatusEvent(
            "op-old", OperationState.Completed, null, "Old install", Now, "VLC", AppCatalog.Action.Install,
            new Result(Outcome.Failed, "execution_failed")
        );
        var active = new OperationStatusEvent(
            "op-current", OperationState.Removing, 50, "Removing", Now.AddSeconds(1), "VLC", AppCatalog.Action.Remove
        );
        var other = new OperationStatusEvent(
            "op-other", OperationState.Installing, null, "Installing", Now.AddSeconds(2), "Other", AppCatalog.Action.Install
        );
        var client = new FakeClient
        {
            ListOperationsAsyncImpl = _ => Task.FromResult<IReadOnlyList<OperationStatusEvent>>([terminal, active, other]),
        };
        var tracker = new OperationTracker(client);

        await tracker.RefreshKnownOperationsAsync(CancellationToken.None);

        Assert.Equal("op-current", tracker.GetCurrentOrLatestForItem("VLC")?.OperationId);
        Assert.Equal("op-current", tracker.GetActiveForItem("VLC")?.OperationId);
        Assert.Equal("op-old", tracker.GetLatestTerminalForItem("VLC")?.OperationId);
        Assert.Equal("op-other", tracker.GetCurrentOrLatestForItem("Other")?.OperationId);
        Assert.True(tracker.TryGetLatest("op-old", out var byId));
        Assert.Equal("op-old", byId?.OperationId);
    }

    private static async IAsyncEnumerable<OperationStatusEvent> DroppedStream()
    {
        yield return Event(OperationState.Queued, "Operation queued");
        yield return Event(OperationState.Installing, "Installing item");
        await Task.Yield();
        throw new IOException("pipe disconnected");
    }

    private static async IAsyncEnumerable<OperationStatusEvent> ReplayedCompletedStream()
    {
        yield return Event(OperationState.Queued, "Operation queued");
        yield return Event(OperationState.Installing, "Installing item");
        yield return new OperationStatusEvent(
            "op-1",
            OperationState.Completed,
            null,
            "Installed",
            Now,
            "VLC",
            AppCatalog.Action.Install,
            new Result(Outcome.Succeeded, "completed", Message: "Installed")
        );
        await Task.CompletedTask;
    }

    private static async IAsyncEnumerable<OperationStatusEvent> ReplayedAfterRecoveredSnapshot()
    {
        yield return new OperationStatusEvent(
            "op-1", OperationState.Queued, null, "Queued", Now, "VLC", AppCatalog.Action.Install
        );
        yield return new OperationStatusEvent(
            "op-1", OperationState.Installing, 25, "Installing", Now.AddSeconds(5), "VLC", AppCatalog.Action.Install
        );
        yield return new OperationStatusEvent(
            "op-1",
            OperationState.Completed,
            null,
            "Installed",
            Now.AddSeconds(15),
            "VLC",
            AppCatalog.Action.Install,
            new Result(Outcome.Succeeded, "completed", Message: "Installed")
        );
        await Task.CompletedTask;
    }

    private static OperationStatusEvent Event(OperationState state, string message) => new(
        "op-1",
        state,
        null,
        message,
        Now,
        "VLC",
        AppCatalog.Action.Install
    );

    private sealed class FakeClient : IGorillaServiceClient
    {
        private int _listOperationCalls;

        public Func<int, Task<IReadOnlyList<OperationStatusEvent>>> ListOperationsAsyncImpl { get; init; }
            = _ => Task.FromResult<IReadOnlyList<OperationStatusEvent>>([]);

        public Func<string, CancellationToken, int, IAsyncEnumerable<OperationStatusEvent>> StreamAsync { get; set; }
            = (_, _, _) => EmptyStream();

        public int StreamCalls { get; private set; }

        public Task<IReadOnlyList<OptionalInstallItem>> ListOptionalInstallsAsync(CancellationToken cancellationToken)
            => Task.FromResult<IReadOnlyList<OptionalInstallItem>>([]);

        public Task<OperationAccepted> InstallItemAsync(string itemName, CancellationToken cancellationToken)
            => throw new NotSupportedException();

        public Task<OperationAccepted> RemoveItemAsync(string itemName, CancellationToken cancellationToken)
            => throw new NotSupportedException();

        public Task<IReadOnlyList<OperationStatusEvent>> ListOperationsAsync(CancellationToken cancellationToken)
        {
            _listOperationCalls++;
            return ListOperationsAsyncImpl(_listOperationCalls);
        }

        public IAsyncEnumerable<OperationStatusEvent> StreamOperationStatusAsync(
            string operationId,
            CancellationToken cancellationToken
        )
        {
            StreamCalls++;
            return StreamAsync(operationId, cancellationToken, StreamCalls);
        }

        private static async IAsyncEnumerable<OperationStatusEvent> EmptyStream(
            [EnumeratorCancellation] CancellationToken cancellationToken = default
        )
        {
            await Task.CompletedTask;
            yield break;
        }
    }
}
