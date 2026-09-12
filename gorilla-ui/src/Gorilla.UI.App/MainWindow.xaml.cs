using System;
using System.ComponentModel;
using Gorilla.UI.App.Services;
using Gorilla.UI.App.Views;
using Gorilla.UI.Core.Models;
using Gorilla.UI.Core.ViewModels;
using Microsoft.UI.Xaml;

namespace Gorilla.UI.App
{
    public sealed partial class MainWindow : Window
    {
        private readonly AppCatalogSession _session;
        private readonly HomeViewModel _viewModel;

        public MainWindow()
        {
            InitializeComponent();
            _session = App.CurrentSession;
            _viewModel = _session.ViewModel;
            _viewModel.PropertyChanged += ViewModel_PropertyChanged;
            Closed += MainWindow_Closed;
            UpdateCatalogFreshnessPresentation();
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
            }
            catch
            {
                // CatalogDataState owns the user-facing failure state and retains
                // the underlying exception for the later troubleshooting surface.
            }
        }

        private void ViewModel_PropertyChanged(object? sender, PropertyChangedEventArgs e)
        {
            if (e.PropertyName == nameof(HomeViewModel.CatalogState))
            {
                UpdateCatalogFreshnessPresentation();
            }
        }

        private void UpdateCatalogFreshnessPresentation()
        {
            var state = _viewModel.CatalogState;
            RefreshButton.IsEnabled = !state.IsRefreshing;
            CatalogRefreshProgress.IsActive = state.IsRefreshing;
            CatalogRefreshProgress.Visibility = state.IsRefreshing
                ? Visibility.Visible
                : Visibility.Collapsed;

            CatalogFreshnessStatus.Text = BuildFreshnessText(state);

            var warning = BuildDegradedWarning(state);
            CatalogDegradedText.Text = warning;
            CatalogDegradedBanner.Visibility = string.IsNullOrWhiteSpace(warning)
                ? Visibility.Collapsed
                : Visibility.Visible;
        }

        private static string BuildFreshnessText(CatalogDataState state)
        {
            string text;
            if (state.IsInitialLoading && !state.HasUsableData)
            {
                text = "Loading App Catalog…";
            }
            else if (state.IsCached)
            {
                text = state.CachedAtUtc is DateTimeOffset cachedAt
                    ? $"Showing saved data from {FormatLocalTime(cachedAt)}"
                    : "Showing saved data";
            }
            else if (state.IsLive && state.LastSuccessfulRefreshUtc is DateTimeOffset refreshedAt)
            {
                text = $"Updated {FormatLocalTime(refreshedAt)}";
            }
            else if (state.HasLoadFailure)
            {
                text = "App Catalog unavailable";
            }
            else
            {
                text = "App Catalog";
            }

            if (state.IsRefreshing && state.HasUsableData)
            {
                text += " · Refreshing…";
            }

            return text;
        }

        private static string BuildDegradedWarning(CatalogDataState state)
        {
            if (state.HasRefreshFailure && state.IsCached)
            {
                return "Showing saved data. Gorilla couldn't refresh the catalog.";
            }

            if (state.HasRefreshFailure && state.IsLive)
            {
                return state.LastSuccessfulRefreshUtc is DateTimeOffset refreshedAt
                    ? $"Couldn't refresh. Showing data from {FormatLocalTime(refreshedAt)}."
                    : "Couldn't refresh. Showing previously loaded data.";
            }

            if (state.HasCacheWriteFailure)
            {
                return "Gorilla couldn't save the latest catalog for fallback use.";
            }

            return string.Empty;
        }

        private static string FormatLocalTime(DateTimeOffset timestamp)
            => timestamp.ToLocalTime().ToString("t");

        private void MainWindow_Closed(object sender, WindowEventArgs args)
        {
            _viewModel.PropertyChanged -= ViewModel_PropertyChanged;
            Closed -= MainWindow_Closed;
        }
    }
}
