using System.Globalization;
using Gorilla.UI.App.Services;
using Gorilla.UI.Core.Models;

namespace Gorilla.UI.Core.Tests;

public sealed class CatalogFreshnessPresentationTests
{
    private static readonly CultureInfo Culture = CultureInfo.GetCultureInfo("en-US");

    [Fact]
    public void BuildStatusText_LiveToday_UsesShortLocalTime()
    {
        var now = LocalDateTime(2026, 9, 28, 20, 0);
        var timestamp = LocalDateTime(2026, 9, 28, 19, 42);
        var state = LiveState(timestamp);

        var text = CatalogFreshnessPresentation.BuildStatusText(state, now, Culture);

        Assert.Equal($"Updated {timestamp.ToString("t", Culture)}", text);
    }

    [Fact]
    public void BuildStatusText_LiveYesterday_UsesYesterday()
    {
        var now = LocalDateTime(2026, 9, 29, 0, 5);
        var timestamp = LocalDateTime(2026, 9, 28, 23, 55);
        var state = LiveState(timestamp);

        var text = CatalogFreshnessPresentation.BuildStatusText(state, now, Culture);

        Assert.Equal("Updated yesterday", text);
    }

    [Fact]
    public void BuildStatusText_LiveOlder_UsesAbbreviatedDate()
    {
        var now = LocalDateTime(2026, 9, 28, 20, 0);
        var timestamp = LocalDateTime(2026, 9, 26, 15, 14);
        var state = LiveState(timestamp);

        var text = CatalogFreshnessPresentation.BuildStatusText(state, now, Culture);

        Assert.Equal("Updated Sep 26", text);
    }

    [Fact]
    public void BuildStatusText_CachedYesterday_UsesSameRelativeFormatter()
    {
        var now = LocalDateTime(2026, 9, 28, 20, 0);
        var timestamp = LocalDateTime(2026, 9, 27, 15, 14);
        var state = CachedState(timestamp);

        var text = CatalogFreshnessPresentation.BuildStatusText(state, now, Culture);

        Assert.Equal("Showing saved data from yesterday", text);
    }

    [Fact]
    public void BuildDegradedWarning_LiveRefreshFailure_UsesSameRelativeFormatter()
    {
        var now = LocalDateTime(2026, 9, 28, 20, 0);
        var timestamp = LocalDateTime(2026, 9, 26, 15, 14);
        var state = LiveState(timestamp) with { RefreshFailure = new InvalidOperationException("refresh failed") };

        var text = CatalogFreshnessPresentation.BuildDegradedWarning(state, now, Culture);

        Assert.Equal("Couldn't refresh. Showing data from Sep 26.", text);
    }

    private static DateTimeOffset LocalDateTime(int year, int month, int day, int hour, int minute)
        => new(new DateTime(year, month, day, hour, minute, 0, DateTimeKind.Local));

    private static CatalogDataState LiveState(DateTimeOffset timestamp)
        => new(
            HasUsableData: true,
            DataSource: CatalogDataSource.Live,
            IsInitialLoading: false,
            IsRefreshing: false,
            IsSuccessfulEmpty: false,
            LastSuccessfulRefreshUtc: timestamp,
            CachedAtUtc: null,
            RefreshFailure: null,
            LoadFailure: null,
            CacheWriteFailure: null);

    private static CatalogDataState CachedState(DateTimeOffset timestamp)
        => new(
            HasUsableData: true,
            DataSource: CatalogDataSource.Cached,
            IsInitialLoading: false,
            IsRefreshing: false,
            IsSuccessfulEmpty: false,
            LastSuccessfulRefreshUtc: null,
            CachedAtUtc: timestamp,
            RefreshFailure: null,
            LoadFailure: null,
            CacheWriteFailure: null);
}
