using System;
using System.Collections.Specialized;
using System.ComponentModel;
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
    private bool _hasInitialized;

    public HomeViewModel ViewModel { get; }

    public HomePage(HomeViewModel viewModel)
    {
        this.InitializeComponent();
        ViewModel = viewModel;
        DataContext = ViewModel;
        Loaded += HomePage_Loaded;
        Unloaded += HomePage_Unloaded;
        ViewModel.PropertyChanged += ViewModel_PropertyChanged;
        ViewModel.Items.CollectionChanged += Items_CollectionChanged;
        _serviceWarningTextChangedToken = ServiceWarning.RegisterPropertyChangedCallback(
            TextBlock.TextProperty,
            ServiceWarning_TextChanged
        );
        UpdateEmptyStates();
    }

    private async void HomePage_Loaded(object sender, RoutedEventArgs e)
    {
        _cts?.Dispose();
        _cts = new CancellationTokenSource();
        await RunSafelyAsync(() => ViewModel.InitializeAsync(_cts.Token));
        _hasInitialized = true;
        UpdateEmptyStates();
        UpdateCardWidths(CatalogItems.ActualWidth);
    }

    private void HomePage_Unloaded(object sender, RoutedEventArgs e)
    {
        ResetCancellation();
    }

    private void ViewModel_PropertyChanged(object? sender, PropertyChangedEventArgs e)
    {
        if (e.PropertyName == nameof(HomeViewModel.SearchQuery))
        {
            UpdateEmptyStates();
        }
    }

    private void Items_CollectionChanged(object? sender, NotifyCollectionChangedEventArgs e)
    {
        UpdateEmptyStates();
    }

    private void UpdateEmptyStates()
    {
        if (!_hasInitialized)
        {
            SearchNoResults.Visibility = Visibility.Collapsed;
            CatalogEmpty.Visibility = Visibility.Collapsed;
            return;
        }

        var query = ViewModel.SearchQuery;
        var noVisibleItems = ViewModel.Items.Count == 0;
        var hasSearch = !string.IsNullOrWhiteSpace(query);

        SearchNoResults.Visibility = hasSearch && noVisibleItems
            ? Visibility.Visible
            : Visibility.Collapsed;
        SearchNoResults.Text = hasSearch ? $"No apps match \"{query}\"." : string.Empty;

        CatalogEmpty.Visibility = !hasSearch && noVisibleItems
            ? Visibility.Visible
            : Visibility.Collapsed;
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

    private void CatalogItems_ContainerContentChanging(
        ListViewBase sender,
        ContainerContentChangingEventArgs args
    )
    {
        if (!ReferenceEquals(sender, CatalogItems))
        {
            return;
        }
        if (args.ItemContainer is null || args.Item is not UiOptionalInstallItem item)
        {
            return;
        }

        AutomationProperties.SetAutomationId(args.ItemContainer, item.ItemName);
        AutomationProperties.SetName(args.ItemContainer, item.DisplayName);
    }

    private void CatalogItems_SizeChanged(object sender, SizeChangedEventArgs e)
    {
        UpdateCardWidths(e.NewSize.Width);
    }

    private void UpdateCardWidths(double availableWidth)
    {
        if (availableWidth <= 0 || CatalogItems.ItemsPanelRoot is not ItemsWrapGrid panel)
        {
            return;
        }

        const double minimumCardWidth = 300;
        const double maximumCardWidth = 340;
        var columns = Math.Max(1, (int)Math.Floor(availableWidth / minimumCardWidth));
        panel.ItemWidth = Math.Max(1, Math.Min(maximumCardWidth, availableWidth / columns));
    }

    private async void ActionButton_Click(object sender, RoutedEventArgs e)
    {
        if (sender is not Button button || button.DataContext is not UiOptionalInstallItem item)
        {
            return;
        }
        if (_cts is null)
        {
            return;
        }

        var action = button.Tag switch
        {
            CatalogCardActionKind kind => kind,
            string text when Enum.TryParse<CatalogCardActionKind>(text, out var parsed) => parsed,
            _ => (CatalogCardActionKind?)null,
        };
        if (action is null)
        {
            return;
        }

        var isPrimary = string.Equals(
            AutomationProperties.GetAutomationId(button),
            "PrimaryActionButton",
            StringComparison.Ordinal
        );
        var currentAction = isPrimary
            ? item.CardPresentation.PrimaryAction
            : item.CardPresentation.SecondaryAction;
        if (currentAction is null || !currentAction.Enabled || currentAction.Kind != action)
        {
            return;
        }

        switch (action.Value)
        {
            case CatalogCardActionKind.Install:
                await RunSafelyAsync(() => ViewModel.InstallAsync(item, _cts.Token));
                break;
            case CatalogCardActionKind.Remove:
                await RunSafelyAsync(() => ViewModel.RemoveAsync(item, _cts.Token));
                break;
        }
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
        ViewModel.PropertyChanged -= ViewModel_PropertyChanged;
        ViewModel.Items.CollectionChanged -= Items_CollectionChanged;
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
