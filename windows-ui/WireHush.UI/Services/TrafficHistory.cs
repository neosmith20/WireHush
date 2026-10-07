using System.Diagnostics;

namespace WireHush.UI.Services;

internal sealed record TrafficPoint(long Tick, double Rx, double Tx);
internal sealed class TrafficHistory
{
    private readonly Queue<TrafficPoint> _samples = new();
    private long? _lastTick;
    private ulong _lastRx, _lastTx;
    internal IReadOnlyList<TrafficPoint> Samples => _samples.ToArray();
    internal TrafficPoint? Current => _samples.LastOrDefault();
    internal void Clear() { _samples.Clear(); _lastTick = null; _lastRx = _lastTx = 0; }
    internal void Add(ulong rx, ulong tx, long tick)
    {
        if (_lastTick is { } previous)
        {
            var elapsed = Stopwatch.GetElapsedTime(previous, tick);
            if (tick <= previous || rx < _lastRx || tx < _lastTx || elapsed > TimeSpan.FromMinutes(5)) Clear();
            else
            {
                if (elapsed.TotalSeconds < .25) return;
                _samples.Enqueue(new TrafficPoint(tick, (rx - _lastRx) * 8d / elapsed.TotalSeconds, (tx - _lastTx) * 8d / elapsed.TotalSeconds));
            }
        }
        _lastTick = tick; _lastRx = rx; _lastTx = tx;
        while (_samples.Count > 600 || (_samples.TryPeek(out var oldest) && Stopwatch.GetElapsedTime(oldest.Tick, tick) > TimeSpan.FromMinutes(5))) _samples.Dequeue();
    }
}
