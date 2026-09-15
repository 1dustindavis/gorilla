using System.Diagnostics.CodeAnalysis;
using Gorilla.UI.Client;
using Gorilla.UI.Core.Models;

namespace Gorilla.UI.Core.ViewModels;

public sealed partial class HomeViewModel
{
    [SuppressMessage(
        "Performance",
        "CA1822:Mark members as static",
        Justification = "This is intentionally exposed on the HomeViewModel presentation boundary used by page action handlers."
    )]
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
