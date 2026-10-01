using Gorilla.UI.Client;
using Gorilla.UI.Core;
using Xunit;

namespace Gorilla.UI.Core.Tests;

public class OptionalInstallsCacheTests
{
    [Fact]
    public async Task JsonFileStore_RoundTripsBothCacheAndSourceTimestamps()
    {
        var tempDir = MakeTempDirectory();
        try
        {
            var cachePath = Path.Combine(tempDir, "optional-installs.json");
            var store = new JsonFileOptionalInstallsCacheStore(cachePath);
            var cachedAt = DateTimeOffset.Parse("2026-02-14T18:10:00Z");
            var sourceGeneratedAt = cachedAt.AddHours(-2);
            var document = new OptionalInstallsCacheDocument(
                cachedAt,
                sourceGeneratedAt,
                [MakeItem("GoogleChrome", false, sourceGeneratedAt)]
            );

            await store.SaveAsync(document, CancellationToken.None);
            var loaded = await store.LoadAsync(CancellationToken.None);

            Assert.NotNull(loaded);
            Assert.Equal(cachedAt, loaded!.CachedAtUtc);
            Assert.Equal(sourceGeneratedAt, loaded.SourceGeneratedAtUtc);
            Assert.Single(loaded.Items);
            Assert.Equal("GoogleChrome", loaded.Items[0].ItemName);
            Assert.Equal("A browser.", loaded.Items[0].Description);
            Assert.Equal(@"C:\ProgramData\Gorilla\cache\catalog-icons\GoogleChrome.png", loaded.Items[0].IconPath);
        }
        finally
        {
            Directory.Delete(tempDir, recursive: true);
        }
    }

    [Fact]
    public async Task JsonFileStore_MalformedJson_ReturnsNull()
    {
        var tempDir = MakeTempDirectory();
        try
        {
            var cachePath = Path.Combine(tempDir, "optional-installs.json");
            await File.WriteAllTextAsync(cachePath, "{\"items\":[", CancellationToken.None);

            var loaded = await new JsonFileOptionalInstallsCacheStore(cachePath).LoadAsync(CancellationToken.None);

            Assert.Null(loaded);
        }
        finally
        {
            Directory.Delete(tempDir, recursive: true);
        }
    }

    [Fact]
    public async Task JsonFileStore_OldDocumentWithoutSourceGeneratedAt_ReturnsNull()
    {
        var tempDir = MakeTempDirectory();
        try
        {
            var cachePath = Path.Combine(tempDir, "optional-installs.json");
            await File.WriteAllTextAsync(cachePath, """
            {"cachedAtUtc":"2026-02-14T18:10:00+00:00","items":[]}
            """, CancellationToken.None);

            var loaded = await new JsonFileOptionalInstallsCacheStore(cachePath).LoadAsync(CancellationToken.None);

            Assert.Null(loaded);
        }
        finally
        {
            Directory.Delete(tempDir, recursive: true);
        }
    }

    [Fact]
    public async Task JsonFileStore_LoadsNewCacheWithItemsCreatedBeforeDescriptionAndIconPath()
    {
        var tempDir = MakeTempDirectory();
        try
        {
            var cachePath = Path.Combine(tempDir, "optional-installs.json");
            await File.WriteAllTextAsync(cachePath, """
            {"cachedAtUtc":"2026-02-14T18:10:00+00:00","sourceGeneratedAtUtc":"2026-02-14T17:00:00+00:00","items":[{"itemName":"GoogleChrome","displayName":"Google Chrome","version":"1.0.0","catalog":"testcatalog","installerType":"nupkg","installerPackageId":"GoogleChrome","installerLocation":"packages/GoogleChrome/GoogleChrome.nupkg","isManaged":true,"isInstalled":false,"status":"NotInstalled","statusUpdatedAtUtc":"2026-02-14T18:10:00+00:00","lastOperationId":null}]}
            """, CancellationToken.None);

            var loaded = await new JsonFileOptionalInstallsCacheStore(cachePath).LoadAsync(CancellationToken.None);

            Assert.NotNull(loaded);
            Assert.Equal(DateTimeOffset.Parse("2026-02-14T17:00:00Z"), loaded!.SourceGeneratedAtUtc);
            var item = Assert.Single(loaded.Items);
            Assert.Null(item.Description);
            Assert.Null(item.IconPath);
        }
        finally
        {
            Directory.Delete(tempDir, recursive: true);
        }
    }

    [Fact]
    public async Task Coordinator_PersistsAcceptedSnapshotWithSourceTimestamp()
    {
        var sourceGeneratedAt = DateTimeOffset.Parse("2026-02-14T18:10:00Z");
        var store = new InMemoryCacheStore();
        var client = new FakeClient
        {
            Result = SnapshotTestData.Idle(
                [MakeItem("VLC", true, sourceGeneratedAt)],
                sourceGeneratedAt
            )
        };
        var coordinator = new OptionalInstallsCacheCoordinator(client, store);

        var refreshed = await coordinator.RefreshAsync(CancellationToken.None);

        Assert.Single(refreshed.Items);
        Assert.Equal(sourceGeneratedAt, refreshed.RefreshedAtUtc);
        Assert.Equal(sourceGeneratedAt, coordinator.State.LastSuccessfulRefreshUtc);
        Assert.True(coordinator.State.IsLive);
        Assert.False(coordinator.State.IsRefreshing);

        await store.Saved.Task.WaitAsync(TimeSpan.FromSeconds(2));
        var saved = store.Document;
        Assert.NotNull(saved);
        Assert.Equal(sourceGeneratedAt, saved!.SourceGeneratedAtUtc);
        Assert.NotEqual(saved.SourceGeneratedAtUtc, saved.CachedAtUtc);
        Assert.Equal(refreshed.Items, saved.Items);
        Assert.Equal(refreshed.Items[0].IconPath, saved.Items[0].IconPath);
    }

    private static string MakeTempDirectory()
    {
        var path = Path.Combine(Path.GetTempPath(), "gorilla-ui-core-tests", Guid.NewGuid().ToString("N"));
        Directory.CreateDirectory(path);
        return path;
    }

    private static OptionalInstallItem MakeItem(string itemName, bool installed, DateTimeOffset now)
    {
        return new OptionalInstallItem(
            ItemName: itemName,
            DisplayName: itemName,
            Version: "1.0.0",
            Catalog: "testcatalog",
            InstallerType: "nupkg",
            InstallerPackageId: itemName,
            InstallerLocation: $"packages/{itemName}/{itemName}.nupkg",
            IsManaged: true,
            IsInstalled: installed,
            Status: installed ? OptionalInstallStatus.Installed : OptionalInstallStatus.NotInstalled,
            StatusUpdatedAtUtc: now,
            LastOperationId: null,
            Description: "A browser.",
            IconPath: $@"C:\ProgramData\Gorilla\cache\catalog-icons\{itemName}.png"
        );
    }

    private sealed class InMemoryCacheStore : IOptionalInstallsCacheStore
    {
        public OptionalInstallsCacheDocument? Document { get; private set; }
        public TaskCompletionSource<bool> Saved { get; } = new(TaskCreationOptions.RunContinuationsAsynchronously);

        public Task<OptionalInstallsCacheDocument?> LoadAsync(CancellationToken cancellationToken) => Task.FromResult(Document);

        public Task SaveAsync(OptionalInstallsCacheDocument document, CancellationToken cancellationToken)
        {
            Document = document;
            Saved.TrySetResult(true);
            return Task.CompletedTask;
        }
    }

    private sealed class FakeClient : IGorillaServiceClient
    {
        public OptionalInstallsSnapshotResult Result { get; init; } = SnapshotTestData.Idle();

        public Task<OptionalInstallsSnapshotResult> ListOptionalInstallsAsync(
            bool refresh,
            CancellationToken cancellationToken
        ) => Task.FromResult(Result);

        public Task<OperationAccepted> InstallItemAsync(string itemName, CancellationToken cancellationToken) =>
            throw new NotSupportedException();

        public Task<OperationAccepted> RemoveItemAsync(string itemName, CancellationToken cancellationToken) =>
            throw new NotSupportedException();

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
