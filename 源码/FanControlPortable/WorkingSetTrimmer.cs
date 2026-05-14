using System.Diagnostics;
using System.Runtime;
using System.Runtime.InteropServices;

namespace FanControlPortable;

internal static class WorkingSetTrimmer
{
    [DllImport("psapi.dll", SetLastError = true)]
    private static extern bool EmptyWorkingSet(IntPtr processHandle);

    [DllImport("kernel32.dll")]
    private static extern bool SetProcessWorkingSetSize(IntPtr processHandle, IntPtr minimumWorkingSetSize, IntPtr maximumWorkingSetSize);

    public static void TrimCurrentProcess()
    {
        try
        {
            GCSettings.LargeObjectHeapCompactionMode = GCLargeObjectHeapCompactionMode.CompactOnce;
            GC.Collect(GC.MaxGeneration, GCCollectionMode.Forced, blocking: true, compacting: true);
            GC.WaitForPendingFinalizers();
            GC.Collect(GC.MaxGeneration, GCCollectionMode.Forced, blocking: true, compacting: true);

            using var process = Process.GetCurrentProcess();
            EmptyWorkingSet(process.Handle);
            SetProcessWorkingSetSize(process.Handle, (IntPtr)(-1), (IntPtr)(-1));
        }
        catch
        {
            // Memory trimming is an optional background optimization.
        }
    }
}
