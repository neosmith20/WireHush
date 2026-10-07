using System.Runtime.InteropServices;

namespace WireHush.UI.Services;

internal static class WireHushService
{
    internal const string ServiceName = "WireHushManager";
    private const uint ScManagerConnect = 0x0001;
    private const uint ServiceQueryStatus = 0x0004;
    private const uint ServiceStart = 0x0010;

    private const uint ServiceRunning = 0x00000004;

    private const uint ScStatusProcessInfo = 0;

    internal static Task EnsureRunningAsync(CancellationToken cancellationToken) => Task.Run(() =>
    {
        using var scm = OpenManager();
        using var service = OpenServiceHandle(scm, ServiceQueryStatus | ServiceStart);
        var status = Query(service);
        if (status.CurrentState == ServiceRunning) return;
        if (!StartService(service, 0, IntPtr.Zero) && Marshal.GetLastWin32Error() != 1056)
            throw new InvalidOperationException("WireHush Manager could not be started.");
        var deadline = DateTimeOffset.UtcNow.AddSeconds(10);
        do
        {
            cancellationToken.ThrowIfCancellationRequested();
            status = Query(service);
            if (status.CurrentState == ServiceRunning) return;
            Thread.Sleep(200);
        } while (DateTimeOffset.UtcNow < deadline);
        throw new TimeoutException("WireHush Manager did not start within 10 seconds.");
    }, cancellationToken);

    internal static uint CurrentProcessId()
    {
        using var scm = OpenManager();
        using var service = OpenServiceHandle(scm, ServiceQueryStatus);
        var status = Query(service);
        return status.CurrentState == ServiceRunning ? status.ProcessId : 0;
    }

    private static SafeServiceHandle OpenManager()
    {
        var handle = OpenSCManager(null, null, ScManagerConnect);
        if (handle.IsInvalid) throw new InvalidOperationException("WireHush Manager is not installed.");
        return handle;
    }

    private static SafeServiceHandle OpenServiceHandle(SafeServiceHandle manager, uint access)
    {
        var handle = OpenService(manager, ServiceName, access);
        if (handle.IsInvalid)
        {
            var error = Marshal.GetLastWin32Error();
            if (error == 1060) throw new InvalidOperationException("WireHush Manager is not installed.");
            if (error == 5) throw new InvalidOperationException("Your Windows account needs WireHush Users access. Sign out and back in after an administrator grants access.");
            throw new InvalidOperationException("WireHush Manager access was denied or unavailable.");
        }
        return handle;
    }

    private static ServiceStatusProcess Query(SafeServiceHandle service)
    {
        var size = Marshal.SizeOf<ServiceStatusProcess>();
        var memory = Marshal.AllocHGlobal(size);
        try
        {
            if (!QueryServiceStatusEx(service, ScStatusProcessInfo, memory, (uint)size, out _))
                throw new InvalidOperationException("WireHush Manager status could not be queried.");
            return Marshal.PtrToStructure<ServiceStatusProcess>(memory);
        }
        finally { Marshal.FreeHGlobal(memory); }
    }

    [StructLayout(LayoutKind.Sequential)]
    private struct ServiceStatusProcess
    {
        internal uint ServiceType, CurrentState, ControlsAccepted, Win32ExitCode, ServiceSpecificExitCode, CheckPoint, WaitHint, ProcessId, ServiceFlags;
    }

    private sealed class SafeServiceHandle : SafeHandle
    {
        internal SafeServiceHandle() : base(IntPtr.Zero, true) { }
        public override bool IsInvalid => handle == IntPtr.Zero;
        protected override bool ReleaseHandle() => CloseServiceHandle(handle);
    }

    [DllImport("advapi32.dll", SetLastError = true, CharSet = CharSet.Unicode)] private static extern SafeServiceHandle OpenSCManager(string? machine, string? database, uint access);
    [DllImport("advapi32.dll", SetLastError = true, CharSet = CharSet.Unicode)] private static extern SafeServiceHandle OpenService(SafeServiceHandle manager, string name, uint access);
    [DllImport("advapi32.dll", SetLastError = true)] private static extern bool QueryServiceStatusEx(SafeServiceHandle service, uint infoLevel, IntPtr buffer, uint size, out uint needed);
    [DllImport("advapi32.dll", SetLastError = true)] private static extern bool StartService(SafeServiceHandle service, uint argc, IntPtr argv);
    [DllImport("advapi32.dll", SetLastError = true)] private static extern bool ControlService(SafeServiceHandle service, uint control, out ServiceStatusProcess status);
    [DllImport("advapi32.dll", SetLastError = true)] private static extern bool CloseServiceHandle(IntPtr handle);
}
