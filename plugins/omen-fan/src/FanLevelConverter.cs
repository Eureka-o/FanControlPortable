using System;

namespace OmenFanDriver
{
    internal struct FanLevelBounds
    {
        public const int SafeDefaultMinLevel = 8;
        public const int SafeDefaultMaxLevel = 52;
        public const int AbsoluteMinLevel = 1;
        public const int AbsoluteMaxLevel = 100;

        public FanLevelBounds(int minLevel, int maxLevel)
        {
            if (!TryNormalize(minLevel, maxLevel, out minLevel, out maxLevel))
            {
                minLevel = SafeDefaultMinLevel;
                maxLevel = SafeDefaultMaxLevel;
            }

            MinLevel = minLevel;
            MaxLevel = maxLevel;
        }

        public int MinLevel { get; private set; }
        public int MaxLevel { get; private set; }

        public static FanLevelBounds Default
        {
            get { return new FanLevelBounds(SafeDefaultMinLevel, SafeDefaultMaxLevel); }
        }

        public static FanLevelBounds FromLevels(int minLevel, int maxLevel)
        {
            return new FanLevelBounds(minLevel, maxLevel);
        }

        public static FanLevelBounds FromRpmLimits(int minRpm, int maxRpm)
        {
            if (minRpm <= 0 || maxRpm <= 0 || minRpm > maxRpm)
            {
                return Default;
            }

            return new FanLevelBounds(
                FanLevelConverter.ToUnboundedLevel(minRpm),
                FanLevelConverter.ToUnboundedLevel(maxRpm));
        }

        public FanLevelBounds Normalize()
        {
            return new FanLevelBounds(MinLevel, MaxLevel);
        }

        internal int Clamp(int level)
        {
            int minLevel;
            int maxLevel;
            if (!TryNormalize(MinLevel, MaxLevel, out minLevel, out maxLevel))
            {
                minLevel = SafeDefaultMinLevel;
                maxLevel = SafeDefaultMaxLevel;
            }

            if (level < minLevel)
            {
                return minLevel;
            }

            if (level > maxLevel)
            {
                return maxLevel;
            }

            return level;
        }

        private static bool TryNormalize(int minLevel, int maxLevel, out int normalizedMin, out int normalizedMax)
        {
            normalizedMin = 0;
            normalizedMax = 0;

            if (minLevel <= 0 || maxLevel <= 0 || minLevel > maxLevel)
            {
                return false;
            }

            if (minLevel < AbsoluteMinLevel || maxLevel > AbsoluteMaxLevel)
            {
                return false;
            }

            normalizedMin = minLevel;
            normalizedMax = maxLevel;

            return normalizedMin <= normalizedMax;
        }
    }

    internal struct FanLevelTarget
    {
        public FanLevelTarget(int requestedRpm, int level)
        {
            RequestedRpm = requestedRpm;
            Level = level;
            EstimatedRpm = FanLevelConverter.ToEstimatedRpm(level);
        }

        public int RequestedRpm { get; private set; }
        public int Level { get; private set; }
        public int EstimatedRpm { get; private set; }
    }

    internal static class FanLevelConverter
    {
        public static FanLevelTarget ToTarget(int rpm, FanLevelBounds bounds)
        {
            int level = ToLevel(rpm, bounds);
            return new FanLevelTarget(rpm, level);
        }

        public static int ToLevel(int rpm, FanLevelBounds bounds)
        {
            return bounds.Clamp(ToUnboundedLevel(rpm));
        }

        public static int ToLevel(int rpm)
        {
            return ToLevel(rpm, FanLevelBounds.Default);
        }

        public static int ToEstimatedRpm(int level)
        {
            return level * 100;
        }

        internal static int ToUnboundedLevel(int rpm)
        {
            return (int)Math.Round(rpm / 100.0, MidpointRounding.AwayFromZero);
        }
    }
}
