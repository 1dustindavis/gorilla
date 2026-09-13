using System;
using System.Collections.Specialized;
using System.ComponentModel;
using System.Diagnostics.CodeAnalysis;
using System.Threading.Tasks;
using Gorilla.UI.App.Services;
using Gorilla.UI.Core.Models;
using Gorilla.UI.Core.ViewModels;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Automation;
using Microsoft.UI.Xaml.Controls;

namespace Gorilla.UI.App.Views;

public sealed partial class ActivityPage : Page
{
    private readonly AppCatalogSession _session;
    private bool _isObserving;
    private bool _isRefreshingRecovery;

    public HomeViewModel ViewModel { get; }

    public ActivityPage()
    {
        InitializeComponent();
        _session = App.CurrentSession;
        ViewModel = _session.ViewModel;
        DataContext = ViewModel;
        Loaded += ActivityPage_Loaded;
        Unloaded += ActivityPage_Unloaded;
        UpdateEmptyState();
    }

    private async void ActivityPage_Loaded(object sender, RoutedEventArgs e)
    {
        StartObserving();
        try
        {
            await _session.EnsureInitializedAsync();
            RefreshRecoverySafely();
        }
        catch (OperationCanceledException) when (_session.LifetimeToken.IsCancellationRequested)
        {
        }
        catch (Exception ex)
        {
            ViewModel.SetWarningBanner($"Operation failed: {ex.Message}");
        }
        UpdateEmptyState();
    }

    private void ActivityPage_Unloaded(object sender, RoutedEventArgs e)
    {
        StopObserving();
    }

    private void StartObserving()
    {
        if (_isObserving)
        {
            return;
        }

        ViewModel.PropertyChanged += ViewModel_PropertyChanged;
        ViewModel.ActivityItems.CollectionChanged += ActivityItems_CollectionChanged;
        foreach (var item in ViewModel.ActivityItems)
        {
            item.PropertyChanged += ActivityItem_PropertyChanged;
        }
        _isObserving = true;
    }

    private void StopObserving()
    {
        if (!_isObserving)
        {
            return;
        }

        ViewModel.PropertyChanged -= ViewModel_PropertyChanged;
        ViewModel.ActivityItems.CollectionChanged -= ActivityItems_CollectionChanged;
        foreach (var item in ViewModel.ActivityItems)
        {
            item.PropertyChanged -= ActivityItem_PropertyChanged;
        }
        _isObserving = false;
    }

    private void ViewModel_PropertyChanged(object? sender, PropertyChangedEventArgs e)
    {
        if (e.PropertyName == nameof(HomeViewModel.IsActivityLoaded))
        {
            UpdateEmptyState();
        }
        if (e.PropertyName == nameof(HomeViewModel.CatalogState))
        {
            RefreshRecoverySafely();
        }
    }

    private void ActivityItems_CollectionChanged(object? sender, NotifyCollectionChangedEventArgs e)
    {
        if (e.OldItems is not null)
        {
            foreach (ActivityOperationPresentation item in e.OldItems)
            {
                item.PropertyChanged -= ActivityItem_PropertyChanged;
            }
        }
        if (e.NewItems is not null)
        {
            foreach (ActivityOperationPresentation item in e.NewItems)
            {
                item.PropertyChanged += ActivityItem_PropertyChanged;
            }
        }

        RefreshRecoverySafely();
        UpdateEmptyState();
    }

    private void ActivityItem_PropertyChanged(object? sender, PropertyChangedEventArgs e)
    {
        if (e.PropertyName is nameof(ActivityOperationPresentation.State)
            or nameof(ActivityOperationPresentation.Result)
            or nameof(ActivityOperationPresentation.ItemName))
        {
            RefreshRecoverySafely();
        }
    }

    private void RefreshRecoverySafely()
    {
        if (_isRefreshingRecovery)
        {
            return;
        }

        try
        {
            _isRefreshingRecovery = true;
            ViewModel.RefreshActivityRecoveryPresentations();
        }
        finally
        {
            _isRefreshingRecovery = false;
        }
    }

    private void UpdateEmptyState()
    {
        ActivityEmpty.Visibility = ViewModel.IsActivityLoaded && ViewModel.ActivityItems.Count == 0
            ? Visibility.Visible
            : Visibility.Collapsed;
    }

    [SuppressMessage(
        "Performance",
        "CA1822:Mark members as static",
        Justification = "WinUI XAML event handlers are wired through the page instance."
    )]
    private void ActivityItems_ContainerContentChanging(
        ListViewBase sender,
        ContainerContentChangingEventArgs args
    )
    {
        if (args.ItemContainer is null || args.Item is not ActivityOperationPresentation item)
        {
            return;
        }

        AutomationProperties.SetAutomationId(args.ItemContainer, item.EntryAutomationId);
        AutomationProperties.SetName(args.ItemContainer, $"{item.DisplayName}, {item.ActionLabel}, {item.StateText}");
        AutomationProperties.SetHelpText(args.ItemContainer, item.OperationId);
    }

    private async void RetryButton_Click(object sender, RoutedEventArgs e)
    {
        if (sender is not Button button || button.DataContext is not ActivityOperationPresentation item || !item.CanRetry)
        {
            return;
        }

        button.IsEnabled = false;
        try
        {
            await ViewModel.RetryAsync(item.OperationId, _session.LifetimeToken);
        }
        catch (OperationCanceledException) when (_session.LifetimeToken.IsCancellationRequested)
        {
        }
        catch (Exception ex)
        {
            ViewModel.SetWarningBanner($"Retry could not be started: {ex.Message}");
        }
        finally
        {
            RefreshRecoverySafely();
            button.IsEnabled = item.CanRetry;
        }
    }

    private void ActivityItems_ItemClick(object sender, ItemClickEventArgs e)
    {
        if (e.ClickedItem is not ActivityOperationPresentation item || !item.CanNavigate)
        {
            return;
        }

        if (!ViewModel.SelectItem(item.ItemName))
        {
            return;
        }

        Frame.Navigate(typeof(AppDetailsPage), item.ItemName);
    }

    private void BackButton_Click(object sender, RoutedEventArgs e)
    {
        if (Frame.CanGoBack)
        {
            Frame.GoBack();
        }
        else
        {
            Frame.Navigate(typeof(HomePage));
        }
    }

    private void CatalogButton_Click(object sender, RoutedEventArgs e)
    {
        Frame.Navigate(typeof(HomePage));
    }
}
