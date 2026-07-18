using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.Linq;
using System.Management;
using System.Runtime.InteropServices;
using System.Text.RegularExpressions;

namespace FanControl.TempBridge
{
    internal sealed class GpuActivityStatus
    {
        public bool HasDiscreteGpu { get; set; }
        public bool IsActive { get; set; }
        public string Detail { get; set; }

        public GpuActivityStatus()
        {
            Detail = string.Empty;
        }
    }

    internal static class GpuActivityDetector
    {
        private const ulong DiscreteMemoryThreshold = 512UL * 1024UL * 1024UL;
        private const double ActiveEngineUtilizationThreshold = 2;
        private const int AdapterCacheSeconds = 300;
        private static readonly object cacheLock = new object();
        private static DateTime cachedAdaptersUtc = DateTime.MinValue;
        private static List<AdapterInfo> cachedAdapters = new List<AdapterInfo>();
        private static readonly Regex EngineLuidPattern = new Regex(
            @"luid_(0x[0-9a-f]+)_(0x[0-9a-f]+).*engtype_([a-z0-9]*)",
            RegexOptions.IgnoreCase | RegexOptions.Compiled);
        [StructLayout(LayoutKind.Sequential)]
        private struct Luid
        {
            public uint LowPart;
            public int HighPart;
        }

        [StructLayout(LayoutKind.Sequential, CharSet = CharSet.Unicode)]
        private struct DxgiAdapterDesc1
        {
            [MarshalAs(UnmanagedType.ByValTStr, SizeConst = 128)]
            public string Description;
            public uint VendorId;
            public uint DeviceId;
            public uint SubSysId;
            public uint Revision;
            public UIntPtr DedicatedVideoMemory;
            public UIntPtr DedicatedSystemMemory;
            public UIntPtr SharedSystemMemory;
            public Luid AdapterLuid;
            public uint Flags;
        }

        [ComImport, Guid("29038f61-3839-4626-91fd-086879011a05"), InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
        private interface IDXGIAdapter1
        {
            [PreserveSig] int SetPrivateData(ref Guid name, uint dataSize, IntPtr data);
            [PreserveSig] int SetPrivateDataInterface(ref Guid name, IntPtr unknown);
            [PreserveSig] int GetPrivateData(ref Guid name, ref uint dataSize, IntPtr data);
            [PreserveSig] int GetParent(ref Guid riid, out IntPtr parent);
            [PreserveSig] int EnumOutputs(uint output, out IntPtr outputPtr);
            [PreserveSig] int GetDesc(out IntPtr desc);
            [PreserveSig] int CheckInterfaceSupport(ref Guid interfaceName, out long umdVersion);
            [PreserveSig] int GetDesc1(out DxgiAdapterDesc1 desc);
        }

        [ComImport, Guid("770aae78-f26f-4dba-a829-253c83d1b387"), InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
        private interface IDXGIFactory1
        {
            [PreserveSig] int SetPrivateData(ref Guid name, uint dataSize, IntPtr data);
            [PreserveSig] int SetPrivateDataInterface(ref Guid name, IntPtr unknown);
            [PreserveSig] int GetPrivateData(ref Guid name, ref uint dataSize, IntPtr data);
            [PreserveSig] int GetParent(ref Guid riid, out IntPtr parent);
            [PreserveSig] int EnumAdapters(uint adapter, out IntPtr adapterPtr);
            [PreserveSig] int MakeWindowAssociation(IntPtr windowHandle, uint flags);
            [PreserveSig] int GetWindowAssociation(out IntPtr windowHandle);
            [PreserveSig] int CreateSwapChain(IntPtr device, IntPtr desc, out IntPtr swapChain);
            [PreserveSig] int CreateSoftwareAdapter(IntPtr module, out IntPtr adapter);
            [PreserveSig] int EnumAdapters1(uint adapter, out IDXGIAdapter1 adapterPtr);
            [PreserveSig] bool IsCurrent();
        }

        [DllImport("dxgi.dll")]
        private static extern int CreateDXGIFactory1(ref Guid riid, out IDXGIFactory1 factory);

        private sealed class AdapterInfo
        {
            public string Name { get; set; }
            public string Vendor { get; set; }
            public string Luid { get; set; }
            public bool Discrete { get; set; }

            public AdapterInfo()
            {
                Name = string.Empty;
                Vendor = string.Empty;
                Luid = string.Empty;
            }
        }

        public static GpuActivityStatus Detect()
        {
            var status = new GpuActivityStatus();
            var adapters = GetCachedDxgiAdapters();
            var discreteLuids = new HashSet<string>(
                adapters.Where(adapter => adapter.Discrete && !string.IsNullOrWhiteSpace(adapter.Luid)).Select(adapter => adapter.Luid),
                StringComparer.OrdinalIgnoreCase);

            status.HasDiscreteGpu = discreteLuids.Count > 0;
            if (!status.HasDiscreteGpu)
            {
                status.Detail = "no discrete GPU";
                return status;
            }

            var activeEngines = ReadActiveGpuEngineLuids(discreteLuids).ToArray();
            status.IsActive = activeEngines.Length > 0;
            status.Detail = status.IsActive
                ? "active discrete GPU: " + string.Join(", ", activeEngines.Take(3))
                : "discrete GPU idle";
            return status;
        }

        public static HardwareGpuDevice[] EnumerateDxgiHardwareGpuDevices()
        {
            return EnumerateDxgiAdapters()
                .Select(adapter => new HardwareGpuDevice
                {
                    Name = adapter.Name,
                    Vendor = adapter.Vendor,
                    PnpDeviceId = adapter.Luid,
                    Luid = adapter.Luid,
                    Discrete = adapter.Discrete
                })
                .ToArray();
        }

        private static List<AdapterInfo> GetCachedDxgiAdapters()
        {
            lock (cacheLock)
            {
                if ((DateTime.UtcNow - cachedAdaptersUtc).TotalSeconds < AdapterCacheSeconds)
                {
                    return cachedAdapters.ToList();
                }
            }

            var adapters = EnumerateDxgiAdapters();
            lock (cacheLock)
            {
                cachedAdapters = adapters;
                cachedAdaptersUtc = DateTime.UtcNow;
                return cachedAdapters.ToList();
            }
        }

        private static IEnumerable<string> ReadActiveGpuEngineLuids(HashSet<string> discreteLuids)
        {
            using (var searcher = new ManagementObjectSearcher("SELECT Name, UtilizationPercentage FROM Win32_PerfFormattedData_GPUPerformanceCounters_GPUEngine"))
            using (var results = searcher.Get())
            {
                foreach (ManagementObject item in results)
                {
                    string name = Convert.ToString(item["Name"]) ?? string.Empty;
                    double utilization = ReadDouble(item["UtilizationPercentage"]);
                    if (utilization < ActiveEngineUtilizationThreshold)
                    {
                        continue;
                    }

                    string luid = ParseEngineLuid(name);
                    if (luid.Length == 0 || !discreteLuids.Contains(luid))
                    {
                        continue;
                    }

                    int pid = ParseEnginePid(name);
                    string processName = pid > 4 ? ReadProcessName(pid) : string.Empty;
                    if (IsIgnoredGpuActivityProcess(processName))
                    {
                        continue;
                    }

                    yield return string.IsNullOrWhiteSpace(processName)
                        ? "engine:" + name
                        : string.Format("engine:{0}:{1:0.#}%", processName, utilization);
                }
            }
        }

        private static List<AdapterInfo> EnumerateDxgiAdapters()
        {
            var adapters = new List<AdapterInfo>();
            IDXGIFactory1 factory = null;
            try
            {
                var iid = new Guid("770aae78-f26f-4dba-a829-253c83d1b387");
                int hr = CreateDXGIFactory1(ref iid, out factory);
                if (hr != 0 || factory == null)
                {
                    return adapters;
                }

                for (uint index = 0; index < 32; index++)
                {
                    IDXGIAdapter1 adapter = null;
                    hr = factory.EnumAdapters1(index, out adapter);
                    if (hr != 0 || adapter == null)
                    {
                        break;
                    }

                    try
                    {
                        DxgiAdapterDesc1 desc;
                        if (adapter.GetDesc1(out desc) == 0)
                        {
                            string vendor = DetectVendor(desc.VendorId, desc.Description);
                            adapters.Add(new AdapterInfo
                            {
                                Name = desc.Description ?? string.Empty,
                                Vendor = vendor,
                                Luid = FormatLuid(desc.AdapterLuid),
                                Discrete = IsDiscreteAdapter(vendor, desc.DedicatedVideoMemory)
                            });
                        }
                    }
                    finally
                    {
                        Marshal.ReleaseComObject(adapter);
                    }
                }
            }
            catch
            {
                return adapters;
            }
            finally
            {
                if (factory != null)
                {
                    Marshal.ReleaseComObject(factory);
                }
            }

            return adapters;
        }

        private static string ParseEngineLuid(string instanceName)
        {
            var match = EngineLuidPattern.Match(instanceName ?? string.Empty);
            if (!match.Success)
            {
                return string.Empty;
            }

            return NormalizeLuid(match.Groups[1].Value, match.Groups[2].Value);
        }

        private static int ParseEnginePid(string instanceName)
        {
            var match = Regex.Match(instanceName ?? string.Empty, @"pid_(\d+)", RegexOptions.IgnoreCase);
            if (!match.Success)
            {
                return 0;
            }

            int pid;
            return int.TryParse(match.Groups[1].Value, out pid) ? pid : 0;
        }

        private static string FormatLuid(Luid luid)
        {
            return string.Format("0x{0:X8}_0x{1:X8}", luid.HighPart, luid.LowPart);
        }

        private static string NormalizeLuid(string highPart, string lowPart)
        {
            try
            {
                uint high = Convert.ToUInt32(StripHexPrefix(highPart), 16);
                uint low = Convert.ToUInt32(StripHexPrefix(lowPart), 16);
                return string.Format("0x{0:X8}_0x{1:X8}", high, low);
            }
            catch
            {
                return string.Empty;
            }
        }

        private static string StripHexPrefix(string value)
        {
            if (value == null)
            {
                return string.Empty;
            }
            return value.StartsWith("0x", StringComparison.OrdinalIgnoreCase) ? value.Substring(2) : value;
        }

        private static string ReadProcessName(int pid)
        {
            try
            {
                using (var process = Process.GetProcessById(pid))
                {
                    return process.ProcessName ?? string.Empty;
                }
            }
            catch
            {
                return string.Empty;
            }
        }

        private static bool IsIgnoredGpuActivityProcess(string processName)
        {
            if (string.IsNullOrWhiteSpace(processName))
            {
                return true;
            }

            string normalized = processName.Trim().ToLowerInvariant();
            return normalized == "system" ||
                normalized == "fancontrol tempbridge" ||
                normalized == "fancontrol" ||
                normalized == "fancontrol core" ||
                normalized == "hnperformancecenter" ||
                normalized == "hnperfpowernexus" ||
                normalized == "mbamessagecenter" ||
                normalized == "qqex" ||
                normalized == "nvidia-smi";
        }

        private static string DetectVendor(uint vendorId, string description)
        {
            switch (vendorId)
            {
                case 0x10DE:
                    return "nvidia";
                case 0x1002:
                case 0x1022:
                    return "amd";
                case 0x8086:
                    return "intel";
                default:
                    string text = (description ?? string.Empty).ToLowerInvariant();
                    if (text.Contains("nvidia")) return "nvidia";
                    if (text.Contains("amd") || text.Contains("radeon")) return "amd";
                    if (text.Contains("intel")) return "intel";
                    return "unknown";
            }
        }

        private static bool IsDiscreteAdapter(string vendor, UIntPtr dedicatedVideoMemory)
        {
            ulong dedicatedBytes = dedicatedVideoMemory.ToUInt64();
            if (string.Equals(vendor, "nvidia", StringComparison.OrdinalIgnoreCase))
            {
                return true;
            }
            if (string.Equals(vendor, "amd", StringComparison.OrdinalIgnoreCase))
            {
                return dedicatedBytes >= DiscreteMemoryThreshold;
            }
            return false;
        }

        private static double ReadDouble(object value)
        {
            try
            {
                return Convert.ToDouble(value);
            }
            catch
            {
                return 0;
            }
        }
    }
}
