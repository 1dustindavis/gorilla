using System;
using System.Collections.Specialized;
using System.ComponentModel;
using System.Threading.Tasks;
using Gorilla.UI.App.Services;
using Gorilla.UI.Core.Models;
using Gorilla.UI.Core.ViewModels;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Automation;
using Microsoft.UI.Xaml.Automation.Peers;
using Microsoft.UI.Xaml.Controls;

namespace Gorilla.UI.App.Views;

public sealed partial class HomePage : Page
{
    private readonly AppCatalogSession _session;
    private long _serviceWarningTextChangedToken;
    private bool _isObservingPageState;

    public HomeViewModel ViewModel { get; }

    public HomePage()
    {
        this.InitializeComponent();
        _session = App.CurrentSession;
        ViewModel = _session.ViewModel;
        DataContext = ViewModel;
        Loaded += HomePage_Loaded;
        Unloaded += HomePage_Unloaded;
        UpdateEmptyStates();
    }

    private async void HomePage_Loaded(object sender, RoutedEventArgs e)
    {
        StartObservingPageState();
        await RunSafelyAsync(_session.EnsureInitializedAsync);
        UpdateEmptyStates();
        UpdateCardWidths(CatalogItems.ActualWidth);
    }

    private void HomePage_Unloaded(object sender, RoutedEventArgs e)
    {
        StopObservingPageState();
    }

    private void StartObservingPageState()
    {
        if (_isObservingPageState)
        {
            return;
        }

        ViewModel.PropertyChanged += ViewModel_PropertyChanged;
        ViewModel.Items.CollectionChanged += Items_CollectionChanged;
        _serviceWarningTextChangedToken = ServiceWarning.RegisterPropertyChangedCallback(
            TextBlock.TextProperty,
            ServiceWarning_TextChanged
        );
        _isObservingPageState = true;
    }

    private void StopObservingPageState()
    {
        if (!_isObservingPageState)
        {
            return;
        }

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
        _isObservingPageState = false;
    }

    private void ViewModel_PropertyChanged(object? sender, PropertyChangedEventArgs e)
    {
        if (e.PropertyName == nameof(HomeViewModel.SearchQuery) ||
            e.PropertyName == nameof(HomeViewModel.CatalogState))
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
        var state = ViewModel.CatalogState;
        var initialLoading = state.IsInitialLoading && !state.HasUsableData;
        var loadFailed = state.HasLoadFailure;
        var successfulEmpty = state.IsSuccessfulEmpty;
        var query = ViewModel.SearchQuery;
        var noVisibleItems = ViewModel.Items.Count == 0;
        var hasSearch = !string.IsNullOrWhiteSpace(query);

        InitialLoadingState.Visibility = initialLoading
            ? Visibility.Visible
            : Visibility.Collapsed;
        LoadFailedState.Visibility = loadFailed
            ? Visibility.Visible
            : Visibility.Collapsed;

        SearchNoResults.Visibility = !initialLoading &&
            !loadFailed &&
            state.HasUsableData &&
            !successfulEmpty &&
            hasSearch &&
            noVisibleItems
                ? Visibility.Visible
                : Visibility.Collapsed;
        SearchNoResults.Text = hasSearch ? $"No apps match \"{query}\"." : string.Empty;

        CatalogEmpty.Visibility = successfulEmpty
            ? Visibility.Visible
            : Visibility.Collapsed;

        CatalogItems.Visibility = state.HasUsableData && !successfulEmpty
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

    private void CatalogItems_ItemClick(object sender, ItemClickEventArgs e)
    {
        if (e.ClickedItem is not UiOptionalInstallItem item)
        {
            return;
        }

        if (!ViewModel.SelectItem(item.ItemName))
        {
            return;
        }

        Frame.Navigate(typeof(AppDetailsPage), item.ItemName);
    }

    private void ActivityButton_Click(object sender, RoutedEventArgs e)
    {
        Frame.Navigate(typeof(ActivityPage));
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
                await RunSafelyAsync(() => ViewModel.InstallAsync(item, _session.LifetimeToken));
                break;
            case CatalogCardActionKind.Remove:
                await RunSafelyAsync(() => ViewModel.RemoveAsync(item, _session.LifetimeToken));
                break;
        }
    }

    private async Task RunSafelyAsync(Func<Task> action)
    {
        try
        {
            await action();
        }
        catch (OperationCanceledException) when (_session.LifetimeToken.IsCancellationRequested)
        {
            // Closing the UI cancels only its wait/tracking work. The service operation
            // remains service-owned and is not presented as canceled by navigation.
        }
        catch (Exception ex)
        {
            ViewModel.SetWarningBanner($"Operation failed: {ex.Message}");
        }
    }
}
