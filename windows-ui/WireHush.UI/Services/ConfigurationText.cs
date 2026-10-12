namespace WireHush.UI.Services;

internal static class ConfigurationText
{
    public static string NormalizeLineEndings(string value) =>
        value.Replace("\r\n", "\n", StringComparison.Ordinal).Replace('\r', '\n');
}
