namespace OmenFanDriver
{
    internal sealed class MockFanLevelWriter
    {
        private readonly FanLevelBounds cpuBounds;
        private readonly FanLevelBounds gpuBounds;

        public MockFanLevelWriter(int minRpm, int maxCpuRpm, int maxGpuRpm)
        {
            cpuBounds = FanLevelBounds.FromRpmLimits(minRpm, maxCpuRpm);
            gpuBounds = FanLevelBounds.FromRpmLimits(minRpm, maxGpuRpm);
            SetTargets(1800, 1700);
        }

        public MockFanLevelSnapshot LastTarget { get; private set; }

        public void SetTargets(int cpuRpm, int gpuRpm)
        {
            LastTarget = new MockFanLevelSnapshot(
                FanLevelConverter.ToTarget(cpuRpm, cpuBounds),
                FanLevelConverter.ToTarget(gpuRpm, gpuBounds));
        }
    }

    internal struct MockFanLevelSnapshot
    {
        public MockFanLevelSnapshot(FanLevelTarget cpu, FanLevelTarget gpu)
        {
            Cpu = cpu;
            Gpu = gpu;
        }

        public FanLevelTarget Cpu { get; private set; }
        public FanLevelTarget Gpu { get; private set; }
    }
}
