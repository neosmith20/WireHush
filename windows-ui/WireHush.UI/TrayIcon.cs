using System;
using System.Runtime.InteropServices;
using Microsoft.UI.Dispatching;

namespace WireHush.UI;

/// <summary>
/// Windows notification-area integration using the existing WinUI HWND.
/// No WPF or Windows Forms dependency is introduced into the WinUI app.
/// </summary>
internal sealed class TrayIcon : IDisposable
{
    private const uint NotifyAdd = 0;
    private const uint NotifyDelete = 2;
    private const uint NotifySetVersion = 4;
    private const uint NotifyMessage = 1;
    private const uint NotifyIconFlag = 2;
    private const uint NotifyTip = 4;
    private const uint ImageIcon = 1;
    private const uint LoadFromFile = 0x10;
    private const uint TrayCallback = 0x8000 + 0x251;
    private const uint MouseLeftUp = 0x0202;
    private const uint MouseLeftDoubleClick = 0x0203;
    private const uint MouseRightUp = 0x0205;
    private const uint ContextMenu = 0x007B;
    private const uint MenuSeparator = 0x0800;
    private const uint MenuCommand = 0x0100;
    private const uint TrackReturnCommand = 0x0100;
    private const uint TrackRightButton = 0x0002;
    private const uint WindowNull = 0;
    private readonly IntPtr _hwnd;
    private readonly IntPtr _icon;
    private readonly DispatcherQueue _dispatcher;
    private readonly Action _show;
    private readonly Action _hide;
    private readonly Action _exit;
    private readonly SubclassProcedure _subclass;
    private readonly uint _taskbarCreated;
    private bool _disposed;

    private delegate IntPtr SubclassProcedure(
        IntPtr hwnd, uint message, UIntPtr wParam, IntPtr lParam,
        UIntPtr subclassId, UIntPtr refData);

    [StructLayout(LayoutKind.Sequential, CharSet = CharSet.Unicode)]
    private struct NotifyData
    {
        public uint Size;
        public IntPtr Hwnd;
        public uint Id;
        public uint Flags;
        public uint CallbackMessage;
        public IntPtr Icon;
        [MarshalAs(UnmanagedType.ByValTStr, SizeConst = 128)]
        public string Tip;
        public uint State;
        public uint StateMask;
        [MarshalAs(UnmanagedType.ByValTStr, SizeConst = 256)]
        public string Info;
        public uint Version;
        [MarshalAs(UnmanagedType.ByValTStr, SizeConst = 64)]
        public string InfoTitle;
        public uint InfoFlags;
        public Guid GuidItem;
        public IntPtr BalloonIcon;
    }

    [StructLayout(LayoutKind.Sequential)]
    private struct ScreenPoint
    {
        public int X;
        public int Y;
    }

    [DllImport("shell32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool Shell_NotifyIconW(uint message, ref NotifyData data);

    [DllImport("user32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
    private static extern IntPtr LoadImageW(IntPtr instance, string name, uint type, int width, int height, uint flags);

    [DllImport("user32.dll")]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool DestroyIcon(IntPtr icon);

    [DllImport("user32.dll", CharSet = CharSet.Unicode)]
    private static extern uint RegisterWindowMessageW(string message);

    [DllImport("comctl32.dll", SetLastError = true)]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool SetWindowSubclass(IntPtr hwnd, SubclassProcedure callback, UIntPtr id, UIntPtr data);

    [DllImport("comctl32.dll")]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool RemoveWindowSubclass(IntPtr hwnd, SubclassProcedure callback, UIntPtr id);

    [DllImport("comctl32.dll")]
    private static extern IntPtr DefSubclassProc(IntPtr hwnd, uint message, UIntPtr wParam, IntPtr lParam);

    [DllImport("user32.dll")]
    private static extern IntPtr CreatePopupMenu();

    [DllImport("user32.dll")]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool DestroyMenu(IntPtr menu);

    [DllImport("user32.dll", CharSet = CharSet.Unicode)]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool AppendMenuW(IntPtr menu, uint flags, nuint id, string? label);

    [DllImport("user32.dll")]
    private static extern uint TrackPopupMenu(IntPtr menu, uint flags, int x, int y, int reserved, IntPtr hwnd, IntPtr rect);

    [DllImport("user32.dll")]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool SetForegroundWindow(IntPtr hwnd);

    [DllImport("user32.dll")]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool GetCursorPos(out ScreenPoint point);

    [DllImport("user32.dll")]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool PostMessageW(IntPtr hwnd, uint message, UIntPtr wParam, IntPtr lParam);

    public TrayIcon(IntPtr hwnd, string iconPath, DispatcherQueue dispatcher, Action show, Action hide, Action exit)
    {
        _hwnd = hwnd;
        _dispatcher = dispatcher;
        _show = show;
        _hide = hide;
        _exit = exit;
        _icon = LoadImageW(IntPtr.Zero, iconPath, ImageIcon, 0, 0, LoadFromFile);
        if (_icon == IntPtr.Zero) throw new InvalidOperationException("The WireHush notification-area icon could not be loaded.");
        _subclass = WindowMessage;
        _taskbarCreated = RegisterWindowMessageW("TaskbarCreated");
        if (!SetWindowSubclass(_hwnd, _subclass, new UIntPtr(1), UIntPtr.Zero))
        {
            DestroyIcon(_icon);
            throw new InvalidOperationException("The WireHush notification-area handler could not be created.");
        }
        if (!InstallIcon())
        {
            RemoveWindowSubclass(_hwnd, _subclass, new UIntPtr(1));
            DestroyIcon(_icon);
            throw new InvalidOperationException("The WireHush notification-area icon could not be registered.");
        }
    }

    private NotifyData Data() => new()
    {
        Size = (uint)Marshal.SizeOf<NotifyData>(),
        Hwnd = _hwnd,
        Id = 1,
        Flags = NotifyMessage | NotifyIconFlag | NotifyTip,
        CallbackMessage = TrayCallback,
        Icon = _icon,
        Tip = "WireHush",
        Info = "",
        InfoTitle = ""
    };

    private bool InstallIcon()
    {
        var data = Data();
        if (!Shell_NotifyIconW(NotifyAdd, ref data)) return false;
        data.Version = 4;
        Shell_NotifyIconW(NotifySetVersion, ref data);
        return true;
    }

    private IntPtr WindowMessage(IntPtr hwnd, uint message, UIntPtr wParam, IntPtr lParam, UIntPtr id, UIntPtr data)
    {
        if (!_disposed && message == _taskbarCreated)
        {
            InstallIcon(); // Explorer restarted; restore the icon.
        }
        if (!_disposed && message == TrayCallback)
        {
            var notification = unchecked((uint)lParam.ToInt64()) & 0xffff;
            if (notification == MouseLeftUp || notification == MouseLeftDoubleClick)
                _dispatcher.TryEnqueue(() => _show());
            else if (notification == MouseRightUp || notification == ContextMenu)
                ShowContextMenu();
            return IntPtr.Zero;
        }
        return DefSubclassProc(hwnd, message, wParam, lParam);
    }

    private void ShowContextMenu()
    {
        var menu = CreatePopupMenu();
        if (menu == IntPtr.Zero) return;
        try
        {
            AppendMenuW(menu, 0, 1, "Show WireHush");
            AppendMenuW(menu, 0, 2, "Hide WireHush");
            AppendMenuW(menu, MenuSeparator, 0, null);
            AppendMenuW(menu, 0, 3, "Exit WireHush...");
            if (!GetCursorPos(out var pos)) return;
            SetForegroundWindow(_hwnd);
            var result = TrackPopupMenu(menu, TrackReturnCommand | TrackRightButton, pos.X, pos.Y, 0, _hwnd, IntPtr.Zero);
            PostMessageW(_hwnd, WindowNull, UIntPtr.Zero, IntPtr.Zero);
            if (result == 1) _dispatcher.TryEnqueue(() => _show());
            else if (result == 2) _dispatcher.TryEnqueue(() => _hide());
            else if (result == 3) _dispatcher.TryEnqueue(() => _exit());
        }
        finally { DestroyMenu(menu); }
    }

    public void Dispose()
    {
        if (_disposed) return;
        _disposed = true;
        var data = Data();
        Shell_NotifyIconW(NotifyDelete, ref data);
        RemoveWindowSubclass(_hwnd, _subclass, new UIntPtr(1));
        DestroyIcon(_icon);
    }
}
