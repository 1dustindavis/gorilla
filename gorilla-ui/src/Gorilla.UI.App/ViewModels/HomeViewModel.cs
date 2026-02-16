using System;
using System.Collections.Generic;
using System.Collections.ObjectModel;
using System.ComponentModel;
using System.Linq;
using System.Runtime.CompilerServices;
using System.Threading;
using System.Threading.Tasks;
using Gorilla.UI.App.Models;
using Gorilla.UI.App.Services;
using Gorilla.UI.Client;

namespace Gorilla.UI.App.ViewModels;

public sealed class HomeViewModel : INotifyPropertyChanged
{
    private readonly IGorillaServiceClient _client;
    private readonly OptionalInstallsCacheCoordinator _cacheCoordinator;
    private readonly OperationTracker _operationTracker;

    private string _warningBanner = string.Empty;

    public HomeViewModel(
        IGorillaServiceClient client,
        OptionalInstallsCacheCoordinator cacheCoordinator,
        OperationTracker operationTracker
    )
    {
        _client = client;
        _cacheCoordinator = cacheCoordinator;
        _operationTracker = operationTracker;
    }

    public ObservableCollection<UiOptionalInstallItem> Items { get; } = [];

    public string WarningBanner
    {
        get => _warningBanner;
        private set
        {
            _warningBanner = value;
            OnPropertyChanged();
        }
    }

    public event PropertyChangedEventHandler? PropertyChanged;

    public async Task InitializeAsync(CancellationToken cancellationToken)
    {
        var cached = await _cacheCoordinator.LoadCachedAsync(cancellationToken);
        if (cached is not null)
        {
            ApplyItems(cached.Items);
        }

        try
        {
            var refreshed = await _cacheCoordinator.RefreshAsync(cancellationToken);
            ApplyItems(refreshed.Items);
            WarningBanner = string.Empty;
        }
        catch (Exception ex)
        {
            WarningBanner = $"Showing cached data. Refresh failed: {ex.Message}";
        }
    }

    public async Task InstallAsync(UiOptionalInstallItem item, CancellationToken cancellationToken)
    {
        item.IsBusy = true;
        try
        {
            var accepted = await _client.InstallItemAsync(item.ItemName, cancellationToken);
            if (!accepted.Accepted)
            {
                WarningBanner = $"Install was not accepted for {item.DisplayName}.";
                return;
            }

            try
            {
                await _operationTracker.TrackAsync(accepted.OperationId, update => ApplyOperationUpdate(item, update), cancellationToken);
            }
            catch (Exception ex)
            {
                WarningBanner = $"Install queued, but live status stream failed: {ex.Message}";
            }
        }
        finally
        {
            item.IsBusy = false;
        }
    }

    public async Task RemoveAsync(UiOptionalInstallItem item, CancellationToken cancellationToken)
    {
        item.IsBusy = true;
        try
        {
            var accepted = await _client.RemoveItemAsync(item.ItemName, cancellationToken);
            if (!accepted.Accepted)
            {
                WarningBanner = $"Remove was not accepted for {item.DisplayName}.";
                return;
            }

            try
            {
                await _operationTracker.TrackAsync(accepted.OperationId, update => ApplyOperationUpdate(item, update), cancellationToken);
            }
            catch (Exception ex)
            {
                WarningBanner = $"Remove queued, but live status stream failed: {ex.Message}";
            }
        }
        finally
        {
            item.IsBusy = false;
        }
    }

    public UiOptionalInstallItem? FindItem(string itemName)
    {
        return Items.FirstOrDefault(i => string.Equals(i.ItemName, itemName, StringComparison.OrdinalIgnoreCase));
    }

    public void SetWarningBanner(string message)
    {
        WarningBanner = message;
    }

    private void ApplyOperationUpdate(UiOptionalInstallItem item, OperationStatusEvent update)
    {
        item.Status = $"{update.State}: {update.Message}";

        if (update.State is OperationState.Failed or OperationState.Canceled)
        {
            var details = string.IsNullOrWhiteSpace(update.ErrorMessage)
                ? update.Message
                : update.ErrorMessage;
            WarningBanner = $"Operation for {item.DisplayName} ended with {update.State}: {details}";
            return;
        }

        if (update.State is OperationState.Succeeded)
        {
            WarningBanner = string.Empty;
        }
    }

    private void ApplyItems(IReadOnlyList<OptionalInstallItem> source)
    {
        Items.Clear();
        foreach (var item in source)
        {
            Items.Add(new UiOptionalInstallItem
            {
                ItemName = item.ItemName,
                DisplayName = item.DisplayName,
                Version = item.Version,
                Status = item.Status.ToString(),
                IsInstalled = item.IsInstalled,
            });
        }
    }

    private void OnPropertyChanged([CallerMemberName] string? propertyName = null)
    {
        PropertyChanged?.Invoke(this, new PropertyChangedEventArgs(propertyName));
    }
}
