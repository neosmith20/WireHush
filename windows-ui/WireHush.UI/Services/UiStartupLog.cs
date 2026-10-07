namespace WireHush.UI.Services;

internal static class UiStartupLog
{
    private static readonly object Gate = new();
    // Only fixed event categories are accepted. Caller text, names, exception
    // messages, configurations and endpoints can never enter the diagnostic log.
    internal static void Write(string category)
    {
        if (category is not ("ui-startup-failed" or "ui-unhandled-error" or "manager-connected" or "manager-unavailable" or "session-exit-complete" or "session-exit-failed")) return;
        try
        {
            lock (Gate)
            {
                var directory = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "WireHush", "Logs");
                Directory.CreateDirectory(directory);
                var path = Path.Combine(directory, "ui.log");
                if (File.Exists(path) && new FileInfo(path).Length > 1 << 20) File.Move(path, path + ".previous", true);
                File.AppendAllText(path, $"{DateTimeOffset.UtcNow:O} {category}{Environment.NewLine}");
            }
        }
        catch { /* Diagnostics failure does not change network or exit state. */ }
    }
}
