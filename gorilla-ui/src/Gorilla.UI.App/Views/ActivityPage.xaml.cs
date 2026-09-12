using System;
using System.Collections.Specialized;
using System.ComponentModel;
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
        _isObserving = false;
    }

    private void ViewModel_PropertyChanged(object? sender, PropertyChangedEventArgs e)
    {
        if (e.PropertyName == nameof(HomeViewModel.IsActivityLoaded))
        {
            UpdateEmptyState();
        }
    }

    private void ActivityItems_CollectionChanged(object? sender, NotifyCollectionChangedEventArgs e)
    {
        UpdateEmptyState();
    }

    private void UpdateEmptyState()
    {
        ActivityEmpty.Visibility = ViewModel.IsActivityLoaded && ViewModel.ActivityItems.Count == 0
            ? Visibility.Visible
            : Visibility.Collapsed;
    }

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
