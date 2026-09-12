using Gorilla.UI.Client;
using Gorilla.UI.Core;
using Xunit;

namespace Gorilla.UI.Core.Tests;

public class OptionalInstallsCacheTests
{
    [Fact]
    public async Task JsonFileStore_RoundTripsDocument()
    {
        var tempDir = MakeTempDirectory();
        try
        {
            var cachePath = Path.Combine(tempDir, "optional-installs.json");
            var store = new JsonFileOptionalInstallsCacheStore(cachePath);
            var now = DateTimeOffset.Parse("2026-02-14T18:10:00Z");
            var document = new OptionalInstallsCacheDocument(now, [MakeItem("GoogleChrome", false, now)]);

            await store.SaveAsync(document, CancellationToken.None);
            var loaded = await store.LoadAsync(CancellationToken.None);

            Assert.NotNull(loaded);
            Assert.Equal(now, loaded!.CachedAtUtc);
            Assert.Single(loaded.Items);
            Assert.Equal("GoogleChrome", loaded.Items[0].ItemName);
            Assert.Equal("A browser.", loaded.Items[0].Description);
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
    public async Task JsonFileStore_LoadsCacheCreatedBeforeDescription()
    {
        var tempDir = MakeTempDirectory();
        try
        {
            var cachePath = Path.Combine(tempDir, "optional-installs.json");
            await File.WriteAllTextAsync(cachePath, """
            {"cachedAtUtc":"2026-02-14T18:10:00+00:00","items":[{"itemName":"GoogleChrome","displayName":"Google Chrome","version":"1.0.0","catalog":"testcatalog","installerType":"nupkg","installerPackageId":"GoogleChrome","installerLocation":"packages/GoogleChrome/GoogleChrome.nupkg","isManaged":true,"isInstalled":false,"status":"NotInstalled","statusUpdatedAtUtc":"2026-02-14T18:10:00+00:00","lastOperationId":null}]}
            """, CancellationToken.None);

            var loaded = await new JsonFileOptionalInstallsCacheStore(cachePath).LoadAsync(CancellationToken.None);

            Assert.NotNull(loaded);
            Assert.Null(Assert.Single(loaded!.Items).Description);
        }
        finally
        {
            Directory.Delete(tempDir, recursive: true);
        }
    }

    [Fact]
    public async Task Coordinator_RefreshSavesAndReturnsFreshData()
    {
        var now = DateTimeOffset.Parse("2026-02-14T18:10:00Z");
        var store = new InMemoryCacheStore();
        var client = new FakeClient { ListResult = [MakeItem("VLC", true, now)] };
        var coordinator = new OptionalInstallsCacheCoordinator(client, store);

        var refreshed = await coordinator.RefreshAsync(CancellationToken.None);
        var cached = await coordinator.LoadCachedAsync(CancellationToken.None);

        Assert.Single(refreshed.Items);
        Assert.Equal("VLC", refreshed.Items[0].ItemName);
        Assert.NotNull(cached);
        Assert.Equal(refreshed.RefreshedAtUtc, cached!.CachedAtUtc);
        Assert.Equal(refreshed.Items, cached.Items);
        Assert.Null(refreshed.CacheWriteFailure);
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
            Description: "A browser."
        );
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
        public IReadOnlyList<OptionalInstallItem> ListResult { get; init; } = [];

        public Task<IReadOnlyList<OptionalInstallItem>> ListOptionalInstallsAsync(CancellationToken cancellationToken) =>
            Task.FromResult(ListResult);

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
