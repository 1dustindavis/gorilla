using System.ComponentModel;
using System.Globalization;
using System.Runtime.CompilerServices;
using Gorilla.UI.Client;
using Gorilla.UI.Client.AppCatalog;
using CatalogAction = Gorilla.UI.Client.AppCatalog.Action;

namespace Gorilla.UI.Core.Models;

// Presentation-only projection of one retained service operation. Historical
// operation identity/result remain immutable truth; Recovery describes only what
// can be done now against the current canonical catalog item.
public sealed class ActivityOperationPresentation : INotifyPropertyChanged
{
    private string _itemName = string.Empty;
    private string _displayName = string.Empty;
    private CatalogAction _action;
    private OperationState _state;
    private int? _progressPercent;
    private Result? _result;
    private string _message = string.Empty;
    private DateTimeOffset _timestampUtc;
    private bool _canNavigate;
    private OperationRecoveryPresentation? _recovery;

    public ActivityOperationPresentation(string operationId)
    {
        OperationId = operationId;
    }

    public string OperationId { get; }
    public string ItemName => _itemName;
    public string DisplayName => _displayName;
    public CatalogAction Action => _action;
    public OperationState State => _state;
    public int? ProgressPercent => _progressPercent;
    public Result? Result => _result;
    public string Message => _message;
    public DateTimeOffset TimestampUtc => _timestampUtc;
    public bool CanNavigate => _canNavigate;
    public OperationRecoveryPresentation? Recovery => _recovery;

    public bool IsActive => State != OperationState.Completed;
    public bool IsTerminal => !IsActive;
    public bool HasDeterminateProgress => IsActive && ProgressPercent.HasValue;
    public bool IsProgressIndeterminate => IsActive && !ProgressPercent.HasValue;
    public double ProgressValue => ProgressPercent ?? 0;
    public bool HasRecovery => Recovery?.IsRetryCandidate == true;
    public bool HasDetail => !HasRecovery && !string.IsNullOrWhiteSpace(DetailText);
    public bool CanRetry => Recovery?.CanRetry == true;
    public bool HasRetryUnavailableReason => Recovery?.HasRetryUnavailableReason == true;
    public bool HasTechnicalDetails => Recovery?.HasTechnicalDetails == true;
    public string FailureTitle => Recovery?.OutcomeTitle ?? StateText;
    public string? FailureMessage => Recovery?.UserMessage;
    public bool HasFailureMessage => !string.IsNullOrWhiteSpace(FailureMessage);
    public string RetryLabel => Recovery?.RetryLabel ?? "Retry";
    public string? RetryUnavailableReason => Recovery?.RetryUnavailableReason;
    public string TechnicalDetails => Recovery?.TechnicalDetails ?? string.Empty;

    // Retained service action is historical truth. Contextual Stage 5 labels such
    // as Update/Keep Installed are intentionally not reconstructed from current state.
    public string ActionLabel => Action switch
    {
        CatalogAction.Remove => "Remove",
        _ => "Install",
    };

    public string StateText => IsActive
        ? ActiveStateLabel(State)
        : Result is null ? "Completed" : OutcomeLabel(Result.Outcome);

    public string DetailText => IsTerminal && Result is not null
        ? OperationDisplay.Details(Result)
        : Message;

    public string TimestampText => TimestampUtc.ToLocalTime().ToString("g", CultureInfo.CurrentCulture);

    public string EntryAutomationId => $"ActivityOperation-{OperationId}";
    public string AppAutomationId => $"ActivityApp-{OperationId}";
    public string ActionAutomationId => $"ActivityAction-{OperationId}";
    public string StateAutomationId => $"ActivityState-{OperationId}";
    public string ProgressAutomationId => $"ActivityProgress-{OperationId}";
    public string DetailAutomationId => $"ActivityDetail-{OperationId}";
    public string FailureTitleAutomationId => $"ActivityFailureTitle-{OperationId}";
    public string RetryAutomationId => $"ActivityRetry-{OperationId}";
    public string RetryUnavailableAutomationId => $"ActivityRetryUnavailable-{OperationId}";
    public string TechnicalDetailsAutomationId => $"OperationTechnicalDetails-{OperationId}";
    public string TechnicalDetailsContentAutomationId => $"OperationTechnicalDetailsContent-{OperationId}";

    internal void Apply(OperationStatusEvent operation, string displayName, bool canNavigate)
    {
        ApplyHistorical(operation, displayName, canNavigate);
        // RebuildActivityProjection predates PR G and intentionally does not own
        // current-action retry policy. HomeViewModel.RefreshActivityRecoveryPresentations
        // immediately overlays current canonical truth for recovery surfaces.
        ApplyRecovery(OperationRecoveryPresentationMapper.Map(
            operation,
            currentItem: null,
            hasConflictingActiveOperation: false
        ));
    }

    internal void ApplyRecovery(OperationRecoveryPresentation recovery)
    {
        SetField(ref _recovery, recovery, nameof(Recovery));
        OnPropertyChanged(nameof(HasRecovery));
        OnPropertyChanged(nameof(HasDetail));
        OnPropertyChanged(nameof(CanRetry));
        OnPropertyChanged(nameof(HasRetryUnavailableReason));
        OnPropertyChanged(nameof(HasTechnicalDetails));
        OnPropertyChanged(nameof(FailureTitle));
        OnPropertyChanged(nameof(FailureMessage));
        OnPropertyChanged(nameof(HasFailureMessage));
        OnPropertyChanged(nameof(RetryLabel));
        OnPropertyChanged(nameof(RetryUnavailableReason));
        OnPropertyChanged(nameof(TechnicalDetails));
    }

    private void ApplyHistorical(OperationStatusEvent operation, string displayName, bool canNavigate)
    {
        if (!string.Equals(operation.OperationId, OperationId, StringComparison.Ordinal))
        {
            throw new InvalidOperationException("Cannot reconcile a different operation into an Activity entry.");
        }

        SetField(ref _itemName, operation.ItemName, nameof(ItemName));
        SetField(ref _displayName, displayName, nameof(DisplayName));
        SetField(ref _action, operation.Action, nameof(Action));
        SetField(ref _state, operation.State, nameof(State));
        SetField(ref _progressPercent, operation.ProgressPercent, nameof(ProgressPercent));
        SetField(ref _result, operation.Result, nameof(Result));
        SetField(ref _message, operation.Message, nameof(Message));
        SetField(ref _timestampUtc, operation.TimestampUtc, nameof(TimestampUtc));
        SetField(ref _canNavigate, canNavigate, nameof(CanNavigate));

        OnPropertyChanged(nameof(IsActive));
        OnPropertyChanged(nameof(IsTerminal));
        OnPropertyChanged(nameof(HasDeterminateProgress));
        OnPropertyChanged(nameof(IsProgressIndeterminate));
        OnPropertyChanged(nameof(ProgressValue));
        OnPropertyChanged(nameof(ActionLabel));
        OnPropertyChanged(nameof(StateText));
        OnPropertyChanged(nameof(DetailText));
        OnPropertyChanged(nameof(HasDetail));
        OnPropertyChanged(nameof(TimestampText));
    }

    public event PropertyChangedEventHandler? PropertyChanged;

    private static string ActiveStateLabel(OperationState state) => state switch
    {
        OperationState.Queued => "Queued",
        OperationState.Validating => "Preparing",
        OperationState.Downloading => "Downloading",
        OperationState.Installing => "Installing",
        OperationState.Removing => "Removing",
        OperationState.Completed => "Completing",
        _ => "Working",
    };

    private static string OutcomeLabel(Outcome outcome) => outcome switch
    {
        Outcome.Succeeded => "Succeeded",
        Outcome.AlreadySatisfied => "Already satisfied",
        Outcome.Failed => "Failed",
        Outcome.Unverified => "Unable to verify",
        Outcome.Interrupted => "Interrupted",
        _ => outcome.ToString(),
    };

    private void SetField<T>(ref T field, T value, string propertyName)
    {
        if (EqualityComparer<T>.Default.Equals(field, value))
        {
            return;
        }

        field = value;
        OnPropertyChanged(propertyName);
    }

    private void OnPropertyChanged([CallerMemberName] string? propertyName = null)
        => PropertyChanged?.Invoke(this, new PropertyChangedEventArgs(propertyName));
}
