using System;
using System.Globalization;
using Newtonsoft.Json.Linq;

namespace OmenFanDriver
{
    internal enum OmenWriteCommandKind
    {
        FanLevel,
        Mode
    }

    internal sealed class OmenEncodedWriteCommand
    {
        public OmenEncodedWriteCommand(OmenWriteCommandKind kind, uint commandType, byte[] payload)
        {
            Kind = kind;
            CommandType = commandType;
            Payload = payload;
        }

        public OmenWriteCommandKind Kind { get; private set; }
        public uint CommandType { get; private set; }
        public byte[] Payload { get; private set; }

        public JObject ToDryRunJson()
        {
            return new JObject
            {
                ["type"] = "write-command-dry-run",
                ["operation"] = Kind == OmenWriteCommandKind.FanLevel ? "fan-level" : "mode",
                ["hardwareWrite"] = false,
                ["command"] = OmenWriteCommandEncoder.FormatHex(OmenWriteCommandEncoder.BiosCommand),
                ["commandType"] = OmenWriteCommandEncoder.FormatHex(CommandType),
                ["signHex"] = OmenWriteCommandEncoder.FormatBytes(OmenWriteCommandEncoder.Sign),
                ["size"] = Payload.Length,
                ["payload"] = OmenWriteCommandEncoder.ToJsonArray(Payload),
                ["payloadHex"] = OmenWriteCommandEncoder.FormatBytes(Payload)
            };
        }
    }

    internal static class OmenWriteCommandEncoder
    {
        public const uint BiosCommand = 0x20008;
        public const uint CommandSetFanLevel = 0x2E;
        public const uint CommandSetFanMode = 0x1A;

        public static readonly byte[] Sign = { 0x53, 0x45, 0x43, 0x55 };

        public static OmenEncodedWriteCommand EncodeFanLevel(int cpuLevel, int gpuLevel)
        {
            ValidateLevel(cpuLevel, "cpuLevel");
            ValidateLevel(gpuLevel, "gpuLevel");
            return new OmenEncodedWriteCommand(
                OmenWriteCommandKind.FanLevel,
                CommandSetFanLevel,
                new[] { (byte)cpuLevel, (byte)gpuLevel, (byte)0x00, (byte)0x00 });
        }

        public static OmenEncodedWriteCommand EncodeMode(string mode)
        {
            byte modeByte = ParseModeByte(mode);
            return new OmenEncodedWriteCommand(
                OmenWriteCommandKind.Mode,
                CommandSetFanMode,
                new[] { (byte)0xFF, modeByte, (byte)0x00, (byte)0x00 });
        }

        internal static string FormatHex(uint value)
        {
            return "0x" + value.ToString("X2", CultureInfo.InvariantCulture);
        }

        internal static string FormatBytes(byte[] value)
        {
            return BitConverter.ToString(value);
        }

        internal static JArray ToJsonArray(byte[] value)
        {
            var result = new JArray();
            foreach (byte item in value)
            {
                result.Add(item);
            }

            return result;
        }

        private static void ValidateLevel(int level, string name)
        {
            if (level < FanLevelBounds.AbsoluteMinLevel || level > FanLevelBounds.AbsoluteMaxLevel)
            {
                throw new ArgumentException(name + " must be between 1 and 100.");
            }
        }

        private static byte ParseModeByte(string mode)
        {
            if (string.IsNullOrWhiteSpace(mode))
            {
                throw new ArgumentException("mode is required.");
            }

            string normalized = mode.Trim().ToLowerInvariant();
            switch (normalized)
            {
                case "default":
                case "balanced":
                    return 0x30;
                case "performance":
                    return 0x31;
                case "cool":
                    return 0x50;
                case "quiet":
                    return 0x03;
            }

            byte value;
            if (TryParseByteLiteral(normalized, out value))
            {
                return value;
            }

            throw new ArgumentException("mode must be default, balanced, performance, cool, quiet, or a byte literal such as 0x31.");
        }

        private static bool TryParseByteLiteral(string value, out byte result)
        {
            if (value.StartsWith("0x", StringComparison.OrdinalIgnoreCase))
            {
                return byte.TryParse(value.Substring(2), NumberStyles.HexNumber, CultureInfo.InvariantCulture, out result);
            }

            return byte.TryParse(value, NumberStyles.Integer, CultureInfo.InvariantCulture, out result);
        }
    }

    internal sealed class OmenHardwareWriteGateResult
    {
        public OmenHardwareWriteGateResult(bool allowed, string category, string message, OmenEncodedWriteCommand command)
        {
            Allowed = allowed;
            Category = category;
            Message = message;
            Command = command;
        }

        public bool Allowed { get; private set; }
        public string Category { get; private set; }
        public string Message { get; private set; }
        public OmenEncodedWriteCommand Command { get; private set; }

        public JObject ToJson()
        {
            return new JObject
            {
                ["type"] = "error",
                ["category"] = Category,
                ["message"] = Message,
                ["commandType"] = OmenWriteCommandEncoder.FormatHex(Command.CommandType),
                ["payloadHex"] = OmenWriteCommandEncoder.FormatBytes(Command.Payload)
            };
        }
    }

    internal static class OmenHardwareWriteSafetyGate
    {
        public static OmenHardwareWriteGateResult Evaluate(OmenEncodedWriteCommand command, bool hardwareWrite)
        {
            if (!hardwareWrite)
            {
                return new OmenHardwareWriteGateResult(
                    false,
                    "hardware-write-disabled",
                    "OMEN hardware writes are disabled unless --hardware-write is present. Use --dry-run-write to inspect the encoded command.",
                    command);
            }

            return new OmenHardwareWriteGateResult(
                true,
                "hardware-write-enabled",
                "OMEN hardware write gate is enabled for this command.",
                command);
        }
    }
}
