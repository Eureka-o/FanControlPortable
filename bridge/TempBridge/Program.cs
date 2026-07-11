using System;
using System.Diagnostics;
using System.IO;
using System.IO.Pipes;
using System.Linq;
using System.Management;
using System.Runtime.InteropServices;
using System.ServiceProcess;
using System.Threading;
using Newtonsoft.Json;
using LibreHardwareMonitor.Hardware;
using LibreHardwareMonitor.PawnIo;

namespace FanControl.TempBridge
{
    public class TemperatureData
    {
        public int CpuTemp { get; set; }
        public int GpuTemp { get; set; }
        public double CpuPowerWatts { get; set; }
        public double GpuPowerWatts { get; set; }
        public string GpuReadState { get; set; }
        public int MaxTemp { get; set; }
        public int ControlTemp { get; set; }
        public string ControlSource { get; set; }
        public string SelectedGpuDevice { get; set; }
        public string CpuModel { get; set; }
        public string GpuModel { get; set; }
        public TemperatureSensor[] CpuSensors { get; set; }
        public TemperatureSensor[] GpuSensors { get; set; }
        public PowerSensor[] CpuPowerSensors { get; set; }
        public PowerSensor[] GpuPowerSensors { get; set; }
        public TemperatureGpuDevice[] GpuDevices { get; set; }
        public long UpdateTime { get; set; }
        public bool Success { get; set; }
        public string Error { get; set; }

        public TemperatureData()
        {
            ControlSource = "max";
            SelectedGpuDevice = "auto";
            GpuReadState = "unknown";
            CpuModel = string.Empty;
            GpuModel = string.Empty;
            CpuSensors = Array.Empty<TemperatureSensor>();
            GpuSensors = Array.Empty<TemperatureSensor>();
            CpuPowerSensors = Array.Empty<PowerSensor>();
            GpuPowerSensors = Array.Empty<PowerSensor>();
            GpuDevices = Array.Empty<TemperatureGpuDevice>();
            Error = string.Empty;
        }
    }

    public class TemperatureSensor
    {
        public string Key { get; set; }
        public string Name { get; set; }
        public int Value { get; set; }

        public TemperatureSensor()
        {
            Key = string.Empty;
            Name = string.Empty;
        }
    }

    public class PowerSensor
    {
        public string Key { get; set; }
        public string Name { get; set; }
        public double Value { get; set; }

        public PowerSensor()
        {
            Key = string.Empty;
            Name = string.Empty;
        }
    }

    public class TemperatureGpuDevice
    {
        public string Key { get; set; }
        public string Name { get; set; }
        public string Vendor { get; set; }
        public TemperatureSensor[] Sensors { get; set; }
        public PowerSensor[] PowerSensors { get; set; }

        public TemperatureGpuDevice()
        {
            Key = "auto";
            Name = string.Empty;
            Vendor = string.Empty;
            Sensors = Array.Empty<TemperatureSensor>();
            PowerSensors = Array.Empty<PowerSensor>();
        }
    }

    public class TemperatureSelection
    {
        public string TempSource { get; set; }
        public string GpuDevice { get; set; }
        public string CpuSensor { get; set; }
        public string GpuSensor { get; set; }
        public string CpuPowerSensor { get; set; }
        public string GpuPowerSensor { get; set; }
        public string GpuReadMode { get; set; }
        public bool GpuLowPowerProtection { get; set; }

        public TemperatureSelection()
        {
            TempSource = "max";
            GpuDevice = "auto";
            CpuSensor = "auto";
            GpuSensor = "auto";
            CpuPowerSensor = "auto";
            GpuPowerSensor = "auto";
            GpuReadMode = "auto";
            GpuLowPowerProtection = true;
        }
    }

    public class GpuCandidate
    {
        public string Key { get; set; }
        public string Model { get; set; }
        public string Vendor { get; set; }
        public HardwareType HardwareType { get; set; }
        public System.Collections.Generic.List<TemperatureSensor> Sensors { get; set; }
        public System.Collections.Generic.List<PowerSensor> PowerSensors { get; set; }
        public double PowerWatts { get; set; }

        public GpuCandidate()
        {
            Key = "auto";
            Model = string.Empty;
            Vendor = string.Empty;
            Sensors = new System.Collections.Generic.List<TemperatureSensor>();
            PowerSensors = new System.Collections.Generic.List<PowerSensor>();
        }
    }

    public class HardwareProfile
    {
        public int SchemaVersion { get; set; }
        public string GeneratedAt { get; set; }
        public string Signature { get; set; }
        public string PreferredGpuProbe { get; set; }
        public HardwareGpuDevice[] GpuDevices { get; set; }

        public HardwareProfile()
        {
            SchemaVersion = 1;
            GeneratedAt = string.Empty;
            Signature = string.Empty;
            PreferredGpuProbe = "windows";
            GpuDevices = Array.Empty<HardwareGpuDevice>();
        }
    }

    public class HardwareGpuDevice
    {
        public string Name { get; set; }
        public string Vendor { get; set; }
        public string PnpDeviceId { get; set; }
        public string Luid { get; set; }
        public bool Discrete { get; set; }

        public HardwareGpuDevice()
        {
            Name = string.Empty;
            Vendor = string.Empty;
            PnpDeviceId = string.Empty;
            Luid = string.Empty;
        }
    }

    public class UpdateVisitor : IVisitor
    {
        public void VisitComputer(IComputer computer)
        {
            computer.Traverse(this);
        }

        public void VisitHardware(IHardware hardware)
        {
            hardware.Update();
            foreach (IHardware subHardware in hardware.SubHardware)
                subHardware.Accept(this);
        }

        public void VisitSensor(ISensor sensor) { }
        public void VisitParameter(IParameter parameter) { }
    }

    public class Command
    {
        public string Type { get; set; }
        public string Data { get; set; }
    }

    public class Response
    {
        public bool Success { get; set; }
        public string Error { get; set; }
        public TemperatureData Data { get; set; }
    }

    partial class Program
    {
        private const string PipeName = "FanControl2_TempBridge";
        private const string MutexName = @"Global\FanControl2_TempBridge_Singleton";
        private const int MaxInitRetries = 3;
        private const int InitRetryDelayMs = 2000;
        private const int ConsecutiveFailuresBeforeReinit = 5;
        private const int MaxReasonableTemperature = 150;
        private const int MemoryTrimIntervalSeconds = 60;
        private const int IntelMsrRuntimeFailureThreshold = 3;
        private const int IntelMsrProbeRetryCooldownSeconds = 15;
        private const int IntelRaplProbeRetryCooldownSeconds = 30;
        private const string MsrUnavailableMarker = "[msr-unavailable]";
        private const string MsrRetryableMarker = "[msr-retryable]";
        private const uint IntelTemperatureTargetMsr = 0x1A2;
        private const uint IntelThermalStatusMsr = 0x19C;
        private const uint IntelPackageThermalStatusMsr = 0x1B1;
        private const uint IntelRaplPowerUnitMsr = 0x606;
        private const uint IntelPackageEnergyStatusMsr = 0x611;
        private const string GpuReadStateActive = "active";
        private const string GpuReadStateNotPolled = "notPolled";
        private const string GpuReadStateUnavailable = "unavailable";
        private const string GpuReadStateError = "error";
        private static Computer computer;
        private static bool running = true;
        private static readonly object lockObject = new object();
        private static Mutex singleInstanceMutex;
        private static int consecutiveFailures = 0;
        private static string lastHardwareMonitorError = string.Empty;
        private static DateTime lastMemoryTrimUtc = DateTime.MinValue;
        private static bool currentGpuMonitoringEnabled = false;
        private static readonly object hardwareProfileLock = new object();
        private static HardwareProfile cachedHardwareProfile;
        private static int hardwareProfileSensorReconcileAttempted = 0;
        private static readonly GpuReadCoordinator gpuReadCoordinator = new GpuReadCoordinator();
        private static readonly IntPtr TrimWorkingSetSentinel = new IntPtr(-1);
        private static IntelMsr intelMsrFallback;
        private static bool intelMsrProbeAttempted;
        private static string intelMsrDiagnostic = string.Empty;
        private static int consecutiveIntelMsrTemperatureFailures;
        private static int consecutiveIntelMsrPowerFailures;
        private static DateTime lastIntelMsrProbeAt = DateTime.MinValue;
        private static DateTime lastIntelRaplProbeAt = DateTime.MinValue;
        private static double intelPackageEnergyUnit;
        private static uint lastIntelPackageEnergy;
        private static DateTime lastIntelPackageEnergyAt = DateTime.MinValue;
        private static readonly string windowsCpuModel = ReadWindowsCpuModel();

        [DllImport("kernel32.dll")]
        private static extern bool SetProcessWorkingSetSize(IntPtr process, IntPtr minimumWorkingSetSize, IntPtr maximumWorkingSetSize);

        static void Main(string[] args)
        {
            bool msrSelfTest = HasArg(args, "--self-test-msr");
            bool diagnosticMode = ShouldRunDiagnosticMode(args);
            bool pipeMode = ShouldRunPipeMode(args);
            try
            {
                if (msrSelfTest)
                {
                    RunIntelMsrSelfTest();
                    return;
                }
                if (diagnosticMode)
                {
                    RunConsoleDiagnostics();
                    return;
                }
                if (!pipeMode)
                {
                    RunStdioMode();
                    return;
                }
                using (var instanceHandle = AcquirePipeInstance())
                {
                    if (instanceHandle == null)
                    {
                        Console.WriteLine($"PIPE:{PipeName}|ATTACH");
                        Console.Out.Flush();
                        return;
                    }
                    InitializeHardwareMonitor(currentGpuMonitoringEnabled);
                    Console.WriteLine($"PIPE:{PipeName}|OWNER");
                    Console.Out.Flush();
                    StartPipeServer();
                }
            }
            catch (Exception ex)
            {
                if (diagnosticMode)
                {
                    Console.Error.WriteLine("FanControl TempBridge startup failed");
                    Console.Error.WriteLine($"Error: {ex.Message}");
                }
                else
                {
                    Console.WriteLine($"ERROR:{ex.Message}");
                }
                Environment.Exit(1);
            }
            finally
            {
                computer?.Close();
                ResetIntelMsrFallback();
                if (singleInstanceMutex != null)
                {
                    singleInstanceMutex.Dispose();
                    singleInstanceMutex = null;
                }
            }
        }
        static void RunStdioMode()
        {
            var initStopwatch = Stopwatch.StartNew();
            LogInitProgress("stdio mode starting; initializing hardware monitor");
            InitializeHardwareMonitor();
            LogInitProgress(string.Format("hardware monitor initialized in {0} ms", initStopwatch.ElapsedMilliseconds));
            using (var stdin = Console.OpenStandardInput())
            using (var stdout = Console.OpenStandardOutput())
            using (var reader = new StreamReader(stdin))
            using (var writer = new StreamWriter(stdout) { AutoFlush = true })
            {
                writer.WriteLine("READY:STDIO");
                ServeCommandLoop(reader, writer);
            }
        }
        /// <summary>
        /// Writes initialization progress to stderr with an "[init]" prefix for backend diagnostics.
        /// </summary>
        static void LogInitProgress(string message)
        {
            try
            {
                Console.Error.WriteLine("[init] " + message);
                Console.Error.Flush();
            }
            catch
            {
            }
        }
        static IDisposable AcquirePipeInstance()
        {
            bool createdNew;
            singleInstanceMutex = new Mutex(false, MutexName, out createdNew);

            bool acquired = false;
            try
            {
                acquired = singleInstanceMutex.WaitOne(0, false);
            }
            catch (AbandonedMutexException)
            {
                acquired = true;
            }

            if (!acquired)
            {
                return null;
            }

            return new MutexHandle(singleInstanceMutex);
        }

        static bool ShouldRunDiagnosticMode(string[] args)
        {
            if (ShouldRunPipeMode(args))
            {
                return false;
            }
            if (HasArg(args, "--diag") || HasArg(args, "--diagnose"))
            {
                return true;
            }
            return Environment.UserInteractive && !Console.IsOutputRedirected;
        }
        static bool ShouldRunPipeMode(string[] args)
        {
            return HasArg(args, "--pipe");
        }
        static bool HasArg(string[] args, string expected)
        {
            if (args == null || args.Length == 0)
            {
                return false;
            }

            return args.Any(arg => string.Equals(arg, expected, StringComparison.OrdinalIgnoreCase));
        }

        static void RunConsoleDiagnostics()
        {
            Console.WriteLine("FanControl TempBridge 诊断模式");
            Console.WriteLine($"时间: {DateTime.Now:yyyy-MM-dd HH:mm:ss}");
            Console.WriteLine();

            InitializeHardwareMonitor();

            TemperatureData data = GetTemperatureData(new TemperatureSelection());
            PrintTemperatureSummary(data);
            Console.WriteLine();
            PrintHardwareSnapshot();

            if (!data.Success)
            {
                Environment.Exit(1);
            }
        }

        static void PrintTemperatureSummary(TemperatureData data)
        {
            Console.WriteLine("温度结果");
            Console.WriteLine($"CPU: {FormatTemperature(data.CpuTemp)}");
            Console.WriteLine($"GPU: {FormatTemperature(data.GpuTemp)}");
            Console.WriteLine($"GPU State: {data.GpuReadState}");
            Console.WriteLine($"MAX: {FormatTemperature(data.MaxTemp)}");
            Console.WriteLine($"Success: {data.Success}");

            if (!string.IsNullOrEmpty(data.Error))
            {
                Console.WriteLine($"Error: {data.Error}");
            }
        }

        static string FormatTemperature(int value)
        {
            return value > 0 ? value + "°C" : "N/A";
        }

        static void PrintHardwareSnapshot()
        {
            Console.WriteLine("温度传感器快照");

            if (computer == null)
            {
                Console.WriteLine("- LibreHardwareMonitor 未初始化，已尝试使用 Windows 温区兜底读取 CPU 温度");
                if (!string.IsNullOrWhiteSpace(lastHardwareMonitorError))
                {
                    Console.WriteLine("- 初始化信息: " + lastHardwareMonitorError);
                }
                return;
            }

            bool foundAny = false;
            foreach (IHardware hardware in computer.Hardware)
            {
                foundAny |= PrintHardwareSnapshotRecursive(hardware, 0);
            }

            if (!foundAny)
            {
                Console.WriteLine("- 未发现可用的温度传感器");
            }
        }

        static bool PrintHardwareSnapshotRecursive(IHardware hardware, int indentLevel)
        {
            bool wroteLine = false;
            string indent = new string(' ', indentLevel * 2);

            foreach (ISensor sensor in hardware.Sensors)
            {
                if (sensor.SensorType != SensorType.Temperature)
                {
                    continue;
                }

                string valueText = sensor.Value.HasValue
                    ? sensor.Value.Value.ToString("F1") + "°C"
                    : "N/A";
                Console.WriteLine(
                    string.Format(
                        "{0}- [{1}] {2} / {3}: {4}",
                        indent,
                        hardware.HardwareType,
                        hardware.Name,
                        sensor.Name,
                        valueText
                    )
                );
                wroteLine = true;
            }

            foreach (IHardware subHardware in hardware.SubHardware)
            {
                if (PrintHardwareSnapshotRecursive(subHardware, indentLevel + 1))
                {
                    wroteLine = true;
                }
            }

            return wroteLine;
        }

        static void InitializeHardwareMonitor(bool includeGpu = false)
        {
            ResetIntelMsrFallback();
            var pawnIoStopwatch = Stopwatch.StartNew();
            string pawnIoMessage = EnsurePawnIoReady();
            LogInitProgress(string.Format("PawnIO check completed in {0} ms{1}", pawnIoStopwatch.ElapsedMilliseconds,
                string.IsNullOrWhiteSpace(pawnIoMessage) ? string.Empty : ": " + pawnIoMessage));

            Exception lastException = null;
            for (int attempt = 1; attempt <= MaxInitRetries; attempt++)
            {
                var attemptStopwatch = Stopwatch.StartNew();
                try
                {
                    if (computer != null)
                    {
                        try { computer.Close(); } catch { }
                        computer = null;
                    }

                    LogInitProgress(string.Format("LibreHardwareMonitor init attempt {0}/{1} (gpu={2})", attempt, MaxInitRetries, includeGpu ? "on" : "off"));

                    computer = new Computer
                    {
                        IsCpuEnabled = true,
                        IsGpuEnabled = includeGpu,
                        IsMemoryEnabled = false,
                        IsMotherboardEnabled = false,
                        IsControllerEnabled = false,
                        IsNetworkEnabled = false,
                        IsStorageEnabled = false
                    };
                    currentGpuMonitoringEnabled = includeGpu;

                    computer.Open();
                    computer.Accept(new UpdateVisitor());

                    bool hasAnyTemperature = HasAnyTemperatureSensor(computer);
                    bool hasCpuTemperature = HasCpuSensor(computer, SensorType.Temperature);
                    bool hasCpuPower = HasCpuSensor(computer, SensorType.Power);
                    string intelCpuModel = FindIntelCpuModel(computer);

                    if (!string.IsNullOrEmpty(intelCpuModel) && (!hasCpuTemperature || !hasCpuPower))
                    {
                        string msrMessage;
                        bool msrReady = EnsureIntelMsrFallback(intelCpuModel, out msrMessage);
                        if (msrReady)
                        {
                            if (!hasCpuTemperature)
                            {
                                hasAnyTemperature = true;
                                LogInitProgress("LibreHardwareMonitor CPU sensors missing; validated Intel MSR fallback enabled for " + intelCpuModel);
                            }
                            if (!hasCpuPower)
                            {
                                LogInitProgress(intelPackageEnergyUnit > 0
                                    ? "LibreHardwareMonitor CPU power sensor missing; validated Intel RAPL fallback enabled for " + intelCpuModel
                                    : "Intel temperature MSR is readable, but package RAPL power is unavailable for " + intelCpuModel);
                            }
                        }
                        else if (!hasCpuTemperature)
                        {
                            lastException = new InvalidOperationException(msrMessage);
                            if (ShouldRetryIntelMsrProbe(attempt, msrMessage))
                            {
                                LogInitProgress(string.Format(
                                    "Intel MSR probe unavailable on attempt {0}/{1}; releasing handles before retry: {2}",
                                    attempt,
                                    MaxInitRetries,
                                    msrMessage));
                                try { computer.Close(); } catch { }
                                computer = null;
                                ResetIntelMsrFallback();
                                Thread.Sleep(InitRetryDelayMs);
                                continue;
                            }

                            if (IsPermanentMsrFailure(msrMessage))
                            {
                                lastHardwareMonitorError = msrMessage;
                                LogInitProgress("Intel CPU sensor access is permanently unavailable; automatic reinitialization disabled: " + msrMessage);
                                TrimWorkingSetIfIdle(true);
                                return;
                            }
                            LogInitProgress("Intel CPU sensor access is temporarily unavailable; delayed recovery remains enabled: " + msrMessage);
                        }
                        else
                        {
                            LogInitProgress("Intel RAPL fallback unavailable: " + msrMessage);
                        }
                    }

                    // A validated raw Intel package sensor is sufficient when LHM omits CPU sensors.
                    if (hasAnyTemperature)
                    {
                        consecutiveFailures = 0;
                        lastHardwareMonitorError = string.Empty;
                        LogInitProgress(string.Format("temperature sensors found on attempt {0} in {1} ms (gpu={2})",
                            attempt, attemptStopwatch.ElapsedMilliseconds, includeGpu ? "on" : "off"));
                        TrimWorkingSetIfIdle(true);
                        return;
                    }

                    if (lastException == null)
                        lastException = new InvalidOperationException("LibreHardwareMonitor 未发现有效温度传感器");

                    LogInitProgress(string.Format("attempt {0} found no valid temperature sensors in {1} ms",
                        attempt, attemptStopwatch.ElapsedMilliseconds));

                    // No sensors found - PawnIO may not be fully ready
                    if (attempt < MaxInitRetries)
                    {
                        computer.Close();
                        computer = null;
                        Thread.Sleep(InitRetryDelayMs);
                    }
                }
                catch (Exception ex)
                {
                    lastException = ex;
                    LogInitProgress(string.Format("attempt {0} failed in {1} ms: {2}",
                        attempt, attemptStopwatch.ElapsedMilliseconds, ex.Message));

                    try { computer?.Close(); } catch { }
                    computer = null;

                    string intelCpuModel = FindIntelCpuModel(null);
                    if (!string.IsNullOrEmpty(intelCpuModel))
                    {
                        string msrMessage;
                        if (EnsureIntelMsrFallback(intelCpuModel, out msrMessage))
                        {
                            consecutiveFailures = 0;
                            lastHardwareMonitorError = "LibreHardwareMonitor unavailable: " + ex.Message;
                            LogInitProgress("LibreHardwareMonitor initialization failed; validated Intel MSR fallback enabled for " + intelCpuModel);
                            TrimWorkingSetIfIdle(true);
                            return;
                        }

                        lastException = new InvalidOperationException(ex.Message + "; " + msrMessage);
                        if (IsPermanentMsrFailure(msrMessage))
                        {
                            LogInitProgress("Intel CPU sensor access is permanently unavailable; automatic reinitialization disabled: " + msrMessage);
                            break;
                        }
                        if (ShouldRetryIntelMsrProbe(attempt, msrMessage))
                        {
                            LogInitProgress(string.Format(
                                "Intel MSR probe unavailable after LHM failure on attempt {0}/{1}; releasing handles before retry: {2}",
                                attempt,
                                MaxInitRetries,
                                msrMessage));
                            ResetIntelMsrFallback();
                            Thread.Sleep(InitRetryDelayMs);
                            continue;
                        }
                        LogInitProgress("Intel CPU sensor access is temporarily unavailable; delayed recovery remains enabled: " + msrMessage);
                    }
                    else if (attempt < MaxInitRetries)
                    {
                        Thread.Sleep(InitRetryDelayMs);
                    }
                }
            }

            // All immediate retries are exhausted. Retryable failures are recovered later
            // through the existing bounded monitor/bridge recovery path.
            lastHardwareMonitorError = BuildHardwareMonitorError(lastException, pawnIoMessage);
            LogInitProgress(string.Format("hardware monitor initialized without valid sensors (gpu={0}); fallback will be used: {1}", includeGpu ? "on" : "off", lastHardwareMonitorError));
            TrimWorkingSetIfIdle(true);
        }

        static string BuildHardwareMonitorError(Exception exception, string pawnIoMessage)
        {
            var parts = new System.Collections.Generic.List<string>();
            if (exception != null && !string.IsNullOrWhiteSpace(exception.Message))
            {
                parts.Add(exception.Message);
            }
            if (!string.IsNullOrWhiteSpace(pawnIoMessage))
            {
                parts.Add(pawnIoMessage);
            }
            if (parts.Count == 0)
            {
                parts.Add("LibreHardwareMonitor 暂未返回有效温度传感器");
            }
            return string.Join("；", parts.ToArray());
        }

        static bool HasAnyTemperatureSensor(Computer comp)
        {
            foreach (IHardware hardware in comp.Hardware)
            {
                if (HasTemperatureSensorRecursive(hardware))
                    return true;
            }
            return false;
        }

        static bool HasTemperatureSensorRecursive(IHardware hardware)
        {
            foreach (ISensor sensor in hardware.Sensors)
            {
                if (sensor.SensorType == SensorType.Temperature && sensor.Value.HasValue && sensor.Value.Value > 0)
                    return true;
            }
            foreach (IHardware sub in hardware.SubHardware)
            {
                if (HasTemperatureSensorRecursive(sub))
                    return true;
            }
            return false;
        }

        static bool HasCpuSensor(Computer comp, SensorType sensorType)
        {
            foreach (IHardware hardware in comp.Hardware)
            {
                if (hardware.HardwareType == HardwareType.Cpu && HasSensorRecursive(hardware, sensorType))
                    return true;
            }
            return false;
        }

        static bool HasSensorRecursive(IHardware hardware, SensorType sensorType)
        {
            foreach (ISensor sensor in hardware.Sensors)
            {
                if (sensor.SensorType == sensorType && sensor.Value.HasValue)
                {
                    if (sensorType != SensorType.Temperature || (sensor.Value.Value > 0 && sensor.Value.Value < MaxReasonableTemperature))
                        return true;
                }
            }
            foreach (IHardware sub in hardware.SubHardware)
            {
                if (HasSensorRecursive(sub, sensorType))
                    return true;
            }
            return false;
        }

        static string FindIntelCpuModel(Computer comp)
        {
            if (comp != null)
            {
                foreach (IHardware hardware in comp.Hardware)
                {
                    string name = hardware.Name ?? string.Empty;
                    if (hardware.HardwareType == HardwareType.Cpu && IsIntelCpuModel(name))
                        return name;
                }
            }
            return SelectIntelCpuModel(string.Empty, windowsCpuModel);
        }

        static string ReadWindowsCpuModel()
        {
            try
            {
                using (var key = Microsoft.Win32.Registry.LocalMachine.OpenSubKey(@"HARDWARE\DESCRIPTION\System\CentralProcessor\0"))
                {
                    object value = key?.GetValue("ProcessorNameString");
                    return value == null ? string.Empty : value.ToString().Trim();
                }
            }
            catch
            {
                return string.Empty;
            }
        }

        static bool IsIntelCpuModel(string model)
        {
            return !string.IsNullOrWhiteSpace(model) &&
                model.IndexOf("Intel", StringComparison.OrdinalIgnoreCase) >= 0;
        }

        static string SelectIntelCpuModel(string primary, string fallback)
        {
            if (IsIntelCpuModel(primary))
                return primary.Trim();
            return IsIntelCpuModel(fallback) ? fallback.Trim() : string.Empty;
        }

        static string EnsurePawnIoReady()
        {
            if (!PawnIo.IsInstalled)
            {
                return "PawnIO 驱动未安装，LibreHardwareMonitor 的部分 CPU 传感器可能不可用；已继续使用 Windows 温区兜底";
            }

            // Check PawnIO driver service is running; start it only when stopped.
            // Do not stop/restart it here because other hardware tools may share the same driver.
            try
            {
                using (var sc = new ServiceController("PawnIO"))
                {
                    if (sc.Status != ServiceControllerStatus.Running)
                    {
                        sc.Start();
                        sc.WaitForStatus(ServiceControllerStatus.Running, TimeSpan.FromSeconds(3));
                    }
                }
            }
            catch (InvalidOperationException)
            {
                // Service not found - PawnIO may use a different service name, continue
            }
            catch (System.ServiceProcess.TimeoutException)
            {
                return "PawnIO 服务启动超时，可能正被系统或其它硬件监控工具占用；已继续使用兼容温度读取";
            }
            catch (Exception ex)
            {
                return "PawnIO 服务检查失败: " + ex.Message;
            }

            return string.Empty;
        }

        static bool EnsureIntelMsrFallback(string cpuModel, out string message)
        {
            if (intelMsrProbeAttempted)
            {
                message = intelMsrDiagnostic;
                if (intelMsrFallback != null)
                    return true;
                if (IsPermanentMsrFailure(message) ||
                    !IsRetryCooldownElapsed(lastIntelMsrProbeAt, DateTime.UtcNow, IntelMsrProbeRetryCooldownSeconds))
                    return false;
            }

            intelMsrProbeAttempted = true;
            lastIntelMsrProbeAt = DateTime.UtcNow;
            if (!PawnIo.IsInstalled)
            {
                intelMsrDiagnostic = MsrUnavailableMarker + " PawnIO is not installed; Intel CPU MSR sensors are unavailable";
                message = intelMsrDiagnostic;
                return false;
            }

            IntelMsr candidate = null;
            try
            {
                candidate = new IntelMsr();
                uint target;
                uint packageStatus;
                uint coreStatus;
                uint ignored;
                bool targetRead = candidate.ReadMsr(IntelTemperatureTargetMsr, out target, out ignored);
                bool packageRead = candidate.ReadMsr(IntelPackageThermalStatusMsr, out packageStatus, out ignored);
                bool coreRead = candidate.ReadMsr(IntelThermalStatusMsr, out coreStatus, out ignored);
                int temperature;
                if (!targetRead || !TryDecodeIntelTemperature(
                    target,
                    packageRead ? packageStatus : 0,
                    coreRead ? coreStatus : 0,
                    out temperature))
                {
                    candidate.Close();
                    intelMsrDiagnostic = string.Format(
                        "{0} PawnIO is registered but Intel MSR reads are temporarily invalid for {1} (target=0x{2:X8}, package=0x{3:X8}, core=0x{4:X8})",
                        MsrRetryableMarker,
                        cpuModel,
                        target,
                        packageStatus,
                        coreStatus);
                    message = intelMsrDiagnostic;
                    return false;
                }

                intelMsrFallback = candidate;
                consecutiveIntelMsrTemperatureFailures = 0;
                consecutiveIntelMsrPowerFailures = 0;
                TryInitializeIntelRaplFallback(DateTime.UtcNow);

                intelMsrDiagnostic = string.Empty;
                message = string.Empty;
                return true;
            }
            catch (Exception ex)
            {
                try { candidate?.Close(); } catch { }
                intelMsrFallback = null;
                intelMsrDiagnostic = MsrRetryableMarker + " PawnIO Intel MSR module could not be opened: " + ex.Message;
                message = intelMsrDiagnostic;
                return false;
            }
        }

        static bool TryReadIntelMsrTemperature(out int temperature)
        {
            temperature = 0;
            if (intelMsrFallback == null)
                return false;

            bool success = false;
            try
            {
                uint target;
                uint packageStatus;
                uint coreStatus;
                uint ignored;
                bool targetRead = intelMsrFallback.ReadMsr(IntelTemperatureTargetMsr, out target, out ignored);
                bool packageRead = intelMsrFallback.ReadMsr(IntelPackageThermalStatusMsr, out packageStatus, out ignored);
                bool coreRead = intelMsrFallback.ReadMsr(IntelThermalStatusMsr, out coreStatus, out ignored);
                success = targetRead && TryDecodeIntelTemperature(
                    target,
                    packageRead ? packageStatus : 0,
                    coreRead ? coreStatus : 0,
                    out temperature);
            }
            catch
            {
                success = false;
            }

            if (success)
            {
                consecutiveIntelMsrTemperatureFailures = 0;
                return true;
            }

            consecutiveIntelMsrTemperatureFailures++;
            if (HasReachedFailureThreshold(consecutiveIntelMsrTemperatureFailures, IntelMsrRuntimeFailureThreshold))
                InvalidateIntelMsrFallback("Intel MSR temperature reads failed repeatedly", DateTime.UtcNow);
            return false;
        }

        static bool TryDecodeIntelTemperature(uint target, uint packageStatus, uint coreStatus, out int temperature)
        {
            temperature = 0;
            int tjMax = (int)((target >> 16) & 0xFF);
            if (tjMax < 50 || tjMax > MaxReasonableTemperature)
                return false;

            uint thermalStatus = (packageStatus & 0x80000000u) != 0 ? packageStatus : coreStatus;
            if ((thermalStatus & 0x80000000u) == 0)
                return false;

            int delta = (int)((thermalStatus >> 16) & 0x7F);
            int decoded = tjMax - delta;
            if (decoded <= 0 || decoded >= MaxReasonableTemperature)
                return false;

            temperature = decoded;
            return true;
        }

        static bool TryReadIntelMsrPower(out double watts)
        {
            watts = 0;
            if (intelMsrFallback == null || intelPackageEnergyUnit <= 0 || lastIntelPackageEnergyAt == DateTime.MinValue)
                return false;

            uint currentEnergy = 0;
            uint ignored;
            bool readSucceeded = false;
            try
            {
                readSucceeded = intelMsrFallback.ReadMsr(IntelPackageEnergyStatusMsr, out currentEnergy, out ignored);
            }
            catch
            {
                readSucceeded = false;
            }

            if (!readSucceeded)
            {
                consecutiveIntelMsrPowerFailures++;
                if (HasReachedFailureThreshold(consecutiveIntelMsrPowerFailures, IntelMsrRuntimeFailureThreshold))
                    InvalidateIntelRaplFallback(DateTime.UtcNow);
                return false;
            }

            DateTime now = DateTime.UtcNow;
            double seconds = (now - lastIntelPackageEnergyAt).TotalSeconds;
            uint previousEnergy = lastIntelPackageEnergy;
            lastIntelPackageEnergy = currentEnergy;
            lastIntelPackageEnergyAt = now;
            if (TryCalculateIntelPackagePower(previousEnergy, currentEnergy, intelPackageEnergyUnit, seconds, out watts))
            {
                consecutiveIntelMsrPowerFailures = 0;
                return true;
            }

            // A first/reprobe sample or a long suspend gap only establishes a new baseline.
            if (seconds < 0.01 || seconds > 30)
            {
                consecutiveIntelMsrPowerFailures = 0;
                return false;
            }

            consecutiveIntelMsrPowerFailures++;
            if (HasReachedFailureThreshold(consecutiveIntelMsrPowerFailures, IntelMsrRuntimeFailureThreshold))
                InvalidateIntelRaplFallback(now);
            return false;
        }

        static bool TryCalculateIntelPackagePower(uint previousEnergy, uint currentEnergy, double energyUnit, double seconds, out double watts)
        {
            watts = 0;
            if (energyUnit <= 0 || seconds < 0.01 || seconds > 30)
                return false;

            uint energyDelta = unchecked(currentEnergy - previousEnergy);
            double value = energyDelta * energyUnit / seconds;
            if (double.IsNaN(value) || double.IsInfinity(value) || value < 0 || value > 1000)
                return false;

            watts = Math.Round(value, 1);
            return true;
        }

        static bool TryInitializeIntelRaplFallback(DateTime now)
        {
            if (intelMsrFallback == null)
                return false;
            if (intelPackageEnergyUnit > 0 && lastIntelPackageEnergyAt != DateTime.MinValue)
                return true;
            if (!IsRetryCooldownElapsed(lastIntelRaplProbeAt, now, IntelRaplProbeRetryCooldownSeconds))
                return false;

            lastIntelRaplProbeAt = now;
            try
            {
                uint raplUnit;
                uint packageEnergy;
                uint ignored;
                if (!intelMsrFallback.ReadMsr(IntelRaplPowerUnitMsr, out raplUnit, out ignored) || raplUnit == 0 ||
                    !intelMsrFallback.ReadMsr(IntelPackageEnergyStatusMsr, out packageEnergy, out ignored))
                    return false;

                int energyExponent = (int)((raplUnit >> 8) & 0x1F);
                intelPackageEnergyUnit = Math.Pow(0.5, energyExponent);
                lastIntelPackageEnergy = packageEnergy;
                lastIntelPackageEnergyAt = now;
                consecutiveIntelMsrPowerFailures = 0;
                return true;
            }
            catch
            {
                return false;
            }
        }

        static bool IsRetryCooldownElapsed(DateTime lastAttempt, DateTime now, int cooldownSeconds)
        {
            return lastAttempt == DateTime.MinValue ||
                cooldownSeconds <= 0 ||
                now >= lastAttempt.AddSeconds(cooldownSeconds);
        }

        static bool HasReachedFailureThreshold(int failures, int threshold)
        {
            return threshold > 0 && failures >= threshold;
        }

        static bool IsPermanentMsrFailure(string message)
        {
            return !string.IsNullOrEmpty(message) &&
                message.IndexOf(MsrUnavailableMarker, StringComparison.OrdinalIgnoreCase) >= 0;
        }

        static bool IsRetryableMsrFailure(string message)
        {
            return !string.IsNullOrEmpty(message) &&
                message.IndexOf(MsrRetryableMarker, StringComparison.OrdinalIgnoreCase) >= 0;
        }

        static bool ShouldRetryIntelMsrProbe(int attempt, string message)
        {
            return attempt < MaxInitRetries && !IsPermanentMsrFailure(message);
        }

        static void InvalidateIntelMsrFallback(string reason, DateTime now)
        {
            try { intelMsrFallback?.Close(); } catch { }
            intelMsrFallback = null;
            intelMsrProbeAttempted = true;
            intelMsrDiagnostic = MsrRetryableMarker + " " + reason;
            consecutiveIntelMsrTemperatureFailures = 0;
            consecutiveIntelMsrPowerFailures = 0;
            lastIntelMsrProbeAt = now;
            lastIntelRaplProbeAt = DateTime.MinValue;
            intelPackageEnergyUnit = 0;
            lastIntelPackageEnergy = 0;
            lastIntelPackageEnergyAt = DateTime.MinValue;
        }

        static void InvalidateIntelRaplFallback(DateTime now)
        {
            consecutiveIntelMsrPowerFailures = 0;
            lastIntelRaplProbeAt = now;
            intelPackageEnergyUnit = 0;
            lastIntelPackageEnergy = 0;
            lastIntelPackageEnergyAt = DateTime.MinValue;
        }

        static void ResetIntelMsrFallback()
        {
            try { intelMsrFallback?.Close(); } catch { }
            intelMsrFallback = null;
            intelMsrProbeAttempted = false;
            intelMsrDiagnostic = string.Empty;
            consecutiveIntelMsrTemperatureFailures = 0;
            consecutiveIntelMsrPowerFailures = 0;
            lastIntelMsrProbeAt = DateTime.MinValue;
            lastIntelRaplProbeAt = DateTime.MinValue;
            intelPackageEnergyUnit = 0;
            lastIntelPackageEnergy = 0;
            lastIntelPackageEnergyAt = DateTime.MinValue;
        }

        static void RunIntelMsrSelfTest()
        {
            int temperature;
            uint target = 100u << 16;
            uint thermal = 0x80000000u | (35u << 16);
            if (!TryDecodeIntelTemperature(target, thermal, 0, out temperature) || temperature != 65)
                throw new InvalidOperationException("Intel MSR temperature decoder self-test failed");
            if (TryDecodeIntelTemperature(0, 0, 0, out temperature))
                throw new InvalidOperationException("Intel MSR zero-read rejection self-test failed");
            string retryable = MsrRetryableMarker + " temporary read failure";
            string permanent = MsrUnavailableMarker + " driver missing";
            if (!ShouldRetryIntelMsrProbe(1, retryable) || !ShouldRetryIntelMsrProbe(2, retryable) ||
                ShouldRetryIntelMsrProbe(3, retryable) || ShouldRetryIntelMsrProbe(1, permanent))
                throw new InvalidOperationException("Intel MSR retry budget self-test failed");
            if (!IsPermanentMsrFailure(permanent) || IsPermanentMsrFailure(retryable) || !IsRetryableMsrFailure(retryable))
                throw new InvalidOperationException("Intel MSR failure classification self-test failed");
            string systemOnlyModel = "Intel(R) Core(TM) Ultra 9 275HX";
            if (SelectIntelCpuModel(string.Empty, systemOnlyModel) != systemOnlyModel ||
                SelectIntelCpuModel(string.Empty, "AMD Ryzen") != string.Empty)
                throw new InvalidOperationException("Intel system CPU fallback entry self-test failed");

            double watts;
            if (!TryCalculateIntelPackagePower(100, 819300, 1.0 / 16384.0, 1, out watts) || Math.Abs(watts - 50) > 0.1)
                throw new InvalidOperationException("Intel RAPL power decoder self-test failed");
            if (TryCalculateIntelPackagePower(100, 200, 1.0 / 16384.0, 0, out watts) ||
                TryCalculateIntelPackagePower(100, 200, 1.0 / 16384.0, 31, out watts))
                throw new InvalidOperationException("Intel RAPL baseline self-test failed");

            DateTime now = new DateTime(2026, 1, 1, 0, 0, 30, DateTimeKind.Utc);
            DateTime recentMsrProbe = now.AddSeconds(-IntelMsrProbeRetryCooldownSeconds + 1);
            DateTime dueMsrProbe = now.AddSeconds(-IntelMsrProbeRetryCooldownSeconds);
            if (!IsRetryCooldownElapsed(DateTime.MinValue, now, IntelMsrProbeRetryCooldownSeconds) ||
                IsRetryCooldownElapsed(recentMsrProbe, now, IntelMsrProbeRetryCooldownSeconds) ||
                !IsRetryCooldownElapsed(dueMsrProbe, now, IntelMsrProbeRetryCooldownSeconds))
                throw new InvalidOperationException("Intel MSR retry cooldown self-test failed");
            DateTime recentRaplProbe = now.AddSeconds(-IntelRaplProbeRetryCooldownSeconds + 1);
            DateTime dueRaplProbe = now.AddSeconds(-IntelRaplProbeRetryCooldownSeconds);
            if (IsRetryCooldownElapsed(recentRaplProbe, now, IntelRaplProbeRetryCooldownSeconds) ||
                !IsRetryCooldownElapsed(dueRaplProbe, now, IntelRaplProbeRetryCooldownSeconds))
                throw new InvalidOperationException("Intel RAPL retry cooldown self-test failed");
            if (HasReachedFailureThreshold(IntelMsrRuntimeFailureThreshold - 1, IntelMsrRuntimeFailureThreshold) ||
                !HasReachedFailureThreshold(IntelMsrRuntimeFailureThreshold, IntelMsrRuntimeFailureThreshold))
                throw new InvalidOperationException("Intel MSR runtime failure threshold self-test failed");
            if (IsTemperatureReadSuccessful(0, 55, permanent) ||
                !IsTemperatureReadSuccessful(0, 55, retryable) ||
                !IsTemperatureReadSuccessful(60, 0, permanent))
                throw new InvalidOperationException("Intel permanent CPU failure visibility self-test failed");

            DateTime preservedMsrProbe = now.AddSeconds(-5);
            intelMsrProbeAttempted = true;
            intelMsrDiagnostic = retryable;
            lastIntelMsrProbeAt = preservedMsrProbe;
            intelPackageEnergyUnit = 1.0 / 16384.0;
            lastIntelPackageEnergyAt = now.AddSeconds(-1);
            InvalidateIntelRaplFallback(now);
            if (!intelMsrProbeAttempted || intelMsrDiagnostic != retryable || lastIntelMsrProbeAt != preservedMsrProbe ||
                lastIntelRaplProbeAt != now || intelPackageEnergyUnit != 0 || lastIntelPackageEnergyAt != DateTime.MinValue)
                throw new InvalidOperationException("Intel RAPL-only invalidation self-test failed");
            ResetIntelMsrFallback();

            Console.WriteLine("MSR self-test OK");
        }

        static void ReinitializeHardwareMonitor()
        {
            lock (lockObject)
            {
                try
                {
                    computer?.Close();
                }
                catch { }
                computer = null;

                // Wait briefly after releasing handles. Avoid stopping PawnIO because other tools may share it.
                Thread.Sleep(250);

                InitializeHardwareMonitor(currentGpuMonitoringEnabled);
            }
        }

        static void EnsureHardwareMonitorGpuMode(bool includeGpu)
        {
            if ((computer != null || intelMsrProbeAttempted) && currentGpuMonitoringEnabled == includeGpu)
            {
                return;
            }

            var stopwatch = Stopwatch.StartNew();
            try
            {
                computer?.Close();
            }
            catch { }
            computer = null;
            Thread.Sleep(120);
            InitializeHardwareMonitor(includeGpu);
            if (stopwatch.ElapsedMilliseconds >= 250)
            {
                LogInitProgress(string.Format("LibreHardwareMonitor GPU mode changed to {0} in {1} ms", includeGpu ? "on" : "off", stopwatch.ElapsedMilliseconds));
            }
        }

        static bool ShouldPollGpu(TemperatureSelection selection, out string gpuReadState)
        {
            gpuReadState = GpuReadStateActive;
            if (selection == null)
            {
                return true;
            }

            if (string.Equals(selection.GpuReadMode, "always", StringComparison.OrdinalIgnoreCase) || !selection.GpuLowPowerProtection)
            {
                return true;
            }

            if (IsGpuReadExplicitlyRequested(selection))
            {
                return true;
            }

            var profile = GetHardwareProfile();
            bool shouldPoll = gpuReadCoordinator.ShouldPollGpu(profile, LogInitProgress);
            if (shouldPoll)
            {
                gpuReadState = GpuReadStateActive;
                return true;
            }

            gpuReadState = GpuReadStateNotPolled;
            return false;
        }

        static bool IsGpuReadExplicitlyRequested(TemperatureSelection selection)
        {
            if (selection == null)
            {
                return false;
            }
            if (string.Equals(selection.TempSource, "gpu", StringComparison.OrdinalIgnoreCase))
            {
                return true;
            }
            return false;
        }

        static bool IsAutoSelection(string value)
        {
            return string.IsNullOrWhiteSpace(value) || string.Equals(value, "auto", StringComparison.OrdinalIgnoreCase);
        }

        static HardwareProfile GetHardwareProfile()
        {
            lock (hardwareProfileLock)
            {
                if (cachedHardwareProfile != null)
                {
                    return cachedHardwareProfile;
                }

                cachedHardwareProfile = LoadOrRefreshHardwareProfile();
                return cachedHardwareProfile;
            }
        }

        static HardwareProfile LoadOrRefreshHardwareProfile()
        {
            string profilePath = GetHardwareProfilePath();
            string currentSignature = BuildCurrentGpuSignature();
            HardwareProfile profile = null;

            try
            {
                if (File.Exists(profilePath))
                {
                    profile = JsonConvert.DeserializeObject<HardwareProfile>(File.ReadAllText(profilePath));
                }
            }
            catch (Exception ex)
            {
                LogInitProgress("hardware profile load failed; refreshing: " + ex.Message);
            }

            if (profile == null ||
                profile.SchemaVersion != 1 ||
                string.IsNullOrWhiteSpace(profile.Signature) ||
                !string.Equals(profile.Signature, currentSignature, StringComparison.Ordinal))
            {
                profile = BuildHardwareProfile(currentSignature);
                SaveHardwareProfile(profilePath, profile);
            }

            return profile;
        }

        static string GetHardwareProfilePath()
        {
            string bridgeDir = AppDomain.CurrentDomain.BaseDirectory
                .TrimEnd(Path.DirectorySeparatorChar, Path.AltDirectorySeparatorChar);
            var bridgeInfo = new DirectoryInfo(bridgeDir);
            string appDir = string.Equals(bridgeInfo.Name, "bridge", StringComparison.OrdinalIgnoreCase) && bridgeInfo.Parent != null
                ? bridgeInfo.Parent.FullName
                : bridgeInfo.FullName;
            return Path.Combine(appDir, "config", "hardware-profile.json");
        }

        static void SaveHardwareProfile(string profilePath, HardwareProfile profile)
        {
            try
            {
                Directory.CreateDirectory(Path.GetDirectoryName(profilePath));
                File.WriteAllText(profilePath, JsonConvert.SerializeObject(profile, Formatting.Indented));
                LogInitProgress("hardware profile saved: " + profilePath);
            }
            catch (Exception ex)
            {
                LogInitProgress("hardware profile save failed: " + ex.Message);
            }
        }

        static HardwareProfile BuildHardwareProfile(string signature)
        {
            var devices = EnumerateHardwareGpuDevices().ToArray();
            var profile = new HardwareProfile
            {
                GeneratedAt = DateTimeOffset.UtcNow.ToString("o"),
                Signature = signature,
                GpuDevices = devices,
                PreferredGpuProbe = ResolvePreferredGpuProbe(devices)
            };
            return profile;
        }

        static string ResolvePreferredGpuProbe(HardwareGpuDevice[] devices)
        {
            if (devices.Any(device => device.Discrete && string.Equals(device.Vendor, "nvidia", StringComparison.OrdinalIgnoreCase)))
            {
                return "nvidia-compatible";
            }
            if (devices.Any(device => device.Discrete && string.Equals(device.Vendor, "amd", StringComparison.OrdinalIgnoreCase)))
            {
                return "amd-compatible";
            }
            if (devices.Any(device => string.Equals(device.Vendor, "intel", StringComparison.OrdinalIgnoreCase)))
            {
                return "intel-compatible";
            }
            return "windows";
        }

        static string BuildCurrentGpuSignature()
        {
            return BuildHardwareSignature(EnumerateHardwareGpuDevices());
        }

        static string BuildHardwareSignature(System.Collections.Generic.IEnumerable<HardwareGpuDevice> devices)
        {
            var parts = devices
                .Select(device => string.Join("|", new[] { device.Vendor ?? string.Empty, device.Name ?? string.Empty, device.PnpDeviceId ?? string.Empty, device.Discrete ? "1" : "0" }))
                .OrderBy(part => part, StringComparer.OrdinalIgnoreCase)
                .ToArray();
            return string.Join(";", parts);
        }

        static System.Collections.Generic.IEnumerable<HardwareGpuDevice> EnumerateHardwareGpuDevices()
        {
            using (var searcher = new ManagementObjectSearcher("SELECT Name, AdapterRAM, PNPDeviceID FROM Win32_VideoController"))
            using (var results = searcher.Get())
            {
                foreach (ManagementObject item in results)
                {
                    string name = Convert.ToString(item["Name"]) ?? string.Empty;
                    string pnp = Convert.ToString(item["PNPDeviceID"]) ?? string.Empty;
                    ulong adapterRam = ReadUInt64(item["AdapterRAM"]);
                    if (IsVirtualVideoController(name, pnp))
                    {
                        continue;
                    }

                    string vendor = DetectGpuVendor(name, pnp);
                    yield return new HardwareGpuDevice
                    {
                        Name = name,
                        Vendor = vendor,
                        PnpDeviceId = pnp,
                        Discrete = IsDiscreteVideoController(name, pnp, adapterRam)
                    };
                }
            }
        }

        static void ReconcileHardwareProfileWithSensors(System.Collections.Generic.IEnumerable<GpuCandidate> gpuCandidates)
        {
            if (gpuCandidates == null)
            {
                return;
            }

            var detectedDevices = gpuCandidates
                .Select(candidate => new HardwareGpuDevice
                {
                    Name = candidate.Model ?? string.Empty,
                    Vendor = candidate.Vendor ?? string.Empty,
                    PnpDeviceId = candidate.Key ?? string.Empty,
                    Discrete = IsDiscreteSensorVendor(candidate.Vendor)
                })
                .Where(device => !string.IsNullOrWhiteSpace(device.Name) || !string.IsNullOrWhiteSpace(device.PnpDeviceId))
                .ToArray();

            if (detectedDevices.Length == 0)
            {
                return;
            }

            string sensorSignature = BuildHardwareSignature(detectedDevices);
            lock (hardwareProfileLock)
            {
                var profile = cachedHardwareProfile ?? LoadOrRefreshHardwareProfile();
                if (string.Equals(profile.Signature, sensorSignature, StringComparison.Ordinal))
                {
                    cachedHardwareProfile = profile;
                    return;
                }

                profile = BuildHardwareProfileFromSensorDevices(detectedDevices, sensorSignature);
                cachedHardwareProfile = profile;
                SaveHardwareProfile(GetHardwareProfilePath(), profile);
            }
        }

        static HardwareProfile BuildHardwareProfileFromSensorDevices(HardwareGpuDevice[] devices, string signature)
        {
            var profile = new HardwareProfile
            {
                GeneratedAt = DateTimeOffset.UtcNow.ToString("o"),
                Signature = signature,
                GpuDevices = devices,
                PreferredGpuProbe = ResolvePreferredGpuProbe(devices)
            };
            return profile;
        }

        static TemperatureGpuDevice[] BuildTemperatureGpuDevicesFromHardwareProfile(HardwareProfile profile)
        {
            if (profile == null || profile.GpuDevices == null || profile.GpuDevices.Length == 0)
            {
                return Array.Empty<TemperatureGpuDevice>();
            }

            return profile.GpuDevices
                .Where(device => device != null && (!string.IsNullOrWhiteSpace(device.Name) || !string.IsNullOrWhiteSpace(device.PnpDeviceId)))
                .Select((device, index) => new TemperatureGpuDevice
                {
                    Key = !string.IsNullOrWhiteSpace(device.PnpDeviceId) ? device.PnpDeviceId : "hardware:" + index,
                    Name = device.Name ?? string.Empty,
                    Vendor = device.Vendor ?? string.Empty,
                    Sensors = Array.Empty<TemperatureSensor>(),
                    PowerSensors = Array.Empty<PowerSensor>()
                })
                .ToArray();
        }

        static string ResolveGpuModelFromHardwareProfile(HardwareProfile profile)
        {
            if (profile == null || profile.GpuDevices == null)
            {
                return string.Empty;
            }

            var preferred = profile.GpuDevices
                .Where(device => device != null && !string.IsNullOrWhiteSpace(device.Name))
                .OrderByDescending(device => device.Discrete)
                .FirstOrDefault();
            return preferred != null ? preferred.Name ?? string.Empty : string.Empty;
        }

        static bool IsDiscreteVideoController(string name, string pnpDeviceId, ulong adapterRam)
        {
            string text = ((name ?? string.Empty) + " " + (pnpDeviceId ?? string.Empty)).ToLowerInvariant();
            if (IsVirtualVideoController(name, pnpDeviceId))
            {
                return false;
            }
            if (text.Contains("nvidia") || text.Contains("ven_10de"))
            {
                return true;
            }
            if ((text.Contains("amd") || text.Contains("radeon") || text.Contains("ven_1002")) && !text.Contains("intel"))
            {
                return adapterRam >= 512UL * 1024UL * 1024UL;
            }
            return false;
        }

        static bool IsVirtualVideoController(string name, string pnpDeviceId)
        {
            string text = ((name ?? string.Empty) + " " + (pnpDeviceId ?? string.Empty)).ToLowerInvariant();
            return text.Contains("virtual") ||
                   text.Contains("microsoft basic") ||
                   text.Contains("idd") ||
                   text.Contains("indirect") ||
                   text.Contains("remotefx") ||
                   text.Contains("todesk");
        }

        static string DetectGpuVendor(string name, string pnpDeviceId)
        {
            string text = ((name ?? string.Empty) + " " + (pnpDeviceId ?? string.Empty)).ToLowerInvariant();
            if (text.Contains("nvidia") || text.Contains("ven_10de"))
            {
                return "nvidia";
            }
            if (text.Contains("amd") || text.Contains("radeon") || text.Contains("ven_1002"))
            {
                return "amd";
            }
            if (text.Contains("intel") || text.Contains("ven_8086"))
            {
                return "intel";
            }
            return "unknown";
        }

        static bool IsDiscreteSensorVendor(string vendor)
        {
            return string.Equals(vendor, "nvidia", StringComparison.OrdinalIgnoreCase) ||
                   string.Equals(vendor, "amd", StringComparison.OrdinalIgnoreCase);
        }

        static ulong ReadUInt64(object value)
        {
            if (value == null)
            {
                return 0;
            }
            try
            {
                return Convert.ToUInt64(value);
            }
            catch
            {
                return 0;
            }
        }

        /// <summary>
        /// Best-effort PawnIO service check. It starts the service only when it is stopped.
        /// It intentionally does not stop/restart PawnIO because other hardware tools may share it.
        /// </summary>
        static string RestartPawnIoDriver()
        {
            return EnsurePawnIoReady();
        }

        static void TrimWorkingSetIfIdle(bool force = false)
        {
            DateTime now = DateTime.UtcNow;
            if (!force && (now - lastMemoryTrimUtc).TotalSeconds < MemoryTrimIntervalSeconds)
            {
                return;
            }

            lastMemoryTrimUtc = now;

            try
            {
                GC.Collect(0, GCCollectionMode.Optimized, false);
                using (var currentProcess = Process.GetCurrentProcess())
                {
                    SetProcessWorkingSetSize(currentProcess.Handle, TrimWorkingSetSentinel, TrimWorkingSetSentinel);
                }
            }
            catch
            {
            }
        }

        static void StartPipeServer()
        {
            while (running)
            {
                try
                {
                    using (var pipeServer = new NamedPipeServerStream(PipeName, PipeDirection.InOut))
                    {
                        pipeServer.WaitForConnection();
                        using (var reader = new StreamReader(pipeServer))
                        using (var writer = new StreamWriter(pipeServer) { AutoFlush = true })
                        {
                            ServeCommandLoop(reader, writer);
                        }
                    }
                }
                catch (Exception ex)
                {
                    if (running)
                    {
                        Console.WriteLine($"Pipe error: {ex.Message}");
                        Thread.Sleep(1000);
                    }
                }
            }
        }
        static void ServeCommandLoop(TextReader reader, TextWriter writer)
        {
            while (running)
            {
                try
                {
                    string commandJson = reader.ReadLine();
                    if (commandJson == null)
                    {
                        break;
                    }
                    if (string.IsNullOrWhiteSpace(commandJson))
                    {
                        continue;
                    }
                    var command = JsonConvert.DeserializeObject<Command>(commandJson) ?? new Command();
                    var response = ProcessCommand(command);
                    string responseJson = JsonConvert.SerializeObject(response);
                    writer.WriteLine(responseJson);
                    writer.Flush();
                    TrimWorkingSetIfIdle();
                    if (string.Equals(command.Type, "Exit", StringComparison.Ordinal))
                    {
                        running = false;
                        break;
                    }
                }
                catch (Exception ex)
                {
                    var errorResponse = new Response
                    {
                        Success = false,
                        Error = ex.Message
                    };
                    string errorJson = JsonConvert.SerializeObject(errorResponse);
                    writer.WriteLine(errorJson);
                    writer.Flush();
                    break;
                }
            }
        }
        static Response ProcessCommand(Command command)
        {
            try
            {
                switch (command.Type)
                {
                    case "GetTemperature":
                        var selection = ParseTemperatureSelection(command.Data);
                        var data = GetTemperatureData(selection);
                        return new Response
                        {
                            Success = data.Success,
                            Error = data.Success ? string.Empty : data.Error,
                            Data = data
                        };

                    case "Ping":
                        return new Response
                        {
                            Success = true,
                            Data = new TemperatureData { Success = true }
                        };

                    case "RestartPawnIO":
                        return HandleRestartPawnIO();

                    case "Exit":
                        return new Response
                        {
                            Success = true
                        };

                    default:
                        return new Response
                        {
                            Success = false,
                            Error = "未知命令类型"
                        };
                }
            }
            catch (Exception ex)
            {
                return new Response
                {
                    Success = false,
                    Error = ex.Message
                };
            }
        }

        static Response HandleRestartPawnIO()
        {
            lock (lockObject)
            {
                // 1. Close existing Computer to release PawnIO handle
                try
                {
                    computer?.Close();
                }
                catch { }
                computer = null;

                // 2. Ensure PawnIO is running if it is stopped, then wait after handle release.
                string pawnIoMessage = RestartPawnIoDriver();
                Thread.Sleep(250);

                // 3. Reinitialize hardware monitor with a fresh handle.
                try
                {
                    InitializeHardwareMonitor();
                    consecutiveFailures = 0;

                    // 4. Do a test read to confirm it works or that fallback can supply data.
                    var testData = GetTemperatureDataUnsafe(new TemperatureSelection());
                    if (!testData.Success && !string.IsNullOrWhiteSpace(pawnIoMessage))
                    {
                        testData.Error = string.IsNullOrWhiteSpace(testData.Error)
                            ? pawnIoMessage
                            : testData.Error + "；" + pawnIoMessage;
                    }
                    return new Response
                    {
                        Success = testData.Success,
                        Error = testData.Success ? string.Empty : testData.Error,
                        Data = testData
                    };
                }
                catch (Exception ex)
                {
                    return new Response
                    {
                        Success = false,
                        Error = string.Format("重新初始化失败: {0}", ex.Message)
                    };
                }
            }
        }

        /// <summary>
        /// GetTemperatureData without acquiring lockObject (caller must hold the lock).
        /// </summary>
        static TemperatureSelection ParseTemperatureSelection(string raw)
        {
            if (string.IsNullOrWhiteSpace(raw))
            {
                return new TemperatureSelection();
            }

            try
            {
                return NormalizeTemperatureSelection(
                    JsonConvert.DeserializeObject<TemperatureSelection>(raw) ?? new TemperatureSelection()
                );
            }
            catch
            {
                return new TemperatureSelection();
            }
        }

        static TemperatureSelection NormalizeTemperatureSelection(TemperatureSelection selection)
        {
            if (selection == null)
            {
                return new TemperatureSelection();
            }

            selection.TempSource = NormalizeTempSource(selection.TempSource);
            selection.GpuDevice = NormalizeDeviceSelection(selection.GpuDevice);
            selection.CpuSensor = NormalizeSensorSelection(selection.CpuSensor);
            selection.GpuSensor = NormalizeSensorSelection(selection.GpuSensor);
            selection.CpuPowerSensor = NormalizeSensorSelection(selection.CpuPowerSensor);
            selection.GpuPowerSensor = NormalizeSensorSelection(selection.GpuPowerSensor);
            if (string.IsNullOrWhiteSpace(selection.GpuReadMode))
            {
                selection.GpuReadMode = selection.GpuLowPowerProtection ? "auto" : "always";
            }
            selection.GpuReadMode = NormalizeGpuReadMode(selection.GpuReadMode);
            selection.GpuLowPowerProtection = !string.Equals(selection.GpuReadMode, "always", StringComparison.OrdinalIgnoreCase);
            return selection;
        }

        static string NormalizeTempSource(string source)
        {
            if (string.Equals(source, "cpu", StringComparison.OrdinalIgnoreCase))
            {
                return "cpu";
            }
            if (string.Equals(source, "gpu", StringComparison.OrdinalIgnoreCase))
            {
                return "gpu";
            }
            return "max";
        }

        static string NormalizeGpuReadMode(string mode)
        {
            if (string.Equals(mode, "always", StringComparison.OrdinalIgnoreCase))
            {
                return "always";
            }
            return "auto";
        }

        static string NormalizeDeviceSelection(string deviceKey)
        {
            return string.IsNullOrWhiteSpace(deviceKey) ? "auto" : deviceKey;
        }

        static string NormalizeSensorSelection(string sensorKey)
        {
            return string.IsNullOrWhiteSpace(sensorKey) ? "auto" : sensorKey;
        }

        static TemperatureData GetTemperatureDataUnsafe(TemperatureSelection selection)
        {
            selection = NormalizeTemperatureSelection(selection);
            var result = new TemperatureData
            {
                UpdateTime = DateTimeOffset.UtcNow.ToUnixTimeSeconds(),
                ControlSource = selection.TempSource
            };

            string hardwareError = string.Empty;
            string cpuModel = string.Empty;
            double cpuPowerWatts = 0;
            string gpuModel = string.Empty;
            string gpuReadState = GpuReadStateUnavailable;
            var cpuSensors = new System.Collections.Generic.List<TemperatureSensor>();
            var cpuPowerSensors = new System.Collections.Generic.List<PowerSensor>();
            var gpuCandidates = new System.Collections.Generic.List<GpuCandidate>();
            int gpuIndex = 0;
            bool shouldPollGpu = ShouldPollGpu(selection, out gpuReadState);

            try
            {
                EnsureHardwareMonitorGpuMode(shouldPollGpu);

                if (computer != null)
                {
                    computer.Accept(new UpdateVisitor());

                    foreach (IHardware hardware in computer.Hardware)
                    {
                        if (hardware.HardwareType == HardwareType.Cpu)
                        {
                            if (cpuSensors.Count == 0)
                            {
                                cpuModel = hardware.Name ?? string.Empty;
                                CollectTemperatureSensors(hardware, "cpu", hardware.Name ?? string.Empty, string.Empty, cpuSensors);
                                CollectPowerSensors(hardware, "cpu", hardware.Name ?? string.Empty, string.Empty, cpuPowerSensors);
                                cpuPowerWatts = SelectPowerWatts(cpuPowerSensors, selection.CpuPowerSensor, new[] { "Package", "CPU Package", "Total", "Core" });
                            }
                        }
                        else if (shouldPollGpu &&
                                 (hardware.HardwareType == HardwareType.GpuNvidia ||
                                 hardware.HardwareType == HardwareType.GpuAmd ||
                                 hardware.HardwareType == HardwareType.GpuIntel))
                        {
                            var sensors = new System.Collections.Generic.List<TemperatureSensor>();
                            var powerSensors = new System.Collections.Generic.List<PowerSensor>();
                            CollectTemperatureSensors(hardware, "gpu", hardware.Name ?? string.Empty, string.Empty, sensors);
                            CollectPowerSensors(hardware, "gpu", hardware.Name ?? string.Empty, string.Empty, powerSensors);
                            gpuCandidates.Add(new GpuCandidate
                            {
                                Key = BuildGpuDeviceKey(hardware, gpuIndex),
                                Model = hardware.Name ?? string.Empty,
                                Vendor = GetGpuVendor(hardware.HardwareType),
                                HardwareType = hardware.HardwareType,
                                Sensors = sensors,
                                PowerSensors = powerSensors,
                                PowerWatts = SelectPowerWatts(powerSensors, selection.GpuPowerSensor, new[] { "GPU Power", "Total", "Package", "Board", "Chip" }),
                            });
                            gpuIndex++;
                        }
                    }
                }
                else
                {
                    hardwareError = lastHardwareMonitorError;
                }
            }
            catch (Exception ex)
            {
                hardwareError = ex.Message;
                lastHardwareMonitorError = ex.Message;
            }

            if (string.IsNullOrWhiteSpace(cpuModel))
                cpuModel = windowsCpuModel;

            if ((cpuSensors.Count == 0 || cpuPowerSensors.Count == 0) &&
                IsIntelCpuModel(cpuModel))
            {
                string msrMessage;
                if (EnsureIntelMsrFallback(cpuModel, out msrMessage))
                {
                    int msrTemperature;
                    if (cpuSensors.Count == 0 && TryReadIntelMsrTemperature(out msrTemperature))
                    {
                        cpuSensors.Add(new TemperatureSensor
                        {
                            Key = "cpu/msr/package-temperature",
                            Name = "CPU Package (MSR)",
                            Value = msrTemperature,
                        });
                    }

                    double msrPower;
                    if (cpuPowerSensors.Count == 0 &&
                        TryInitializeIntelRaplFallback(DateTime.UtcNow) &&
                        TryReadIntelMsrPower(out msrPower))
                    {
                        cpuPowerSensors.Add(new PowerSensor
                        {
                            Key = "cpu/msr/package-power",
                            Name = "CPU Package (MSR)",
                            Value = msrPower,
                        });
                    }
                }
                else if (cpuSensors.Count == 0)
                {
                    hardwareError = msrMessage;
                    lastHardwareMonitorError = msrMessage;
                }
            }

            if (cpuSensors.Count == 0)
            {
                var fallbackCpuSensor = TryReadWindowsCpuTemperatureSensor();
                if (fallbackCpuSensor != null)
                {
                    cpuSensors.Add(fallbackCpuSensor);
                    if (string.IsNullOrWhiteSpace(cpuModel))
                    {
                        cpuModel = "Windows Thermal Zone";
                    }
                }
            }

            cpuPowerWatts = SelectPowerWatts(cpuPowerSensors, selection.CpuPowerSensor, new[] { "Package", "CPU Package", "Total", "Core" });

            var selectedGpu = SelectGpuCandidate(gpuCandidates, selection.GpuDevice, selection.GpuSensor, selection.GpuPowerSensor);
            ReconcileHardwareProfileWithSensorsOnce(gpuCandidates);
            var gpuSensors = selectedGpu != null ? selectedGpu.Sensors : new System.Collections.Generic.List<TemperatureSensor>();
            var gpuPowerSensors = selectedGpu != null ? selectedGpu.PowerSensors : new System.Collections.Generic.List<PowerSensor>();
            gpuModel = selectedGpu != null ? selectedGpu.Model : string.Empty;
            double gpuPowerWatts = selectedGpu != null ? selectedGpu.PowerWatts : 0;
            var hardwareProfileGpuDevices = Array.Empty<TemperatureGpuDevice>();
            if (selectedGpu == null || gpuCandidates.Count == 0)
            {
                var hardwareProfile = GetHardwareProfile();
                hardwareProfileGpuDevices = BuildTemperatureGpuDevicesFromHardwareProfile(hardwareProfile);
                if (string.IsNullOrWhiteSpace(gpuModel))
                {
                    gpuModel = ResolveGpuModelFromHardwareProfile(hardwareProfile);
                }
            }
            if (shouldPollGpu)
            {
                gpuReadState = selectedGpu != null ? GpuReadStateActive : GpuReadStateUnavailable;
            }

            int cpuTemp = SelectTemperature(cpuSensors, selection.CpuSensor, new[] { "Average", "Package", "Tctl", "Tdie", "Core", "Windows" });
            int gpuTemp = SelectTemperature(gpuSensors, selection.GpuSensor, new[] { "Average", "GPU Core", "Core", "Edge", "Junction", "Hot Spot", "Temperature" });

            result.CpuTemp = cpuTemp;
            result.GpuTemp = gpuTemp;
            result.CpuPowerWatts = cpuPowerWatts;
            result.GpuPowerWatts = gpuPowerWatts;
            result.GpuReadState = gpuReadState;
            result.MaxTemp = Math.Max(cpuTemp, gpuTemp);
            result.ControlTemp = ResolveControlTemp(cpuTemp, gpuTemp, selection.TempSource);
            result.SelectedGpuDevice = selectedGpu != null ? selectedGpu.Key : selection.GpuDevice;
            result.CpuModel = cpuModel;
            result.GpuModel = gpuModel;
            result.CpuSensors = cpuSensors.ToArray();
            result.GpuSensors = gpuSensors.ToArray();
            result.CpuPowerSensors = cpuPowerSensors.ToArray();
            result.GpuPowerSensors = gpuPowerSensors.ToArray();
            var runtimeGpuDevices = gpuCandidates.Select(candidate => new TemperatureGpuDevice
            {
                Key = candidate.Key,
                Name = candidate.Model,
                Vendor = candidate.Vendor,
                Sensors = candidate.Sensors != null ? candidate.Sensors.ToArray() : Array.Empty<TemperatureSensor>(),
                PowerSensors = candidate.PowerSensors != null ? candidate.PowerSensors.ToArray() : Array.Empty<PowerSensor>()
            }).ToArray();
            result.GpuDevices = runtimeGpuDevices.Length > 0 ? runtimeGpuDevices : hardwareProfileGpuDevices;

            string temperatureErrorDetail = !string.IsNullOrWhiteSpace(hardwareError)
                ? hardwareError
                : lastHardwareMonitorError;
            if (!IsTemperatureReadSuccessful(cpuTemp, gpuTemp, temperatureErrorDetail))
            {
                result.Success = false;
                result.Error = BuildTemperatureReadError(hardwareError);
            }
            else
            {
                result.Success = true;
                result.Error = string.Empty;
            }

            return result;
        }

        static bool IsTemperatureReadSuccessful(int cpuTemp, int gpuTemp, string errorDetail)
        {
            return cpuTemp > 0 || (gpuTemp > 0 && !IsPermanentMsrFailure(errorDetail));
        }

        static void ReconcileHardwareProfileWithSensorsOnce(System.Collections.Generic.IEnumerable<GpuCandidate> gpuCandidates)
        {
            if (Interlocked.Exchange(ref hardwareProfileSensorReconcileAttempted, 1) == 1)
            {
                return;
            }

            ReconcileHardwareProfileWithSensors(gpuCandidates);
        }

        static string BuildTemperatureReadError(string hardwareError)
        {
            var parts = new System.Collections.Generic.List<string>();
            parts.Add("未读取到有效的 CPU/GPU 温度");

            string detail = !string.IsNullOrWhiteSpace(hardwareError) ? hardwareError : lastHardwareMonitorError;
            if (!string.IsNullOrWhiteSpace(detail))
            {
                parts.Add("硬件监控信息: " + detail);
            }

            if (IsPermanentMsrFailure(detail))
            {
                parts.Add("PawnIO 驱动未安装或未登记，Intel MSR 读取不可用；请安装或重新安装 PawnIO");
            }
            else if (IsRetryableMsrFailure(detail))
            {
                parts.Add("PawnIO/MSR 暂时不可用；程序会按限次与冷却策略自动重试，可关闭其它硬件监控软件后等待恢复");
            }
            else
            {
                parts.Add("已尝试 Windows 温区兜底；可重新初始化温度监控，或安装/更新 PawnIO 并关闭可能独占硬件传感器的软件");
            }
            return string.Join("；", parts.ToArray());
        }

        static TemperatureSensor TryReadWindowsCpuTemperatureSensor()
        {
            int temp = TryReadPerformanceCounterCpuTemperature();
            if (temp > 0)
            {
                return new TemperatureSensor
                {
                    Key = "cpu/windows/thermal-zone",
                    Name = "Windows Thermal Zone",
                    Value = temp,
                };
            }

            temp = TryReadWmiCpuTemperature();
            if (temp > 0)
            {
                return new TemperatureSensor
                {
                    Key = "cpu/windows/wmi-thermal-zone",
                    Name = "Windows WMI Thermal Zone",
                    Value = temp,
                };
            }

            return null;
        }

        static int TryReadPerformanceCounterCpuTemperature()
        {
            const string categoryName = "Thermal Zone Information";
            const string counterName = "Temperature";
            string[] preferredInstances = new[] { @"\_TZ.THRM", "_TZ.THRM", "THRM" };

            foreach (string instance in preferredInstances)
            {
                int temp = TryReadTemperatureCounter(categoryName, counterName, instance);
                if (temp > 0)
                {
                    return temp;
                }
            }

            try
            {
                if (!PerformanceCounterCategory.Exists(categoryName))
                {
                    return 0;
                }

                var category = new PerformanceCounterCategory(categoryName);
                foreach (string instance in category.GetInstanceNames())
                {
                    int temp = TryReadTemperatureCounter(categoryName, counterName, instance);
                    if (temp > 0)
                    {
                        return temp;
                    }
                }
            }
            catch
            {
            }

            return 0;
        }

        static int TryReadTemperatureCounter(string categoryName, string counterName, string instanceName)
        {
            try
            {
                using (var counter = new PerformanceCounter(categoryName, counterName, instanceName, true))
                {
                    return NormalizeCounterTemperature(counter.NextValue());
                }
            }
            catch
            {
                return 0;
            }
        }

        static int TryReadWmiCpuTemperature()
        {
            int best = 0;
            try
            {
                using (var searcher = new ManagementObjectSearcher(@"root\WMI", "SELECT CurrentTemperature FROM MSAcpi_ThermalZoneTemperature"))
                {
                    foreach (ManagementObject obj in searcher.Get())
                    {
                        using (obj)
                        {
                            object value = obj["CurrentTemperature"];
                            if (value == null)
                            {
                                continue;
                            }

                            int temp = NormalizeWmiTemperature(Convert.ToDouble(value));
                            if (temp > best)
                            {
                                best = temp;
                            }
                        }
                    }
                }
            }
            catch
            {
            }

            return best;
        }

        static int NormalizeCounterTemperature(double raw)
        {
            if (raw <= 0)
            {
                return 0;
            }

            double celsius = raw;
            if (raw > 1000)
            {
                celsius = (raw / 10.0) - 273.15;
            }
            else if (raw > 200)
            {
                celsius = raw - 273.15;
            }

            return NormalizeCelsius(celsius);
        }

        static int NormalizeWmiTemperature(double raw)
        {
            if (raw <= 0)
            {
                return 0;
            }

            return NormalizeCelsius((raw / 10.0) - 273.15);
        }

        static int NormalizeCelsius(double celsius)
        {
            int rounded = (int)Math.Round(celsius);
            return rounded > 0 && rounded < MaxReasonableTemperature ? rounded : 0;
        }

        static TemperatureData GetTemperatureData(TemperatureSelection selection)
        {
            lock (lockObject)
            {
	                var result = GetTemperatureDataUnsafe(selection);

                if ((!result.Success || (result.CpuTemp == 0 && result.GpuTemp == 0)) &&
                    (IsPermanentMsrFailure(result.Error) || IsRetryableMsrFailure(result.Error)))
                {
                    consecutiveFailures = 0;
                }
                else if (!result.Success || (result.CpuTemp == 0 && result.GpuTemp == 0))
                {
                    consecutiveFailures++;

                    // Auto-reinitialize after consecutive failures. This refreshes LHM/PawnIO handles
                    // without stopping the shared PawnIO driver service.
                    if (consecutiveFailures >= ConsecutiveFailuresBeforeReinit)
                    {
                        consecutiveFailures = 0;
                        result.Error = "连续读取失败，正在重新初始化温度监控并重新获取硬件句柄...";

                        ThreadPool.QueueUserWorkItem(_ =>
                        {
                            try { ReinitializeHardwareMonitor(); }
                            catch { }
                        });
                    }
                    else if (string.IsNullOrEmpty(result.Error))
                    {
                        result.Error = string.Format(
                            "未读取到有效的 CPU/GPU 温度（连续失败 {0}/{1}，达到阈值后将自动重新初始化温度监控）",
                            consecutiveFailures, ConsecutiveFailuresBeforeReinit);
                    }
                }
                else
                {
                    consecutiveFailures = 0;
                }

                return result;
            }
        }

            static void CollectTemperatureSensors(IHardware hardware, string devicePrefix, string keyPath, string displayPath, System.Collections.Generic.List<TemperatureSensor> sensors)
        {
                foreach (ISensor sensor in hardware.Sensors)
                {
                    if (sensor.SensorType != SensorType.Temperature || !sensor.Value.HasValue)
                    {
                        continue;
                    }

                    int temp = (int)Math.Round(sensor.Value.Value);
                    if (temp <= 0 || temp >= 150)
                    {
                        continue;
                    }

                    string sensorPath = string.IsNullOrEmpty(keyPath) ? sensor.Name : keyPath + "/" + sensor.Name;
                    sensors.Add(new TemperatureSensor
                    {
                        Key = devicePrefix + "/" + sensorPath,
                        Name = string.IsNullOrEmpty(displayPath) ? sensor.Name : displayPath + " / " + sensor.Name,
                        Value = temp,
                    });
                }

                foreach (IHardware subHardware in hardware.SubHardware)
                {
                    string subKeyPath = string.IsNullOrEmpty(keyPath)
                        ? (subHardware.Name ?? string.Empty)
                        : keyPath + "/" + (subHardware.Name ?? string.Empty);
                    string subDisplayPath = string.IsNullOrEmpty(displayPath)
                        ? (subHardware.Name ?? string.Empty)
                        : displayPath + " / " + (subHardware.Name ?? string.Empty);
                    CollectTemperatureSensors(subHardware, devicePrefix, subKeyPath, subDisplayPath, sensors);
                }
        }

        static int SelectTemperature(System.Collections.Generic.IReadOnlyList<TemperatureSensor> sensors, string selectedKey, string[] preferredSensorNames)
        {
                if (sensors == null || sensors.Count == 0)
            {
                    return 0;
            }

                if (!string.Equals(selectedKey, "auto", StringComparison.OrdinalIgnoreCase))
                {
                    foreach (var sensor in sensors)
                    {
                        if (string.Equals(sensor.Key, selectedKey, StringComparison.OrdinalIgnoreCase))
                        {
                            return sensor.Value;
                        }
                    }
                }

                foreach (var sensor in sensors)
                {
                    if (ContainsAnyKeyword(sensor.Name, preferredSensorNames))
                    {
                        return sensor.Value;
                    }
                }

                return sensors[0].Value;
            }

        static double SelectPowerWatts(System.Collections.Generic.IReadOnlyList<PowerSensor> sensors, string selectedKey, string[] preferredSensorNames)
        {
            if (sensors == null || sensors.Count == 0)
            {
                return 0;
            }

            if (!string.Equals(selectedKey, "auto", StringComparison.OrdinalIgnoreCase))
            {
                foreach (var sensor in sensors)
                {
                    if (string.Equals(sensor.Key, selectedKey, StringComparison.OrdinalIgnoreCase))
                    {
                        return Math.Round(sensor.Value, 1);
                    }
                }
            }

            foreach (string preferred in preferredSensorNames)
            {
                foreach (var sensor in sensors)
                {
                    if (sensor.Value > 0 && sensor.Name.IndexOf(preferred, StringComparison.OrdinalIgnoreCase) >= 0)
                    {
                        return Math.Round(sensor.Value, 1);
                    }
                }
            }

            foreach (var sensor in sensors)
            {
                if (sensor.Value > 0)
                {
                    return Math.Round(sensor.Value, 1);
                }
            }

            return Math.Round(sensors[0].Value, 1);
        }

        static void CollectPowerSensors(IHardware hardware, string devicePrefix, string keyPath, string displayPath, System.Collections.Generic.List<PowerSensor> sensors)
        {
            foreach (ISensor sensor in hardware.Sensors)
            {
                if (sensor.SensorType != SensorType.Power || !sensor.Value.HasValue)
                {
                    continue;
                }
                double watts = sensor.Value.Value;
                if (watts < 0 || watts > 1000)
                {
                    continue;
                }
                string sensorPath = string.IsNullOrEmpty(keyPath) ? sensor.Name : keyPath + "/" + sensor.Name;
                string name = string.IsNullOrEmpty(displayPath) ? sensor.Name : displayPath + " / " + sensor.Name;
                sensors.Add(new PowerSensor
                {
                    Key = devicePrefix + "/" + sensorPath,
                    Name = name,
                    Value = Math.Round(watts, 1),
                });
            }

            foreach (IHardware subHardware in hardware.SubHardware)
            {
                string subKeyPath = string.IsNullOrEmpty(keyPath)
                    ? (subHardware.Name ?? string.Empty)
                    : keyPath + "/" + (subHardware.Name ?? string.Empty);
                string subDisplayPath = string.IsNullOrEmpty(displayPath)
                    ? (subHardware.Name ?? string.Empty)
                    : displayPath + " / " + (subHardware.Name ?? string.Empty);
                CollectPowerSensors(subHardware, devicePrefix, subKeyPath, subDisplayPath, sensors);
            }
        }

        static GpuCandidate SelectGpuCandidate(System.Collections.Generic.IReadOnlyList<GpuCandidate> candidates, string selectedDeviceKey, string selectedSensorKey, string selectedPowerSensorKey)
        {
            if (candidates == null || candidates.Count == 0)
            {
                return null;
            }

            if (!string.Equals(selectedDeviceKey, "auto", StringComparison.OrdinalIgnoreCase))
            {
                foreach (var candidate in candidates)
                {
                    if (candidate != null && string.Equals(candidate.Key, selectedDeviceKey, StringComparison.OrdinalIgnoreCase))
                    {
                        return candidate;
                    }
                }
            }

            if (!string.Equals(selectedSensorKey, "auto", StringComparison.OrdinalIgnoreCase))
            {
                foreach (var candidate in candidates)
                {
                    if (candidate == null || candidate.Sensors == null)
                    {
                        continue;
                    }

                    foreach (var sensor in candidate.Sensors)
                    {
                        if (string.Equals(sensor.Key, selectedSensorKey, StringComparison.OrdinalIgnoreCase))
                        {
                            return candidate;
                        }
                    }
                }
            }

            if (!string.Equals(selectedPowerSensorKey, "auto", StringComparison.OrdinalIgnoreCase))
            {
                foreach (var candidate in candidates)
                {
                    if (candidate == null || candidate.PowerSensors == null)
                    {
                        continue;
                    }

                    foreach (var sensor in candidate.PowerSensors)
                    {
                        if (string.Equals(sensor.Key, selectedPowerSensorKey, StringComparison.OrdinalIgnoreCase))
                        {
                            return candidate;
                        }
                    }
                }
            }

            GpuCandidate bestWithSensors = null;
            GpuCandidate bestFallback = null;

            foreach (var candidate in candidates)
            {
                if (candidate == null)
                {
                    continue;
                }

                if (bestFallback == null || CompareGpuCandidate(candidate, bestFallback) > 0)
                {
                    bestFallback = candidate;
                }

                if (candidate.Sensors != null && candidate.Sensors.Count > 0)
                {
                    if (bestWithSensors == null || CompareGpuCandidate(candidate, bestWithSensors) > 0)
                    {
                        bestWithSensors = candidate;
                    }
                }
            }

            return bestWithSensors ?? bestFallback;
        }

            static string BuildGpuDeviceKey(IHardware hardware, int index)
            {
                string vendor = GetGpuVendor(hardware.HardwareType);
                string name = hardware != null && !string.IsNullOrWhiteSpace(hardware.Name) ? hardware.Name.Trim() : "gpu";
                return string.Format("{0}:{1}:{2}", vendor, index, name);
            }

            static string GetGpuVendor(HardwareType hardwareType)
            {
                switch (hardwareType)
                {
                    case HardwareType.GpuNvidia:
                        return "nvidia";
                    case HardwareType.GpuAmd:
                        return "amd";
                    case HardwareType.GpuIntel:
                        return "intel";
                    default:
                        return "gpu";
                }
            }

        static int CompareGpuCandidate(GpuCandidate left, GpuCandidate right)
        {
            int leftPriority = GetGpuPriority(left);
            int rightPriority = GetGpuPriority(right);
            if (leftPriority != rightPriority)
            {
                return leftPriority - rightPriority;
            }

            int leftSensorCount = left != null && left.Sensors != null ? left.Sensors.Count : 0;
            int rightSensorCount = right != null && right.Sensors != null ? right.Sensors.Count : 0;
            return leftSensorCount - rightSensorCount;
        }

        static int GetGpuPriority(GpuCandidate candidate)
        {
            if (candidate == null)
            {
                return 0;
            }

            switch (candidate.HardwareType)
            {
                case HardwareType.GpuNvidia:
                    return 300;
                case HardwareType.GpuAmd:
                    return 200;
                case HardwareType.GpuIntel:
                    return 100;
                default:
                    return 0;
            }
        }

            static int ResolveControlTemp(int cpuTemp, int gpuTemp, string source)
            {
                switch (NormalizeTempSource(source))
                {
                    case "cpu":
                        return cpuTemp;
                    case "gpu":
                        return gpuTemp;
                    default:
                        return Math.Max(cpuTemp, gpuTemp);
                }
        }

        static bool ContainsAnyKeyword(string source, string[] keywords)
        {
            if (string.IsNullOrEmpty(source) || keywords == null)
            {
                return false;
            }

            foreach (string keyword in keywords)
            {
                if (!string.IsNullOrEmpty(keyword) &&
                    source.IndexOf(keyword, StringComparison.OrdinalIgnoreCase) >= 0)
                {
                    return true;
                }
            }

            return false;
        }

        sealed class MutexHandle : IDisposable
        {
            private Mutex mutex;

            public MutexHandle(Mutex mutex)
            {
                this.mutex = mutex;
            }

            public void Dispose()
            {
                if (mutex == null)
                {
                    return;
                }

                try
                {
                    mutex.ReleaseMutex();
                }
                catch (ApplicationException)
                {
                }
            }
        }
    }
}
