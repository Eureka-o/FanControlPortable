using System;
using System.Collections.Generic;
using System.IO;
using System.Net;
using System.Text;
using System.Threading;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;

namespace OmenFanDriver
{
    internal static class Program
    {
        private const int Success = 0;
        private const int UsageError = 2;
        private const int SafetyError = 3;
        private const int MockMinRpm = 800;
        private const int MockMaxCpuRpm = 5200;
        private const int MockMaxGpuRpm = 5000;
        private const int DefaultMockHttpPort = 8787;

        private static int Main(string[] args)
        {
            // Force UTF-8 so Chinese error messages don't arrive as GBK bytes
            Console.OutputEncoding = Encoding.UTF8;
            Console.InputEncoding  = Encoding.UTF8;
            try
            {
                var options = ParseOptions(args);
                if (options == null)
                {
                    PrintUsage();
                    return UsageError;
                }

                if (options.SelfTestParsers)
                {
                    var result = OmenDriverSelfTest.Run();
                    Console.WriteLine(new JObject
                    {
                        ["type"] = "self-test",
                        ["ok"] = result.Ok,
                        ["tests"] = result.Tests
                    }.ToString(Formatting.None));
                    return Success;
                }

                if (options.WriteRequested)
                {
                    return RunWriteCommand(options);
                }

                if (options.DetectOnly)
                {
                    RunDetectOnly(options.Mock);
                    return Success;
                }

                if (options.Daemon && options.Mock)
                {
                    RunMockDaemon();
                    return Success;
                }

                if (options.MockHttp)
                {
                    RunMockHttpServer(options.Port);
                    return Success;
                }

                PrintUsage();
                return UsageError;
            }
            catch (Exception ex)
            {
                Console.Error.WriteLine("omen-fan-driver failed: " + ex.Message);
                return 1;
            }
        }

        private static Options ParseOptions(string[] args)
        {
            var options = new Options();
            var seen = new HashSet<string>(StringComparer.OrdinalIgnoreCase);

            for (var i = 0; i < args.Length; i++)
            {
                var arg = args[i];
                if (arg == "--detect-only")
                {
                    options.DetectOnly = true;
                }
                else if (arg == "--daemon")
                {
                    options.Daemon = true;
                }
                else if (arg == "--mock")
                {
                    options.Mock = true;
                }
                else if (arg == "--mock-http")
                {
                    options.MockHttp = true;
                }
                else if (arg == "--self-test-parsers")
                {
                    options.SelfTestParsers = true;
                }
                else if (arg == "--dry-run-write")
                {
                    options.DryRunWrite = true;
                }
                else if (arg == "--hardware-write")
                {
                    options.HardwareWrite = true;
                }
                else if (arg == "--write-fan-level")
                {
                    if (!seen.Add("--write-fan-level") || i + 2 >= args.Length
                        || !TryParseByte(args[++i], out var cpuLevel)
                        || !TryParseByte(args[++i], out var gpuLevel))
                    {
                        return null;
                    }

                    options.WriteFanLevel = true;
                    options.CpuLevel = cpuLevel;
                    options.GpuLevel = gpuLevel;
                    continue;
                }
                else if (arg == "--write-mode")
                {
                    if (!seen.Add("--write-mode") || i + 1 >= args.Length)
                    {
                        return null;
                    }

                    options.WriteMode = true;
                    options.Mode = args[++i];
                    continue;
                }
                else if (arg == "--port")
                {
                    if (!seen.Add("--port") || i + 1 >= args.Length || !TryParsePort(args[++i], out var port))
                    {
                        return null;
                    }

                    options.Port = port;
                    options.PortSpecified = true;
                    continue;
                }
                else if (arg.StartsWith("--port=", StringComparison.OrdinalIgnoreCase))
                {
                    if (!seen.Add("--port") || !TryParsePort(arg.Substring("--port=".Length), out var port))
                    {
                        return null;
                    }

                    options.Port = port;
                    options.PortSpecified = true;
                    continue;
                }
                else
                {
                    return null;
                }

                if (!seen.Add(arg))
                {
                    return null;
                }
            }

            var writeCommandCount = (options.WriteFanLevel ? 1 : 0) + (options.WriteMode ? 1 : 0);
            if (writeCommandCount > 1)
            {
                return null;
            }

            options.WriteRequested = options.DryRunWrite || options.HardwareWrite || writeCommandCount == 1;

            var modeCount = (options.DetectOnly ? 1 : 0)
                + (options.Daemon ? 1 : 0)
                + (options.MockHttp ? 1 : 0)
                + (options.SelfTestParsers ? 1 : 0)
                + (options.WriteRequested ? 1 : 0);
            if (modeCount != 1)
            {
                return null;
            }

            if (options.WriteRequested)
            {
                if (writeCommandCount != 1)
                {
                    return null;
                }

                if (options.DryRunWrite && options.HardwareWrite)
                {
                    return null;
                }
            }

            if (options.Daemon && !options.Mock)
            {
                return null;
            }

            if (options.MockHttp && (options.Mock || options.Port < 1 || options.Port > 65535))
            {
                return null;
            }

            if (!options.MockHttp && options.PortSpecified)
            {
                return null;
            }

            return options;
        }

        private static bool TryParsePort(string raw, out int port)
        {
            port = 0;
            return int.TryParse(raw, out port) && port >= 1 && port <= 65535;
        }

        private static bool TryParseByte(string raw, out int value)
        {
            value = 0;
            return int.TryParse(raw, out value) && value >= byte.MinValue && value <= byte.MaxValue;
        }

        private static int RunWriteCommand(Options options)
        {
            OmenEncodedWriteCommand command;
            try
            {
                command = options.WriteFanLevel
                    ? OmenWriteCommandEncoder.EncodeFanLevel(options.CpuLevel, options.GpuLevel)
                    : OmenWriteCommandEncoder.EncodeMode(options.Mode);
            }
            catch (ArgumentException ex)
            {
                WriteConsoleJson(new JObject
                {
                    ["type"] = "error",
                    ["message"] = ex.Message
                });
                return UsageError;
            }

            if (options.DryRunWrite)
            {
                WriteConsoleJson(command.ToDryRunJson());
                return Success;
            }

            var gate = OmenHardwareWriteSafetyGate.Evaluate(command, options.HardwareWrite);
            if (!gate.Allowed)
            {
                WriteConsoleJson(gate.ToJson());
                return SafetyError;
            }

            WriteConsoleJson(new JObject
            {
                ["type"] = "error",
                ["category"] = "hardware-write-not-implemented",
                ["message"] = "Hardware write gate accepted, but real OMEN WMI writes are not implemented in this driver slice.",
                ["command"] = command.ToDryRunJson()
            });
            return 1;
        }

        private static void RunDetectOnly(bool mock)
        {
            if (mock)
            {
                Console.WriteLine("{\"type\":\"detect-result\",\"supported\":true,\"mock\":true,\"reason\":\"Mock OMEN fan driver enabled.\"}");
                return;
            }

            try
            {
                Console.WriteLine(WmiReadOnlyProbe.Detect().ToString(Formatting.None));
            }
            catch (Exception ex)
            {
                var payload = new JObject
                {
                    ["type"] = "detect-result",
                    ["supported"] = false,
                    ["mock"] = false,
                    ["reason"] = "OMEN WMI read-only probe failed safely: " + ex.Message,
                    ["category"] = "probe-exception"
                };
                Console.WriteLine(payload.ToString(Formatting.None));
            }
        }

        private static void RunMockHttpServer(int port)
        {
            var state = new MockHttpState(new MockFanLevelWriter(MockMinRpm, MockMaxCpuRpm, MockMaxGpuRpm));
            using (var listener = new HttpListener())
            {
                listener.Prefixes.Add("http://127.0.0.1:" + port + "/");
                listener.Prefixes.Add("http://localhost:" + port + "/");
                Console.CancelKeyPress += delegate
                {
                    if (listener.IsListening)
                    {
                        listener.Stop();
                    }
                };

                listener.Start();
                Console.Error.WriteLine("OMEN mock HTTP server listening on http://127.0.0.1:" + port + "/");

                while (listener.IsListening)
                {
                    try
                    {
                        HandleMockHttpRequest(listener.GetContext(), state);
                    }
                    catch (HttpListenerException)
                    {
                        break;
                    }
                    catch (ObjectDisposedException)
                    {
                        break;
                    }
                }
            }
        }

        private static void HandleMockHttpRequest(HttpListenerContext context, MockHttpState state)
        {
            try
            {
                var request = context.Request;
                var path = request.Url.AbsolutePath.TrimEnd('/');
                if (path.Length == 0)
                {
                    path = "/";
                }

                if (string.Equals(request.HttpMethod, "OPTIONS", StringComparison.OrdinalIgnoreCase))
                {
                    WriteHttpNoContent(context.Response);
                    return;
                }

                if (string.Equals(request.HttpMethod, "GET", StringComparison.OrdinalIgnoreCase)
                    && string.Equals(path, "/health", StringComparison.OrdinalIgnoreCase))
                {
                    WriteHttpJson(context.Response, HttpStatusCode.OK, new JObject
                    {
                        ["ok"] = true,
                        ["mock"] = true
                    });
                    return;
                }

                if (string.Equals(request.HttpMethod, "GET", StringComparison.OrdinalIgnoreCase)
                    && string.Equals(path, "/status", StringComparison.OrdinalIgnoreCase))
                {
                    WriteHttpJson(context.Response, HttpStatusCode.OK, state.CreateStatusPayload());
                    return;
                }

                if (string.Equals(request.HttpMethod, "POST", StringComparison.OrdinalIgnoreCase)
                    && string.Equals(path, "/set-fan", StringComparison.OrdinalIgnoreCase))
                {
                    var body = ReadJsonBody(request);
                    int cpuRpm;
                    int gpuRpm;
                    if (!TryReadInt(body, "cpuRpm", out cpuRpm) || !TryReadInt(body, "gpuRpm", out gpuRpm))
                    {
                        WriteHttpError(context.Response, HttpStatusCode.BadRequest, "set-fan requires numeric cpuRpm and gpuRpm");
                        return;
                    }

                    state.SetFanTargets(cpuRpm, gpuRpm);
                    WriteHttpJson(context.Response, HttpStatusCode.OK, state.CreateStatusPayload());
                    return;
                }

                if (string.Equals(request.HttpMethod, "POST", StringComparison.OrdinalIgnoreCase)
                    && string.Equals(path, "/mode", StringComparison.OrdinalIgnoreCase))
                {
                    var body = ReadJsonBody(request);
                    var mode = ReadString(body, "mode");
                    if (!IsAllowedValue(mode, "balanced", "performance", "quiet", "custom"))
                    {
                        WriteHttpError(context.Response, HttpStatusCode.BadRequest, "mode must be balanced, performance, quiet, or custom");
                        return;
                    }

                    state.SetMode(mode);
                    WriteHttpJson(context.Response, HttpStatusCode.OK, state.CreateStatusPayload());
                    return;
                }

                if (string.Equals(request.HttpMethod, "POST", StringComparison.OrdinalIgnoreCase)
                    && string.Equals(path, "/power", StringComparison.OrdinalIgnoreCase))
                {
                    var body = ReadJsonBody(request);
                    var powerMode = ReadString(body, "powerMode");
                    int powerLimitWatts;
                    if (!IsAllowedValue(powerMode, "quiet", "balanced", "performance")
                        || !TryReadInt(body, "powerLimitWatts", out powerLimitWatts)
                        || powerLimitWatts <= 0)
                    {
                        WriteHttpError(context.Response, HttpStatusCode.BadRequest, "power requires powerMode quiet, balanced, or performance and positive numeric powerLimitWatts");
                        return;
                    }

                    state.SetPower(powerMode, powerLimitWatts);
                    WriteHttpJson(context.Response, HttpStatusCode.OK, state.CreateStatusPayload());
                    return;
                }

                if (string.Equals(request.HttpMethod, "POST", StringComparison.OrdinalIgnoreCase)
                    && string.Equals(path, "/gpu-mode", StringComparison.OrdinalIgnoreCase))
                {
                    var body = ReadJsonBody(request);
                    var gpuMode = ReadString(body, "gpuMode");
                    if (!IsAllowedValue(gpuMode, "hybrid", "discrete"))
                    {
                        WriteHttpError(context.Response, HttpStatusCode.BadRequest, "gpuMode must be hybrid or discrete");
                        return;
                    }
                    state.SetGpuMode(gpuMode);
                    WriteHttpJson(context.Response, HttpStatusCode.OK, state.CreateStatusPayload());
                    return;
                }

                if (string.Equals(request.HttpMethod, "POST", StringComparison.OrdinalIgnoreCase)
                    && string.Equals(path, "/dynamic-boost", StringComparison.OrdinalIgnoreCase))
                {
                    var body = ReadJsonBody(request);
                    bool enabled;
                    if (!TryReadBool(body, "enabled", out enabled))
                    {
                        WriteHttpError(context.Response, HttpStatusCode.BadRequest, "dynamic-boost requires boolean enabled");
                        return;
                    }
                    state.SetDynamicBoost(enabled);
                    WriteHttpJson(context.Response, HttpStatusCode.OK, state.CreateStatusPayload());
                    return;
                }

                if (string.Equals(request.HttpMethod, "POST", StringComparison.OrdinalIgnoreCase)
                    && string.Equals(path, "/display-overdrive", StringComparison.OrdinalIgnoreCase))
                {
                    var body = ReadJsonBody(request);
                    bool enabled;
                    if (!TryReadBool(body, "enabled", out enabled))
                    {
                        WriteHttpError(context.Response, HttpStatusCode.BadRequest, "display-overdrive requires boolean enabled");
                        return;
                    }
                    state.SetDisplayOverdrive(enabled);
                    WriteHttpJson(context.Response, HttpStatusCode.OK, state.CreateStatusPayload());
                    return;
                }

                if (string.Equals(request.HttpMethod, "POST", StringComparison.OrdinalIgnoreCase)
                    && string.Equals(path, "/battery-limit", StringComparison.OrdinalIgnoreCase))
                {
                    var body = ReadJsonBody(request);
                    int percent;
                    if (!TryReadInt(body, "percent", out percent) || (percent != 80 && percent != 100))
                    {
                        WriteHttpError(context.Response, HttpStatusCode.BadRequest, "battery-limit requires percent 80 or 100");
                        return;
                    }
                    state.SetBatteryLimit(percent);
                    WriteHttpJson(context.Response, HttpStatusCode.OK, state.CreateStatusPayload());
                    return;
                }

                WriteHttpError(context.Response, HttpStatusCode.NotFound, "unknown endpoint");
            }
            catch (JsonException ex)
            {
                WriteHttpError(context.Response, HttpStatusCode.BadRequest, "invalid JSON body: " + ex.Message);
            }
            catch (Exception ex)
            {
                WriteHttpError(context.Response, HttpStatusCode.InternalServerError, "mock HTTP request failed: " + ex.Message);
            }
        }

        private static JObject ReadJsonBody(HttpListenerRequest request)
        {
            using (var reader = new StreamReader(request.InputStream, request.ContentEncoding ?? Encoding.UTF8))
            {
                return JObject.Parse(reader.ReadToEnd());
            }
        }

        private static string ReadString(JObject body, string propertyName)
        {
            var token = body[propertyName];
            return token == null || token.Type != JTokenType.String ? null : token.Value<string>();
        }

        private static bool IsAllowedValue(string value, params string[] allowed)
        {
            if (value == null)
            {
                return false;
            }

            foreach (var item in allowed)
            {
                if (string.Equals(value, item, StringComparison.OrdinalIgnoreCase))
                {
                    return true;
                }
            }

            return false;
        }

        private static void RunMockDaemon()
        {
            var writer = new MockFanLevelWriter(MockMinRpm, MockMaxCpuRpm, MockMaxGpuRpm);
            WriteStatus(writer.LastTarget);

            string line;
            while ((line = Console.In.ReadLine()) != null)
            {
                HandleMockCommand(line, writer);
            }
        }

        private static void HandleMockCommand(string line, MockFanLevelWriter writer)
        {
            JObject command;
            try
            {
                command = JObject.Parse(line);
            }
            catch (JsonException ex)
            {
                WriteError("invalid JSON command: " + ex.Message);
                return;
            }

            var type = (string)command["type"];
            if (!string.Equals(type, "set-fan", StringComparison.OrdinalIgnoreCase))
            {
                WriteError("unknown command: " + (type ?? "<missing>"));
                return;
            }

            int cpuRpm;
            int gpuRpm;
            if (!TryReadInt(command, "cpuRpm", out cpuRpm) || !TryReadInt(command, "gpuRpm", out gpuRpm))
            {
                WriteError("set-fan requires numeric cpuRpm and gpuRpm");
                return;
            }

            writer.SetTargets(cpuRpm, gpuRpm);
            WriteStatus(writer.LastTarget);
        }

        private static bool TryReadInt(JObject command, string propertyName, out int value)
        {
            value = 0;
            var token = command[propertyName];
            if (token == null) return false;
            if (token.Type != JTokenType.Integer && token.Type != JTokenType.Float) return false;
            try { value = Convert.ToInt32(token.Value<double>()); return true; }
            catch { return false; }
        }

        private static bool TryReadBool(JObject command, string propertyName, out bool value)
        {
            value = false;
            var token = command[propertyName];
            if (token == null) return false;
            if (token.Type == JTokenType.Boolean) { value = token.Value<bool>(); return true; }
            // also accept 0/1
            if (token.Type == JTokenType.Integer) { value = token.Value<int>() != 0; return true; }
            return false;
        }

        private static void WriteStatus(MockFanLevelSnapshot state)
        {
            Console.WriteLine(CreateStatusPayload(state, "mock", null, null).ToString(Formatting.None));
            Console.Out.Flush();
        }

        private static JObject CreateStatusPayload(MockFanLevelSnapshot state, string mode, string powerMode, int? powerLimitWatts)
        {
            var status = new JObject
            {
                ["supported"] = true,
                ["mock"] = true,
                ["readOnly"] = false,
                ["swFanControl"] = true,
                ["fanCount"] = 2,
                ["thermalPolicyVersion"] = 1,
                ["adapterWatts"] = 230,
                ["cpuRpm"] = state.Cpu.EstimatedRpm,
                ["gpuRpm"] = state.Gpu.EstimatedRpm,
                ["cpuLevel"] = state.Cpu.Level,
                ["gpuLevel"] = state.Gpu.Level,
                ["rpmEstimated"] = true,
                ["cpuTemp"] = 0,
                ["gpuTemp"] = 0,
                ["maxCpuRpm"] = MockMaxCpuRpm,
                ["maxGpuRpm"] = MockMaxGpuRpm,
                ["maxLevel"] = Math.Max(FanLevelConverter.ToUnboundedLevel(MockMaxCpuRpm), FanLevelConverter.ToUnboundedLevel(MockMaxGpuRpm)),
                ["fanTable"] = new JObject
                {
                    ["fanCount"] = 2,
                    ["levelCount"] = FanLevelBounds.AbsoluteMaxLevel,
                    ["fan1MinLevel"] = FanLevelConverter.ToUnboundedLevel(MockMinRpm),
                    ["fan1MaxLevel"] = FanLevelConverter.ToUnboundedLevel(MockMaxCpuRpm),
                    ["fan2MinLevel"] = FanLevelConverter.ToUnboundedLevel(MockMinRpm),
                    ["fan2MaxLevel"] = FanLevelConverter.ToUnboundedLevel(MockMaxGpuRpm),
                    ["minTemperature"] = 0,
                    ["maxTemperature"] = 100
                },
                ["mode"] = mode,
                ["lastUpdated"] = DateTimeOffset.UtcNow.ToUnixTimeMilliseconds()
            };

            if (powerMode != null)
            {
                status["powerMode"] = powerMode;
            }

            if (powerLimitWatts.HasValue)
            {
                status["powerLimitWatts"] = powerLimitWatts.Value;
            }

            return new JObject
            {
                ["type"] = "status",
                ["status"] = status
            };
        }

        private static void WriteError(string message)
        {
            var payload = new JObject
            {
                ["type"] = "error",
                ["message"] = message
            };

            Console.WriteLine(payload.ToString(Formatting.None));
            Console.Out.Flush();
        }

        private static void WriteConsoleJson(JObject payload)
        {
            Console.WriteLine(payload.ToString(Formatting.None));
            Console.Out.Flush();
        }

        private static void WriteHttpJson(HttpListenerResponse response, HttpStatusCode statusCode, JObject payload)
        {
            var bytes = Encoding.UTF8.GetBytes(payload.ToString(Formatting.None));
            response.StatusCode = (int)statusCode;
            response.ContentType = "application/json; charset=utf-8";
            response.ContentEncoding = Encoding.UTF8;
            WriteCorsHeaders(response);
            response.ContentLength64 = bytes.Length;
            response.OutputStream.Write(bytes, 0, bytes.Length);
            response.OutputStream.Close();
        }

        private static void WriteHttpError(HttpListenerResponse response, HttpStatusCode statusCode, string message)
        {
            WriteHttpJson(response, statusCode, new JObject
            {
                ["type"] = "error",
                ["message"] = message
            });
        }

        private static void WriteHttpNoContent(HttpListenerResponse response)
        {
            response.StatusCode = (int)HttpStatusCode.NoContent;
            WriteCorsHeaders(response);
            response.ContentLength64 = 0;
            response.OutputStream.Close();
        }

        private static void WriteCorsHeaders(HttpListenerResponse response)
        {
            response.Headers["Access-Control-Allow-Origin"] = "*";
            response.Headers["Access-Control-Allow-Methods"] = "GET, POST, OPTIONS";
            response.Headers["Access-Control-Allow-Headers"] = "Content-Type";
            response.Headers["Access-Control-Max-Age"] = "600";
        }

        private static void PrintUsage()
        {
            Console.Error.WriteLine("Usage: omen-fan-driver.exe --detect-only [--mock]");
            Console.Error.WriteLine("       omen-fan-driver.exe --daemon --mock");
            Console.Error.WriteLine("       omen-fan-driver.exe --mock-http [--port 8787]");
            Console.Error.WriteLine("       omen-fan-driver.exe --dry-run-write --write-fan-level <cpuLevel> <gpuLevel>");
            Console.Error.WriteLine("       omen-fan-driver.exe --dry-run-write --write-mode <default|balanced|performance|cool|quiet|0xNN>");
            Console.Error.WriteLine("       omen-fan-driver.exe --write-fan-level <cpuLevel> <gpuLevel> --hardware-write");
            Console.Error.WriteLine("       omen-fan-driver.exe --write-mode <default|balanced|performance|cool|quiet|0xNN> --hardware-write");
            Console.Error.WriteLine("       omen-fan-driver.exe --self-test-parsers");
        }

        private sealed class Options
        {
            public bool DetectOnly { get; set; }
            public bool Daemon { get; set; }
            public bool Mock { get; set; }
            public bool MockHttp { get; set; }
            public bool SelfTestParsers { get; set; }
            public bool DryRunWrite { get; set; }
            public bool HardwareWrite { get; set; }
            public bool WriteRequested { get; set; }
            public bool WriteFanLevel { get; set; }
            public bool WriteMode { get; set; }
            public int CpuLevel { get; set; }
            public int GpuLevel { get; set; }
            public string Mode { get; set; }
            public bool PortSpecified { get; set; }
            public int Port { get; set; } = DefaultMockHttpPort;
        }

        private sealed class MockHttpState
        {
            private readonly object gate = new object();
            private readonly MockFanLevelWriter writer;
            private string mode = "balanced";
            private string powerMode = "balanced";
            private int powerLimitWatts = 45;
            private string gpuMode = "hybrid";
            private bool dynamicBoostUnlocked = false;
            private bool displayOverdriveEnabled = false;
            private int batteryChargeLimit = 100;
            // pending GPU mode change (set but not yet rebooted)
            private string pendingGpuMode = null;

            public MockHttpState(MockFanLevelWriter writer)
            {
                this.writer = writer;
            }

            public void SetFanTargets(int cpuRpm, int gpuRpm)
            {
                lock (gate) { writer.SetTargets(cpuRpm, gpuRpm); }
            }

            public void SetMode(string value)
            {
                lock (gate) { mode = value.ToLowerInvariant(); }
            }

            public void SetPower(string value, int watts)
            {
                lock (gate)
                {
                    powerMode = value.ToLowerInvariant();
                    powerLimitWatts = watts;
                }
            }

            public void SetGpuMode(string value)
            {
                lock (gate)
                {
                    // GPU mode change requires reboot — store as pending
                    pendingGpuMode = value.ToLowerInvariant();
                }
            }

            public void SetDynamicBoost(bool enabled)
            {
                lock (gate) { dynamicBoostUnlocked = enabled; }
            }

            public void SetDisplayOverdrive(bool enabled)
            {
                lock (gate) { displayOverdriveEnabled = enabled; }
            }

            public void SetBatteryLimit(int percent)
            {
                lock (gate) { batteryChargeLimit = percent; }
            }

            public JObject CreateStatusPayload()
            {
                lock (gate)
                {
                    var payload = Program.CreateStatusPayload(writer.LastTarget, mode, powerMode, powerLimitWatts);
                    var status = payload["status"] as JObject;
                    if (status != null)
                    {
                        status["gpuMode"] = gpuMode;
                        status["pendingGpuMode"] = pendingGpuMode != null ? (JToken)pendingGpuMode : JValue.CreateNull();
                        status["dynamicBoostUnlocked"] = dynamicBoostUnlocked;
                        status["displayOverdriveEnabled"] = displayOverdriveEnabled;
                        status["batteryChargeLimit"] = batteryChargeLimit;
                        status["capabilities"] = new JObject
                        {
                            ["fanControl"] = true,
                            ["thermalPolicy"] = true,
                            ["displayOverdrive"] = true,
                            ["gpuMux"] = true,
                            ["smartAdapter"] = true,
                            ["batteryLimit"] = true,
                            ["unleashed"] = true,
                            ["detected"] = true
                        };
                    }
                    return payload;
                }
            }
        }
    }
}
