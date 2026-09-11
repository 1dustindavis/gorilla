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
    private ActionDecision _installDecision = new(false, "Refresh required before installing.");
    private ActionDecision _removeDecision = new(false, "Refresh required before removing.");
    private UiOperationPresentation? _activeOperation;
    private UiOperationPresentation? _latestOperation;
    private bool _isBusy;
    private string? _legacyStatus;

    public required string ItemName { get; init; }

    public string DisplayName
    {
        get => _displayName;
        set => SetField(ref _displayName, value);
    }

    public string? Description
    {
        get => _description;
        set => SetField(ref _description, value);
    }

    public string? TargetVersion
    {
        get => _targetVersion;
        set
        {
            if (SetField(ref _targetVersion, value))
            {
                OnPropertyChanged(nameof(Version));
            }
        }
    }

    // Compatibility alias for the existing Stage 4 surface. Version is target/offered
    // metadata, never an inferred installed version.
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
            }
        }
    }

    public ObservedState ObservedState => Observation.State;

    public string? InstalledVersion => Observation.InstalledVersion;

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
            }
        }
    }

    // The latest retained terminal operation. This remains separate from both
    // ActiveOperation and Observation.
    public UiOperationPresentation? LatestOperation
    {
        get => _latestOperation;
        set
        {
            if (SetField(ref _latestOperation, value))
            {
                OnPropertyChanged(nameof(Status));
            }
        }
    }

    public bool IsInstalled
    {
        get => Observation.State is ObservedState.Installed or ObservedState.UpdateAvailable;
        // Compatibility for existing callers that construct presentation items
        // directly. Service snapshots always set Observation instead.
        set => Observation = Observation with
        {
            State = value ? ObservedState.Installed : ObservedState.Absent,
        };
    }

    public bool CanInstall => InstallDecision.Allowed && !IsBusy;

    public bool CanRemove => RemoveDecision.Allowed && !IsBusy;

    // Transitional compatibility state for the existing Stage 4 ListView. New UI
    // should bind the typed Observation/ActiveOperation/LatestOperation properties.
    public string Status
    {
        get => ActiveOperation is not null
        ? $"{ActiveOperation.State}: {ActiveOperation.Message}"
        : LatestOperation?.Result is { } result
            ? $"{result.Outcome}: {OperationDisplay.Details(result)}"
            : _legacyStatus ?? Observation.State.ToString();
        set
        {
            _legacyStatus = value;
            OnPropertyChanged();
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
