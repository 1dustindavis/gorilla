using System.ComponentModel;
using System.Runtime.CompilerServices;
using Gorilla.UI.Client;
using CatalogAction = Gorilla.UI.Client.AppCatalog.Action;

namespace Gorilla.UI.Core.Models;

// Presentation-only projection of one retained service operation. The structured
// operation identity/result are deliberately preserved so a later retry surface
// can re-enter the current service-authorized action path without replaying the
// historical mutation.
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

    public bool IsActive => State != OperationState.Completed;
    public bool IsTerminal => !IsActive;
    public bool HasDeterminateProgress => IsActive && ProgressPercent.HasValue;
    public bool IsProgressIndeterminate => IsActive && !ProgressPercent.HasValue;
    public double ProgressValue => ProgressPercent ?? 0;
    public bool HasDetail => !string.IsNullOrWhiteSpace(DetailText);

    // Retained service action is historical truth. Contextual Stage 5 labels such
    // as Update/Keep Installed are intentionally not reconstructed from current state.
    public string ActionLabel => Action switch
    {
        CatalogAction.Remove => "Remove",
        _ => "Install",
    };

    public string StateText => IsActive
        ? State.ToString()
        : Result?.Outcome.ToString() ?? OperationState.Completed.ToString();

    public string DetailText => IsTerminal && Result is not null
        ? (string.IsNullOrWhiteSpace(Result.Message) ? Result.Code : Result.Message)
        : Message;

    public string TimestampText => TimestampUtc.ToLocalTime().ToString("g");

    public string EntryAutomationId => $"ActivityOperation-{OperationId}";
    public string AppAutomationId => $"ActivityApp-{OperationId}";
    public string ActionAutomationId => $"ActivityAction-{OperationId}";
    public string StateAutomationId => $"ActivityState-{OperationId}";
    public string ProgressAutomationId => $"ActivityProgress-{OperationId}";
    public string DetailAutomationId => $"ActivityDetail-{OperationId}";

    internal void Apply(OperationStatusEvent operation, string displayName, bool canNavigate)
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
