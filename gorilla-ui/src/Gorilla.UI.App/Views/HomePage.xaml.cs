using System;
using System.Threading;
using System.Threading.Tasks;
using Gorilla.UI.Core.Models;
using Gorilla.UI.Core.ViewModels;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Automation;
using Microsoft.UI.Xaml.Automation.Peers;
using Microsoft.UI.Xaml.Controls;

namespace Gorilla.UI.App.Views;

public sealed partial class HomePage : Page, IDisposable
{
    private CancellationTokenSource? _cts;
    private long _serviceWarningTextChangedToken;

    public HomeViewModel ViewModel { get; }

    public HomePage(HomeViewModel viewModel)
    {
        this.InitializeComponent();
        ViewModel = viewModel;
        DataContext = ViewModel;
        Loaded += HomePage_Loaded;
        Unloaded += HomePage_Unloaded;
        _serviceWarningTextChangedToken = ServiceWarning.RegisterPropertyChangedCallback(
            TextBlock.TextProperty,
            ServiceWarning_TextChanged
        );
    }

    private async void HomePage_Loaded(object sender, RoutedEventArgs e)
    {
        _cts?.Dispose();
        _cts = new CancellationTokenSource();
        await RunSafelyAsync(() => ViewModel.InitializeAsync(_cts.Token));
    }

    private void HomePage_Unloaded(object sender, RoutedEventArgs e)
    {
        ResetCancellation();
    }

    private void ServiceWarning_TextChanged(DependencyObject sender, DependencyProperty dp)
    {
        if (string.IsNullOrWhiteSpace(ServiceWarning.Text))
        {
            return;
        }

        var peer = FrameworkElementAutomationPeer.FromElement(ServiceWarning)
            ?? FrameworkElementAutomationPeer.CreatePeerForElement(ServiceWarning);
        peer?.RaiseAutomationEvent(AutomationEvents.LiveRegionChanged);
    }

    private void ItemsList_ContainerContentChanging(
        ListViewBase sender,
        ContainerContentChangingEventArgs args
    )
    {
        if (args.ItemContainer is null || args.Item is not UiOptionalInstallItem item)
        {
            return;
        }

        AutomationProperties.SetAutomationId(args.ItemContainer, item.ItemName);
        AutomationProperties.SetName(args.ItemContainer, item.DisplayName);
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

    public void Dispose()
    {
        Loaded -= HomePage_Loaded;
        Unloaded -= HomePage_Unloaded;
        if (_serviceWarningTextChangedToken != 0)
        {
            ServiceWarning.UnregisterPropertyChangedCallback(
                TextBlock.TextProperty,
                _serviceWarningTextChangedToken
            );
            _serviceWarningTextChangedToken = 0;
        }
        ResetCancellation();
    }

    private void ResetCancellation()
    {
        _cts?.Cancel();
        _cts?.Dispose();
        _cts = null;
    }
}
