using Gorilla.UI.Client;

namespace Gorilla.UI.Client.Tests;

public class ContractsSmokeTests
{
    [Fact]
    public void OperationState_HasTerminalValues()
    {
        Assert.Contains(OperationState.Succeeded, Enum.GetValues<OperationState>());
        Assert.Contains(OperationState.Failed, Enum.GetValues<OperationState>());
        Assert.Contains(OperationState.Canceled, Enum.GetValues<OperationState>());
    }
}
