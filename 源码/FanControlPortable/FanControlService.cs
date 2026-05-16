using System.Net.Http;
using System.Text;
using System.Text.Json;
using System.Text.Json.Serialization;

namespace FanControlPortable;

public sealed class FanControlService : IDisposable
{
    private readonly AppSettings _cfg;
    private readonly HttpClient _http;
    private readonly object _sync = new();
    private int? _lastSentSpeed;
    private DateTime _lastSentAtUtc = DateTime.MinValue;
    private string _lastMode = "";
    private int? _lastManualSpeed;
    private float? _smoothedTemperature;

    public FanControlStatus Status { get; private set; } = new();

    public FanControlService(AppSettings cfg)
    {
        _cfg = cfg;
        _http = new HttpClient(new HttpClientHandler { UseProxy = false }) { Timeout = TimeSpan.FromSeconds(2) };
    }

    public Task UpdateAsync(float? cpuTemp, float? gpuTemp, CancellationToken cancellationToken = default)
    {
        return Task.Run(() =>
        {
            cancellationToken.ThrowIfCancellationRequested();
            lock (_sync) UpdateInternal(cpuTemp, gpuTemp, false);
        }, cancellationToken);
    }

    public Task<FanControlStatus> ForceApplyAsync(float? cpuTemp, float? gpuTemp, CancellationToken cancellationToken = default)
    {
        return Task.Run(() =>
        {
            cancellationToken.ThrowIfCancellationRequested();
            lock (_sync)
            {
                UpdateInternal(cpuTemp, gpuTemp, true);
                return Status;
            }
        }, cancellationToken);
    }

    public void Update(float? cpuTemp, float? gpuTemp)
    {
        lock (_sync) UpdateInternal(cpuTemp, gpuTemp, false);
    }

    public FanControlStatus ForceApply(float? cpuTemp, float? gpuTemp)
    {
        lock (_sync)
        {
            UpdateInternal(cpuTemp, gpuTemp, true);
            return Status;
        }
    }

    public FanControlStatus TestConnection()
    {
        lock (_sync)
        {
            try
            {
                var cooler = ReadCoolerState();
                Status = BuildStatus(cooler, NormalizeMode(_cfg.FanControlMode), Status.TargetSpeed, Status.CpuTemperature, Status.GpuTemperature, Status.ControlTemperature, false, "设备已连接");
            }
            catch (Exception ex)
            {
                Status = Status with { Enabled = _cfg.FanControlEnabled, Mode = NormalizeMode(_cfg.FanControlMode), Online = false, Message = ex.Message };
            }
            return Status;
        }
    }

    private void UpdateInternal(float? cpuTemp, float? gpuTemp, bool forceSend)
    {
        var mode = NormalizeMode(_cfg.FanControlMode);
        cpuTemp = SanitizeTemperature(cpuTemp);
        gpuTemp = SanitizeTemperature(gpuTemp);

        if (!_cfg.FanControlEnabled)
        {
            Status = Status with { Enabled = false, Mode = mode, CpuTemperature = cpuTemp, GpuTemperature = gpuTemp, TargetSpeed = 0, Message = "风扇控制未启用" };
            return;
        }

        var manualSpeed = Clamp(_cfg.FanControlManualSpeed);
        if (!string.Equals(_lastMode, mode, StringComparison.OrdinalIgnoreCase) || _lastManualSpeed != manualSpeed)
        {
            _lastSentSpeed = null;
            _lastMode = mode;
            _lastManualSpeed = manualSpeed;
        }

        float? controlTemp = SelectControlTemperature(cpuTemp, gpuTemp);
        if (controlTemp.HasValue)
        {
            var alpha = PlatformCompat.Clamp(_cfg.FanControlSmoothingAlpha, 0.05, 1.0);
            _smoothedTemperature = _smoothedTemperature.HasValue
                ? (float)(alpha * controlTemp.Value + (1.0 - alpha) * _smoothedTemperature.Value)
                : controlTemp.Value;
        }

        var hasAutoTemperature = _smoothedTemperature.HasValue;
        var target = mode switch
        {
            "off" => 0,
            "manual" => manualSpeed,
            "auto" when _smoothedTemperature.HasValue => Math.Max(Clamp(_cfg.FanControlMinimumAutoSpeed), SpeedFromCurve(_smoothedTemperature.Value, ParseCurve(_cfg.FanControlCurve))),
            _ => 0
        };

        CoolerState cooler;
        try
        {
            cooler = ReadCoolerState();
        }
        catch (Exception ex)
        {
            Status = new FanControlStatus
            {
                Enabled = true,
                Online = false,
                Mode = mode,
                TargetSpeed = target,
                CpuTemperature = cpuTemp,
                GpuTemperature = gpuTemp,
                ControlTemperature = controlTemp,
                SmoothedTemperature = _smoothedTemperature,
                Message = "散热器离线或地址不可达：" + ex.Message
            };
            return;
        }

        var sent = false;
        var sendMessage = "";
        var canSend = mode switch
        {
            "auto" => hasAutoTemperature,
            "manual" or "off" => true,
            _ => false
        };

        if (canSend && cooler.Online && (forceSend || ShouldSend(target, mode)))
        {
            try
            {
                var result = SetSpeed(target);
                sent = result.Success;
                if (result.Success)
                {
                    _lastSentSpeed = target;
                    _lastSentAtUtc = DateTime.UtcNow;
                    sendMessage = "转速命令已下发";
                    cooler = TryReadCoolerState() ?? cooler;
                }
                else sendMessage = "设备拒绝转速命令：" + result.Message;
            }
            catch (Exception ex)
            {
                Status = BuildStatus(cooler, mode, target, cpuTemp, gpuTemp, controlTemp, sent, "转速下发失败：" + ex.Message);
                return;
            }
        }

        var message = mode == "auto" && !hasAutoTemperature ? "等待温度数据" : sent ? sendMessage : "状态正常";
        Status = BuildStatus(cooler, mode, target, cpuTemp, gpuTemp, controlTemp, sent, message);
    }

    private CoolerState ReadCoolerState()
    {
        var baseUrl = GetBaseUrl();
        var body = _http.GetStringAsync($"{baseUrl}/api/data").GetAwaiter().GetResult();
        var data = JsonSerializer.Deserialize<CoolerData>(body)
            ?? throw new InvalidOperationException("empty response");
        return new CoolerState(true, data.Speed, data.Temperature, data.Power, data.WifiControl, data.WifiTargetSpeed, data.ControlMode, null);
    }

    private CoolerState? TryReadCoolerState()
    {
        try { return ReadCoolerState(); } catch { return null; }
    }

    private FanCommandResult SetSpeed(int speed)
    {
        speed = Clamp(speed);
        var baseUrl = GetBaseUrl();
        var json = JsonSerializer.Serialize(new { speed });
        using var content = new StringContent(json, Encoding.UTF8, "application/json");
        using var response = _http.PostAsync($"{baseUrl}/api/speed", content).GetAwaiter().GetResult();
        var body = response.Content.ReadAsStringAsync().GetAwaiter().GetResult();
        if (!response.IsSuccessStatusCode)
            return new FanCommandResult(false, (int)response.StatusCode, body, $"HTTP {(int)response.StatusCode}: {response.ReasonPhrase}");
        SetSpeedResponse? data = null;
        try { data = JsonSerializer.Deserialize<SetSpeedResponse>(body); } catch { }
        var ok = string.Equals(data?.Status, "success", StringComparison.OrdinalIgnoreCase) ||
                 string.Equals(data?.ControlMode, "wifi", StringComparison.OrdinalIgnoreCase);
        return new FanCommandResult(ok, (int)response.StatusCode, body, ok ? "下发成功" : "设备未返回 success");
    }

    private string GetBaseUrl()
    {
        if (TryGetBaseUrl(out var baseUrl, out var error))
            return baseUrl;

        throw new InvalidOperationException(error);
    }

    private bool TryGetBaseUrl(out string baseUrl, out string error)
    {
        var raw = (_cfg.FanControlDeviceIp ?? "").Trim();
        if (string.IsNullOrWhiteSpace(raw))
        {
            baseUrl = "";
            error = "请先填写散热器 IP 或 IP:端口";
            return false;
        }

        if (!raw.StartsWith("http://", StringComparison.OrdinalIgnoreCase) &&
            !raw.StartsWith("https://", StringComparison.OrdinalIgnoreCase))
        {
            raw = "http://" + raw;
        }

        raw = raw.TrimEnd('/');
        if (!Uri.TryCreate(raw, UriKind.Absolute, out var uri) || string.IsNullOrWhiteSpace(uri.Host))
        {
            baseUrl = "";
            error = "散热器地址格式不正确，请填写 IP 或 IP:端口";
            return false;
        }

        baseUrl = raw;
        error = "";
        return true;
    }

    private FanControlStatus BuildStatus(CoolerState cooler, string mode, int target, float? cpuTemp, float? gpuTemp, float? controlTemp, bool sent, string message) => new()
    {
        Enabled = _cfg.FanControlEnabled,
        Online = cooler.Online,
        Mode = mode,
        CurrentSpeed = cooler.Speed,
        DeviceTemperature = cooler.Temperature,
        DeviceWifiControl = cooler.WifiControl,
        DeviceTargetSpeed = cooler.WifiTargetSpeed,
        DeviceControlMode = cooler.ControlMode ?? "",
        TargetSpeed = target,
        CpuTemperature = cpuTemp,
        GpuTemperature = gpuTemp,
        ControlTemperature = controlTemp,
        SmoothedTemperature = _smoothedTemperature,
        Sent = sent,
        Message = message
    };

    private bool ShouldSend(int target, string mode)
    {
        if (mode is "manual" or "off")
            return !_lastSentSpeed.HasValue || target != _lastSentSpeed.Value;

        if (mode != "auto") return false;
        if (!_lastSentSpeed.HasValue) return true;
        if (Math.Abs(target - _lastSentSpeed.Value) < Math.Max(1, _cfg.FanControlMinSpeedDelta))
            return false;

        var minInterval = Math.Max(0, _cfg.FanControlMinSendIntervalSeconds);
        return minInterval == 0 || DateTime.UtcNow - _lastSentAtUtc >= TimeSpan.FromSeconds(minInterval);
    }

    private float? SelectControlTemperature(float? cpuTemp, float? gpuTemp)
    {
        return (_cfg.FanControlTemperatureSource ?? "cpu").ToLowerInvariant() switch
        {
            "gpu" => gpuTemp ?? cpuTemp,
            "max" => cpuTemp.HasValue && gpuTemp.HasValue ? Math.Max(cpuTemp.Value, gpuTemp.Value) : cpuTemp ?? gpuTemp,
            _ => cpuTemp ?? gpuTemp
        };
    }

    private static float? SanitizeTemperature(float? value) => value is > -20 and < 130 ? value : null;
    public static string NormalizeMode(string? mode) => (mode ?? "monitor").ToLowerInvariant() is var m && (m is "auto" or "manual" or "off") ? m : "monitor";
    public static int Clamp(int speed) => PlatformCompat.Clamp(speed, 0, 100);

    public static List<(int Temp, int Speed)> ParseCurve(string? text)
    {
        var result = ParseCurveCore(text);
        if (result.Count >= 2)
            return result;

        return ParseCurveCore(AppSettings.DefaultCurve);
    }

    private static List<(int Temp, int Speed)> ParseCurveCore(string? text)
    {
        var result = new List<(int Temp, int Speed)>();
        foreach (var rawValue in (text ?? AppSettings.DefaultCurve).Split(new[] { ',' }, StringSplitOptions.RemoveEmptyEntries))
        {
            var raw = rawValue.Trim();
            var parts = raw.Split(new[] { ':' }, StringSplitOptions.None);
            if (parts.Length != 2 || !int.TryParse(parts[0], out var temp) || !int.TryParse(parts[1], out var speed))
                continue;
            result.Add((PlatformCompat.Clamp(temp, 0, 120), Clamp(speed)));
        }
        return result.GroupBy(p => p.Temp).Select(g => g.Last()).OrderBy(p => p.Temp).ToList();
    }

    public static bool TryNormalizeCurve(string? text, out string normalized, out string error)
    {
        var points = ParseCurveCore(text);
        if (points.Count < 2)
        {
            normalized = AppSettings.DefaultCurve;
            error = "至少需要两个不同温度的控制点";
            return false;
        }
        normalized = string.Join(",", points.Select(p => $"{p.Temp}:{p.Speed}"));
        error = "";
        return true;
    }

    public static int SpeedFromCurve(float temp, List<(int Temp, int Speed)> points)
    {
        points = points.OrderBy(p => p.Temp).ToList();
        if (temp <= points[0].Temp) return points[0].Speed;
        if (temp >= points[points.Count - 1].Temp) return points[points.Count - 1].Speed;
        for (var i = 0; i < points.Count - 1; i++)
        {
            var a = points[i];
            var b = points[i + 1];
            if (temp < a.Temp || temp > b.Temp) continue;
            var ratio = (temp - a.Temp) / Math.Max(1.0, b.Temp - a.Temp);
            return Clamp((int)Math.Round(a.Speed + ratio * (b.Speed - a.Speed)));
        }
        return points[points.Count - 1].Speed;
    }

    public void Dispose() => _http.Dispose();
}

public record FanControlStatus
{
    public bool Enabled { get; init; }
    public bool Online { get; init; }
    public string Mode { get; init; } = "monitor";
    public int? CurrentSpeed { get; init; }
    public float? DeviceTemperature { get; init; }
    public bool? DeviceWifiControl { get; init; }
    public int? DeviceTargetSpeed { get; init; }
    public string DeviceControlMode { get; init; } = "";
    public int TargetSpeed { get; init; }
    public float? CpuTemperature { get; init; }
    public float? GpuTemperature { get; init; }
    public float? ControlTemperature { get; init; }
    public float? SmoothedTemperature { get; init; }
    public bool Sent { get; init; }
    public string Message { get; init; } = "";
}

public record CoolerState(bool Online, int? Speed, float? Temperature, bool? Power, bool? WifiControl, int? WifiTargetSpeed, string? ControlMode, string? Error);
internal record FanCommandResult(bool Success, int StatusCode, string Body, string Message);
internal sealed class CoolerData
{
    [JsonPropertyName("speed")] public int? Speed { get; set; }
    [JsonPropertyName("temperature")] public float? Temperature { get; set; }
    [JsonPropertyName("power")] public bool? Power { get; set; }
    [JsonPropertyName("wifiControl")] public bool? WifiControl { get; set; }
    [JsonPropertyName("wifiTargetSpeed")] public int? WifiTargetSpeed { get; set; }
    [JsonPropertyName("controlMode")] public string? ControlMode { get; set; }
}
internal sealed class SetSpeedResponse
{
    [JsonPropertyName("status")] public string? Status { get; set; }
    [JsonPropertyName("controlMode")] public string? ControlMode { get; set; }
}
