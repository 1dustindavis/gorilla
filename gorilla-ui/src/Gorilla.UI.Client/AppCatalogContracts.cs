// Planned v2 payloads. The live v1 transport remains unchanged until stage 2/3.
namespace Gorilla.UI.Client.AppCatalog;

public enum ObservedState { Absent, Installed, UpdateAvailable, Unknown, DetectionFailed }
public enum Selection { None, Install }
public enum Action { Install, Remove }
public enum OperationPhase { Queued, Running, Completed }
public enum Outcome { Succeeded, AlreadySatisfied, Failed, Unverified, Interrupted }

public sealed record Observation(
    ObservedState State,
    string? InstalledVersion,
    DateTimeOffset? CheckedAtUtc,
    string DetailCode
);

public sealed record Policy(
    bool Optional,
    bool RequiredInstall,
    bool RequiredUninstall,
    bool RequiredDependency,
    Selection Selection
);

public sealed record ActionDecision(bool Allowed, string Reason);
public sealed record Actions(ActionDecision Install, ActionDecision Remove);
public sealed record Result(Outcome Outcome, string Code);

public sealed record Operation(
    string OperationId,
    string ItemName,
    Action Action,
    OperationPhase Phase,
    int? ProgressPercent,
    Result? Result
);

// TargetVersion is catalog display metadata. State and verification come from
// the service's selected Go detection check; clients must not compare versions.
public sealed record Item(
    string ItemName,
    string DisplayName,
    string Catalog,
    string? TargetVersion,
    Observation Observation,
    Policy Policy,
    Actions Actions,
    Operation? ActiveOperation,
    Operation? LastOperation
);
