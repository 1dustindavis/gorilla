using System;
using System.ComponentModel;
using System.Threading.Tasks;
using Gorilla.UI.App.Services;
using Gorilla.UI.Core.Models;
using Gorilla.UI.Core.ViewModels;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Automation;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Navigation;

namespace Gorilla.UI.App.Views;

public sealed partial class AppDetailsPage : Page
{
    private readonly AppCatalogSession _session;
    private readonly HomeViewModel _viewModel;
    private string? _itemName;

    public AppDetailsPage()
    {
        this.InitializeComponent();
        _session = App.CurrentSession;
        _viewModel = _session.ViewModel;
    }

    protected override async void OnNavigatedTo(NavigationEventArgs e)
    {
        base.OnNavigatedTo(e);
        _itemName = e.Parameter as string;
        _viewModel.PropertyChanged += ViewModel_PropertyChanged;

        await RunSafelyAsync(_session.EnsureInitializedAsync);
        if (!string.IsNullOrWhiteSpace(_itemName))
        {
            _viewModel.SelectItem(_itemName);
        }
        ResolveCanonicalItem();
    }

    protected override void OnNavigatedFrom(NavigationEventArgs e)
    {
        _viewModel.PropertyChanged -= ViewModel_PropertyChanged;
        base.OnNavigatedFrom(e);
    }

    private void ViewModel_PropertyChanged(object? sender, PropertyChangedEventArgs e)
    {
        if (e.PropertyName == nameof(HomeViewModel.SelectedItem) ||
            e.PropertyName == nameof(HomeViewModel.SelectedItemName))
        {
            ResolveCanonicalItem();
        }
    }

    private void ResolveCanonicalItem()
    {
        var selected = _viewModel.SelectedItem;
        var item = !string.IsNullOrWhiteSpace(_itemName) &&
            selected is not null &&
            string.Equals(selected.ItemName, _itemName, StringComparison.OrdinalIgnoreCase)
                ? selected
                : null;

        DataContext = item;
        DetailsContent.Visibility = item is null ? Visibility.Collapsed : Visibility.Visible;
        UnavailableContent.Visibility = item is null ? Visibility.Visible : Visibility.Collapsed;
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

    private async void ActionButton_Click(object sender, RoutedEventArgs e)
    {
        if (sender is not Button button || DataContext is not UiOptionalInstallItem item)
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
            "DetailsPrimaryAction",
            StringComparison.Ordinal
        );
        var presentation = item.DetailsPresentation;
        var currentAction = isPrimary ? presentation.PrimaryAction : presentation.SecondaryAction;
        if (currentAction is null || !currentAction.Enabled || currentAction.Kind != action)
        {
            return;
        }

        switch (action.Value)
        {
            case CatalogCardActionKind.Install:
                await RunSafelyAsync(() => _viewModel.InstallAsync(item, _session.LifetimeToken));
                break;
            case CatalogCardActionKind.Remove:
                await RunSafelyAsync(() => _viewModel.RemoveAsync(item, _session.LifetimeToken));
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
            // Application shutdown cancels UI tracking only; the service owns the operation.
        }
        catch (Exception ex)
        {
            _viewModel.SetWarningBanner($"Operation failed: {ex.Message}");
        }
    }
}
