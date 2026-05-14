using LibreHardwareMonitor.Hardware;
using System.Diagnostics;
using System.IO;
using System.Management;

namespace FanControlPortable;

public sealed class HardwareReader : IDisposable
{
    private readonly object _sync = new();
    private readonly AppSettings _settings;
    private readonly Computer _computer;
    private readonly IVisitor _updateVisitor = new UpdateVisitor();
    private readonly Dictionary<string, ISensor> _temperatureSensorsById = new(StringComparer.OrdinalIgnoreCase);
    private IReadOnlyList<TemperatureSensorOption> _cpuTemperatureOptions = Array.Empty<TemperatureSensorOption>();
    private IReadOnlyList<TemperatureSensorOption> _gpuTemperatureOptions = Array.Empty<TemperatureSensorOption>();
    private DateTime _lastReopenUtc = DateTime.MinValue;
    private DateTime _lastDiagnosticLogUtc = DateTime.MinValue;
    private string _lastDiagnosticLogSignature = "";
    private bool _diagnosticLogWritten;

    public HardwareSnapshot Current { get; private set; } = HardwareSnapshot.Empty;

    public HardwareReader(AppSettings? settings = null)
    {
        _settings = settings ?? new AppSettings();
        _computer = new Computer
        {
            IsCpuEnabled = true,
            IsGpuEnabled = true,
            IsMemoryEnabled = false,
            IsMotherboardEnabled = false,
            IsControllerEnabled = false,
            IsNetworkEnabled = false,
            IsStorageEnabled = false,
            IsBatteryEnabled = false,
            IsPsuEnabled = false
        };
        _computer.Open();
        WarmUpComputer();
    }

    public HardwareSnapshot Update(bool backgroundMode = false)
    {
        lock (_sync)
        {
            if (backgroundMode && TryReadSelectedTemperatures(out var leanSnapshot))
            {
                Current = leanSnapshot;
                return Current;
            }

            Current = UpdateFullScan();
            return Current;
        }
    }

    private HardwareSnapshot UpdateFullScan()
    {
            var allTemps = ReadAllTemperatures();
            if (allTemps.Count == 0 && ShouldTryReopen())
            {
                ReopenComputer();
                allTemps = ReadAllTemperatures();
            }

            var hardwareLines = allTemps.Count == 0 ? ListHardware() : new List<string>();
            var cpuTemps = FindCpuTemperatureSensors(allTemps);
            var gpuTemps = FindGpuTemperatureSensors(allTemps);
            PinDefaultTemperatureSensors(cpuTemps, gpuTemps);
            var cpu = ChooseCpuTemperature(cpuTemps, _settings.FanControlCpuTemperatureSensorId);
            var gpu = ChooseGpuTemperature(gpuTemps, _settings.FanControlGpuTemperatureSensorId);

            if (cpu == null)
            {
                cpu = TryReadAcpiTemperature() ?? TryReadThermalZonePerformanceCounter();
                AddSensorIfMissing(cpuTemps, cpu);
            }

            if (gpu == null)
            {
                gpu = TryReadNvidiaSmiTemperature();
                AddSensorIfMissing(gpuTemps, gpu);
            }

            var diagnostic = BuildReadableDiagnostic(allTemps, hardwareLines, cpu, gpu);
            _cpuTemperatureOptions = BuildTemperatureOptions(cpuTemps);
            _gpuTemperatureOptions = BuildTemperatureOptions(gpuTemps);
            WriteDiagnosticLogIfNeeded(allTemps, hardwareLines, cpu, gpu, diagnostic);
            return new HardwareSnapshot(
                cpu?.Value,
                cpu?.Name ?? "",
                gpu?.Value,
                gpu?.Name ?? "",
                DateTime.Now,
                allTemps.Count,
                diagnostic,
                _cpuTemperatureOptions,
                _gpuTemperatureOptions);
    }

    private List<SensorValue> ReadAllTemperatures()
    {
        var values = new List<SensorValue>();
        _temperatureSensorsById.Clear();
        _computer.Accept(_updateVisitor);

        foreach (var hardware in _computer.Hardware)
            CollectTemperatures(hardware, values, _temperatureSensorsById);

        return values;
    }

    private bool TryReadSelectedTemperatures(out HardwareSnapshot snapshot)
    {
        snapshot = Current;
        var cpuSensorId = _settings.FanControlCpuTemperatureSensorId;
        var gpuSensorId = _settings.FanControlGpuTemperatureSensorId;
        if (IsAutoSensorId(cpuSensorId) && _cpuTemperatureOptions.Count > 0)
            return false;
        if (IsAutoSensorId(gpuSensorId) && _gpuTemperatureOptions.Count > 0)
            return false;
        if (IsAutoSensorId(cpuSensorId) && IsAutoSensorId(gpuSensorId))
            return false;

        var cpuSensor = IsAutoSensorId(cpuSensorId) ? null : FindCachedTemperatureSensor(cpuSensorId);
        var gpuSensor = IsAutoSensorId(gpuSensorId) ? null : FindCachedTemperatureSensor(gpuSensorId);
        if (cpuSensor == null && gpuSensor == null)
            return false;

        var hardwareToUpdate = new HashSet<IHardware>();
        AddSensorHardware(cpuSensor, hardwareToUpdate);
        AddSensorHardware(gpuSensor, hardwareToUpdate);

        foreach (var hardware in hardwareToUpdate)
        {
            try { hardware.Update(); } catch { }
        }

        var cpu = TryReadSensorValue(cpuSensor);
        var gpu = TryReadSensorValue(gpuSensor);
        if (cpu == null && gpu == null)
            return false;

        var allTemps = new List<SensorValue>();
        AddSensorIfMissing(allTemps, cpu);
        AddSensorIfMissing(allTemps, gpu);

        var diagnostic = $"后台精简轮询：已读取 {allTemps.Count} 个已选温度传感器";
        WriteDiagnosticLogIfNeeded(allTemps, new List<string>(), cpu, gpu, diagnostic);

        snapshot = new HardwareSnapshot(
            cpu?.Value,
            cpu?.Name ?? Current.CpuSensorName,
            gpu?.Value,
            gpu?.Name ?? Current.GpuSensorName,
            DateTime.Now,
            Current.TemperatureSensorCount > 0 ? Current.TemperatureSensorCount : allTemps.Count,
            diagnostic,
            _cpuTemperatureOptions.Count > 0 ? _cpuTemperatureOptions : BuildTemperatureOptions(cpu == null ? new List<SensorValue>() : new List<SensorValue> { cpu }),
            _gpuTemperatureOptions.Count > 0 ? _gpuTemperatureOptions : BuildTemperatureOptions(gpu == null ? new List<SensorValue>() : new List<SensorValue> { gpu }));
        return true;
    }

    private ISensor? FindCachedTemperatureSensor(string sensorId)
    {
        if (string.IsNullOrWhiteSpace(sensorId))
            return null;

        return _temperatureSensorsById.TryGetValue(sensorId, out var sensor) ? sensor : null;
    }

    private static void AddSensorHardware(ISensor? sensor, HashSet<IHardware> hardwareToUpdate)
    {
        if (sensor?.Hardware != null)
            hardwareToUpdate.Add(sensor.Hardware);
    }

    private void WarmUpComputer()
    {
        try
        {
            _computer.Accept(_updateVisitor);
            DisableSensorHistory();
            _computer.Accept(_updateVisitor);
        }
        catch
        {
            // The regular update loop will emit diagnostics and retry.
        }
    }

    private bool ShouldTryReopen()
    {
        return (DateTime.UtcNow - _lastReopenUtc).TotalSeconds > 30;
    }

    private void ReopenComputer()
    {
        _lastReopenUtc = DateTime.UtcNow;
        try { _computer.Close(); } catch { }
        try { _computer.Hardware.Clear(); } catch { }
        _computer.Open();
        WarmUpComputer();
    }

    private static void CollectTemperatures(IHardware hardware, List<SensorValue> values, Dictionary<string, ISensor>? sensorMap = null)
    {
        foreach (var sensor in hardware.Sensors)
        {
            var sensorValue = TryReadSensorValue(sensor);
            if (sensorValue == null)
                continue;

            values.Add(sensorValue);
            if (sensorMap != null)
            {
                sensorMap[SensorKey(sensorValue)] = sensor;
                if (!string.IsNullOrWhiteSpace(sensorValue.Identifier))
                    sensorMap[sensorValue.Identifier] = sensor;
            }
        }

        foreach (var subHardware in hardware.SubHardware)
            CollectTemperatures(subHardware, values, sensorMap);
    }

    private static SensorValue? TryReadSensorValue(ISensor? sensor)
    {
        if (sensor?.Hardware == null || sensor.SensorType != SensorType.Temperature || !sensor.Value.HasValue)
            return null;

        var value = sensor.Value.Value;
        if (float.IsNaN(value) || float.IsInfinity(value) || value <= 0.1f || value > 130f)
            return null;

        var hardware = sensor.Hardware;
        var sensorName = sensor.Name ?? "Temperature";
        var displayName = string.IsNullOrWhiteSpace(hardware.Name)
            ? sensorName
            : $"{hardware.Name} / {sensorName}";
        var hardwarePath = BuildHardwarePath(hardware);
        return new SensorValue(
            displayName,
            value,
            hardware.HardwareType,
            hardware.Name ?? "",
            sensorName,
            hardwarePath,
            sensor.Identifier?.ToString() ?? "");
    }

    private static List<SensorValue> FindCpuTemperatureSensors(List<SensorValue> values)
    {
        var cpuTemps = values.Where(IsCpuSensor).ToList();
        if (cpuTemps.Count == 0)
            cpuTemps = values.Where(v => !IsGpuSensor(v) && IsCpuNamedSensor(v)).ToList();

        return cpuTemps;
    }

    private static List<SensorValue> FindGpuTemperatureSensors(List<SensorValue> values)
    {
        return values.Where(IsGpuSensor).ToList();
    }

    private static SensorValue? ChooseTemperature(List<SensorValue> values, string selectedSensorId, Func<SensorValue, int> score)
    {
        if (values.Count == 0) return null;
        if (TryChooseConfiguredSensor(values, selectedSensorId, out var configured))
            return configured;

        var preferred = values
            .Select(v => new { Sensor = v, Score = score(v) })
            .Where(v => v.Score > 0)
            .OrderByDescending(v => v.Score)
            .ThenByDescending(v => v.Sensor.Value)
            .FirstOrDefault();
        if (preferred != null)
            return preferred.Sensor;

        return values
            .Where(v => !IsDistanceSensor(v))
            .OrderByDescending(v => v.Value)
            .FirstOrDefault()
            ?? values.OrderByDescending(v => v.Value).First();
    }

    private static SensorValue? ChooseCpuTemperature(List<SensorValue> values, string selectedSensorId)
    {
        return ChooseTemperature(values, selectedSensorId, ScoreCpuPackageTemperature);
    }

    private static SensorValue? ChooseGpuTemperature(List<SensorValue> values, string selectedSensorId)
    {
        return ChooseTemperature(values, selectedSensorId, ScoreGpuPackageTemperature);
    }

    private void PinDefaultTemperatureSensors(List<SensorValue> cpuTemps, List<SensorValue> gpuTemps)
    {
        var changed = false;
        changed |= PinDefaultTemperatureSensor(
            cpuTemps,
            _settings.FanControlCpuTemperatureSensorId,
            ChooseCpuTemperature,
            value => _settings.FanControlCpuTemperatureSensorId = value);
        changed |= PinDefaultTemperatureSensor(
            gpuTemps,
            _settings.FanControlGpuTemperatureSensorId,
            ChooseGpuTemperature,
            value => _settings.FanControlGpuTemperatureSensorId = value);

        if (!changed)
            return;

        try { _settings.Save(); } catch { }
    }

    private static bool PinDefaultTemperatureSensor(
        List<SensorValue> values,
        string selectedSensorId,
        Func<List<SensorValue>, string, SensorValue?> chooseSensor,
        Action<string> setSensorId)
    {
        if (values.Count == 0)
            return false;

        if (!IsAutoSensorId(selectedSensorId) && TryChooseConfiguredSensor(values, selectedSensorId, out _))
            return false;

        var selected = chooseSensor(values, "auto");
        if (selected == null)
            return false;

        var selectedId = SensorKey(selected);
        if (string.Equals(selectedSensorId, selectedId, StringComparison.OrdinalIgnoreCase))
            return false;

        setSensorId(selectedId);
        return true;
    }

    private static bool TryChooseConfiguredSensor(List<SensorValue> values, string? selectedSensorId, out SensorValue? sensor)
    {
        sensor = null;
        if (IsAutoSensorId(selectedSensorId))
        {
            return false;
        }

        sensor = values.FirstOrDefault(value =>
            string.Equals(SensorKey(value), selectedSensorId, StringComparison.OrdinalIgnoreCase) ||
            string.Equals(value.Identifier, selectedSensorId, StringComparison.OrdinalIgnoreCase));
        return sensor != null;
    }

    private static bool IsAutoSensorId(string? sensorId)
    {
        return string.IsNullOrWhiteSpace(sensorId) ||
               string.Equals(sensorId, "auto", StringComparison.OrdinalIgnoreCase);
    }

    private static void AddSensorIfMissing(List<SensorValue> values, SensorValue? sensor)
    {
        if (sensor == null)
            return;

        var key = SensorKey(sensor);
        if (!values.Any(value => string.Equals(SensorKey(value), key, StringComparison.OrdinalIgnoreCase)))
            values.Add(sensor);
    }

    private static List<TemperatureSensorOption> BuildTemperatureOptions(List<SensorValue> values)
    {
        return values
            .Where(value => !IsDistanceSensor(value))
            .GroupBy(SensorKey, StringComparer.OrdinalIgnoreCase)
            .Select(group => group.First())
            .OrderBy(value => value.HardwareName)
            .ThenBy(value => value.SensorName)
            .Select(value => new TemperatureSensorOption(SensorKey(value), value.Name, value.Value))
            .ToList();
    }

    private static string SensorKey(SensorValue value)
    {
        return !string.IsNullOrWhiteSpace(value.Identifier)
            ? value.Identifier
            : $"{value.HardwareType}|{value.HardwarePath}|{value.SensorName}";
    }

    private static int ScoreCpuPackageTemperature(SensorValue value)
    {
        if (IsDistanceSensor(value))
            return -1;

        var sensorName = value.SensorName;
        if (ContainsAny(sensorName, "cpu package", "package"))
            return 100;
        if (ContainsAny(sensorName, "tctl/tdie", "tctl", "tdie"))
            return 95;
        if (ContainsAny(sensorName, "cpu die", "die average", "die"))
            return 90;
        if (ContainsAny(sensorName, "ccd"))
            return 80;
        if (ContainsAny(sensorName, "core max"))
            return 60;
        if (ContainsAny(sensorName, "core average"))
            return 50;
        if (ContainsAny(sensorName, "cpu"))
            return 40;
        if (ContainsAny(sensorName, "core"))
            return 30;
        if (ContainsAny(sensorName, "soc"))
            return 20;

        return 0;
    }

    private static int ScoreGpuPackageTemperature(SensorValue value)
    {
        if (IsDistanceSensor(value))
            return -1;

        var sensorName = value.SensorName;
        if (ContainsAny(sensorName, "gpu package", "package"))
            return 100;
        if (ContainsAny(sensorName, "gpu core", "core"))
            return 95;
        if (ContainsAny(sensorName, "gpu temperature", "temperature.gpu", "edge"))
            return 90;
        if (ContainsAny(sensorName, "hot spot", "hotspot"))
            return 70;
        if (ContainsAny(sensorName, "memory junction", "junction", "memory"))
            return 40;
        if (ContainsAny(sensorName, "gpu"))
            return 30;

        return 0;
    }

    private static bool IsDistanceSensor(SensorValue value)
    {
        return ContainsAny(value.SensorName, "distance", "tjmax") ||
               ContainsAny(value.Name, "distance", "tjmax");
    }

    private static bool ContainsAny(string source, params string[] needles)
    {
        foreach (var needle in needles)
        {
            if (source.Contains(needle, StringComparison.OrdinalIgnoreCase))
                return true;
        }
        return false;
    }

    private static bool IsGpuSensor(SensorValue value)
    {
        return value.HardwareType is HardwareType.GpuAmd or HardwareType.GpuIntel or HardwareType.GpuNvidia ||
               ContainsAny(value.HardwareName, "gpu", "nvidia", "geforce", "radeon", "intel graphics", "arc") ||
               ContainsAny(value.HardwarePath, "gpu", "nvidia", "geforce", "radeon", "intel graphics", "arc") ||
               ContainsAny(value.Identifier, "/gpu/", "gpu");
    }

    private static bool IsCpuSensor(SensorValue value)
    {
        return value.HardwareType == HardwareType.Cpu ||
               ContainsAny(value.Identifier, "/cpu/", "cpu") ||
               ContainsAny(value.HardwareName, "cpu", "processor", "ryzen", "intel core", "core i", "apu") ||
               ContainsAny(value.HardwarePath, "cpu", "processor", "ryzen", "intel core", "core i", "apu");
    }

    private static bool IsCpuNamedSensor(SensorValue value)
    {
        return ContainsAny(value.Name, "cpu", "package", "tctl", "tdie", "ccd", "core", "die", "processor", "apu", "soc");
    }

    private static SensorValue? TryReadAcpiTemperature()
    {
        try
        {
            using var searcher = new ManagementObjectSearcher(@"root\WMI", "SELECT CurrentTemperature, InstanceName FROM MSAcpi_ThermalZoneTemperature");
            SensorValue? best = null;
            foreach (ManagementObject obj in searcher.Get().Cast<ManagementObject>())
            {
                if (obj["CurrentTemperature"] is not uint raw)
                    continue;

                var celsius = raw / 10.0f - 273.15f;
                if (float.IsNaN(celsius) || float.IsInfinity(celsius) || celsius <= 0.1f || celsius > 130f)
                    continue;

                var name = obj["InstanceName"]?.ToString();
                var sensor = new SensorValue(
                    string.IsNullOrWhiteSpace(name) ? "ACPI Thermal Zone" : $"ACPI Thermal Zone / {name}",
                    celsius,
                    HardwareType.Cpu,
                    "ACPI",
                    "CurrentTemperature",
                    "ACPI Thermal Zone",
                    "acpi/thermal-zone");
                if (best == null || sensor.Value > best.Value)
                    best = sensor;
            }
            return best;
        }
        catch
        {
            return null;
        }
    }

    private static SensorValue? TryReadThermalZonePerformanceCounter()
    {
        try
        {
            using var searcher = new ManagementObjectSearcher(
                @"root\CIMV2",
                "SELECT Name, Temperature FROM Win32_PerfFormattedData_Counters_ThermalZoneInformation");

            SensorValue? best = null;
            foreach (ManagementObject obj in searcher.Get().Cast<ManagementObject>())
            {
                if (obj["Temperature"] == null)
                    continue;

                var raw = Convert.ToSingle(obj["Temperature"]);
                var celsius = raw > 200.0f ? raw / 10.0f - 273.15f : raw;
                if (float.IsNaN(celsius) || float.IsInfinity(celsius) || celsius <= 0.1f || celsius > 130f)
                    continue;

                var name = obj["Name"]?.ToString();
                var sensor = new SensorValue(
                    string.IsNullOrWhiteSpace(name) ? "Windows Thermal Zone" : $"Windows Thermal Zone / {name}",
                    celsius,
                    HardwareType.Cpu,
                    "Windows Thermal Zone",
                    "Temperature",
                    "Win32_PerfFormattedData_Counters_ThermalZoneInformation",
                    "wmi/thermal-zone-performance");
                if (best == null || sensor.Value > best.Value)
                    best = sensor;
            }

            return best;
        }
        catch
        {
            return null;
        }
    }

    private static SensorValue? TryReadNvidiaSmiTemperature()
    {
        var candidates = new[]
        {
            "nvidia-smi.exe",
            Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.ProgramFiles), "NVIDIA Corporation", "NVSMI", "nvidia-smi.exe")
        };

        foreach (var fileName in candidates)
        {
            try
            {
                using var process = Process.Start(new ProcessStartInfo
                {
                    FileName = fileName,
                    Arguments = "--query-gpu=temperature.gpu --format=csv,noheader,nounits",
                    UseShellExecute = false,
                    CreateNoWindow = true,
                    RedirectStandardOutput = true,
                    RedirectStandardError = true
                });
                if (process == null)
                    continue;

                if (!process.WaitForExit(1500))
                {
                    try { process.Kill(); } catch { }
                    continue;
                }

                var output = process.StandardOutput.ReadToEnd().Trim().Split('\n', StringSplitOptions.RemoveEmptyEntries).FirstOrDefault()?.Trim();
                if (float.TryParse(output, out var value) && value is > 0.1f and < 130f)
                {
                    return new SensorValue("NVIDIA SMI / GPU Temperature", value, HardwareType.GpuNvidia, "NVIDIA SMI", "temperature.gpu", "NVIDIA SMI", "nvidia-smi");
                }
            }
            catch
            {
                // Try the next known nvidia-smi location.
            }
        }

        return null;
    }

    private static string BuildReadableDiagnostic(List<SensorValue> values, List<string> hardwareLines, SensorValue? cpu, SensorValue? gpu)
    {
        if (values.Count == 0)
        {
            if (cpu != null || gpu != null)
                return "LibreHardwareMonitor 未枚举到温度传感器，已通过备用通道读取温度。";

            return hardwareLines.Count == 0
                ? "未发现硬件节点，请确认以管理员权限运行，并已安装 PawnIO 驱动。"
                : $"已发现 {hardwareLines.Count} 个硬件节点，但没有温度传感器数值。";
        }

        if (cpu != null || gpu != null)
            return $"已读取 {values.Count} 个温度传感器";

        var samples = string.Join("，", values.Take(4).Select(v => $"{v.Name} {Math.Round(v.Value)}°C"));
        return $"已读取 {values.Count} 个温度传感器，但未识别出 CPU/GPU：{samples}";
    }

    public void Dispose()
    {
        lock (_sync)
        {
            _computer.Close();
        }
    }

    private static string BuildDiagnostic(List<SensorValue> values, List<string> hardwareLines, SensorValue? cpu, SensorValue? gpu)
    {
        if (values.Count == 0)
        {
            if (cpu != null || gpu != null)
                return "LibreHardwareMonitor 未枚举到温度传感器，已通过备用通道读取温度。";

            return hardwareLines.Count == 0
                ? "未发现硬件节点，请确认以管理员权限运行并已安装 PawnIO 驱动。"
                : $"已发现 {hardwareLines.Count} 个硬件节点，但没有温度传感器数值。";
        }

        if (cpu != null || gpu != null)
            return $"已读取 {values.Count} 个温度传感器";

        var samples = string.Join("；", values.Take(4).Select(v => $"{v.Name} {Math.Round(v.Value)}°C"));
        return $"已读取 {values.Count} 个温度传感器，但未识别出 CPU/GPU：{samples}";
    }

    private void WriteDiagnosticLogIfNeeded(List<SensorValue> values, List<string> hardwareLines, SensorValue? cpu, SensorValue? gpu, string diagnostic)
    {
        var now = DateTime.UtcNow;
        var signature = BuildDiagnosticLogSignature(values, cpu, gpu);
        var topologyChanged = !string.Equals(signature, _lastDiagnosticLogSignature, StringComparison.Ordinal);
        var important = values.Count == 0 || cpu == null || gpu == null;
        var interval = important ? TimeSpan.FromSeconds(30) : TimeSpan.FromMinutes(5);
        if (_diagnosticLogWritten && !topologyChanged && now - _lastDiagnosticLogUtc < interval)
            return;

        if (hardwareLines.Count == 0)
            hardwareLines = ListHardware();

        WriteDiagnosticLog(values, hardwareLines, cpu, gpu, diagnostic);
        _diagnosticLogWritten = true;
        _lastDiagnosticLogUtc = now;
        _lastDiagnosticLogSignature = signature;
    }

    private static string BuildDiagnosticLogSignature(List<SensorValue> values, SensorValue? cpu, SensorValue? gpu)
    {
        var sensorIds = values
            .Select(SensorKey)
            .OrderBy(value => value, StringComparer.OrdinalIgnoreCase);
        return string.Join("|", sensorIds) + $"|cpu={SensorKeyOrEmpty(cpu)}|gpu={SensorKeyOrEmpty(gpu)}";
    }

    private static string SensorKeyOrEmpty(SensorValue? value)
    {
        return value == null ? "" : SensorKey(value);
    }

    private static void WriteDiagnosticLog(List<SensorValue> values, List<string> hardwareLines, SensorValue? cpu, SensorValue? gpu, string diagnostic)
    {
        try
        {
            var path = Path.Combine(AppContext.BaseDirectory, "hardware-sensors.log");
            var lines = new List<string>
            {
                $"[{DateTime.Now:yyyy-MM-dd HH:mm:ss}] {diagnostic}",
                $"CPU: {(cpu == null ? "--" : $"{cpu.Name} {cpu.Value:0.#}C")}",
                $"GPU: {(gpu == null ? "--" : $"{gpu.Name} {gpu.Value:0.#}C")}",
                $"Hardware: {hardwareLines.Count}",
                $"Sensors: {values.Count}",
                "Hardware list:"
            };
            lines.AddRange(hardwareLines.Select(v => "  " + v));
            lines.Add("Temperature sensors:");
            lines.AddRange(values.Select(v => $"{v.HardwareType} | {v.HardwarePath} | {v.Name} | {v.Value:0.#}C | {v.Identifier}"));
            File.WriteAllLines(path, lines);
        }
        catch
        {
            // Diagnostics must never interrupt fan control.
        }
    }

    private static string BuildHardwarePath(IHardware hardware)
    {
        var names = new Stack<string>();
        for (IHardware? node = hardware; node != null; node = node.Parent)
        {
            if (!string.IsNullOrWhiteSpace(node.Name))
                names.Push(node.Name);
        }
        return string.Join(" / ", names);
    }

    private List<string> ListHardware()
    {
        var lines = new List<string>();
        foreach (var hardware in _computer.Hardware)
            AddHardwareLine(hardware, lines, 0);
        return lines;
    }

    private static void AddHardwareLine(IHardware hardware, List<string> lines, int depth)
    {
        lines.Add($"{new string(' ', depth * 2)}{hardware.HardwareType} | {hardware.Name} | sensors={hardware.Sensors.Length}");
        foreach (var subHardware in hardware.SubHardware)
            AddHardwareLine(subHardware, lines, depth + 1);
    }

    private void DisableSensorHistory()
    {
        try { _computer.Accept(new DisableHistoryVisitor()); }
        catch { }
    }

    private sealed class UpdateVisitor : IVisitor
    {
        public void VisitComputer(IComputer computer) => computer.Traverse(this);
        public void VisitHardware(IHardware hardware)
        {
            try { hardware.Update(); } catch { }
            foreach (var subHardware in hardware.SubHardware)
                subHardware.Accept(this);
        }
        public void VisitSensor(ISensor sensor) { }
        public void VisitParameter(IParameter parameter) { }
    }

    private sealed class DisableHistoryVisitor : IVisitor
    {
        public void VisitComputer(IComputer computer) => computer.Traverse(this);

        public void VisitHardware(IHardware hardware)
        {
            foreach (var sensor in hardware.Sensors)
                VisitSensor(sensor);

            foreach (var subHardware in hardware.SubHardware)
                subHardware.Accept(this);
        }

        public void VisitSensor(ISensor sensor)
        {
            try
            {
                var property = sensor.GetType().GetProperty("ValuesTimeWindow");
                if (property?.CanWrite == true)
                    property.SetValue(sensor, TimeSpan.Zero);
            }
            catch { }
        }

        public void VisitParameter(IParameter parameter) { }
    }

    private sealed record SensorValue(string Name, float Value, HardwareType HardwareType, string HardwareName, string SensorName, string HardwarePath, string Identifier);
}

public sealed record TemperatureSensorOption(string Id, string Name, float Value);

public sealed record HardwareSnapshot(
    float? CpuTemperature,
    string CpuSensorName,
    float? GpuTemperature,
    string GpuSensorName,
    DateTime UpdatedAt,
    int TemperatureSensorCount,
    string Diagnostic,
    IReadOnlyList<TemperatureSensorOption> CpuTemperatureSensors,
    IReadOnlyList<TemperatureSensorOption> GpuTemperatureSensors)
{
    public static HardwareSnapshot Empty { get; } = new(
        null,
        "",
        null,
        "",
        DateTime.MinValue,
        0,
        "",
        Array.Empty<TemperatureSensorOption>(),
        Array.Empty<TemperatureSensorOption>());
}
