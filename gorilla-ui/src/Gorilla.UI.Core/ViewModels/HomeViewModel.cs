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
        if (!item.CanInstall)
        {
            WarningBanner = ActionUnavailableMessage("Install", item.DisplayName, item.InstallUnavailableReason);
            return;
        }

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
                AppCatalog.Action.Install,
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
        if (!item.CanRemove)
        {
            WarningBanner = ActionUnavailableMessage("Remove", item.DisplayName, item.RemoveUnavailableReason);
            return;
        }

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
                AppCatalog.Action.Remove,
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
        AppCatalog.Action expectedAction,
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
                    ApplyOperationUpdate(item, expectedAction, update);
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

    private void ApplyOperationUpdate(
        UiOptionalInstallItem item,
        AppCatalog.Action expectedAction,
        OperationStatusEvent update
    )
    {
        ValidateOperationIdentity(item, expectedAction, update);

        if (update.Result is not null)
        {
            ApplyAuthoritativeResult(item, update);
            return;
        }

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

    private void ApplyAuthoritativeResult(UiOptionalInstallItem item, OperationStatusEvent update)
    {
        var result = update.Result!;
        var details = string.IsNullOrWhiteSpace(result.Message) ? update.Message : result.Message;
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
        UiOptionalInstallItem item,
        AppCatalog.Action expectedAction,
        OperationStatusEvent update
    )
    {
        if (!string.IsNullOrWhiteSpace(update.ItemName) &&
            !string.Equals(update.ItemName, item.ItemName, StringComparison.OrdinalIgnoreCase))
        {
            throw new InvalidOperationException(
                $"Operation status identity mismatch. Expected item '{item.ItemName}', got '{update.ItemName}'."
            );
        }

        if (update.Action is not null && update.Action != expectedAction)
        {
            throw new InvalidOperationException(
                $"Operation status action mismatch. Expected '{expectedAction}', got '{update.Action}'."
            );
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
                InstallAllowed = item.Actions?.Install.Allowed ?? false,
                RemoveAllowed = item.Actions?.Remove.Allowed ?? false,
                InstallUnavailableReason = item.Actions?.Install.Reason ?? "Refresh required before installing.",
                RemoveUnavailableReason = item.Actions?.Remove.Reason ?? "Refresh required before removing.",
            });
        }
    }

    private static string ActionUnavailableMessage(string action, string displayName, string reason)
    {
        return string.IsNullOrWhiteSpace(reason)
            ? $"{action} is not available for {displayName}."
            : $"{action} is not available for {displayName}: {reason}";
    }

    private void OnPropertyChanged([CallerMemberName] string? propertyName = null)
    {
        PropertyChanged?.Invoke(this, new PropertyChangedEventArgs(propertyName));
    }
}
