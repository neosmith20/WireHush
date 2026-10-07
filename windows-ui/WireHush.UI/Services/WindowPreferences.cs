using System.Runtime.InteropServices;
using System.Text.Json;
using Microsoft.UI.Windowing;
using Windows.Graphics;

namespace WireHush.UI.Services;

internal sealed class WindowPreferences : IDisposable
{
    private readonly IntPtr _window;
    private readonly SubclassProcedure _procedure;
    private static string PathName => Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "WireHush", "window.json");
    private sealed record SizePreference(int Width, int Height);

    internal WindowPreferences(IntPtr window, AppWindow appWindow)
    {
        _window = window;
        _procedure = Procedure;
        if (!SetWindowSubclass(window, _procedure, 1, UIntPtr.Zero)) throw new InvalidOperationException("Window sizing could not be initialized.");
        var width = 1280; var height = 800;
        try
        {
            var info = new FileInfo(PathName);
            if (info.Exists && info.Length < 1024 && (info.Attributes & FileAttributes.ReparsePoint) == 0)
            {
                var stored = JsonSerializer.Deserialize<SizePreference>(File.ReadAllText(PathName));
                if (stored is not null) { width = Math.Clamp(stored.Width, 1100, 3840); height = Math.Clamp(stored.Height, 680, 2160); }
            }
        }
        catch { }
        var scale = GetDpiForWindow(window) / 96d;
        var area = DisplayArea.GetFromWindowId(appWindow.Id, DisplayAreaFallback.Nearest).WorkArea;
        appWindow.Resize(new SizeInt32(Math.Min((int)(width * scale), area.Width), Math.Min((int)(height * scale), area.Height)));
        appWindow.Changed += (_, args) =>
        {
            if (!args.DidSizeChange || appWindow.Presenter is not OverlappedPresenter { State: OverlappedPresenterState.Restored }) return;
            try
            {
                var currentScale = GetDpiForWindow(window) / 96d;
                var size = appWindow.Size;
                if (size.Width < 1100 * currentScale || size.Height < 680 * currentScale) return;
                Directory.CreateDirectory(Path.GetDirectoryName(PathName)!);
                File.WriteAllText(PathName, JsonSerializer.Serialize(new SizePreference((int)(size.Width / currentScale), (int)(size.Height / currentScale))));
            }
            catch { }
        };
    }

    private IntPtr Procedure(IntPtr window, uint message, UIntPtr wParam, IntPtr lParam, UIntPtr id, UIntPtr data)
    {
        if (message == 0x24) // WM_GETMINMAXINFO, in physical pixels at current DPI.
        {
            var info = Marshal.PtrToStructure<MinMaxInfo>(lParam);
            var scale = GetDpiForWindow(window) / 96d;
            info.MinimumTrackSize = new PointInt32((int)(1100 * scale), (int)(680 * scale));
            Marshal.StructureToPtr(info, lParam, false);
            return IntPtr.Zero;
        }
        return DefSubclassProc(window, message, wParam, lParam);
    }
    public void Dispose() => RemoveWindowSubclass(_window, _procedure, 1);
    [StructLayout(LayoutKind.Sequential)] private struct MinMaxInfo { internal PointInt32 Reserved, MaximumSize, MaximumPosition, MinimumTrackSize, MaximumTrackSize; }
    private delegate IntPtr SubclassProcedure(IntPtr window, uint message, UIntPtr wParam, IntPtr lParam, UIntPtr id, UIntPtr data);
    [DllImport("comctl32.dll", SetLastError = true)] [return: MarshalAs(UnmanagedType.Bool)] private static extern bool SetWindowSubclass(IntPtr window, SubclassProcedure callback, nuint id, UIntPtr data);
    [DllImport("comctl32.dll")] [return: MarshalAs(UnmanagedType.Bool)] private static extern bool RemoveWindowSubclass(IntPtr window, SubclassProcedure callback, nuint id);
    [DllImport("comctl32.dll")] private static extern IntPtr DefSubclassProc(IntPtr window, uint message, UIntPtr wParam, IntPtr lParam);
    [DllImport("user32.dll")] private static extern uint GetDpiForWindow(IntPtr window);
}
