using System.Diagnostics;
using System.Reflection;

namespace FanControlPortable;

internal static class PlatformCompat
{
    public static int Clamp(int value, int min, int max)
    {
        if (value < min) return min;
        return value > max ? max : value;
    }

    public static double Clamp(double value, double min, double max)
    {
        if (value < min) return min;
        return value > max ? max : value;
    }

    public static string CurrentProcessPath()
    {
#if NET5_0_OR_GREATER
        return Environment.ProcessPath
            ?? Process.GetCurrentProcess().MainModule?.FileName
            ?? AppContext.BaseDirectory;
#else
        return Process.GetCurrentProcess().MainModule?.FileName
            ?? Assembly.GetEntryAssembly()?.Location
            ?? "";
#endif
    }
}
