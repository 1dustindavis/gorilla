using System;
using System.Threading.Tasks;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Controls.Primitives;

namespace Gorilla.UI.App.Services;

/// <summary>
/// Resolves a virtualized ListView/GridView container from the framework's actual
/// realization signal rather than assuming layout finishes within a fixed number
/// of dispatcher turns.
/// </summary>
internal static class VirtualizedContainerRealizer
{
    public static async Task<SelectorItem?> RealizeAsync(
        ListViewBase list,
        object item,
        TimeSpan timeout
    )
    {
        if (list.ContainerFromItem(item) is SelectorItem existing)
        {
            return existing;
        }

        var realized = new TaskCompletionSource<SelectorItem?>();

        void OnContainerContentChanging(ListViewBase sender, ContainerContentChangingEventArgs args)
        {
            if (ReferenceEquals(args.Item, item) && args.ItemContainer is SelectorItem container)
            {
                realized.TrySetResult(container);
            }
        }

        list.ContainerContentChanging += OnContainerContentChanging;
        try
        {
            // Subscribe first so an immediately realized container cannot race the
            // observer. Re-check after ScrollIntoView for the same reason.
            list.ScrollIntoView(item);
            if (list.ContainerFromItem(item) is SelectorItem afterScroll)
            {
                return afterScroll;
            }

            var completed = await Task.WhenAny(realized.Task, Task.Delay(timeout));
            if (completed == realized.Task)
            {
                return await realized.Task;
            }

            // A final framework query handles a container that became available at
            // the timeout boundary without delivering another callback to this page.
            return list.ContainerFromItem(item) as SelectorItem;
        }
        finally
        {
            list.ContainerContentChanging -= OnContainerContentChanging;
        }
    }
}
