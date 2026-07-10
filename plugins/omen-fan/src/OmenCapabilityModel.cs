using System;
using System.Collections.Generic;
using Newtonsoft.Json.Linq;

namespace OmenFanDriver
{
    internal sealed class OmenSystemDesignData
    {
        public int? AdapterWatts { get; private set; }
        public int? ThermalPolicyVersion { get; private set; }
        public bool? SwFanControl { get; private set; }

        public static OmenSystemDesignData Parse(byte[] data)
        {
            var result = new OmenSystemDesignData();
            if (data == null)
            {
                return result;
            }

            if (data.Length >= 2)
            {
                result.AdapterWatts = data[0] | (data[1] << 8);
            }

            if (data.Length >= 4)
            {
                result.ThermalPolicyVersion = data[3];
            }

            if (data.Length >= 5)
            {
                result.SwFanControl = (data[4] & 0x01) != 0;
            }

            return result;
        }
    }

    internal sealed class OmenFanState
    {
        public int? FanCount { get; private set; }

        public static OmenFanState Parse(byte[] data)
        {
            return new OmenFanState
            {
                FanCount = data != null && data.Length >= 1 ? (int?)data[0] : null
            };
        }
    }

    internal sealed class OmenFanTypeDescriptor
    {
        public OmenFanTypeDescriptor(int index, int raw, string type)
        {
            Index = index;
            Raw = raw;
            Type = type;
        }

        public int Index { get; private set; }
        public int Raw { get; private set; }
        public string Type { get; private set; }
    }

    internal sealed class OmenFanTypes
    {
        private readonly List<OmenFanTypeDescriptor> fans;

        private OmenFanTypes(List<OmenFanTypeDescriptor> fans)
        {
            this.fans = fans;
        }

        public IList<OmenFanTypeDescriptor> Fans
        {
            get { return fans.AsReadOnly(); }
        }

        public static OmenFanTypes Parse(byte[] data)
        {
            var fans = new List<OmenFanTypeDescriptor>();
            if (data == null)
            {
                return new OmenFanTypes(fans);
            }

            int fanIndex = 0;
            for (int i = 0; i < data.Length; i++)
            {
                AddFan(fans, fanIndex++, data[i] & 0x0F);
                AddFan(fans, fanIndex++, (data[i] >> 4) & 0x0F);
            }

            return new OmenFanTypes(fans);
        }

        private static void AddFan(List<OmenFanTypeDescriptor> fans, int index, int raw)
        {
            if (raw == 0)
            {
                return;
            }

            fans.Add(new OmenFanTypeDescriptor(index, raw, FanTypeName(raw)));
        }

        private static string FanTypeName(int value)
        {
            switch (value)
            {
                case 1:
                    return "cpu";
                case 2:
                    return "gpu";
                case 3:
                    return "exhaust";
                case 4:
                    return "pump";
                case 5:
                    return "intake";
                default:
                    return "unknown";
            }
        }
    }

    internal sealed class OmenCurrentLevels
    {
        public int? CpuLevel { get; private set; }
        public int? GpuLevel { get; private set; }

        public static OmenCurrentLevels Parse(byte[] data)
        {
            var result = new OmenCurrentLevels();
            if (data == null)
            {
                return result;
            }

            if (data.Length >= 1)
            {
                result.CpuLevel = data[0];
            }

            if (data.Length >= 2)
            {
                result.GpuLevel = data[1];
            }

            return result;
        }
    }

    internal sealed class OmenFanTable
    {
        public int? FanCount { get; private set; }
        public int? LevelCount { get; private set; }
        public int? Fan1MinLevel { get; private set; }
        public int? Fan1MaxLevel { get; private set; }
        public int? Fan2MinLevel { get; private set; }
        public int? Fan2MaxLevel { get; private set; }
        public int? MinTemperature { get; private set; }
        public int? MaxTemperature { get; private set; }

        public static OmenFanTable Parse(byte[] data)
        {
            var result = new OmenFanTable();
            if (data == null)
            {
                return result;
            }

            if (data.Length >= 1)
            {
                result.FanCount = data[0];
            }

            if (data.Length >= 2)
            {
                result.LevelCount = data[1];
            }

            int triplesToRead = result.LevelCount.HasValue && result.LevelCount.Value > 0
                ? result.LevelCount.Value
                : int.MaxValue;
            int triplesRead = 0;
            for (int offset = 2; offset + 2 < data.Length && triplesRead < triplesToRead; offset += 3)
            {
                result.AddTriple(data[offset], data[offset + 1], data[offset + 2]);
                triplesRead++;
            }

            return result;
        }

        private void AddTriple(int fan1Level, int fan2Level, int temperature)
        {
            Fan1MinLevel = MinValue(Fan1MinLevel, fan1Level);
            Fan1MaxLevel = MaxValue(Fan1MaxLevel, fan1Level);
            Fan2MinLevel = MinValue(Fan2MinLevel, fan2Level);
            Fan2MaxLevel = MaxValue(Fan2MaxLevel, fan2Level);
            MinTemperature = MinValue(MinTemperature, temperature);
            MaxTemperature = MaxValue(MaxTemperature, temperature);
        }

        private static int MinValue(int? current, int value)
        {
            return !current.HasValue || value < current.Value ? value : current.Value;
        }

        private static int MaxValue(int? current, int value)
        {
            return !current.HasValue || value > current.Value ? value : current.Value;
        }
    }

    internal static class OmenCapabilityJson
    {
        public static JObject BuildStatusFragment(
            OmenSystemDesignData design,
            OmenFanState fanState,
            OmenFanTypes fanTypes,
            OmenCurrentLevels levels,
            OmenFanTable table)
        {
            var status = new JObject
            {
                ["swFanControl"] = ValueOrNull(design == null ? null : design.SwFanControl),
                ["thermalPolicyVersion"] = ValueOrNull(design == null ? null : design.ThermalPolicyVersion),
                ["adapterWatts"] = ValueOrNull(design == null ? null : design.AdapterWatts),
                ["fanCount"] = ValueOrNull(fanState == null ? null : fanState.FanCount),
                ["cpuLevel"] = ValueOrNull(levels == null ? null : levels.CpuLevel),
                ["gpuLevel"] = ValueOrNull(levels == null ? null : levels.GpuLevel),
                ["fanTypes"] = FanTypesToJson(fanTypes),
                ["fanTable"] = FanTableToJson(table)
            };

            return status;
        }

        private static JToken ValueOrNull(int? value)
        {
            return value.HasValue ? new JValue(value.Value) : JValue.CreateNull();
        }

        private static JToken ValueOrNull(bool? value)
        {
            return value.HasValue ? new JValue(value.Value) : JValue.CreateNull();
        }

        private static JArray FanTypesToJson(OmenFanTypes fanTypes)
        {
            var items = new JArray();
            if (fanTypes == null)
            {
                return items;
            }

            foreach (var fan in fanTypes.Fans)
            {
                items.Add(new JObject
                {
                    ["index"] = fan.Index,
                    ["type"] = fan.Type,
                    ["raw"] = fan.Raw
                });
            }

            return items;
        }

        private static JObject FanTableToJson(OmenFanTable table)
        {
            if (table == null)
            {
                return new JObject();
            }

            return new JObject
            {
                ["fanCount"] = ValueOrNull(table.FanCount),
                ["levelCount"] = ValueOrNull(table.LevelCount),
                ["fan1MinLevel"] = ValueOrNull(table.Fan1MinLevel),
                ["fan1MaxLevel"] = ValueOrNull(table.Fan1MaxLevel),
                ["fan2MinLevel"] = ValueOrNull(table.Fan2MinLevel),
                ["fan2MaxLevel"] = ValueOrNull(table.Fan2MaxLevel),
                ["minTemperature"] = ValueOrNull(table.MinTemperature),
                ["maxTemperature"] = ValueOrNull(table.MaxTemperature)
            };
        }
    }

    internal sealed class OmenCapabilitySelfTestResult
    {
        public OmenCapabilitySelfTestResult(bool ok, int tests)
        {
            Ok = ok;
            Tests = tests;
        }

        public bool Ok { get; private set; }
        public int Tests { get; private set; }
    }

    internal static class OmenCapabilitySelfTest
    {
        public static OmenCapabilitySelfTestResult Run()
        {
            int tests = 0;

            var design = OmenSystemDesignData.Parse(new byte[] { 0xE6, 0x00, 0x00, 0x07, 0x01 });
            ExpectEqual(230, design.AdapterWatts, "system design adapter watts", ref tests);
            ExpectEqual(7, design.ThermalPolicyVersion, "system design thermal policy", ref tests);
            ExpectEqual(true, design.SwFanControl, "system design sw fan control", ref tests);
            ExpectEqual((bool?)null, OmenSystemDesignData.Parse(new byte[] { 0x01, 0x02, 0x03, 0x04 }).SwFanControl, "system design missing sw fan control", ref tests);

            var fanState = OmenFanState.Parse(new byte[] { 0x02 });
            ExpectEqual(2, fanState.FanCount, "fan state count", ref tests);
            ExpectEqual((int?)null, OmenFanState.Parse(new byte[0]).FanCount, "fan state missing count", ref tests);

            var fanTypes = OmenFanTypes.Parse(new byte[] { 0x21, 0x40, 0xF5 });
            ExpectEqual(5, fanTypes.Fans.Count, "fan type nonzero count", ref tests);
            ExpectFan(fanTypes.Fans[0], 0, 1, "cpu", "fan type low nibble", ref tests);
            ExpectFan(fanTypes.Fans[1], 1, 2, "gpu", "fan type high nibble", ref tests);
            ExpectFan(fanTypes.Fans[2], 3, 4, "pump", "fan type ignores zero low nibble", ref tests);
            ExpectFan(fanTypes.Fans[3], 4, 5, "intake", "fan type second byte low nibble", ref tests);
            ExpectFan(fanTypes.Fans[4], 5, 15, "unknown", "fan type unknown high nibble", ref tests);

            var levels = OmenCurrentLevels.Parse(new byte[] { 18, 21 });
            ExpectEqual(18, levels.CpuLevel, "current levels cpu", ref tests);
            ExpectEqual(21, levels.GpuLevel, "current levels gpu", ref tests);
            ExpectEqual((int?)null, OmenCurrentLevels.Parse(new byte[] { 18 }).GpuLevel, "current levels missing gpu", ref tests);

            var table = OmenFanTable.Parse(new byte[] { 2, 3, 10, 12, 45, 20, 22, 55, 30, 31, 65, 0, 0, 0, 0, 0, 0, 99 });
            ExpectEqual(2, table.FanCount, "fan table fan count", ref tests);
            ExpectEqual(3, table.LevelCount, "fan table level count", ref tests);
            ExpectEqual(10, table.Fan1MinLevel, "fan table fan1 min", ref tests);
            ExpectEqual(30, table.Fan1MaxLevel, "fan table fan1 max", ref tests);
            ExpectEqual(12, table.Fan2MinLevel, "fan table fan2 min", ref tests);
            ExpectEqual(31, table.Fan2MaxLevel, "fan table fan2 max", ref tests);
            ExpectEqual(45, table.MinTemperature, "fan table temp min", ref tests);
            ExpectEqual(65, table.MaxTemperature, "fan table temp max", ref tests);

            var shortTable = OmenFanTable.Parse(new byte[] { 2, 3, 10, 12 });
            ExpectEqual((int?)null, shortTable.Fan1MinLevel, "fan table ignores incomplete triple", ref tests);

            var fragment = OmenCapabilityJson.BuildStatusFragment(design, fanState, fanTypes, levels, table);
            ExpectEqual(true, fragment.Value<bool?>("swFanControl"), "json sw fan control", ref tests);
            ExpectEqual(18, fragment.Value<int?>("cpuLevel"), "json cpu level", ref tests);
            ExpectEqual(5, ((JArray)fragment["fanTypes"]).Count, "json fan types", ref tests);
            ExpectEqual(65, fragment["fanTable"].Value<int?>("maxTemperature"), "json fan table max temp", ref tests);

            return new OmenCapabilitySelfTestResult(true, tests);
        }

        private static void ExpectFan(OmenFanTypeDescriptor actual, int index, int raw, string type, string name, ref int tests)
        {
            ExpectEqual(index, actual.Index, name + " index", ref tests);
            ExpectEqual(raw, actual.Raw, name + " raw", ref tests);
            ExpectEqual(type, actual.Type, name + " type", ref tests);
        }

        private static void ExpectEqual<T>(T expected, T actual, string name, ref int tests)
        {
            tests++;
            if (!object.Equals(expected, actual))
            {
                throw new InvalidOperationException(name + " expected " + FormatValue(expected) + " but got " + FormatValue(actual));
            }
        }

        private static string FormatValue<T>(T value)
        {
            return value == null ? "<null>" : value.ToString();
        }
    }
}
