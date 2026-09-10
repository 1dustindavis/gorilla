using System.Collections.Concurrent;
using System.Collections.ObjectModel;
using System.ComponentModel;
using System.Runtime.CompilerServices;
using Gorilla.UI.Client;
using Gorilla.UI.Client.AppCatalog;
using Gorilla.UI.Core.Models;
using Gorilla.UI.Core.Services;
using AppCatalog = Gorilla.UI.Client.AppCatalog;

namespace Gorilla.UI.Core.ViewModels;

public sealed class HomeViewModel : INotifyPropertyChanged
{
    private static readonly TimeSpan RecoveryRetryDelay = TimeSpan.FromSeconds(1);
    private readonly IGorillaServiceClient _client;
    private readonly OptionalInstallsCacheCoordinator _cacheCoordinator;
    private readonly OptionalInstallsStartupLoader _startupLoader;
    private readonly OperationTracker _operationTracker;
    private readonly ConcurrentDictionary<string, byte> _recoveryTracking = new(StringComparer.Ordinal);

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
        var catalogInitialization = _startupLoader.InitializeAsync(
            applyCachedItems: ApplyItems,
            applyRefreshedItems: ApplyItems,
            cancellationToken: cancellationToken
        );

        try
        {
            var operations = await _operationTracker.RefreshKnownOperationsAsync(cancellationToken);
            foreach (var operation in operations.Where(operation => operation.State != OperationState.Completed))
            {
                ProjectOperation(operation);
                StartRecoveredTracking(operation, cancellationToken);
            }
        }
        catch (OperationCanceledException) when (cancellationToken.IsCancellationRequested)
        {
            throw;
        }
        catch (Exception ex)
        {
            WarningBanner = $"Operation status is temporarily unavailable: {ex.Message}";
        }

        var startupWarning = await catalogInitialization;
        if (!string.IsNullOrWhiteSpace(startupWarning))
        {
            WarningBanner = startupWarning;
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

            await TrackAndRefreshAsync(
                item.ItemName,
                item.DisplayName,
                accepted.OperationId,
                AppCatalog.Action.Install,
                streamFailurePrefix: "Install was accepted, but operation status is temporarily unavailable",
                cancellationToken,
                initiatingItem: item
            );
        }
        finally
        {
            item.IsBusy = false;
            var current = FindItem(item.ItemName);
            if (current is not null && _operationTracker.GetActiveForItem(item.ItemName) is null)
            {
                current.IsBusy = false;
            }
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
                item.ItemName,
                item.DisplayName,
                accepted.OperationId,
                AppCatalog.Action.Remove,
                streamFailurePrefix: "Remove was accepted, but operation status is temporarily unavailable",
                cancellationToken,
                initiatingItem: item
            );
        }
        finally
        {
            item.IsBusy = false;
            var current = FindItem(item.ItemName);
            if (current is not null && _operationTracker.GetActiveForItem(item.ItemName) is null)
            {
                current.IsBusy = false;
            }
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
        string itemName,
        string displayName,
        string operationId,
        AppCatalog.Action expectedAction,
        string streamFailurePrefix,
        CancellationToken cancellationToken,
        UiOptionalInstallItem? initiatingItem = null
    )
    {
        while (true)
        {
            var completedObserved = false;
            try
            {
                await _operationTracker.TrackAsync(
                    operationId,
                    update =>
                    {
                        ValidateOperationIdentity(itemName, expectedAction, update);
                        ProjectOperation(update, initiatingItem);
                        completedObserved |= update.State == OperationState.Completed;
                    },
                    cancellationToken
                );
            }
            catch (OperationCanceledException) when (cancellationToken.IsCancellationRequested)
            {
                throw;
            }
            catch (InvalidOperationException ex)
            {
                // A malformed or mismatched service event is a protocol/status error,
                // not evidence that the service restarted or forgot the operation.
                WarningBanner = $"{streamFailurePrefix}: {ex.Message}";
                return;
            }
            catch (Exception ex)
            {
                var continueTracking = await ReconcileTrackingLossAsync(
                    operationId,
                    itemName,
                    displayName,
                    expectedAction,
                    $"{streamFailurePrefix}: {ex.Message}",
                    cancellationToken,
                    initiatingItem
                );
                if (continueTracking)
                {
                    await Task.Delay(RecoveryRetryDelay, cancellationToken);
                    continue;
                }
                return;
            }

            if (!completedObserved)
            {
                return;
            }

            await RefreshCatalogAfterOperationAsync(cancellationToken);
            return;
        }
    }

    private void StartRecoveredTracking(OperationStatusEvent operation, CancellationToken cancellationToken)
    {
        if (!_recoveryTracking.TryAdd(operation.OperationId, 0))
        {
            return;
        }

        _ = TrackRecoveredOperationAsync(operation, cancellationToken);
    }

    private async Task TrackRecoveredOperationAsync(OperationStatusEvent operation, CancellationToken cancellationToken)
    {
        try
        {
            await TrackAndRefreshAsync(
                operation.ItemName,
                FindItem(operation.ItemName)?.DisplayName ?? operation.ItemName,
                operation.OperationId,
                operation.Action,
                streamFailurePrefix: "Recovered operation is still known, but live status is temporarily unavailable",
                cancellationToken
            );
        }
        catch (OperationCanceledException) when (cancellationToken.IsCancellationRequested)
        {
        }
        catch (Exception ex)
        {
            WarningBanner = $"Recovered operation status is temporarily unavailable: {ex.Message}";
        }
        finally
        {
            _recoveryTracking.TryRemove(operation.OperationId, out _);
        }
    }

    private async Task<bool> ReconcileTrackingLossAsync(
        string operationId,
        string itemName,
        string displayName,
        AppCatalog.Action expectedAction,
        string uncertaintyMessage,
        CancellationToken cancellationToken,
        UiOptionalInstallItem? fallbackItem = null
    )
    {
        try
        {
            await _operationTracker.RefreshKnownOperationsAsync(cancellationToken);
            if (_operationTracker.TryGetLatest(operationId, out var latest) && latest is not null)
            {
                ValidateOperationIdentity(itemName, expectedAction, latest);
                ProjectOperation(latest, fallbackItem);
                if (latest.State == OperationState.Completed)
                {
                    await RefreshCatalogAfterOperationAsync(cancellationToken);
                    return false;
                }

                WarningBanner = uncertaintyMessage;
                return true;
            }

            WarningBanner = $"Operation tracking for {displayName} is no longer available. The service may have restarted; current installation state will be refreshed without assuming the previous operation succeeded or failed.";
            await RefreshCatalogAfterOperationAsync(cancellationToken, preserveExistingWarning: true);
            return false;
        }
        catch (OperationCanceledException) when (cancellationToken.IsCancellationRequested)
        {
            throw;
        }
        catch (Exception ex)
        {
            WarningBanner = $"{uncertaintyMessage}. Reconciliation also failed: {ex.Message}";
            return false;
        }
    }

    private async Task RefreshCatalogAfterOperationAsync(
        CancellationToken cancellationToken,
        bool preserveExistingWarning = false
    )
    {
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
            if (!preserveExistingWarning && string.IsNullOrWhiteSpace(WarningBanner))
            {
                WarningBanner = $"Operation completed, but optional installs refresh failed: {ex.Message}";
            }
            else if (preserveExistingWarning)
            {
                WarningBanner = $"{WarningBanner} Refresh also failed: {ex.Message}";
            }
        }
    }

    private void ProjectOperation(OperationStatusEvent update, UiOptionalInstallItem? fallbackItem = null)
    {
        var item = FindItem(update.ItemName);
        if (item is null && fallbackItem is not null &&
            string.Equals(fallbackItem.ItemName, update.ItemName, StringComparison.OrdinalIgnoreCase))
        {
            item = fallbackItem;
        }
        if (item is null)
        {
            return;
        }

        if (update.State == OperationState.Completed)
        {
            item.IsBusy = false;
            ApplyAuthoritativeResult(item, update.Result!);
            return;
        }

        item.IsBusy = true;
        item.Status = $"{update.State}: {update.Message}";
    }

    private void ApplyAuthoritativeResult(UiOptionalInstallItem item, Result result)
    {
        var details = string.IsNullOrWhiteSpace(result.Message) ? result.Code : result.Message;
        item.Status = $"{result.Outcome}: {details}";

        switch (result.Outcome)
        {
            case Outcome.Succeeded:
            case Outcome.AlreadySatisfied:
                WarningBanner = string.Empty;
                break;
            case Outcome.Failed:
            case Outcome.Unverified:
            case Outcome.Interrupted:
                WarningBanner = $"Operation for {item.DisplayName} ended with {result.Outcome}: {details}";
                break;
        }
    }

    private static void ValidateOperationIdentity(
        string itemName,
        AppCatalog.Action expectedAction,
        OperationStatusEvent update
    )
    {
        if (!string.Equals(update.ItemName, itemName, StringComparison.OrdinalIgnoreCase))
        {
            throw new InvalidOperationException(
                $"Operation status identity mismatch. Expected item '{itemName}', got '{update.ItemName}'."
            );
        }

        if (update.Action != expectedAction)
        {
            throw new InvalidOperationException(
                $"Operation status action mismatch. Expected '{expectedAction}', got '{update.Action}'."
            );
        }
    }

    private void ApplyItems(IReadOnlyList<OptionalInstallItem> source)
    {
        Items.Clear();
        foreach (var item in source)
        {
            var uiItem = new UiOptionalInstallItem
            {
                ItemName = item.ItemName,
                DisplayName = item.DisplayName,
                Version = item.Version,
                Status = item.Status.ToString(),
                IsInstalled = item.IsInstalled,
                InstallAllowed = item.Actions?.Install.Allowed ?? false,
                RemoveAllowed = item.Actions?.Remove.Allowed ?? false,
                InstallUnavailableReason = item.Actions?.Install.Reason ?? "Refresh required before installing.",
                RemoveUnavailableReason = item.Actions?.Remove.Reason ?? "Refresh required before removing.",
            };
            Items.Add(uiItem);

            var active = _operationTracker.GetActiveForItem(item.ItemName);
            if (active is not null)
            {
                ProjectOperation(active);
            }
        }
    }

    private void OnPropertyChanged([CallerMemberName] string? propertyName = null)
    {
        PropertyChanged?.Invoke(this, new PropertyChangedEventArgs(propertyName));
    }
}
