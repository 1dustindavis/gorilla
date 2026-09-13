using Gorilla.UI.Core.Models;
using Xunit;

namespace Gorilla.UI.Core.Tests;

public sealed class CatalogTroubleshootingPresentationTests
{
    [Fact]
    public void BuildTechnicalDetails_IncludesAllActiveCatalogFailures()
    {
        var refresh = new InvalidOperationException("refresh exploded");
        var cacheWrite = new IOException("cache write exploded");
        var state = new CatalogDataState(
            HasUsableData: true,
            DataSource: CatalogDataSource.Live,
            IsInitialLoading: false,
            IsRefreshing: false,
            IsSuccessfulEmpty: false,
            LastSuccessfulRefreshUtc: DateTimeOffset.Parse("2026-09-13T20:00:00Z"),
            CachedAtUtc: null,
            RefreshFailure: refresh,
            LoadFailure: null,
            CacheWriteFailure: cacheWrite
        );

        var details = CatalogTroubleshootingPresentation.BuildTechnicalDetails(state);

        Assert.Contains("Catalog refresh failure", details, StringComparison.Ordinal);
        Assert.Contains("refresh exploded", details, StringComparison.Ordinal);
        Assert.Contains("Catalog cache write failure", details, StringComparison.Ordinal);
        Assert.Contains("cache write exploded", details, StringComparison.Ordinal);
    }

    [Fact]
    public void BuildTechnicalDetails_NoFailuresReturnsEmpty()
    {
        var state = new CatalogDataState(
            HasUsableData: true,
            DataSource: CatalogDataSource.Live,
            IsInitialLoading: false,
            IsRefreshing: false,
            IsSuccessfulEmpty: false,
            LastSuccessfulRefreshUtc: DateTimeOffset.Parse("2026-09-13T20:00:00Z"),
            CachedAtUtc: null,
            RefreshFailure: null,
            LoadFailure: null,
            CacheWriteFailure: null
        );

        Assert.Equal(string.Empty, CatalogTroubleshootingPresentation.BuildTechnicalDetails(state));
    }
}
