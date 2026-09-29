using System;
using System.ComponentModel;
using System.Globalization;
using Gorilla.UI.App.Services;
using Gorilla.UI.App.Views;
using Gorilla.UI.Core.Models;
using Gorilla.UI.Core.ViewModels;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Navigation;

namespace Gorilla.UI.App
{
    public sealed partial class MainWindow : Window
    {
        private enum ShellHeaderMode
        {
            None,
            Home,
            Details,
            Activity,
        }

        private readonly AppCatalogSession _session;
        private readonly HomeViewModel _viewModel;

        public MainWindow()
        {
            InitializeComponent();
            _session = App.CurrentSession;
            _viewModel = _session.ViewModel;
            _viewModel.PropertyChanged += ViewModel_PropertyChanged;
            RootFrame.Navigated += RootFrame_Navigated;
            Activated += MainWindow_Activated;
            Closed += MainWindow_Closed;
            UpdateCatalogFreshnessPresentation();
            UpdateInfrastructureWarningPresentation();
            RootFrame.Navigate(typeof(HomePage));
        }

        private async void RefreshButton_Click(object sender, RoutedEventArgs e)
        {
            try
            {
                await _viewModel.RefreshCatalogAsync(_session.LifetimeToken);
            }
            catch (OperationCanceledException) when (_session.LifetimeToken.IsCancellationRequested)
            {
                return;
            }
            catch
            {
                // CatalogDataState owns the user-facing failure state and retains
                // the underlying exception for the troubleshooting disclosure.
            }

            if (!_viewModel.IsActivityLoaded)
            {
                try
                {
                    await _viewModel.RetryActivityLoadAsync(_session.LifetimeToken);
                }
                catch (OperationCanceledException) when (_session.LifetimeToken.IsCancellationRequested)
                {
                }
            }
        }

        private void ActivityNavigationButton_Click(object sender, RoutedEventArgs e)
        {
            RootFrame.Navigate(typeof(ActivityPage));
        }

        private void DetailsBackButton_Click(object sender, RoutedEventArgs e)
        {
            NavigateBackOrCatalog();
        }

        private void ActivityBackButton_Click(object sender, RoutedEventArgs e)
        {
            NavigateBackOrCatalog();
        }

        private void CatalogNavigationButton_Click(object sender, RoutedEventArgs e)
        {
            NavigationFocusState.RequestCatalogFallback();
            RootFrame.Navigate(typeof(HomePage));
        }

        private void NavigateBackOrCatalog()
        {
            if (RootFrame.CanGoBack)
            {
                RootFrame.GoBack();
            }
            else
            {
                NavigationFocusState.RequestCatalogFallback();
                RootFrame.Navigate(typeof(HomePage));
            }
        }

        private void RootFrame_Navigated(object sender, NavigationEventArgs e)
        {
            var mode = e.SourcePageType == typeof(HomePage)
                ? ShellHeaderMode.Home
                : e.SourcePageType == typeof(AppDetailsPage)
                    ? ShellHeaderMode.Details
                    : e.SourcePageType == typeof(ActivityPage)
                        ? ShellHeaderMode.Activity
                        : ShellHeaderMode.None;

            SetShellHeaderMode(mode);

            if (mode == ShellHeaderMode.Details)
            {
                DispatcherQueue.TryEnqueue(() => DetailsBackButton.Focus(FocusState.Programmatic));
            }
            else if (mode == ShellHeaderMode.Activity)
            {
                DispatcherQueue.TryEnqueue(() => ActivityBackButton.Focus(FocusState.Programmatic));
            }
        }

        private void SetShellHeaderMode(ShellHeaderMode mode)
        {
            HomeHeaderContext.Visibility = mode == ShellHeaderMode.Home
                ? Visibility.Visible
                : Visibility.Collapsed;
            DetailsHeaderContext.Visibility = mode == ShellHeaderMode.Details
                ? Visibility.Visible
                : Visibility.Collapsed;
            ActivityHeaderContext.Visibility = mode == ShellHeaderMode.Activity
                ? Visibility.Visible
                : Visibility.Collapsed;
        }

        private void ViewModel_PropertyChanged(object? sender, PropertyChangedEventArgs e)
        {
            if (e.PropertyName == nameof(HomeViewModel.CatalogState))
            {
                UpdateCatalogFreshnessPresentation();
            }
            else if (e.PropertyName == nameof(HomeViewModel.InfrastructureWarning))
            {
                UpdateInfrastructureWarningPresentation();
            }
        }

        private void MainWindow_Activated(object sender, WindowActivatedEventArgs args)
        {
            if (args.WindowActivationState != WindowActivationState.Deactivated)
            {
                UpdateCatalogFreshnessPresentation();
            }
        }

        private void UpdateCatalogFreshnessPresentation()
        {
            var state = _viewModel.CatalogState;
            var now = DateTimeOffset.Now;
            var culture = CultureInfo.CurrentCulture;

            RefreshButton.IsEnabled = !state.IsRefreshing;
            CatalogRefreshProgress.IsActive = state.IsRefreshing;
            CatalogRefreshProgress.Visibility = state.IsRefreshing
                ? Visibility.Visible
                : Visibility.Collapsed;

            CatalogFreshnessStatus.Text = CatalogFreshnessPresentation.BuildStatusText(state, now, culture);
            ToolTipService.SetToolTip(
                CatalogFreshnessStatus,
                CatalogFreshnessPresentation.BuildToolTip(state, culture));

            var warning = CatalogFreshnessPresentation.BuildDegradedWarning(state, now, culture);
            CatalogDegradedText.Text = warning;
            CatalogDegradedBanner.Visibility = string.IsNullOrWhiteSpace(warning)
                ? Visibility.Collapsed
                : Visibility.Visible;

            var technicalDetails = CatalogTroubleshootingPresentation.BuildTechnicalDetails(state);
            CatalogTechnicalDetailsText.Text = technicalDetails;
            CatalogTechnicalDetails.Visibility = string.IsNullOrWhiteSpace(technicalDetails)
                ? Visibility.Collapsed
                : Visibility.Visible;
        }

        private void UpdateInfrastructureWarningPresentation()
        {
            var warning = _viewModel.InfrastructureWarning;
            InfrastructureWarningText.Text = warning.Message;
            InfrastructureWarningBanner.Visibility = string.IsNullOrWhiteSpace(warning.Message)
                ? Visibility.Collapsed
                : Visibility.Visible;

            InfrastructureTechnicalDetailsText.Text = warning.TechnicalDetails;
            InfrastructureTechnicalDetails.Visibility = warning.HasTechnicalDetails
                ? Visibility.Visible
                : Visibility.Collapsed;
        }

        private void MainWindow_Closed(object sender, WindowEventArgs args)
        {
            _viewModel.PropertyChanged -= ViewModel_PropertyChanged;
            RootFrame.Navigated -= RootFrame_Navigated;
            Activated -= MainWindow_Activated;
            Closed -= MainWindow_Closed;
        }
    }
}
