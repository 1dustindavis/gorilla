using System;

namespace Gorilla.UI.App.Services;

internal static class NavigationFocusState
{
    private static readonly object Gate = new();
    private static string? _catalogItemName;
    private static string? _activityOperationId;
    private static bool _catalogFallbackRequested;

    public static void RememberCatalogItem(string itemName)
    {
        lock (Gate)
        {
            _catalogItemName = itemName;
            _catalogFallbackRequested = false;
        }
    }

    public static string? ConsumeCatalogItem()
    {
        lock (Gate)
        {
            var itemName = _catalogItemName;
            _catalogItemName = null;
            return itemName;
        }
    }

    public static void RequestCatalogFallback()
    {
        lock (Gate)
        {
            _catalogItemName = null;
            _catalogFallbackRequested = true;
        }
    }

    public static bool ConsumeCatalogFallbackRequest()
    {
        lock (Gate)
        {
            var requested = _catalogFallbackRequested;
            _catalogFallbackRequested = false;
            return requested;
        }
    }

    public static void RememberActivityOperation(string operationId)
    {
        lock (Gate)
        {
            _activityOperationId = operationId;
        }
    }

    public static string? ConsumeActivityOperation()
    {
        lock (Gate)
        {
            var operationId = _activityOperationId;
            _activityOperationId = null;
            return operationId;
        }
    }
}
