using Xunit;

// These end-to-end tests share one machine-wide Gorilla service plus registry,
// marker-file, package, and catalog fixture state. Running test classes in
// parallel allows one scenario to mutate another scenario's authoritative state.
[assembly: CollectionBehavior(DisableTestParallelization = true)]
