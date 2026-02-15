using Gorilla.UI.App.ViewModels;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;

namespace Gorilla.UI.App.Views;

public sealed partial class HomePage : Page
{
    private CancellationTokenSource? _cts;

    public HomeViewModel ViewModel { get; }

    public HomePage(HomeViewModel viewModel)
    {
        this.InitializeComponent();
        ViewModel = viewModel;
        DataContext = ViewModel;
        Loaded += HomePage_Loaded;
        Unloaded += HomePage_Unloaded;
    }

    private async void HomePage_Loaded(object sender, RoutedEventArgs e)
    {
        _cts?.Dispose();
        _cts = new CancellationTokenSource();
        await RunSafelyAsync(() => ViewModel.InitializeAsync(_cts.Token));
    }

    private void HomePage_Unloaded(object sender, RoutedEventArgs e)
    {
        _cts?.Cancel();
        _cts?.Dispose();
        _cts = null;
    }

    private async void InstallButton_Click(object sender, RoutedEventArgs e)
    {
        if (sender is not Button button || button.Tag is not string itemName)
        {
            return;
        }

        var item = ViewModel.FindItem(itemName);
        if (item is null)
        {
            return;
        }

        if (_cts is null)
        {
            return;
        }

        await RunSafelyAsync(() => ViewModel.InstallAsync(item, _cts.Token));
    }

    private async void RemoveButton_Click(object sender, RoutedEventArgs e)
    {
        if (sender is not Button button || button.Tag is not string itemName)
        {
            return;
        }

        var item = ViewModel.FindItem(itemName);
        if (item is null)
        {
            return;
        }

        if (_cts is null)
        {
            return;
        }

        await RunSafelyAsync(() => ViewModel.RemoveAsync(item, _cts.Token));
    }

    private async Task RunSafelyAsync(Func<Task> action)
    {
        try
        {
            await action();
        }
        catch (OperationCanceledException) when (_cts?.IsCancellationRequested == true)
        {
            // Ignore cancellation caused by page unload.
        }
        catch (Exception ex)
        {
            ViewModel.SetWarningBanner($"Operation failed: {ex.Message}");
        }
    }
}
