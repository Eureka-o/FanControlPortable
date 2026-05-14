namespace FanControlPortable;

public sealed class FanControlRuntime : IDisposable
{
    private readonly AppSettings _settings;
    private readonly HardwareReader? _hardware;
    private readonly FanControlService _service;
    private readonly CancellationTokenSource _cts = new();
    private readonly SemaphoreSlim _applyGate = new(1, 1);
    private Task? _loopTask;
    private HardwareSnapshot _hardwareSnapshot = HardwareSnapshot.Empty;
    private string? _hardwareError;
    private volatile bool _backgroundMode;
    private DateTime _lastWorkingSetTrimUtc = DateTime.MinValue;
    private int _workingSetTrimRunning;

    public event EventHandler<FanRuntimeSnapshot>? SnapshotReady;

    public FanControlRuntime(AppSettings settings)
    {
        _settings = settings;
        _service = new FanControlService(settings);
        try
        {
            _hardware = new HardwareReader(settings);
        }
        catch (Exception ex)
        {
            _hardwareError = ex.Message;
        }
    }

    public void Start()
    {
        _loopTask ??= Task.Run(LoopAsync);
    }

    public async Task ForceApplyAsync()
    {
        await _applyGate.WaitAsync(_cts.Token);
        try
        {
            var hardware = ReadHardware();
            var status = await _service.ForceApplyAsync(hardware.CpuTemperature, hardware.GpuTemperature, _cts.Token);
            Emit(hardware, status);
        }
        catch (OperationCanceledException) { }
        catch (Exception ex)
        {
            Emit(_hardwareSnapshot, _service.Status with { Message = ex.Message });
        }
        finally
        {
            _applyGate.Release();
        }
    }

    public FanControlStatus TestConnection()
    {
        var status = _service.TestConnection();
        Emit(_hardwareSnapshot, status);
        return status;
    }

    public HardwareSnapshot CurrentHardware => _hardwareSnapshot;
    public FanControlStatus CurrentStatus => _service.Status;
    public string? HardwareError => _hardwareError;

    public void SetBackgroundMode(bool backgroundMode)
    {
        _backgroundMode = backgroundMode;
    }

    private async Task LoopAsync()
    {
        while (!_cts.IsCancellationRequested)
        {
            try
            {
                var startedAt = DateTime.UtcNow;
                await TickAsync();
                MaybeTrimWorkingSetInBackground();

                var interval = _backgroundMode ? TimeSpan.FromSeconds(2) : TimeSpan.FromSeconds(1);
                var elapsed = DateTime.UtcNow - startedAt;
                var delay = interval - elapsed;
                if (delay < TimeSpan.FromMilliseconds(100))
                    delay = TimeSpan.FromMilliseconds(100);
                await Task.Delay(delay, _cts.Token);
            }
            catch (OperationCanceledException)
            {
                break;
            }
            catch (Exception ex)
            {
                Emit(_hardwareSnapshot, _service.Status with { Message = ex.Message });
                try { await Task.Delay(1000, _cts.Token); } catch { break; }
            }
        }
    }

    private async Task TickAsync()
    {
        if (!await _applyGate.WaitAsync(0, _cts.Token))
            return;

        try
        {
            var hardware = ReadHardware();
            await _service.UpdateAsync(hardware.CpuTemperature, hardware.GpuTemperature, _cts.Token);
            Emit(hardware, _service.Status);
        }
        finally
        {
            _applyGate.Release();
        }
    }

    private HardwareSnapshot ReadHardware()
    {
        if (_hardware == null)
            return HardwareSnapshot.Empty with
            {
                UpdatedAt = DateTime.Now,
                Diagnostic = _hardwareError ?? "硬件读取器初始化失败"
            };

        try
        {
            _hardwareError = null;
            _hardwareSnapshot = _hardware.Update(_backgroundMode);
        }
        catch (Exception ex)
        {
            _hardwareError = ex.Message;
            _hardwareSnapshot = HardwareSnapshot.Empty with
            {
                UpdatedAt = DateTime.Now,
                Diagnostic = ex.Message
            };
        }

        return _hardwareSnapshot;
    }

    private void MaybeTrimWorkingSetInBackground()
    {
        if (!_backgroundMode || !_settings.PerformanceTrimWorkingSetInBackground)
            return;

        if ((DateTime.UtcNow - _lastWorkingSetTrimUtc) < TimeSpan.FromMinutes(2))
            return;

        _lastWorkingSetTrimUtc = DateTime.UtcNow;
        if (Interlocked.Exchange(ref _workingSetTrimRunning, 1) == 1)
            return;

        _ = Task.Run(() =>
        {
            try
            {
                WorkingSetTrimmer.TrimCurrentProcess();
            }
            finally
            {
                Interlocked.Exchange(ref _workingSetTrimRunning, 0);
            }
        });
    }

    private void Emit(HardwareSnapshot hardware, FanControlStatus status)
    {
        SnapshotReady?.Invoke(this, new FanRuntimeSnapshot(hardware, status, _hardwareError));
    }

    public void Dispose()
    {
        _cts.Cancel();
        try { _loopTask?.Wait(1000); } catch { }
        _cts.Dispose();
        _applyGate.Dispose();
        _service.Dispose();
        _hardware?.Dispose();
    }
}

public sealed record FanRuntimeSnapshot(HardwareSnapshot Hardware, FanControlStatus Status, string? HardwareError);
