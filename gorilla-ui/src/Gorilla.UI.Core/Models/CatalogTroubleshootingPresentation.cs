using System.Text;

namespace Gorilla.UI.Core.Models;

public static class CatalogTroubleshootingPresentation
{
    public static string BuildTechnicalDetails(CatalogDataState state)
    {
        var sections = new List<(string Label, Exception Exception)>();

        if (state.RefreshFailure is not null)
        {
            sections.Add(("Catalog refresh failure", state.RefreshFailure));
        }
        if (state.CacheWriteFailure is not null)
        {
            sections.Add(("Catalog cache write failure", state.CacheWriteFailure));
        }
        if (state.LoadFailure is not null)
        {
            sections.Add(("Catalog load failure", state.LoadFailure));
        }

        if (sections.Count == 0)
        {
            return string.Empty;
        }

        var text = new StringBuilder();
        for (var index = 0; index < sections.Count; index++)
        {
            if (index > 0)
            {
                text.AppendLine();
                text.AppendLine();
            }

            var section = sections[index];
            text.AppendLine(section.Label);
            text.Append(section.Exception);
        }

        return text.ToString();
    }
}
