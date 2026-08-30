using System.Collections.ObjectModel;
using System.ComponentModel;
using System.Runtime.CompilerServices;
using Gorilla.UI.Client;
using Gorilla.UI.Core.Models;
using Gorilla.UI.Core.Services;

namespace Gorilla.UI.Core.ViewModels;

public sealed class HomeViewModel : INotifyPropertyChanged
{
    private readonly IGorillaServiceClient _client;
    private readonly OptionalInstallsCacheCoordinator _cacheCoordinator;
    private readonly OptionalInstallsStartupLoader _startupLoader;
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
        _startupLoader = new OptionalInstallsStartupLoader(cacheCoordinator);
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
        WarningBanner = await _startupLoader.InitializeAsync(
            applyCachedItems: ApplyItems,
            applyRefreshedItems: ApplyItems,
            cancellationToken: cancellationToken
        );
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

            await TrackAndRefreshAsync(
                item,
                accepted.OperationId,
                streamFailurePrefix: "Install queued, but live status stream failed",
                cancellationToken
            );
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

            await TrackAndRefreshAsync(
                item,
                accepted.OperationId,
                streamFailurePrefix: "Remove queued, but live status stream failed",
                cancellationToken
            );
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

    private async Task TrackAndRefreshAsync(
        UiOptionalInstallItem item,
        string operationId,
        string streamFailurePrefix,
        CancellationToken cancellationToken
    )
    {
        var terminalStateObserved = false;
        try
        {
            await _operationTracker.TrackAsync(
                operationId,
                update =>
                {
                    ApplyOperationUpdate(item, update);
                    terminalStateObserved |= IsTerminalState(update.State);
                },
                cancellationToken
            );
        }
        catch (OperationCanceledException) when (cancellationToken.IsCancellationRequested)
        {
            throw;
        }
        catch (Exception ex)
        {
            WarningBanner = $"{streamFailurePrefix}: {ex.Message}";
            return;
        }

        if (!terminalStateObserved)
        {
            return;
        }

        try
        {
            var refreshed = await _cacheCoordinator.RefreshAsync(cancellationToken);
            ApplyItems(refreshed.Items);
        }
        catch (OperationCanceledException) when (cancellationToken.IsCancellationRequested)
        {
            throw;
        }
        catch (Exception ex)
        {
            if (string.IsNullOrWhiteSpace(WarningBanner))
            {
                WarningBanner = $"Operation completed, but optional installs refresh failed: {ex.Message}";
            }
        }
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

    private static bool IsTerminalState(OperationState state)
    {
        return state is OperationState.Succeeded or OperationState.Failed or OperationState.Canceled;
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
