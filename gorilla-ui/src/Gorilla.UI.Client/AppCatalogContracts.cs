namespace Gorilla.UI.Client.AppCatalog;

public enum ObservedState { Absent, Installed, UpdateAvailable, Unknown, DetectionFailed }
public enum RequirementState { Unknown, Satisfied, NotSatisfied }
public enum Selection { None, Install }
public enum Action { Install, Remove }
public enum OperationPhase { Queued, Running, Completed }
public enum Outcome { Succeeded, AlreadySatisfied, Failed, Unverified, Interrupted }

public sealed record Observation(
    ObservedState State,
    string? InstalledVersion,
    DateTimeOffset? CheckedAtUtc,
    string DetailCode,
    RequirementState InstallRequirement
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
public sealed record Result(
    Outcome Outcome,
    string Code,
    string? DetailCode = null,
    string? Message = null
);

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
