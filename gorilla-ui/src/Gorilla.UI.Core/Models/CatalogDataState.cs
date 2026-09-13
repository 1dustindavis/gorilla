namespace Gorilla.UI.Core.Models;

public enum CatalogDataSource
{
    None,
    Live,
    Cached,
}

public sealed record CatalogDataState(
    bool HasUsableData,
    CatalogDataSource DataSource,
    bool IsInitialLoading,
    bool IsRefreshing,
    bool IsSuccessfulEmpty,
    DateTimeOffset? LastSuccessfulRefreshUtc,
    DateTimeOffset? CachedAtUtc,
    Exception? RefreshFailure,
    Exception? LoadFailure,
    Exception? CacheWriteFailure
)
{
    public static CatalogDataState InitialLoading { get; } = new(
        HasUsableData: false,
        DataSource: CatalogDataSource.None,
        IsInitialLoading: true,
        IsRefreshing: true,
        IsSuccessfulEmpty: false,
        LastSuccessfulRefreshUtc: null,
        CachedAtUtc: null,
        RefreshFailure: null,
        LoadFailure: null,
        CacheWriteFailure: null
    );

    public bool IsLive => HasUsableData && DataSource == CatalogDataSource.Live;

    public bool IsCached => HasUsableData && DataSource == CatalogDataSource.Cached;

    public bool HasRefreshFailure => HasUsableData && RefreshFailure is not null;

    public bool HasLoadFailure => !HasUsableData && LoadFailure is not null;

    public bool HasCacheWriteFailure => CacheWriteFailure is not null;
}
