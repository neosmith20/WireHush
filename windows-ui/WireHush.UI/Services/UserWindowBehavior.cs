using System.Text.Json;

namespace WireHush.UI.Services;

/// <summary>Per-Windows-user UI behavior, independent of privileged Manager settings.</summary>
internal static class UserWindowBehavior
{
    internal enum CloseAction { MinimizeToTray, Exit }

    private sealed record Preferences(string CloseButton);
    private static string FileName => Path.Combine(
        Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
        "WireHush", "ui-preferences.json");

    internal static CloseAction Load()
    {
        try
        {
            var file = new FileInfo(FileName);
            if (!file.Exists || file.Length > 512 || (file.Attributes & FileAttributes.ReparsePoint) != 0)
                return CloseAction.MinimizeToTray;
            var stored = JsonSerializer.Deserialize<Preferences>(File.ReadAllText(FileName));
            return stored?.CloseButton == "exit" ? CloseAction.Exit : CloseAction.MinimizeToTray;
        }
        catch { return CloseAction.MinimizeToTray; }
    }

    internal static void Save(CloseAction action)
    {
        var directory = Path.GetDirectoryName(FileName)!;
        Directory.CreateDirectory(directory);
        // Never follow a symlink or junction when replacing a settings file.
        if (File.Exists(FileName) && (File.GetAttributes(FileName) & FileAttributes.ReparsePoint) != 0)
            throw new IOException("The window preferences file cannot be a symbolic link.");
        var staged = Path.Combine(directory, "ui-preferences-" + Guid.NewGuid().ToString("N") + ".tmp");
        try
        {
            File.WriteAllText(staged, JsonSerializer.Serialize(
                new Preferences(action == CloseAction.Exit ? "exit" : "tray")));
            File.Move(staged, FileName, true);
        }
        finally
        {
            if (File.Exists(staged)) File.Delete(staged);
        }
    }
}
