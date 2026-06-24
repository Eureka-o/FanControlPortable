using System;
using System.Diagnostics;
using System.Linq;
using System.Threading;

namespace FanControl.TempBridge
{
    internal sealed class GpuReadCoordinator
    {
        private const int ProbeIntervalSeconds = 10;
        private const int ActiveSamplesToWake = 1;
        private const int IdleSamplesToSleep = 3;
        private const long SlowProbeLogThresholdMs = 200;

        private readonly object stateLock = new object();
        private DateTime lastProbeUtc = DateTime.MinValue;
        private int activeSamples;
        private int idleSamples;
        private bool gpuPollingActive;
        private bool lastHasDiscreteGpu;
        private string lastDetail = string.Empty;
        private bool probeRunning;
        private int probeGeneration;

        public bool ShouldPollGpu(HardwareProfile profile, Action<string> log)
        {
            bool integratedOnlyProfile = IsKnownIntegratedOnlyProfile(profile);
            if (integratedOnlyProfile)
            {
                lock (stateLock)
                {
                    probeGeneration++;
                    probeRunning = false;
                    lastHasDiscreteGpu = false;
                    lastDetail = "integrated GPU profile";
                    gpuPollingActive = true;
                    activeSamples = 0;
                    idleSamples = 0;
                }
                return true;
            }

            bool shouldQueueProbe = false;
            bool cachedGpuPollingActive;
            int generation = 0;
            DateTime now = DateTime.UtcNow;
            lock (stateLock)
            {
                lastHasDiscreteGpu = true;
                if (!probeRunning && (now - lastProbeUtc).TotalSeconds >= ProbeIntervalSeconds)
                {
                    lastProbeUtc = now;
                    probeRunning = true;
                    generation = probeGeneration;
                    shouldQueueProbe = true;
                }
                cachedGpuPollingActive = gpuPollingActive;
            }

            if (shouldQueueProbe)
            {
                ThreadPool.QueueUserWorkItem(_ => ProbeGpuActivity(log, true, generation));
            }

            return cachedGpuPollingActive;
        }

        public bool LastHasDiscreteGpu
        {
            get
            {
                lock (stateLock)
                {
                    return lastHasDiscreteGpu;
                }
            }
        }

        public string LastDetail
        {
            get
            {
                lock (stateLock)
                {
                    return lastDetail;
                }
            }
        }

        private void ProbeGpuActivity(Action<string> log, bool expectDiscreteGpu, int generation)
        {
            var stopwatch = Stopwatch.StartNew();
            GpuActivityStatus status;
            try
            {
                status = GpuActivityDetector.Detect();
            }
            catch (Exception ex)
            {
                status = new GpuActivityStatus
                {
                    HasDiscreteGpu = true,
                    IsActive = false,
                    Detail = "GPU activity probe failed: " + ex.Message
                };
            }

            bool changed = false;
            bool pollingActive = false;
            string detail = status.Detail ?? string.Empty;
            lock (stateLock)
            {
                if (generation != probeGeneration)
                {
                    return;
                }

                probeRunning = false;
                lastHasDiscreteGpu = status.HasDiscreteGpu || expectDiscreteGpu;
                lastDetail = detail;

                if (!status.HasDiscreteGpu)
                {
                    bool wasActiveWithoutDiscrete = gpuPollingActive;
                    if (expectDiscreteGpu)
                    {
                        RecordIdleSampleLocked();
                    }
                    else
                    {
                        gpuPollingActive = true;
                        activeSamples = 0;
                        idleSamples = 0;
                    }
                    changed = wasActiveWithoutDiscrete != gpuPollingActive;
                    pollingActive = gpuPollingActive;
                }
                else
                {
                    bool wasActive = gpuPollingActive;
                    if (status.IsActive)
                    {
                        activeSamples++;
                        idleSamples = 0;
                        if (activeSamples >= ActiveSamplesToWake)
                        {
                            gpuPollingActive = true;
                        }
                    }
                    else
                    {
                        idleSamples++;
                        activeSamples = 0;
                        if (idleSamples >= IdleSamplesToSleep)
                        {
                            gpuPollingActive = false;
                        }
                    }

                    changed = wasActive != gpuPollingActive;
                    pollingActive = gpuPollingActive;
                }
            }

            if (changed && log != null)
            {
                log("GPU auto read " + (pollingActive ? "enabled" : "paused") + ": " + detail);
            }

            if (stopwatch.ElapsedMilliseconds >= SlowProbeLogThresholdMs && log != null)
            {
                log("GPU activity probe completed in " + stopwatch.ElapsedMilliseconds + " ms: " + detail);
            }
        }

        private void RecordIdleSampleLocked()
        {
            idleSamples++;
            activeSamples = 0;
            if (idleSamples >= IdleSamplesToSleep)
            {
                gpuPollingActive = false;
            }
        }

        private static bool IsKnownIntegratedOnlyProfile(HardwareProfile profile)
        {
            return profile != null &&
                profile.GpuDevices != null &&
                profile.GpuDevices.Length > 0 &&
                !profile.GpuDevices.Any(device => device.Discrete);
        }
    }
}
