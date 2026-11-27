using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.Linq;
using System.Management;
using System.Runtime.InteropServices;
using System.Text;
using System.Text.RegularExpressions;
using System.Threading;

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
        private const int PdhSampleIntervalMs = 50;
        private const int D3dkmtQueryBufferSize = 1024;
        private const uint D3dkmtQueryTypeAdapter = 0;
        private const uint D3dkmtQueryTypeNode = 5;
        private const int D3dkmtAdapterLuidOffset = 4;
        private const int D3dkmtAdapterNodeCountOffset = 28;
        private const int D3dkmtNodeRunningTimeOffset = 24;
        private const int D3dkmtQueryNodeIdOffset = 800;
        private const int D3dkmtMaxNodeCount = 64;
        private const long D3dkmtIdleDeltaThresholdTicks = 100_000;
        private const uint CmGetIdListFilterClass = 0x00000200;
        private const uint CmGetIdListFilterPresent = 0x00000100;
        private const uint PdhFmtDouble = 0x00000200;
        private const uint PdhMoreData = 0x800007D2;
        private const uint PdhCstatusValidData = 0;
        private const uint PdhCstatusNewData = 1;
        private const string DisplayClassGuid = "{4d36e968-e325-11ce-bfc1-08002be10318}";
        private const string GpuEngineUtilizationCounterPath = @"\GPU Engine(*)\Utilization Percentage";

        private static readonly object cacheLock = new object();
        private static DateTime cachedAdaptersUtc = DateTime.MinValue;
        private static List<AdapterInfo> cachedAdapters = new List<AdapterInfo>();
        private static readonly object d3dkmtLock = new object();
        private static readonly Dictionary<string, D3dkmtAdapterSample> d3dkmtCache =
            new Dictionary<string, D3dkmtAdapterSample>(StringComparer.OrdinalIgnoreCase);
        private static readonly byte[] d3dkmtQueryBuffer = new byte[D3dkmtQueryBufferSize];
        private static readonly object pdhLock = new object();
        private static IntPtr pdhQuery = IntPtr.Zero;
        private static IntPtr pdhCounter = IntPtr.Zero;
        private static byte[] pdhBuffer = Array.Empty<byte>();
        private static readonly Regex EngineLuidPattern = new Regex(
            @"luid_(0x[0-9a-f]+)_(0x[0-9a-f]+).*engtype_([a-z0-9]*)",
            RegexOptions.IgnoreCase | RegexOptions.Compiled);
        private static readonly Regex EnginePidPattern = new Regex(
            @"pid_(\d+)",
            RegexOptions.IgnoreCase | RegexOptions.Compiled);

        private static DevPropKey DevPkeyDeviceDesc = new DevPropKey
        {
            fmtid = new Guid("A45C254E-DF1C-4EFD-8020-67D146A850E0"),
            pid = 2
        };

        private static DevPropKey DevPkeyDeviceFriendlyName = new DevPropKey
        {
            fmtid = new Guid("A45C254E-DF1C-4EFD-8020-67D146A850E0"),
            pid = 14
        };

        private static DevPropKey DevPkeyDevicePowerData = new DevPropKey
        {
            fmtid = new Guid("A45C254E-DF1C-4EFD-8020-67D146A850E0"),
            pid = 32
        };

        [StructLayout(LayoutKind.Sequential)]
        private struct DevPropKey
        {
            public Guid fmtid;
            public uint pid;
        }

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

        [StructLayout(LayoutKind.Sequential)]
        private struct PdhFmtCounterValueItemDouble
        {
            public IntPtr szName;
            public uint CStatus;
            public double doubleValue;
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

        [DllImport("gdi32.dll")]
        private static extern int D3DKMTQueryStatistics(IntPtr queryStatistics);

        [DllImport("cfgmgr32.dll", CharSet = CharSet.Unicode)]
        private static extern uint CM_Get_Device_ID_List_SizeW(out uint pulLen, string pszFilter, uint ulFlags);

        [DllImport("cfgmgr32.dll", CharSet = CharSet.Unicode)]
        private static extern uint CM_Get_Device_ID_ListW(string pszFilter, char[] buffer, uint bufferLen, uint ulFlags);

        [DllImport("cfgmgr32.dll", CharSet = CharSet.Unicode)]
        private static extern uint CM_Locate_DevNodeW(out uint pdnDevInst, string pDeviceID, uint ulFlags);

        [DllImport("cfgmgr32.dll", CharSet = CharSet.Unicode)]
        private static extern uint CM_Get_DevNode_PropertyW(
            uint dnDevInst,
            ref DevPropKey propertyKey,
            out uint propertyType,
            byte[] propertyBuffer,
            ref uint propertyBufferSize,
            uint ulFlags);

        [DllImport("pdh.dll", CharSet = CharSet.Unicode)]
        private static extern uint PdhOpenQueryW(string dataSource, IntPtr userData, out IntPtr query);

        [DllImport("pdh.dll", CharSet = CharSet.Unicode)]
        private static extern uint PdhAddEnglishCounterW(IntPtr query, string fullCounterPath, IntPtr userData, out IntPtr counter);

        [DllImport("pdh.dll")]
        private static extern uint PdhRemoveCounter(IntPtr counter);

        [DllImport("pdh.dll")]
        private static extern uint PdhCollectQueryData(IntPtr query);

        [DllImport("pdh.dll", CharSet = CharSet.Unicode)]
        private static extern uint PdhGetFormattedCounterArrayW(
            IntPtr counter,
            uint format,
            ref uint bufferSize,
            ref uint itemCount,
            IntPtr itemBuffer);

        [DllImport("pdh.dll")]
        private static extern uint PdhCloseQuery(IntPtr query);

        private sealed class AdapterInfo
        {
            public string Name { get; set; }
            public string Vendor { get; set; }
            public string Luid { get; set; }
            public Luid RawLuid { get; set; }
            public bool Discrete { get; set; }

            public AdapterInfo()
            {
                Name = string.Empty;
                Vendor = string.Empty;
                Luid = string.Empty;
            }
        }

        private sealed class D3dkmtAdapterSample
        {
            public int NodeCount;
            public long TimestampTicks100ns;
            public long[] RunningTimes = Array.Empty<long>();
            public long[] ScratchTimes = Array.Empty<long>();
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

            string sleepDetail;
            if (IsDiscreteDisplaySleepingViaDState(adapters, out sleepDetail))
            {
                DisposePdhOnly();
                status.IsActive = false;
                status.Detail = sleepDetail;
                return status;
            }

            bool d3dkmtIdle;
            string d3dkmtDetail;
            if (TryCheckDiscreteGpuIdleViaD3dkmt(adapters, out d3dkmtIdle, out d3dkmtDetail) && d3dkmtIdle)
            {
                status.IsActive = false;
                status.Detail = d3dkmtDetail;
                return status;
            }

            List<string> activeEngines;
            if (!TryReadActiveGpuEngineLuidsPdh(discreteLuids, out activeEngines))
            {
                activeEngines = ReadActiveGpuEngineLuidsWmi(discreteLuids);
            }

            status.IsActive = activeEngines.Count > 0;
            if (!status.IsActive)
            {
                DisposePdhOnly();
            }
            status.Detail = status.IsActive
                ? "active discrete GPU: " + string.Join(", ", activeEngines.Take(3))
                : "discrete GPU idle";
            return status;
        }

        private static void DisposePdhOnly()
        {
            lock (pdhLock)
            {
                ClosePdhQueryLocked();
                pdhBuffer = Array.Empty<byte>();
            }
        }

        public static void DisposeResources()
        {
            DisposePdhOnly();
            lock (d3dkmtLock)
            {
                d3dkmtCache.Clear();
            }
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
            PruneStaleD3dkmtCache(adapters);
            lock (cacheLock)
            {
                cachedAdapters = adapters;
                cachedAdaptersUtc = DateTime.UtcNow;
                return cachedAdapters.ToList();
            }
        }

        private static void PruneStaleD3dkmtCache(List<AdapterInfo> adapters)
        {
            lock (d3dkmtLock)
            {
                if (d3dkmtCache.Count == 0)
                {
                    return;
                }

                var activeLuids = new HashSet<string>(StringComparer.OrdinalIgnoreCase);
                if (adapters != null)
                {
                    for (int i = 0; i < adapters.Count; i++)
                    {
                        string luid = adapters[i].Luid;
                        if (!string.IsNullOrEmpty(luid))
                        {
                            activeLuids.Add(luid);
                        }
                    }
                }

                List<string> staleKeys = null;
                foreach (string key in d3dkmtCache.Keys)
                {
                    if (!activeLuids.Contains(key))
                    {
                        if (staleKeys == null)
                        {
                            staleKeys = new List<string>();
                        }
                        staleKeys.Add(key);
                    }
                }

                if (staleKeys != null)
                {
                    for (int i = 0; i < staleKeys.Count; i++)
                    {
                        d3dkmtCache.Remove(staleKeys[i]);
                    }
                }
            }
        }

        private static bool IsDiscreteDisplaySleepingViaDState(List<AdapterInfo> adapters, out string detail)
        {
            detail = string.Empty;
            var discreteAdapters = adapters.Where(a => a.Discrete).ToList();
            if (discreteAdapters.Count == 0)
            {
                return false;
            }

            var discreteVendors = new HashSet<string>(
                discreteAdapters.Select(a => a.Vendor).Where(v => !string.IsNullOrEmpty(v)),
                StringComparer.OrdinalIgnoreCase);

            var deviceIds = EnumeratePresentDisplayDeviceIds();
            if (deviceIds.Count == 0)
            {
                return false;
            }

            int matchedCount = 0;
            int maxSleepState = 0;
            byte[] propBuf = new byte[512];

            foreach (string deviceId in deviceIds)
            {
                if (!deviceId.StartsWith(@"PCI\", StringComparison.OrdinalIgnoreCase))
                {
                    continue;
                }

                string vendor = DetectVendorFromPnpId(deviceId);
                if (!discreteVendors.Contains(vendor))
                {
                    continue;
                }

                uint devNode;
                if (CM_Locate_DevNodeW(out devNode, deviceId, 0) != 0 || devNode == 0)
                {
                    continue;
                }

                bool vendorHasIntegrated = adapters.Any(a => !a.Discrete && string.Equals(a.Vendor, vendor, StringComparison.OrdinalIgnoreCase));
                if (vendorHasIntegrated)
                {
                    string devDesc = ReadDevNodeStringProperty(devNode, ref DevPkeyDeviceFriendlyName, propBuf);
                    if (string.IsNullOrEmpty(devDesc))
                    {
                        devDesc = ReadDevNodeStringProperty(devNode, ref DevPkeyDeviceDesc, propBuf);
                    }

                    bool matchesDiscrete = !string.IsNullOrEmpty(devDesc) &&
                        discreteAdapters.Any(a => string.Equals(a.Vendor, vendor, StringComparison.OrdinalIgnoreCase) &&
                            (devDesc.IndexOf(a.Name, StringComparison.OrdinalIgnoreCase) >= 0 ||
                             a.Name.IndexOf(devDesc, StringComparison.OrdinalIgnoreCase) >= 0));
                    bool matchesIntegrated = !string.IsNullOrEmpty(devDesc) &&
                        adapters.Any(a => !a.Discrete && string.Equals(a.Vendor, vendor, StringComparison.OrdinalIgnoreCase) &&
                            (devDesc.IndexOf(a.Name, StringComparison.OrdinalIgnoreCase) >= 0 ||
                             a.Name.IndexOf(devDesc, StringComparison.OrdinalIgnoreCase) >= 0));

                    if (!matchesDiscrete || matchesIntegrated)
                    {
                        if (!matchesIntegrated)
                        {
                            // Ambiguous same-vendor integrated vs discrete adapter: fall through to PDH.
                            return false;
                        }
                        continue;
                    }
                }

                int powerState;
                if (!TryReadDevNodePowerState(devNode, propBuf, out powerState))
                {
                    return false;
                }

                // DEVICE_POWER_STATE: 1 = D0 (active), 2 = D1, 3 = D2, 4 = D3 (sleeping).
                if (powerState <= 1 || powerState > 4)
                {
                    return false;
                }

                matchedCount++;
                if (powerState > maxSleepState)
                {
                    maxSleepState = powerState;
                }
            }

            if (matchedCount == 0)
            {
                return false;
            }

            detail = maxSleepState >= 4 ? "discrete GPU idle (D3)" : string.Format("discrete GPU idle (D{0})", Math.Max(0, maxSleepState - 1));
            return true;
        }

        private static List<string> EnumeratePresentDisplayDeviceIds()
        {
            var result = new List<string>();
            uint len;
            if (CM_Get_Device_ID_List_SizeW(out len, DisplayClassGuid, CmGetIdListFilterClass | CmGetIdListFilterPresent) != 0 || len <= 1)
            {
                return result;
            }

            char[] buffer = new char[len];
            if (CM_Get_Device_ID_ListW(DisplayClassGuid, buffer, len, CmGetIdListFilterClass | CmGetIdListFilterPresent) != 0)
            {
                return result;
            }

            int start = 0;
            for (int i = 0; i < buffer.Length; i++)
            {
                if (buffer[i] == '\0')
                {
                    if (i > start)
                    {
                        result.Add(new string(buffer, start, i - start));
                    }
                    else
                    {
                        break;
                    }
                    start = i + 1;
                }
            }
            return result;
        }

        private static bool TryReadDevNodePowerState(uint devNode, byte[] buffer, out int powerState)
        {
            powerState = 0;
            uint propType;
            uint propSize = (uint)buffer.Length;
            if (CM_Get_DevNode_PropertyW(devNode, ref DevPkeyDevicePowerData, out propType, buffer, ref propSize, 0) != 0 || propSize < 8)
            {
                return false;
            }

            powerState = BitConverter.ToInt32(buffer, 4);
            return true;
        }

        private static string ReadDevNodeStringProperty(uint devNode, ref DevPropKey key, byte[] buffer)
        {
            uint propType;
            uint propSize = (uint)buffer.Length;
            if (CM_Get_DevNode_PropertyW(devNode, ref key, out propType, buffer, ref propSize, 0) != 0 || propSize < 2)
            {
                return string.Empty;
            }

            int byteLen = (int)Math.Min(propSize, (uint)buffer.Length);
            string text = Encoding.Unicode.GetString(buffer, 0, byteLen);
            int nullIdx = text.IndexOf('\0');
            return nullIdx >= 0 ? text.Substring(0, nullIdx) : text;
        }

        private static string DetectVendorFromPnpId(string deviceId)
        {
            if (string.IsNullOrEmpty(deviceId))
            {
                return string.Empty;
            }
            if (deviceId.IndexOf("VEN_10DE", StringComparison.OrdinalIgnoreCase) >= 0) return "nvidia";
            if (deviceId.IndexOf("VEN_1002", StringComparison.OrdinalIgnoreCase) >= 0 || deviceId.IndexOf("VEN_1022", StringComparison.OrdinalIgnoreCase) >= 0) return "amd";
            if (deviceId.IndexOf("VEN_8086", StringComparison.OrdinalIgnoreCase) >= 0) return "intel";
            return string.Empty;
        }

        private static long GetMonotonic100nsTicks()
        {
            return (long)(Stopwatch.GetTimestamp() * (10_000_000.0 / Stopwatch.Frequency));
        }

        private static bool TryCheckDiscreteGpuIdleViaD3dkmt(List<AdapterInfo> adapters, out bool isIdle, out string detail)
        {
            isIdle = false;
            detail = string.Empty;
            if (IntPtr.Size != 8 || adapters == null)
            {
                return false;
            }

            List<AdapterInfo> discreteAdapters = null;
            for (int i = 0; i < adapters.Count; i++)
            {
                AdapterInfo adapter = adapters[i];
                if (adapter != null &&
                    adapter.Discrete &&
                    !string.IsNullOrWhiteSpace(adapter.Luid) &&
                    (adapter.RawLuid.LowPart != 0 || adapter.RawLuid.HighPart != 0))
                {
                    if (discreteAdapters == null)
                    {
                        discreteAdapters = new List<AdapterInfo>();
                    }
                    discreteAdapters.Add(adapter);
                }
            }

            if (discreteAdapters == null || discreteAdapters.Count == 0)
            {
                return false;
            }

            bool needsInstantConfirm;
            if (!RunD3dkmtSamplePass(discreteAdapters, false, out needsInstantConfirm, out isIdle))
            {
                return false;
            }

            if (!needsInstantConfirm)
            {
                DisposePdhOnly();
                isIdle = true;
                detail = "discrete GPU idle (D3DKMT)";
                return true;
            }

            Thread.Sleep(PdhSampleIntervalMs);

            bool unused;
            if (!RunD3dkmtSamplePass(discreteAdapters, true, out unused, out isIdle))
            {
                return false;
            }

            if (isIdle)
            {
                DisposePdhOnly();
                detail = "discrete GPU idle (D3DKMT)";
            }
            return true;
        }

        private static bool RunD3dkmtSamplePass(
            List<AdapterInfo> discreteAdapters,
            bool computeInstantUtilization,
            out bool needsInstantConfirm,
            out bool isIdle)
        {
            needsInstantConfirm = false;
            isIdle = false;

            lock (d3dkmtLock)
            {
                GCHandle handle = GCHandle.Alloc(d3dkmtQueryBuffer, GCHandleType.Pinned);
                try
                {
                    IntPtr bufferPtr = handle.AddrOfPinnedObject();
                    long nowTicks = GetMonotonic100nsTicks();
                    bool anyActive = false;

                    for (int i = 0; i < discreteAdapters.Count; i++)
                    {
                        AdapterInfo adapter = discreteAdapters[i];
                        D3dkmtAdapterSample sample;
                        if (!d3dkmtCache.TryGetValue(adapter.Luid, out sample))
                        {
                            sample = new D3dkmtAdapterSample();
                            d3dkmtCache[adapter.Luid] = sample;
                        }

                        int nodeCount;
                        if (!TryReadD3dkmtNodesLocked(adapter.RawLuid, bufferPtr, sample, out nodeCount))
                        {
                            return false;
                        }

                        long elapsed100ns = sample.TimestampTicks100ns > 0
                            ? nowTicks - sample.TimestampTicks100ns
                            : 0;
                        bool hasBaseline = sample.NodeCount == nodeCount &&
                            sample.RunningTimes.Length == nodeCount &&
                            elapsed100ns > 0;

                        if (!hasBaseline)
                        {
                            if (computeInstantUtilization)
                            {
                                return false;
                            }
                            needsInstantConfirm = true;
                        }
                        else if (computeInstantUtilization)
                        {
                            for (int node = 0; node < nodeCount; node++)
                            {
                                long delta = sample.ScratchTimes[node] - sample.RunningTimes[node];
                                if (delta < 0)
                                {
                                    return false;
                                }
                                double utilization = (delta * 100.0) / elapsed100ns;
                                if (utilization >= ActiveEngineUtilizationThreshold)
                                {
                                    anyActive = true;
                                    break;
                                }
                            }
                        }
                        else
                        {
                            for (int node = 0; node < nodeCount; node++)
                            {
                                long delta = sample.ScratchTimes[node] - sample.RunningTimes[node];
                                if (delta < 0 || delta >= D3dkmtIdleDeltaThresholdTicks)
                                {
                                    needsInstantConfirm = true;
                                    break;
                                }
                            }
                        }

                        sample.NodeCount = nodeCount;
                        sample.TimestampTicks100ns = nowTicks;
                        long[] prevTimes = sample.RunningTimes;
                        sample.RunningTimes = sample.ScratchTimes;
                        sample.ScratchTimes = prevTimes;
                    }

                    isIdle = !anyActive;
                    return true;
                }
                finally
                {
                    handle.Free();
                }
            }
        }

        private static bool TryReadD3dkmtNodesLocked(
            Luid luid,
            IntPtr bufferPtr,
            D3dkmtAdapterSample sample,
            out int nodeCount)
        {
            nodeCount = sample.NodeCount;
            if (nodeCount <= 0 || nodeCount > D3dkmtMaxNodeCount)
            {
                Array.Clear(d3dkmtQueryBuffer, 0, d3dkmtQueryBuffer.Length);
                Marshal.WriteInt32(bufferPtr, 0, (int)D3dkmtQueryTypeAdapter);
                Marshal.WriteInt32(bufferPtr, D3dkmtAdapterLuidOffset, (int)luid.LowPart);
                Marshal.WriteInt32(bufferPtr, D3dkmtAdapterLuidOffset + 4, luid.HighPart);
                if (D3DKMTQueryStatistics(bufferPtr) != 0)
                {
                    return false;
                }

                nodeCount = Marshal.ReadInt32(bufferPtr, D3dkmtAdapterNodeCountOffset);
                if (nodeCount <= 0 || nodeCount > D3dkmtMaxNodeCount)
                {
                    return false;
                }
            }

            if (sample.ScratchTimes.Length != nodeCount)
            {
                sample.ScratchTimes = new long[nodeCount];
            }

            for (int node = 0; node < nodeCount; node++)
            {
                Array.Clear(d3dkmtQueryBuffer, 0, d3dkmtQueryBuffer.Length);
                Marshal.WriteInt32(bufferPtr, 0, (int)D3dkmtQueryTypeNode);
                Marshal.WriteInt32(bufferPtr, D3dkmtAdapterLuidOffset, (int)luid.LowPart);
                Marshal.WriteInt32(bufferPtr, D3dkmtAdapterLuidOffset + 4, luid.HighPart);
                Marshal.WriteInt32(bufferPtr, D3dkmtQueryNodeIdOffset, node);
                if (D3DKMTQueryStatistics(bufferPtr) != 0)
                {
                    sample.NodeCount = 0;
                    return false;
                }

                sample.ScratchTimes[node] = Marshal.ReadInt64(bufferPtr, D3dkmtNodeRunningTimeOffset);
            }

            return true;
        }

        private static bool TryReadActiveGpuEngineLuidsPdh(HashSet<string> discreteLuids, out List<string> activeEngines)
        {
            activeEngines = new List<string>();
            var pidNames = new Dictionary<int, string>();

            lock (pdhLock)
            {
                if (!EnsurePdhCounterRefreshedLocked())
                {
                    ClosePdhQueryLocked();
                    return false;
                }

                if (PdhCollectQueryData(pdhQuery) != 0)
                {
                    ClosePdhQueryLocked();
                    return false;
                }

                Thread.Sleep(PdhSampleIntervalMs);

                if (PdhCollectQueryData(pdhQuery) != 0)
                {
                    ClosePdhQueryLocked();
                    return false;
                }

                uint bufferSize = 0;
                uint itemCount = 0;
                uint status = PdhGetFormattedCounterArrayW(pdhCounter, PdhFmtDouble, ref bufferSize, ref itemCount, IntPtr.Zero);
                if (status != 0 && status != PdhMoreData)
                {
                    ClosePdhQueryLocked();
                    return false;
                }

                if (itemCount == 0 || bufferSize == 0)
                {
                    return true;
                }

                if (pdhBuffer.Length < bufferSize)
                {
                    pdhBuffer = new byte[bufferSize];
                }

                GCHandle handle = GCHandle.Alloc(pdhBuffer, GCHandleType.Pinned);
                try
                {
                    status = PdhGetFormattedCounterArrayW(pdhCounter, PdhFmtDouble, ref bufferSize, ref itemCount, handle.AddrOfPinnedObject());
                    if (status != 0)
                    {
                        return false;
                    }

                    int structSize = Marshal.SizeOf(typeof(PdhFmtCounterValueItemDouble));
                    IntPtr basePtr = handle.AddrOfPinnedObject();
                    for (int i = 0; i < itemCount; i++)
                    {
                        var item = Marshal.PtrToStructure<PdhFmtCounterValueItemDouble>(IntPtr.Add(basePtr, i * structSize));
                        if ((item.CStatus != PdhCstatusValidData && item.CStatus != PdhCstatusNewData) ||
                            item.doubleValue < ActiveEngineUtilizationThreshold ||
                            item.szName == IntPtr.Zero)
                        {
                            continue;
                        }

                        string name = Marshal.PtrToStringUni(item.szName) ?? string.Empty;
                        AddMatchingEngineSample(name, item.doubleValue, discreteLuids, pidNames, activeEngines);
                    }
                }
                finally
                {
                    handle.Free();
                }
            }

            return true;
        }

        private static bool EnsurePdhCounterRefreshedLocked()
        {
            if (pdhQuery == IntPtr.Zero)
            {
                if (PdhOpenQueryW(null, IntPtr.Zero, out pdhQuery) != 0 || pdhQuery == IntPtr.Zero)
                {
                    pdhQuery = IntPtr.Zero;
                    return false;
                }
            }

            if (pdhCounter != IntPtr.Zero)
            {
                PdhRemoveCounter(pdhCounter);
                pdhCounter = IntPtr.Zero;
            }

            if (PdhAddEnglishCounterW(pdhQuery, GpuEngineUtilizationCounterPath, IntPtr.Zero, out pdhCounter) != 0 || pdhCounter == IntPtr.Zero)
            {
                pdhCounter = IntPtr.Zero;
                return false;
            }

            return true;
        }

        private static void ClosePdhQueryLocked()
        {
            if (pdhCounter != IntPtr.Zero)
            {
                try { PdhRemoveCounter(pdhCounter); } catch { }
                pdhCounter = IntPtr.Zero;
            }
            if (pdhQuery != IntPtr.Zero)
            {
                try { PdhCloseQuery(pdhQuery); } catch { }
                pdhQuery = IntPtr.Zero;
            }
        }

        private static void AddMatchingEngineSample(
            string name,
            double utilization,
            HashSet<string> discreteLuids,
            Dictionary<int, string> pidNames,
            List<string> activeEngines)
        {
            string luid = ParseEngineLuid(name);
            if (luid.Length == 0 || !discreteLuids.Contains(luid))
            {
                return;
            }

            int pid = ParseEnginePid(name);
            string processName = string.Empty;
            if (pid > 4 && !pidNames.TryGetValue(pid, out processName))
            {
                processName = ReadProcessName(pid);
                pidNames[pid] = processName;
            }
            if (IsIgnoredGpuActivityProcess(processName))
            {
                return;
            }

            activeEngines.Add(string.IsNullOrWhiteSpace(processName)
                ? "engine:" + name
                : string.Format("engine:{0}:{1:0.#}%", processName, utilization));
        }

        private static List<string> ReadActiveGpuEngineLuidsWmi(HashSet<string> discreteLuids)
        {
            var activeEngines = new List<string>();
            var pidNames = new Dictionary<int, string>();
            using (var searcher = new ManagementObjectSearcher("SELECT Name, UtilizationPercentage FROM Win32_PerfFormattedData_GPUPerformanceCounters_GPUEngine"))
            using (var results = searcher.Get())
            {
                foreach (ManagementObject item in results)
                {
                    using (item)
                    {
                        string name = Convert.ToString(item["Name"]) ?? string.Empty;
                        double utilization = ReadDouble(item["UtilizationPercentage"]);
                        if (utilization < ActiveEngineUtilizationThreshold)
                        {
                            continue;
                        }
                        AddMatchingEngineSample(name, utilization, discreteLuids, pidNames, activeEngines);
                    }
                }
            }
            return activeEngines;
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
                                RawLuid = desc.AdapterLuid,
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
            var match = EnginePidPattern.Match(instanceName ?? string.Empty);
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
