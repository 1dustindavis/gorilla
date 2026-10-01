using System;
using System.Globalization;
using Gorilla.UI.Core.Models;

namespace Gorilla.UI.App.Services;

public static class CatalogFreshnessPresentation
{
    public static string BuildStatusText(
        CatalogDataState state,
        DateTimeOffset now,
        CultureInfo culture)
    {
        string text;
        if (state.IsInitialLoading && !state.HasUsableData)
        {
            text = "Loading App Catalog…";
        }
        else if (state.IsCached)
        {
            text = state.LastSuccessfulRefreshUtc is DateTimeOffset sourceGeneratedAt
                ? $"Showing saved data from {FormatTimestampAge(sourceGeneratedAt, now, culture)}"
                : "Showing saved data";
        }
        else if (state.IsLive && state.LastSuccessfulRefreshUtc is DateTimeOffset refreshedAt)
        {
            text = $"Updated {FormatTimestampAge(refreshedAt, now, culture)}";
        }
        else if (state.HasLoadFailure)
        {
            text = "App Catalog unavailable";
        }
        else
        {
            text = "App Catalog";
        }

        if (state.IsRefreshing && state.HasUsableData)
        {
            text += " · Refreshing…";
        }

        return text;
    }

    public static string BuildDegradedWarning(
        CatalogDataState state,
        DateTimeOffset now,
        CultureInfo culture)
    {
        if (state.HasRefreshFailure && state.IsCached)
        {
            return "Showing saved data. Gorilla couldn't refresh the catalog.";
        }

        if (state.HasRefreshFailure && state.IsLive)
        {
            return state.LastSuccessfulRefreshUtc is DateTimeOffset refreshedAt
                ? $"Couldn't refresh. Showing data from {FormatTimestampAge(refreshedAt, now, culture)}."
                : "Couldn't refresh. Showing previously loaded data.";
        }

        if (state.HasCacheWriteFailure)
        {
            return "Gorilla couldn't save the latest catalog for fallback use.";
        }

        if (state.HasLoadFailure)
        {
            return state.HasNoUsableCache
                ? "Gorilla couldn't load the App Catalog, and no saved catalog is available."
                : "Gorilla couldn't load the App Catalog.";
        }

        return string.Empty;
    }

    public static string? BuildToolTip(CatalogDataState state, CultureInfo culture)
    {
        if (state.IsCached && state.LastSuccessfulRefreshUtc is DateTimeOffset sourceGeneratedAt)
        {
            return $"Catalog data from {FormatExactLocalTimestamp(sourceGeneratedAt, culture)}";
        }

        if (state.IsLive && state.LastSuccessfulRefreshUtc is DateTimeOffset refreshedAt)
        {
            return $"Last updated {FormatExactLocalTimestamp(refreshedAt, culture)}";
        }

        return null;
    }

    public static string FormatTimestampAge(
        DateTimeOffset timestamp,
        DateTimeOffset now,
        CultureInfo culture)
    {
        var localTimestamp = timestamp.ToLocalTime();
        var localNow = now.ToLocalTime();

        if (localTimestamp.Date == localNow.Date)
        {
            return localTimestamp.ToString("t", culture);
        }

        if (localTimestamp.Date == localNow.Date.AddDays(-1))
        {
            return "yesterday";
        }

        return localTimestamp.ToString("MMM d", culture);
    }

    private static string FormatExactLocalTimestamp(DateTimeOffset timestamp, CultureInfo culture)
        => timestamp.ToLocalTime().ToString("MMM d, yyyy 'at' t", culture);
}
