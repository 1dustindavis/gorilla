using System.ComponentModel;
using System.Runtime.CompilerServices;
using Gorilla.UI.Client;
using Gorilla.UI.Client.AppCatalog;
using CatalogAction = Gorilla.UI.Client.AppCatalog.Action;

namespace Gorilla.UI.Core.Models;

// This is a presentation projection of service-owned catalog and operation truth.
// It deliberately does not decide whether an action is allowed or derive software
// state by comparing versions.
public sealed class UiOptionalInstallItem : INotifyPropertyChanged
{
    private string _displayName = string.Empty;
    private string? _description;
    private string? _targetVersion;
    private Observation _observation = new(ObservedState.Unknown, null, null, string.Empty, RequirementState.Unknown);
    private Policy? _policy;
    private ActionDecision _installDecision = new(false, "Refresh required before installing.");
    private ActionDecision _removeDecision = new(false, "Refresh required before removing.");
    private UiOperationPresentation? _activeOperation;
    private UiOperationPresentation? _latestOperation;
    private string? _transientFeedback;
    private string? _installRetryBlockedReason;
    private string? _removeRetryBlockedReason;
    private bool _isBusy;
    private string? _legacyStatus;
    private bool _preferObservedStatus;

    public required string ItemName { get; init; }

    public string DisplayName
    {
        get => _displayName;
        set
        {
            if (SetField(ref _displayName, value))
            {
                OnPresentationsChanged();
            }
        }
    }

    public string? Description
    {
        get => _description;
        set
        {
            if (SetField(ref _description, value))
            {
                OnPresentationsChanged();
            }
        }
    }

    public string? TargetVersion
    {
        get => _targetVersion;
        set
        {
            if (SetField(ref _targetVersion, value))
            {
                OnPropertyChanged(nameof(Version));
                OnPresentationsChanged();
            }
        }
    }

    public string Version
    {
        get => TargetVersion ?? string.Empty;
        set => TargetVersion = value;
    }

    public Observation Observation
    {
        get => _observation;
        set
        {
            if (SetField(ref _observation, value))
            {
                _legacyStatus = null;
                OnPropertyChanged(nameof(ObservedState));
                OnPropertyChanged(nameof(InstalledVersion));
                OnPropertyChanged(nameof(IsInstalled));
                OnPropertyChanged(nameof(Status));
                OnPresentationsChanged();
            }
        }
    }

    public ObservedState ObservedState => Observation.State;
    public string? InstalledVersion => Observation.InstalledVersion;

    public Policy? Policy
    {
        get => _policy;
        set
        {
            if (SetField(ref _policy, value))
            {
                OnPresentationsChanged();
            }
        }
    }

    public ActionDecision InstallDecision
    {
        get => _installDecision;
        set
        {
            if (SetField(ref _installDecision, value))
            {
                OnPropertyChanged(nameof(InstallAllowed));
                OnPropertyChanged(nameof(InstallUnavailableReason));
                OnPropertyChanged(nameof(CanInstall));
                OnPresentationsChanged();
            }
        }
    }

    public ActionDecision RemoveDecision
    {
        get => _removeDecision;
        set
        {
            if (SetField(ref _removeDecision, value))
            {
                OnPropertyChanged(nameof(RemoveAllowed));
                OnPropertyChanged(nameof(RemoveUnavailableReason));
                OnPropertyChanged(nameof(CanRemove));
                OnPresentationsChanged();
            }
        }
    }

    public bool InstallAllowed
    {
        get => InstallDecision.Allowed;
        set => InstallDecision = InstallDecision with { Allowed = value };
    }

    public bool RemoveAllowed
    {
        get => RemoveDecision.Allowed;
        set => RemoveDecision = RemoveDecision with { Allowed = value };
    }

    public string InstallUnavailableReason
    {
        get => InstallDecision.Reason;
        set => InstallDecision = InstallDecision with { Reason = value };
    }

    public string RemoveUnavailableReason
    {
        get => RemoveDecision.Reason;
        set => RemoveDecision = RemoveDecision with { Reason = value };
    }

    public UiOperationPresentation? ActiveOperation
    {
        get => _activeOperation;
        set
        {
            if (SetField(ref _activeOperation, value))
            {
                OnPropertyChanged(nameof(Status));
                OnPresentationsChanged();
            }
        }
    }

    public UiOperationPresentation? LatestOperation
    {
        get => _latestOperation;
        set
        {
            if (SetField(ref _latestOperation, value))
            {
                OnPropertyChanged(nameof(Status));
                OnPresentationsChanged();
            }
        }
    }

    public string? TransientFeedback
    {
        get => _transientFeedback;
        set
        {
            if (SetField(ref _transientFeedback, value))
            {
                OnPresentationsChanged();
            }
        }
    }

    // A service-side admission rejection is fresher than the cached action
    // decision, but it must not rewrite that service-derived snapshot. Admission
    // applies to the current item/action, not to one historical row, so retain an
    // independent temporary guard for Install and Remove until manual Refresh.
    public string? InstallRetryBlockedReason => _installRetryBlockedReason;
    public string? RemoveRetryBlockedReason => _removeRetryBlockedReason;

    public string? RetryBlockReasonFor(CatalogAction action)
        => action == CatalogAction.Remove ? _removeRetryBlockedReason : _installRetryBlockedReason;

    public void BlockRetry(CatalogAction action, string reason)
    {
        ref var field = ref action == CatalogAction.Remove
            ? ref _removeRetryBlockedReason
            : ref _installRetryBlockedReason;
        if (string.Equals(field, reason, StringComparison.Ordinal))
        {
            return;
        }

        field = reason;
        OnPropertyChanged(action == CatalogAction.Remove
            ? nameof(RemoveRetryBlockedReason)
            : nameof(InstallRetryBlockedReason));
        OnPresentationsChanged();
    }

    public void ClearRetryBlocks()
    {
        if (_installRetryBlockedReason is null && _removeRetryBlockedReason is null)
        {
            return;
        }

        _installRetryBlockedReason = null;
        _removeRetryBlockedReason = null;
        OnPropertyChanged(nameof(InstallRetryBlockedReason));
        OnPropertyChanged(nameof(RemoveRetryBlockedReason));
        OnPresentationsChanged();
    }

    public bool IsInstalled
    {
        get => Observation.State is ObservedState.Installed or ObservedState.UpdateAvailable;
        set => Observation = Observation with
        {
            State = value ? ObservedState.Installed : ObservedState.Absent,
        };
    }

    public bool CanInstall => InstallDecision.Allowed && !IsBusy;
    public bool CanRemove => RemoveDecision.Allowed && !IsBusy;
    public CatalogCardPresentation CardPresentation => CatalogCardPresentationMapper.Map(this);
    public AppDetailsPresentation DetailsPresentation => AppDetailsPresentationMapper.Map(this);

    public string Status
    {
        get => ActiveOperation is not null
        ? $"{ActiveOperation.State}: {ActiveOperation.Message}"
        : !_preferObservedStatus && LatestOperation?.Result is { } result
            ? $"{result.Outcome}: {OperationDisplay.Details(result)}"
            : _legacyStatus ?? Observation.State switch
            {
                ObservedState.Absent => "NotInstalled",
                _ => Observation.State.ToString(),
            };
        set
        {
            _legacyStatus = value;
            OnPropertyChanged();
        }
    }

    internal void PreferObservedStatus()
    {
        if (!_preferObservedStatus)
        {
            _preferObservedStatus = true;
            OnPropertyChanged(nameof(Status));
        }
    }

    internal void PreferOperationStatus()
    {
        if (_preferObservedStatus)
        {
            _preferObservedStatus = false;
            OnPropertyChanged(nameof(Status));
        }
    }

    public bool IsBusy
    {
        get => _isBusy;
        set
        {
            if (SetField(ref _isBusy, value))
            {
                OnPropertyChanged(nameof(CanInstall));
                OnPropertyChanged(nameof(CanRemove));
                OnPresentationsChanged();
            }
        }
    }

    public event PropertyChangedEventHandler? PropertyChanged;

    private bool SetField<T>(ref T field, T value, [CallerMemberName] string? propertyName = null)
    {
        if (EqualityComparer<T>.Default.Equals(field, value))
        {
            return false;
        }

        field = value;
        OnPropertyChanged(propertyName);
        return true;
    }

    private void OnPresentationsChanged()
    {
        OnPropertyChanged(nameof(CardPresentation));
        OnPropertyChanged(nameof(DetailsPresentation));
    }

    private void OnPropertyChanged([CallerMemberName] string? propertyName = null)
        => PropertyChanged?.Invoke(this, new PropertyChangedEventArgs(propertyName));
}

public sealed record UiOperationPresentation(
    string OperationId,
    CatalogAction Action,
    OperationState State,
    int? ProgressPercent,
    Result? Result,
    string Message,
    DateTimeOffset TimestampUtc
)
{
    public OperationPhase Phase => State switch
    {
        OperationState.Queued => OperationPhase.Queued,
        OperationState.Completed => OperationPhase.Completed,
        _ => OperationPhase.Running,
    };
}

internal static class OperationDisplay
{
    public static string Details(Result result)
        => string.IsNullOrWhiteSpace(result.Message) ? result.Code : result.Message;
}
