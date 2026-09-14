using Gorilla.UI.Client;
using Gorilla.UI.Core.Models;

namespace Gorilla.UI.Core.ViewModels;

public sealed partial class HomeViewModel
{
    public bool TryPresentActionAdmissionFailure(
        UiOptionalInstallItem item,
        ServiceErrorException error
    )
    {
        ArgumentNullException.ThrowIfNull(item);
        ArgumentNullException.ThrowIfNull(error);

        if (!ActionRejectionReasons.Contains(error.ErrorCode))
        {
            return false;
        }

        item.TransientFeedback = OperationRecoveryPresentationMapper.ReasonText(error.ErrorCode);
        return true;
    }
}
