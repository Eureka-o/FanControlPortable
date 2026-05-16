using System.IO;
using System.Text.Json;

namespace FanControlPortable;

public sealed class AppSettings
{
    public const int CurrentSettingsSchemaVersion = 2;
    public const string DefaultCurve = "30:20,36:22,42:26,48:34,54:44,60:56,66:70,74:86,82:100";

    public int SettingsSchemaVersion { get; set; }
    public bool FanControlEnabled { get; set; } = true;
    public string FanControlDeviceIp { get; set; } = "";
    public string FanControlMode { get; set; } = "monitor";
    public string FanControlTemperatureSource { get; set; } = "max";
    public string FanControlCpuTemperatureSensorId { get; set; } = "auto";
    public string FanControlGpuTemperatureSensorId { get; set; } = "auto";
    public int FanControlManualSpeed { get; set; } = 45;
    public int FanControlMinimumAutoSpeed { get; set; } = 20;
    public int FanControlMinSpeedDelta { get; set; } = 4;
    public int FanControlMinSendIntervalSeconds { get; set; } = 0;
    public double FanControlSmoothingAlpha { get; set; } = 0.22;
    public string FanControlCurve { get; set; } = DefaultCurve;
    public List<FanCurvePreset> FanControlCurvePresets { get; set; } = new();
    public string SpeedControlBehavior { get; set; } = "manualOnly";
    public bool StartMinimized { get; set; } = false;
    public bool StartWithWindows { get; set; } = false;
    public bool CloseToTray { get; set; } = true;
    public bool PerformanceReleaseWebViewInBackground { get; set; } = true;
    public bool PerformanceTrimWorkingSetInBackground { get; set; } = true;
    public string UiNavigationPlacement { get; set; } = "top";

    public static string FilePath => Path.Combine(AppContext.BaseDirectory, "settings.json");

    public static AppSettings Load()
    {
        try
        {
            if (File.Exists(FilePath))
            {
                var loaded = JsonSerializer.Deserialize<AppSettings>(File.ReadAllText(FilePath), JsonOptions()) ?? new AppSettings();
                loaded.Migrate();
                loaded.Normalize();
                return loaded;
            }
        }
        catch { }
        var settings = new AppSettings();
        settings.Save();
        return settings;
    }

    public void Save()
    {
        Normalize();
        File.WriteAllText(FilePath, JsonSerializer.Serialize(this, JsonOptions()));
    }

    public static AppSettings? LoadFrom(string path)
    {
        var loaded = JsonSerializer.Deserialize<AppSettings>(File.ReadAllText(path), JsonOptions());
        loaded?.Migrate();
        loaded?.Normalize();
        return loaded;
    }

    public void SaveTo(string path)
    {
        Normalize();
        File.WriteAllText(path, JsonSerializer.Serialize(this, JsonOptions()));
    }

    public void CopyFrom(AppSettings other)
    {
        SettingsSchemaVersion = other.SettingsSchemaVersion;
        FanControlEnabled = other.FanControlEnabled;
        FanControlDeviceIp = other.FanControlDeviceIp;
        FanControlMode = other.FanControlMode;
        FanControlTemperatureSource = other.FanControlTemperatureSource;
        FanControlCpuTemperatureSensorId = other.FanControlCpuTemperatureSensorId;
        FanControlGpuTemperatureSensorId = other.FanControlGpuTemperatureSensorId;
        FanControlManualSpeed = other.FanControlManualSpeed;
        FanControlMinimumAutoSpeed = other.FanControlMinimumAutoSpeed;
        FanControlMinSpeedDelta = other.FanControlMinSpeedDelta;
        FanControlMinSendIntervalSeconds = other.FanControlMinSendIntervalSeconds;
        FanControlSmoothingAlpha = other.FanControlSmoothingAlpha;
        FanControlCurve = string.IsNullOrWhiteSpace(other.FanControlCurve) ? DefaultCurve : other.FanControlCurve;
        FanControlCurvePresets = other.FanControlCurvePresets ?? new List<FanCurvePreset>();
        SpeedControlBehavior = other.SpeedControlBehavior;
        StartMinimized = other.StartMinimized;
        StartWithWindows = other.StartWithWindows;
        CloseToTray = other.CloseToTray;
        PerformanceReleaseWebViewInBackground = other.PerformanceReleaseWebViewInBackground;
        PerformanceTrimWorkingSetInBackground = other.PerformanceTrimWorkingSetInBackground;
        UiNavigationPlacement = other.UiNavigationPlacement;
        Normalize();
    }

    private void Normalize()
    {
        SettingsSchemaVersion = CurrentSettingsSchemaVersion;
        FanControlMode = FanControlService.NormalizeMode(FanControlMode);
        FanControlTemperatureSource = FanControlTemperatureSource is "gpu" or "max" ? FanControlTemperatureSource : "cpu";
        FanControlCpuTemperatureSensorId = NormalizeSensorId(FanControlCpuTemperatureSensorId);
        FanControlGpuTemperatureSensorId = NormalizeSensorId(FanControlGpuTemperatureSensorId);
        UiNavigationPlacement = UiNavigationPlacement == "side" ? "side" : "top";
        FanControlManualSpeed = FanControlService.Clamp(FanControlManualSpeed);
        FanControlMinimumAutoSpeed = FanControlService.Clamp(FanControlMinimumAutoSpeed);
        FanControlMinSpeedDelta = PlatformCompat.Clamp(FanControlMinSpeedDelta, 1, 100);
        FanControlMinSendIntervalSeconds = PlatformCompat.Clamp(FanControlMinSendIntervalSeconds, 0, 120);
        FanControlSmoothingAlpha = PlatformCompat.Clamp(FanControlSmoothingAlpha, 0.05, 1.0);
        SpeedControlBehavior = SpeedControlBehavior == "switchToManual" ? "switchToManual" : "manualOnly";
        if (FanControlCurvePresets == null)
            FanControlCurvePresets = new List<FanCurvePreset>();
        FanControlCurve = FanControlService.TryNormalizeCurve(FanControlCurve, out var normalizedCurve, out _)
            ? normalizedCurve
            : DefaultCurve;
    }

    private static string NormalizeSensorId(string? sensorId)
    {
        sensorId = sensorId?.Trim();
        return string.IsNullOrWhiteSpace(sensorId) ? "auto" : sensorId!;
    }

    private void Migrate()
    {
        if (SettingsSchemaVersion < 2)
        {
            PerformanceReleaseWebViewInBackground = true;
            PerformanceTrimWorkingSetInBackground = true;
        }

        SettingsSchemaVersion = CurrentSettingsSchemaVersion;
    }

    private static JsonSerializerOptions JsonOptions() => new() { WriteIndented = true };
}

public sealed class FanCurvePreset
{
    public string Name { get; set; } = "";
    public string Curve { get; set; } = "";
}
