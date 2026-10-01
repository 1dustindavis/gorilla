using Gorilla.UI.Client;

namespace Gorilla.UI.Core.Tests;

internal static class SnapshotTestData
{
    internal static readonly DateTimeOffset GeneratedAtUtc =
        new(2026, 1, 1, 12, 0, 0, TimeSpan.Zero);

    internal static OptionalInstallsSnapshotResult Idle(
        IReadOnlyList<OptionalInstallItem>? items = null,
        DateTimeOffset? generatedAtUtc = null
    ) => new(
        Items: items ?? [],
        SnapshotAvailable: true,
        SnapshotGeneratedAtUtc: generatedAtUtc ?? GeneratedAtUtc,
        RefreshState: CatalogRefreshState.Idle,
        RefreshRequestedAtUtc: null,
        RefreshCompletedAtUtc: null,
        RefreshErrorCode: null
    );

    internal static OptionalInstallsSnapshotResult Unavailable(CatalogRefreshState state) => new(
        Items: [],
        SnapshotAvailable: false,
        SnapshotGeneratedAtUtc: null,
        RefreshState: state,
        RefreshRequestedAtUtc: null,
        RefreshCompletedAtUtc: null,
        RefreshErrorCode: state == CatalogRefreshState.Failed ? "refresh_failed" : null
    );
}
