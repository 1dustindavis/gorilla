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
    private readonly Dictionary<string, UiOptionalInstallItem> _catalogItems = new(StringComparer.OrdinalIgnoreCase);

    private string _warningBanner = string.Empty;
    private string _searchQuery = string.Empty;
    private string? _selectedItemName;

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

    // Items is the ordered, searchable presentation projection. _catalogItems is
    // the canonical catalog and remains the source of selection and reconciliation.
    public string SearchQuery
    {
        get => _searchQuery;
        set
        {
            var normalized = value ?? string.Empty;
            if (string.Equals(_searchQuery, normalized, StringComparison.Ordinal))
            {
                return;
            }

            _searchQuery = normalized;
            OnPropertyChanged();
            RebuildVisibleItems();
        }
    }

    public string? SelectedItemName
    {
        get => _selectedItemName;
        private set
        {
            if (string.Equals(_selectedItemName, value, StringComparison.Ordinal))
            {
                return;
            }

            _selectedItemName = value;
            OnPropertyChanged();
            OnPropertyChanged(nameof(SelectedItem));
        }
    }

    public UiOptionalInstallItem? SelectedItem => SelectedItemName is null ? null : FindItem(SelectedItemName);

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
            foreach (var operation in operations)
            {
                ProjectOperation(operation);
                if (operation.State != OperationState.Completed)
                {
                    StartRecoveredTracking(operation, cancellationToken);
                }
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
        if (_catalogItems.TryGetValue(itemName, out var item))
        {
            return item;
        }

        // Supports the existing Stage 4 action entry points while callers migrate
        // from manually supplied list items to canonical catalog state.
        return Items.FirstOrDefault(i => string.Equals(i.ItemName, itemName, StringComparison.OrdinalIgnoreCase));
    }

    public bool SelectItem(string? itemName)
    {
        if (string.IsNullOrWhiteSpace(itemName))
        {
            SelectedItemName = null;
            return true;
        }

        if (!_catalogItems.TryGetValue(itemName, out var item))
        {
            return false;
        }

        SelectedItemName = item.ItemName;
        return true;
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
            item.PreferOperationStatus();
        }

        ReprojectOperation(item);
        if (update.State == OperationState.Completed)
        {
            ApplyAuthoritativeResult(item, update.Result!);
        }
    }

    private void ApplyAuthoritativeResult(UiOptionalInstallItem item, Result result)
    {
        var details = OperationDisplay.Details(result);

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
        var incoming = source.ToDictionary(item => item.ItemName, StringComparer.OrdinalIgnoreCase);

        foreach (var itemName in _catalogItems.Keys.Where(itemName => !incoming.ContainsKey(itemName)).ToArray())
        {
            _catalogItems.Remove(itemName);
        }

        foreach (var snapshot in incoming.Values)
        {
            if (!_catalogItems.TryGetValue(snapshot.ItemName, out var item))
            {
                item = new UiOptionalInstallItem { ItemName = snapshot.ItemName };
                _catalogItems.Add(snapshot.ItemName, item);
            }

            ApplyCatalogSnapshot(item, snapshot);
            ReprojectOperation(item);
            item.PreferObservedStatus();
        }

        if (SelectedItemName is not null && !_catalogItems.ContainsKey(SelectedItemName))
        {
            SelectedItemName = null;
        }
        else
        {
            OnPropertyChanged(nameof(SelectedItem));
        }

        RebuildVisibleItems();
    }

    private static void ApplyCatalogSnapshot(UiOptionalInstallItem item, OptionalInstallItem snapshot)
    {
        item.DisplayName = snapshot.DisplayName;
        item.Description = snapshot.Description;
        item.TargetVersion = snapshot.TargetVersion ??
            (string.IsNullOrEmpty(snapshot.Version) ? null : snapshot.Version);
        item.Observation = snapshot.Observation ?? LegacyObservation(snapshot);
        item.Policy = snapshot.Policy;
        item.InstallDecision = snapshot.Actions?.Install ?? new AppCatalog.ActionDecision(false, "Refresh required before installing.");
        item.RemoveDecision = snapshot.Actions?.Remove ?? new AppCatalog.ActionDecision(false, "Refresh required before removing.");
    }

    private static AppCatalog.Observation LegacyObservation(OptionalInstallItem item)
    {
        var state = item.Status switch
        {
            OptionalInstallStatus.Installed => AppCatalog.ObservedState.Installed,
            OptionalInstallStatus.UpdateAvailable => AppCatalog.ObservedState.UpdateAvailable,
            OptionalInstallStatus.NotInstalled => AppCatalog.ObservedState.Absent,
            _ => AppCatalog.ObservedState.Unknown,
        };
        return new AppCatalog.Observation(
            state,
            InstalledVersion: null,
            CheckedAtUtc: item.StatusUpdatedAtUtc,
            DetailCode: string.Empty,
            InstallRequirement: AppCatalog.RequirementState.Unknown
        );
    }

    private void ReprojectOperation(UiOptionalInstallItem item)
    {
        var active = _operationTracker.GetActiveForItem(item.ItemName);
        var latest = _operationTracker.GetLatestTerminalForItem(item.ItemName);
        item.ActiveOperation = active is null ? null : ToPresentation(active);
        item.LatestOperation = latest is null ? null : ToPresentation(latest);
        item.IsBusy = active is not null;
    }

    private static UiOperationPresentation ToPresentation(OperationStatusEvent operation) => new(
        operation.OperationId,
        operation.Action,
        operation.State,
        operation.ProgressPercent,
        operation.Result,
        operation.Message,
        operation.TimestampUtc
    );

    private void RebuildVisibleItems()
    {
        var query = SearchQuery;
        var desired = _catalogItems.Values
            .Where(item => MatchesSearch(item, query))
            .OrderBy(item => item.DisplayName, StringComparer.OrdinalIgnoreCase)
            .ThenBy(item => item.ItemName, StringComparer.OrdinalIgnoreCase)
            .ThenBy(item => item.ItemName, StringComparer.Ordinal)
            .ToArray();

        foreach (var existing in Items.Where(item => !desired.Contains(item)).ToArray())
        {
            Items.Remove(existing);
        }

        for (var index = 0; index < desired.Length; index++)
        {
            if (index < Items.Count && ReferenceEquals(Items[index], desired[index]))
            {
                continue;
            }

            var existingIndex = Items.IndexOf(desired[index]);
            if (existingIndex >= 0)
            {
                Items.Move(existingIndex, index);
            }
            else
            {
                Items.Insert(index, desired[index]);
            }
        }
    }

    private static bool MatchesSearch(UiOptionalInstallItem item, string query)
    {
        if (string.IsNullOrWhiteSpace(query))
        {
            return true;
        }

        return item.DisplayName.Contains(query, StringComparison.OrdinalIgnoreCase)
            || item.ItemName.Contains(query, StringComparison.OrdinalIgnoreCase)
            || (!string.IsNullOrEmpty(item.Description) && item.Description.Contains(query, StringComparison.OrdinalIgnoreCase));
    }

    private void OnPropertyChanged([CallerMemberName] string? propertyName = null)
    {
        PropertyChanged?.Invoke(this, new PropertyChangedEventArgs(propertyName));
    }
}
