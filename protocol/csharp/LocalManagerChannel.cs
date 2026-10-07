// SPDX-License-Identifier: MIT
// Copyright (C) 2026 WireHush. All Rights Reserved.
using System;
using System.ComponentModel;
using System.IO;
using System.IO.Pipes;
using System.Net.Http;
using System.Runtime.InteropServices;
using System.Runtime.Versioning;
using System.Security.Principal;
using System.Threading;
using System.Threading.Tasks;
using Grpc.Net.Client;
using Microsoft.Win32.SafeHandles;

namespace WireHush.Protocol;

/// <summary>Local HTTP/2 pipe transport; this channel never opens TCP.</summary>
[SupportedOSPlatform("windows")]
public static class LocalManagerChannel
{
    public const string PipePath = @"\\.\pipe\WireHush.Manager.v1";
    private const uint ClientAccess = 0x12019b; // excludes CREATE_PIPE_INSTANCE
    private const uint OpenExisting = 3;
    private const uint PipeFlags = 0x40110000; // OVERLAPPED | SQOS | IDENTIFICATION

    // The callback queries SCM for the current WireHushManager PID on every
    // connection. A pipe name or owner alone is not the complete server identity.
    public static GrpcChannel Create(Func<uint> currentManagerProcessId)
    {
        ArgumentNullException.ThrowIfNull(currentManagerProcessId);
        var handler = new SocketsHttpHandler
        {
            ConnectCallback = (_, cancellation) => ConnectAsync(currentManagerProcessId, cancellation),
            UseProxy = false,
            EnableMultipleHttp2Connections = false
        };
        return GrpcChannel.ForAddress("http://wirehush.local", new GrpcChannelOptions
        {
            HttpHandler = handler,
            DisposeHttpClient = true,
            MaxReceiveMessageSize = 1 << 20,
            MaxSendMessageSize = 1 << 20
        });
    }

    private static async ValueTask<Stream> ConnectAsync(Func<uint> currentManagerProcessId, CancellationToken cancellation)
    {
        using var deadline = CancellationTokenSource.CreateLinkedTokenSource(cancellation);
        deadline.CancelAfter(TimeSpan.FromSeconds(5));
        while (true)
        {
            deadline.Token.ThrowIfCancellationRequested();
            var handle = CreateFileW(PipePath, ClientAccess, 0, IntPtr.Zero, OpenExisting, PipeFlags, IntPtr.Zero);
            if (!handle.IsInvalid)
            {
                try
                {
                    VerifyServer(handle, currentManagerProcessId);
                    return new NamedPipeClientStream(PipeDirection.InOut, true, true, handle);
                }
                catch { handle.Dispose(); throw; }
            }
            var error = Marshal.GetLastWin32Error();
            handle.Dispose();
            if (error is not (2 or 231)) throw new IOException("WireHush Manager pipe is unavailable or access is denied", new Win32Exception(error));
            await Task.Delay(50, deadline.Token).ConfigureAwait(false);
        }
    }

    private static void VerifyServer(SafePipeHandle pipe, Func<uint> currentManagerProcessId)
    {
        if (!GetNamedPipeServerProcessId(pipe, out var serverId)) throw new IOException("Cannot verify WireHush Manager identity");
        var expectedId = currentManagerProcessId();
        if (expectedId == 0 || serverId != expectedId) throw new IOException("WireHush Manager service identity does not match the pipe");
        var error = GetSecurityInfo(pipe, 1, 1, out var owner, out _, out _, out _, out var descriptor);
        if (error != 0) throw new IOException("Cannot verify WireHush Manager pipe protection");
        try
        {
            if (owner == IntPtr.Zero || new SecurityIdentifier(owner).Value != "S-1-5-18")
                throw new IOException("WireHush Manager pipe is not owned by SYSTEM");
        }
        finally { if (descriptor != IntPtr.Zero) LocalFree(descriptor); }
    }

    [DllImport("kernel32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
    private static extern SafePipeHandle CreateFileW(string name, uint access, uint share, IntPtr security, uint disposition, uint flags, IntPtr template);
    [DllImport("kernel32.dll", SetLastError = true)]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool GetNamedPipeServerProcessId(SafePipeHandle pipe, out uint processId);
    [DllImport("advapi32.dll", SetLastError = true)]
    private static extern uint GetSecurityInfo(SafePipeHandle handle, uint type, uint information, out IntPtr owner, out IntPtr group, out IntPtr dacl, out IntPtr sacl, out IntPtr descriptor);
    [DllImport("kernel32.dll")]
    private static extern IntPtr LocalFree(IntPtr memory);
}
