using System;
using System.Linq;

namespace FanControl.TempBridge
{
    internal sealed class GpuReadCoordinator
    {
        private const int ProbeIntervalSeconds = 10;
        private const int ActiveSamplesToWake = 1;
        private const int IdleSamplesToSleep = 3;

        private DateTime lastProbeUtc = DateTime.MinValue;
        private int activeSamples;
        private int idleSamples;
        private bool gpuPollingActive;
        private bool lastHasDiscreteGpu;
        private string lastDetail = string.Empty;

        public bool ShouldPollGpu(HardwareProfile profile, Action<string> log)
        {
            bool integratedOnlyProfile = IsKnownIntegratedOnlyProfile(profile);
            if (integratedOnlyProfile)
            {
                lastHasDiscreteGpu = false;
                gpuPollingActive = true;
                return true;
            }

            lastHasDiscreteGpu = true;
            if ((DateTime.UtcNow - lastProbeUtc).TotalSeconds >= ProbeIntervalSeconds)
            {
                ProbeGpuActivity(log, true);
            }

            return gpuPollingActive;
        }

        public bool LastHasDiscreteGpu
        {
            get { return lastHasDiscreteGpu; }
        }

        public string LastDetail
        {
            get { return lastDetail; }
        }

        private void ProbeGpuActivity(Action<string> log, bool expectDiscreteGpu)
        {
            lastProbeUtc = DateTime.UtcNow;

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

            lastHasDiscreteGpu = status.HasDiscreteGpu || expectDiscreteGpu;
            lastDetail = status.Detail ?? string.Empty;

            if (!status.HasDiscreteGpu)
            {
                if (expectDiscreteGpu)
                {
                    RecordIdleSample();
                }
                else
                {
                    gpuPollingActive = true;
                    activeSamples = 0;
                    idleSamples = 0;
                }
                return;
            }

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

            if (wasActive != gpuPollingActive && log != null)
            {
                log("GPU auto read " + (gpuPollingActive ? "enabled" : "paused") + ": " + lastDetail);
            }
        }

        private void RecordIdleSample()
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
