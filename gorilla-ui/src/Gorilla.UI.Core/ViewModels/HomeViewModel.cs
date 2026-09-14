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

public sealed partial class HomeViewModel : INotifyPropertyChanged
{
    private static readonly TimeSpan RecoveryRetryDelay = TimeSpan.FromSeconds(1);
    private readonly IGorillaServiceClient _client;
    private readonly OptionalInstallsCacheCoordinator _cacheCoordinator;
    private readonly OptionalInstallsStartupLoader _startupLoader;
    private readonly OperationTracker _operationTracker;
    private readonly ConcurrentDictionary<string, byte> _recoveryTracking = new(StringComparer.Ordinal);
    private readonly Dictionary<string, UiOptionalInstallItem> _catalogItems = new(StringComparer.OrdinalIgnoreCase);
    private readonly Dictionary<string, ActivityOperationPresentation> _activityItems = new(StringComparer.Ordinal);
    private readonly object _projectionStateLock = new();

    private InfrastructureWarningPresentation _infrastructureWarning = InfrastructureWarningPresentation.None;
    private string _searchQuery = string.Empty;
    private string? _selectedItemName;
    private bool _isActivityLoaded;

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
        _operationTracker.OperationsChanged += OperationTracker_OperationsChanged;
    }

    public ObservableCollection<UiOptionalInstallItem> Items { get; } = [];
    public ObservableCollection<ActivityOperationPresentation> ActivityItems { get; } = [];

    public bool IsActivityLoaded
    {
        get => _isActivityLoaded;
        private set
        {
            if (_isActivityLoaded == value)
            {
                return;
            }

            _isActivityLoaded = value;
            OnPropertyChanged();
        }
    }

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

    public InfrastructureWarningPresentation InfrastructureWarning
    {
        get => _infrastructureWarning;
        private set
        {
            if (Equals(_infrastructureWarning, value))
            {
                return;
            }

            _infrastructureWarning = value;
            OnPropertyChanged();
            OnPropertyChanged(nameof(WarningBanner));
        }
    }

    public string WarningBanner => InfrastructureWarning.Message;

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
            IsActivityLoaded = true;
            RebuildActivityProjection();
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
            SetInfrastructureWarning(
                "Operation status is temporarily unavailable.",
                "Retained operation lookup during App Catalog initialization",
                ex
            );
        }

        var startupWarning = await catalogInitialization;
        if (!string.IsNullOrWhiteSpace(startupWarning))
        {
            SetInfrastructureWarning(startupWarning, "App Catalog startup warning");
        }
    }

    public async Task InstallAsync(UiOptionalInstallItem item, CancellationToken cancellationToken)
    {
        item.TransientFeedback = null;
        item.IsBusy = true;
        try
        {
            var accepted = await _client.InstallItemAsync(item.ItemName, cancellationToken);
            if (!accepted.Accepted)
            {
                item.TransientFeedback = $"Install was not accepted for {item.DisplayName}.";
                return;
            }

            ClearActionStartInfrastructureWarning(AppCatalog.Action.Install, item.ItemName);
            await TrackAndRefreshAsync(
                item.ItemName,
                item.DisplayName,
                accepted.OperationId,
                AppCatalog.Action.Install,
                streamFailureMessage: "Install was accepted, but Gorilla can't currently confirm its status.",
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
        item.TransientFeedback = null;
        item.IsBusy = true;
        try
        {
            var accepted = await _client.RemoveItemAsync(item.ItemName, cancellationToken);
            if (!accepted.Accepted)
            {
                item.TransientFeedback = $"Remove was not accepted for {item.DisplayName}.";
                return;
            }

            ClearActionStartInfrastructureWarning(AppCatalog.Action.Remove, item.ItemName);
            await TrackAndRefreshAsync(
                item.ItemName,
                item.DisplayName,
                accepted.OperationId,
                AppCatalog.Action.Remove,
                streamFailureMessage: "Remove was accepted, but Gorilla can't currently confirm its status.",
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
        lock (_projectionStateLock)
        {
            if (_catalogItems.TryGetValue(itemName, out var item))
            {
                return item;
            }

            return Items.FirstOrDefault(i => string.Equals(i.ItemName, itemName, StringComparison.OrdinalIgnoreCase));
        }
    }

    public bool SelectItem(string? itemName)
    {
        lock (_projectionStateLock)
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
    }

    public void SetWarningBanner(string message)
    {
        SetInfrastructureWarning(message, "App Catalog UI warning");
    }

    public void SetActionStartInfrastructureWarning(
        AppCatalog.Action action,
        string itemName,
        Exception exception
    )
    {
        SetInfrastructureWarning(
            "Gorilla couldn't start that action. Refresh and try again.",
            $"Unexpected {action} action-start failure",
            exception,
            itemName: itemName,
            expectedAction: action.ToString()
        );
    }

    public void ClearInfrastructureWarning()
    {
        InfrastructureWarning = InfrastructureWarningPresentation.None;
    }

    private void SetInfrastructureWarning(
        string message,
        string context,
        Exception? exception = null,
        string? operationId = null,
        string? itemName = null,
        string? expectedAction = null,
        string? additionalTechnicalDetails = null
    )
    {
        InfrastructureWarning = InfrastructureWarningPresentation.Create(
            message,
            context,
            exception,
            operationId,
            itemName,
            expectedAction,
            additionalTechnicalDetails
        );
    }

    private async Task TrackAndRefreshAsync(
        string itemName,
        string displayName,
        string operationId,
        AppCatalog.Action expectedAction,
        string streamFailureMessage,
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
                        ClearOperationStatusInfrastructureWarning(operationId, itemName, expectedAction);
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
                SetInfrastructureWarning(
                    streamFailureMessage,
                    "Malformed or mismatched operation-status event",
                    ex,
                    operationId,
                    itemName,
                    expectedAction.ToString()
                );
                return;
            }
            catch (Exception ex)
            {
                var continueTracking = await ReconcileTrackingLossAsync(
                    operationId,
                    itemName,
                    displayName,
                    expectedAction,
                    streamFailureMessage,
                    ex,
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
                streamFailureMessage: "Recovered operation is still known, but Gorilla can't currently confirm its status.",
                cancellationToken
            );
        }
        catch (OperationCanceledException) when (cancellationToken.IsCancellationRequested)
        {
        }
        catch (Exception ex)
        {
            SetInfrastructureWarning(
                "Recovered operation status is temporarily unavailable.",
                "Recovered-operation tracking failure",
                ex,
                operation.OperationId,
                operation.ItemName,
                operation.Action.ToString()
            );
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
        Exception trackingException,
        CancellationToken cancellationToken,
        UiOptionalInstallItem? fallbackItem = null
    )
    {
        try
        {
            await _operationTracker.RefreshKnownOperationsAsync(cancellationToken);
            IsActivityLoaded = true;
            RebuildActivityProjection();
            if (_operationTracker.TryGetLatest(operationId, out var latest) && latest is not null)
            {
                ValidateOperationIdentity(itemName, expectedAction, latest);
                ClearOperationStatusInfrastructureWarning(operationId, itemName, expectedAction);
                ProjectOperation(latest, fallbackItem);
                if (latest.State == OperationState.Completed)
                {
                    await RefreshCatalogAfterOperationAsync(cancellationToken);
                    return false;
                }

                SetInfrastructureWarning(
                    uncertaintyMessage,
                    "Operation status stream lost; operation remains known and active",
                    trackingException,
                    operationId,
                    itemName,
                    expectedAction.ToString()
                );
                return true;
            }

            SetInfrastructureWarning(
                $"Operation tracking for {displayName} is no longer available. Gorilla refreshed current installation state without assuming the previous operation succeeded or failed.",
                "Operation disappeared during tracking-loss reconciliation; the service may have restarted",
                trackingException,
                operationId,
                itemName,
                expectedAction.ToString()
            );
            await RefreshCatalogAfterOperationAsync(cancellationToken, preserveExistingWarning: true);
            return false;
        }
        catch (OperationCanceledException) when (cancellationToken.IsCancellationRequested)
        {
            throw;
        }
        catch (Exception ex)
        {
            SetInfrastructureWarning(
                "Gorilla couldn't reconcile the operation after losing status updates.",
                "Operation tracking-loss reconciliation failed",
                ex,
                operationId,
                itemName,
                expectedAction.ToString(),
                additionalTechnicalDetails: $"Original tracking exception type: {trackingException.GetType().FullName}\nOriginal tracking exception message: {trackingException.Message}"
            );
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
        catch
        {
            _ = preserveExistingWarning;
        }
    }

    private void OperationTracker_OperationsChanged(object? sender, EventArgs e)
    {
        RebuildActivityProjection();
    }

    private void ProjectOperation(OperationStatusEvent update, UiOptionalInstallItem? fallbackItem = null)
    {
        lock (_projectionStateLock)
        {
            RebuildActivityProjection();
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
        lock (_projectionStateLock)
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

            RebuildVisibleItems();
            RebuildActivityProjection();
        }
    }

    private void RebuildVisibleItems()
    {
        var visible = _catalogItems.Values
            .Where(item => string.IsNullOrWhiteSpace(SearchQuery) ||
                           item.DisplayName.Contains(SearchQuery, StringComparison.OrdinalIgnoreCase) ||
                           item.ItemName.Contains(SearchQuery, StringComparison.OrdinalIgnoreCase))
            .OrderBy(item => item.DisplayName, StringComparer.CurrentCultureIgnoreCase)
            .ToArray();

        Items.Clear();
        foreach (var item in visible)
        {
            Items.Add(item);
        }
    }

    private void RebuildActivityProjection()
    {
        lock (_projectionStateLock)
        {
            var operations = _operationTracker.Snapshot();
            var currentIds = operations.Select(operation => operation.OperationId).ToHashSet(StringComparer.Ordinal);

            foreach (var staleId in _activityItems.Keys.Where(id => !currentIds.Contains(id)).ToArray())
            {
                _activityItems.Remove(staleId);
            }

            foreach (var operation in operations)
            {
                var displayName = FindItem(operation.ItemName)?.DisplayName ?? operation.ItemName;
                if (!_activityItems.TryGetValue(operation.OperationId, out var presentation))
                {
                    presentation = new ActivityOperationPresentation(operation.OperationId);
                    _activityItems.Add(operation.OperationId, presentation);
                }

                presentation.Apply(operation, displayName, FindItem(operation.ItemName) is not null);
                presentation.ApplyRecovery(BuildRecoveryPresentation(operation));
            }

            var ordered = _activityItems.Values
                .OrderByDescending(item => item.TimestampUtc)
                .ToArray();

            ActivityItems.Clear();
            foreach (var item in ordered)
            {
                ActivityItems.Add(item);
            }
        }
    }

    private OperationRecoveryPresentation BuildRecoveryPresentation(OperationStatusEvent operation)
    {
        var item = FindItem(operation.ItemName);
        return OperationRecoveryPresentationMapper.Map(operation, item);
    }

    private void ReprojectOperation(UiOptionalInstallItem item)
    {
        var active = _operationTracker.GetActiveForItem(item.ItemName);
        item.ActiveOperation = active;
        item.LatestOperation = _operationTracker.GetLatestForItem(item.ItemName);
    }

    private static void ApplyCatalogSnapshot(UiOptionalInstallItem target, OptionalInstallItem snapshot)
    {
        target.ItemName = snapshot.ItemName;
        target.DisplayName = snapshot.DisplayName;
        target.Description = snapshot.Description;
        target.Installed = snapshot.Installed;
        target.Selected = snapshot.Selected;
        target.NeedsUpdate = snapshot.NeedsUpdate;
        target.IsManaged = snapshot.IsManaged;
        target.InstallDecision = snapshot.InstallDecision;
        target.RemoveDecision = snapshot.RemoveDecision;
    }

    private void OnPropertyChanged([CallerMemberName] string? propertyName = null)
        => PropertyChanged?.Invoke(this, new PropertyChangedEventArgs(propertyName));
}
