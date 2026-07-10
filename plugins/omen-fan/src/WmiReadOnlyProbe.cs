using System;
using System.Globalization;
using System.Management;
using System.Runtime.InteropServices;
using System.Threading.Tasks;
using Newtonsoft.Json.Linq;

namespace OmenFanDriver
{
    internal static class WmiReadOnlyProbe
    {
        private const string NamespaceName = @"root\wmi";
        private const string BiosClassName = "hpqBIntM";
        private const string DataInClassName = "hpqBDataIn";
        private const string MethodName = "hpqBIOSInt128";
        private const uint Command = 0x20008;
        private const uint CommandSystemDesign = 0x28;
        private const uint CommandFanState = 0x10;
        private const uint CommandFanType = 0x2C;
        private const uint CommandCurrentLevels = 0x2D;
        private const uint CommandFanTable = 0x2F;
        private const int OutputSize = 128;
        private const int ProbeTimeoutMilliseconds = 6000;

        private static readonly byte[] Sign = { 0x53, 0x45, 0x43, 0x55 };

        public static JObject Detect()
        {
            if (!IsWindows())
            {
                return Unsupported("non-windows", "OMEN WMI probe is only available on Windows.");
            }

            Task<JObject> task = Task.Run(new Func<JObject>(DetectCore));
            try
            {
                if (!task.Wait(ProbeTimeoutMilliseconds))
                {
                    return Unsupported("timeout", "OMEN WMI read-only probe timed out.");
                }

                return task.Result;
            }
            catch (AggregateException ex)
            {
                return FromException(ex.Flatten().InnerException ?? ex);
            }
            catch (Exception ex)
            {
                return FromException(ex);
            }
        }

        private static JObject DetectCore()
        {
            try
            {
                var scope = new ManagementScope(NamespaceName);
                scope.Connect();

                using (var dataInClass = new ManagementClass(scope, new ManagementPath(DataInClassName), null))
                {
                    dataInClass.Get();

                    using (var bios = FindBiosObject(scope))
                    {
                        if (bios == null)
                        {
                            return Unsupported("wmi-missing", "OMEN WMI class hpqBIntM was not found.");
                        }

                        var design = InvokeRead(bios, dataInClass, CommandSystemDesign);
                        if (!design.Success)
                        {
                            return UnsupportedFromCommand(design);
                        }

                        if (design.Data == null || design.Data.Length <= 4)
                        {
                            return Unsupported("malformed-output", "System design data was missing the SwFanControl bit.");
                        }

                        bool supportedByDesign = (design.Data[4] & 0x01) != 0;
                        if (!supportedByDesign)
                        {
                            return Unsupported("unsupported-hardware", "OMEN WMI is present but software fan control is not advertised.");
                        }

                        var fanState = InvokeRead(bios, dataInClass, CommandFanState);
                        if (!fanState.Success)
                        {
                            return UnsupportedFromCommand(fanState);
                        }

                        if (fanState.Data == null || fanState.Data.Length < 2)
                        {
                            return Unsupported("malformed-output", "Fan state output was shorter than expected.");
                        }

                        var fanTypes = InvokeRead(bios, dataInClass, CommandFanType);
                        if (!fanTypes.Success)
                        {
                            return UnsupportedFromCommand(fanTypes);
                        }

                        var levels = InvokeRead(bios, dataInClass, CommandCurrentLevels);
                        if (!levels.Success)
                        {
                            return UnsupportedFromCommand(levels);
                        }

                        if (levels.Data == null || levels.Data.Length < 2)
                        {
                            return Unsupported("malformed-output", "Current fan level output was shorter than expected.");
                        }

                        var table = InvokeRead(bios, dataInClass, CommandFanTable);
                        if (!table.Success)
                        {
                            return UnsupportedFromCommand(table);
                        }

                        return Supported(design.Data, fanState.Data, fanTypes.Data, levels.Data, table.Data);
                    }
                }
            }
            catch (Exception ex)
            {
                return FromException(ex);
            }
        }

        private static ManagementObject FindBiosObject(ManagementScope scope)
        {
            var query = new ObjectQuery("SELECT * FROM " + BiosClassName);
            var options = new EnumerationOptions
            {
                Timeout = TimeSpan.FromSeconds(2),
                ReturnImmediately = true
            };

            using (var searcher = new ManagementObjectSearcher(scope, query, options))
            using (var collection = searcher.Get())
            {
                foreach (ManagementObject item in collection)
                {
                    return item;
                }
            }

            return null;
        }

        private static CommandResult InvokeRead(ManagementObject bios, ManagementClass dataInClass, uint commandType)
        {
            try
            {
                using (var dataIn = dataInClass.CreateInstance())
                using (var inParams = bios.GetMethodParameters(MethodName))
                {
                    if (dataIn == null || inParams == null)
                    {
                        return CommandResult.Malformed(commandType, "WMI method parameters could not be created.");
                    }

                    dataIn["Command"] = Command;
                    dataIn["CommandType"] = commandType;
                    dataIn["Sign"] = Sign;
                    byte[] payload = CreateReadPayload();
                    dataIn["Size"] = (uint)payload.Length;
                    dataIn["hpqBData"] = payload;
                    inParams["InData"] = dataIn;

                    using (var result = bios.InvokeMethod(MethodName, inParams, null))
                    {
                        if (result == null)
                        {
                            return CommandResult.Malformed(commandType, "WMI method returned no result.");
                        }

                        var outData = result["OutData"] as ManagementBaseObject;
                        if (outData == null)
                        {
                            return CommandResult.Malformed(commandType, "WMI method returned no output object.");
                        }

                        using (outData)
                        {
                            object returnValue = outData["rwReturnCode"];
                            if (returnValue == null)
                            {
                                return CommandResult.Malformed(commandType, "WMI output did not include a BIOS return code.");
                            }

                            uint returnCode = Convert.ToUInt32(returnValue, CultureInfo.InvariantCulture);
                            if (returnCode != 0)
                            {
                                return CommandResult.BiosCode(commandType, returnCode);
                            }

                            var data = outData["Data"] as byte[];
                            if (data == null)
                            {
                                return CommandResult.Malformed(commandType, "WMI output did not include data.");
                            }

                            if (data.Length < OutputSize)
                            {
                                return CommandResult.Malformed(commandType, "WMI output data was shorter than expected.");
                            }

                            return CommandResult.Ok(commandType, data);
                        }
                    }
                }
            }
            catch (Exception ex)
            {
                return CommandResult.Failed(commandType, ex);
            }
        }

        private static JObject Supported(byte[] designData, byte[] fanStateData, byte[] fanTypeData, byte[] levelData, byte[] tableData)
        {
            var design = OmenSystemDesignData.Parse(designData);
            var fanState = OmenFanState.Parse(fanStateData);
            var fanTypes = OmenFanTypes.Parse(fanTypeData);
            var levels = OmenCurrentLevels.Parse(levelData);
            var table = OmenFanTable.Parse(tableData);

            int cpuLevel = levels.CpuLevel ?? levelData[0];
            int gpuLevel = levels.GpuLevel ?? levelData[1];
            int? fallbackMaxLevel = InferMaxLevel(tableData, cpuLevel, gpuLevel);
            int? maxCpuLevel = DeriveMaxLevel(table.Fan1MaxLevel, cpuLevel, fallbackMaxLevel);
            int? maxGpuLevel = DeriveMaxLevel(table.Fan2MaxLevel, gpuLevel, fallbackMaxLevel);

            var status = OmenCapabilityJson.BuildStatusFragment(design, fanState, fanTypes, levels, table);
            status["type"] = "detect-result";
            status["supported"] = true;
            status["mock"] = false;
            status["reason"] = "OMEN WMI read-only probe succeeded.";
            status["mode"] = "read-only";
            status["readOnly"] = true;
            status["swFanControl"] = design.SwFanControl ?? true;
            status["cpuLevel"] = cpuLevel;
            status["gpuLevel"] = gpuLevel;
            status["cpuRpm"] = cpuLevel * 100;
            status["gpuRpm"] = gpuLevel * 100;
            status["rpmEstimated"] = true;
            status["fanState"] = ToByteArray(fanStateData, 8);
            status["lastUpdated"] = DateTimeOffset.UtcNow.ToUnixTimeMilliseconds();

            if (maxCpuLevel.HasValue)
            {
                status["maxCpuRpm"] = maxCpuLevel.Value * 100;
            }

            if (maxGpuLevel.HasValue)
            {
                status["maxGpuRpm"] = maxGpuLevel.Value * 100;
            }

            int? maxLevel = MaxNullable(maxCpuLevel, maxGpuLevel);
            if (maxLevel.HasValue)
            {
                status["maxLevel"] = maxLevel.Value;
            }

            return status;
        }

        private static byte[] CreateReadPayload()
        {
            return new byte[] { 0, 0, 0, 0 };
        }

        private static int? DeriveMaxLevel(int? parsedMaxLevel, int observedLevel, int? fallbackMaxLevel)
        {
            if (IsPlausibleMaxLevel(parsedMaxLevel, observedLevel))
            {
                return parsedMaxLevel.Value;
            }

            return fallbackMaxLevel;
        }

        private static bool IsPlausibleMaxLevel(int? value, int observedLevel)
        {
            return value.HasValue
                && value.Value >= 10
                && value.Value <= 100
                && value.Value >= observedLevel;
        }

        private static int? MaxNullable(int? left, int? right)
        {
            if (!left.HasValue)
            {
                return right;
            }

            if (!right.HasValue)
            {
                return left;
            }

            return Math.Max(left.Value, right.Value);
        }

        private static int? InferMaxLevel(byte[] tableData, int cpuLevel, int gpuLevel)
        {
            if (tableData == null || tableData.Length == 0)
            {
                return null;
            }

            int observed = Math.Max(cpuLevel, gpuLevel);
            int max = 0;
            int plausibleValues = 0;

            foreach (byte value in tableData)
            {
                if (value > 0 && value <= 100)
                {
                    plausibleValues++;
                    if (value > max)
                    {
                        max = value;
                    }
                }
            }

            if (plausibleValues == 0 || max < observed || max < 10)
            {
                return null;
            }

            return max;
        }

        private static JArray ToByteArray(byte[] data, int count)
        {
            var result = new JArray();
            if (data == null)
            {
                return result;
            }

            int limit = Math.Min(count, data.Length);
            for (int i = 0; i < limit; i++)
            {
                result.Add(data[i]);
            }

            return result;
        }

        private static JObject UnsupportedFromCommand(CommandResult result)
        {
            string reason = result.Reason;
            if (string.IsNullOrWhiteSpace(reason))
            {
                reason = "OMEN WMI read command " + FormatCommand(result.CommandType) + " failed safely.";
            }

            var payload = Unsupported(result.Category, reason);
            payload["commandType"] = FormatCommand(result.CommandType);
            if (result.ReturnCode.HasValue)
            {
                payload["biosReturnCode"] = "0x" + result.ReturnCode.Value.ToString("X2", CultureInfo.InvariantCulture);
            }

            return payload;
        }

        private static JObject FromException(Exception ex)
        {
            string category = "wmi-error";
            if (IsAccessDenied(ex))
            {
                category = "access-denied";
            }
            else if (IsMissingWmi(ex))
            {
                category = "wmi-missing";
            }

            string message = ex == null ? "unknown error" : ex.Message;
            return Unsupported(category, "OMEN WMI read-only probe is not available: " + message);
        }

        private static bool IsAccessDenied(Exception ex)
        {
            var management = ex as ManagementException;
            if (management != null && management.ErrorCode == ManagementStatus.AccessDenied)
            {
                return true;
            }

            var unauthorized = ex as UnauthorizedAccessException;
            if (unauthorized != null)
            {
                return true;
            }

            var com = ex as COMException;
            return com != null && unchecked((uint)com.ErrorCode) == 0x80070005u;
        }

        private static bool IsMissingWmi(Exception ex)
        {
            var management = ex as ManagementException;
            if (management == null)
            {
                return false;
            }

            return management.ErrorCode == ManagementStatus.InvalidClass
                || management.ErrorCode == ManagementStatus.InvalidNamespace
                || management.ErrorCode == ManagementStatus.NotFound;
        }

        private static JObject Unsupported(string category, string reason)
        {
            return new JObject
            {
                ["type"] = "detect-result",
                ["supported"] = false,
                ["mock"] = false,
                ["reason"] = reason,
                ["category"] = category
            };
        }

        private static string FormatCommand(uint commandType)
        {
            return "0x" + commandType.ToString("X2", CultureInfo.InvariantCulture);
        }

        private static bool IsWindows()
        {
            PlatformID platform = Environment.OSVersion.Platform;
            return platform == PlatformID.Win32NT
                || platform == PlatformID.Win32S
                || platform == PlatformID.Win32Windows
                || platform == PlatformID.WinCE;
        }

        private sealed class CommandResult
        {
            private CommandResult(uint commandType)
            {
                CommandType = commandType;
            }

            public uint CommandType { get; private set; }
            public bool Success { get; private set; }
            public byte[] Data { get; private set; }
            public string Category { get; private set; }
            public string Reason { get; private set; }
            public uint? ReturnCode { get; private set; }

            public static CommandResult Ok(uint commandType, byte[] data)
            {
                return new CommandResult(commandType)
                {
                    Success = true,
                    Data = data
                };
            }

            public static CommandResult BiosCode(uint commandType, uint returnCode)
            {
                return new CommandResult(commandType)
                {
                    Category = "bios-return-code",
                    ReturnCode = returnCode,
                    Reason = "OMEN BIOS returned code 0x" + returnCode.ToString("X2", CultureInfo.InvariantCulture) + " for read command " + FormatCommand(commandType) + "."
                };
            }

            public static CommandResult Malformed(uint commandType, string reason)
            {
                return new CommandResult(commandType)
                {
                    Category = "malformed-output",
                    Reason = reason
                };
            }

            public static CommandResult Failed(uint commandType, Exception ex)
            {
                string category = "wmi-error";
                if (IsAccessDenied(ex))
                {
                    category = "access-denied";
                }
                else if (IsMissingWmi(ex))
                {
                    category = "wmi-missing";
                }

                return new CommandResult(commandType)
                {
                    Category = category,
                    Reason = "OMEN WMI read command " + FormatCommand(commandType) + " failed safely: " + ex.Message
                };
            }
        }
    }
}
