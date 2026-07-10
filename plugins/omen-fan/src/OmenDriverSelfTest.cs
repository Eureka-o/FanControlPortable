using System;

namespace OmenFanDriver
{
    internal static class OmenDriverSelfTest
    {
        public static OmenCapabilitySelfTestResult Run()
        {
            var capability = OmenCapabilitySelfTest.Run();
            int tests = capability.Tests;

            var fan = OmenWriteCommandEncoder.EncodeFanLevel(18, 21);
            ExpectEqual(OmenWriteCommandKind.FanLevel, fan.Kind, "fan write kind", ref tests);
            ExpectEqual("0x2E", OmenWriteCommandEncoder.FormatHex(fan.CommandType), "fan write command type", ref tests);
            ExpectEqual("12-15-00-00", OmenWriteCommandEncoder.FormatBytes(fan.Payload), "fan write payload", ref tests);

            var mode = OmenWriteCommandEncoder.EncodeMode("performance");
            ExpectEqual(OmenWriteCommandKind.Mode, mode.Kind, "mode write kind", ref tests);
            ExpectEqual("0x1A", OmenWriteCommandEncoder.FormatHex(mode.CommandType), "mode write command type", ref tests);
            ExpectEqual("FF-31-00-00", OmenWriteCommandEncoder.FormatBytes(mode.Payload), "mode write payload", ref tests);

            var rawMode = OmenWriteCommandEncoder.EncodeMode("0x50");
            ExpectEqual("FF-50-00-00", OmenWriteCommandEncoder.FormatBytes(rawMode.Payload), "raw mode byte payload", ref tests);

            var rejected = OmenHardwareWriteSafetyGate.Evaluate(fan, false);
            ExpectEqual(false, rejected.Allowed, "write gate rejects missing flag", ref tests);
            ExpectEqual("hardware-write-disabled", rejected.Category, "write gate missing flag category", ref tests);

            var allowed = OmenHardwareWriteSafetyGate.Evaluate(fan, true);
            ExpectEqual(true, allowed.Allowed, "write gate accepts explicit flag", ref tests);

            return new OmenCapabilitySelfTestResult(true, tests);
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
