using WireHush.UI.Models;

namespace WireHush.UI.Services;

public interface IManagerClient : IAsyncDisposable
{
    bool MayEditMachineSettings { get; }
    bool Connected { get; }
    bool DeviceBusyForAnotherUser { get; }
    event Action? SnapshotChanged;
    event Action? ConnectionLost;
    Task ConnectAsync(CancellationToken cancellationToken);
    Task<IReadOnlyList<TunnelSummary>> ListTunnelsAsync(CancellationToken cancellationToken);
    Task<TunnelDetails> GetTunnelAsync(string name, CancellationToken cancellationToken);
    Task<TunnelDetails> GetRuntimeAsync(string name, CancellationToken cancellationToken);
    Task StartTunnelAsync(string name, CancellationToken cancellationToken);
    Task StopTunnelAsync(string name, CancellationToken cancellationToken);
    Task DeleteTunnelAsync(string name, CancellationToken cancellationToken);
    Task<string> ImportTunnelAsync(string name, string content, CancellationToken cancellationToken, bool shared = false);
    Task<IReadOnlyList<BootstrapResolver>> GetBootstrapAsync(CancellationToken cancellationToken);
    Task SaveBootstrapAsync(IReadOnlyList<BootstrapResolver> resolvers, CancellationToken cancellationToken);
    Task<string> ExportAsync(string id, CancellationToken cancellationToken);
    Task UpdateAsync(string id, string name, string configuration, CancellationToken cancellationToken);
    Task<IReadOnlyList<string>> GetLogSnapshotAsync(CancellationToken cancellationToken);
    Task ShutdownAsync(CancellationToken cancellationToken);
}
